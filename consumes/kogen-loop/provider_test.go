package consumes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/KogenAI/kogen-loop/provider"
	"github.com/KogenAI/kogen-loop/sys"
)

// attempts is a provider.Observer that keeps the attempt records, as kogen-build's run log does.
type attempts struct {
	mu   sync.Mutex
	recs []map[string]any
}

func (a *attempts) Attempt(record map[string]any) {
	a.mu.Lock()
	a.recs = append(a.recs, record)
	a.mu.Unlock()
}

func (a *attempts) records() []map[string]any {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]map[string]any(nil), a.recs...)
}

// chatgptWorld is a session with a seeded ChatGPT login, a client whose Sleep takes no time and a
// loopback provider the session's KOGEN_PROVIDER_URL and KOGEN_AUTH_URL point at.
func chatgptWorld(t *testing.T, script ...http.HandlerFunc) (*provider.Client, *sys.Session, *fakeProvider, string) {
	t.Helper()
	f := newFakeProvider(t, script...)
	s, _, home := newSession(t, f.env()...)
	seedChatGPT(t, home)
	c := provider.NewClient(fixtureVersion)
	c.Sleep = noWait
	return c, s, f, home
}

func chatgptRequest(id string) provider.Request {
	return provider.Request{Role: chatgptRole, Instructions: "policy", CacheID: id, Input: []any{user("hello")}}
}

// failure answers with a provider error of this status and code.
func failure(status int, code, param, message string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		b, _ := json.Marshal(map[string]any{"error": map[string]any{"code": code, "param": param, "message": message, "type": "invalid_request_error"}})
		w.Write(b)
	}
}

// Every conversation has one cache identity of at most provider.MaxCacheID characters, created
// when it starts (build/context.go cacheID) and checked on a resume (build/state.go).
func TestProviderNewCacheIDAndMaxCacheID(t *testing.T) {
	var _ func() string = provider.NewCacheID
	var _ int = provider.MaxCacheID

	re := regexp.MustCompile(`^conversation-[0-9a-f]{16}$`)
	a, b := provider.NewCacheID(), provider.NewCacheID()
	if !re.MatchString(a) || !re.MatchString(b) || a == b {
		t.Errorf("provider.NewCacheID: %q and %q", a, b)
	}
	if provider.MaxCacheID != 64 {
		t.Errorf("provider.MaxCacheID = %d", provider.MaxCacheID)
	}
	if len(a) > provider.MaxCacheID {
		t.Errorf("provider.NewCacheID: %d characters, more than provider.MaxCacheID", len(a))
	}
	// An identity of exactly MaxCacheID characters goes on the wire as is, a longer one as a
	// stable digest of MaxCacheID characters.
	c := provider.NewClient(fixtureVersion)
	exact := strings.Repeat("c", provider.MaxCacheID)
	if got := c.Body(chatgptRequest(exact))["prompt_cache_key"]; got != exact {
		t.Errorf("provider.MaxCacheID: an identity of %d characters became %v", provider.MaxCacheID, got)
	}
	long := c.Body(chatgptRequest(strings.Repeat("c", 100)))["prompt_cache_key"].(string)
	if len(long) != provider.MaxCacheID || long != c.Body(chatgptRequest(strings.Repeat("c", 100)))["prompt_cache_key"] {
		t.Errorf("provider.MaxCacheID: an identity of 100 characters became %q", long)
	}
}

// A Build checkpoint stores the conversations' cache states and checks them on a resume
// (build/state.go, build/legacy.go): the checkpoint form of one, its validity and the check of a
// whole set.
func TestProviderCacheStateCheckpointForm(t *testing.T) {
	st := provider.CacheState{ID: "conversation-0123456789abcdef", Unsupported: true, Response: "resp-1"}
	var _ string = st.ID
	var _ bool = st.Unsupported
	var _ string = st.Response
	var _ func(provider.CacheState) bool = provider.CacheState.Valid
	var _ func(map[string]provider.CacheState) bool = provider.ValidCacheStates

	if got := jsonText(t, st); got != `{"id":"conversation-0123456789abcdef","unsupported":true,"response":"resp-1"}` {
		t.Errorf("provider.CacheState: checkpoint form %s", got)
	}
	if got := jsonText(t, provider.CacheState{ID: "x"}); got != `{"id":"x","unsupported":false,"response":null}` {
		t.Errorf("provider.CacheState: checkpoint form without a response %s", got)
	}
	var back provider.CacheState
	if err := json.Unmarshal([]byte(`{"id":"x","unsupported":true,"response":"r"}`), &back); err != nil || back.ID != "x" || !back.Unsupported || back.Response != "r" {
		t.Errorf("provider.CacheState: read back %+v, %v", back, err)
	}
	var states map[string]provider.CacheState
	if err := json.Unmarshal([]byte(`{"build":{"id":"conversation-0123456789abcdef","unsupported":false,"response":null}}`), &states); err != nil || states["build"].ID != "conversation-0123456789abcdef" || states["build"].Unsupported || states["build"].Response != "" {
		t.Errorf("provider.CacheState: a checkpoint's states read back %+v, %v", states, err)
	}

	if !st.Valid() || (provider.CacheState{}).Valid() {
		t.Errorf("provider.CacheState.Valid: a state with an identity is valid, an empty one is not")
	}
	if !(provider.CacheState{ID: strings.Repeat("i", 256)}).Valid() || (provider.CacheState{ID: strings.Repeat("i", 257)}).Valid() {
		t.Errorf("provider.CacheState.Valid: identities are 1..256 bytes")
	}
	if !(provider.CacheState{ID: "x", Response: strings.Repeat("r", 256)}).Valid() || (provider.CacheState{ID: "x", Response: strings.Repeat("r", 257)}).Valid() {
		t.Errorf("provider.CacheState.Valid: a response is at most 256 bytes")
	}
	if !provider.ValidCacheStates(map[string]provider.CacheState{"build": st, "review-1": {ID: "y"}}) || !provider.ValidCacheStates(nil) || !provider.ValidCacheStates(map[string]provider.CacheState{}) {
		t.Errorf("provider.ValidCacheStates refuses valid states")
	}
	if provider.ValidCacheStates(map[string]provider.CacheState{"": st}) {
		t.Errorf("provider.ValidCacheStates accepts a state without a key")
	}
	if provider.ValidCacheStates(map[string]provider.CacheState{"build": st, "review-1": {}}) {
		t.Errorf("provider.ValidCacheStates accepts a state without an identity")
	}
}

// build/build.go keeps one provider.Cache for the Build, stores Cache.States() in its checkpoint
// and restores it with provider.NewCache on a resume; a request names its conversation with
// CacheID and holds its state in Request.Cache. ChatGPT sends the identity as prompt_cache_key,
// except for a conversation whose state says the route refuses it.
func TestProviderCacheKeepsConversationStates(t *testing.T) {
	var _ func(map[string]provider.CacheState) *provider.Cache = provider.NewCache
	var _ func(*provider.Cache) map[string]provider.CacheState = (*provider.Cache).States

	fresh := provider.NewCache(nil)
	if got := fresh.States(); got == nil || len(got) != 0 {
		t.Fatalf("provider.NewCache(nil).States() = %v, want an empty map", got)
	}
	id := provider.NewCacheID()
	req := chatgptRequest(id)
	req.Cache = fresh
	c := provider.NewClient(fixtureVersion)
	if got := c.Body(req)["prompt_cache_key"]; got != id {
		t.Fatalf("provider.Request.CacheID: prompt_cache_key = %v, want %s", got, id)
	}
	states := fresh.States()
	if len(states) != 1 {
		t.Fatalf("provider.Cache.States after a request: %v, want the conversation's state", states)
	}
	var key string
	for k, st := range states {
		key = k
		if st.ID != id || st.Unsupported || !st.Valid() {
			t.Errorf("provider.Cache.States: state %+v, want the conversation's identity, supported", st)
		}
	}
	if !provider.ValidCacheStates(states) {
		t.Errorf("provider.Cache.States: the states are not valid checkpoint states")
	}

	// NewCache holds a copy, and States returns one.
	restored := provider.NewCache(states)
	states[key] = provider.CacheState{ID: "changed"}
	delete(states, "nothing")
	if got := restored.States()[key]; got.ID != id {
		t.Errorf("provider.NewCache did not copy its states: %+v", got)
	}
	out := restored.States()
	out[key] = provider.CacheState{ID: "changed"}
	if got := restored.States()[key]; got.ID != id {
		t.Errorf("provider.Cache.States did not return a copy: %+v", got)
	}
	req.Cache = restored
	if got := c.Body(req)["prompt_cache_key"]; got != id {
		t.Errorf("provider.NewCache: a restored cache gave prompt_cache_key %v, want %s", got, id)
	}

	// A state that records a refused prompt_cache_key keeps the key off the wire.
	marked := restored.States()
	st := marked[key]
	st.Unsupported = true
	marked[key] = st
	req.Cache = provider.NewCache(marked)
	if _, ok := c.Body(req)["prompt_cache_key"]; ok {
		t.Errorf("provider.CacheState.Unsupported: prompt_cache_key sent for a conversation whose key was refused")
	}
}

// A ChatGPT route that refuses prompt_cache_key (a 400 that names the parameter) is retried
// without it and remembered in the conversation's state, which the Build checkpoints.
func TestProviderRespondRemembersARefusedCacheKey(t *testing.T) {
	c, s, f, _ := chatgptWorld(t,
		failure(400, "unknown_parameter", "prompt_cache_key", "Unknown parameter: 'prompt_cache_key'."),
		sse(textItem("hello back")))
	id := provider.NewCacheID()
	cache := provider.NewCache(nil)
	req := chatgptRequest(id)
	req.Cache = cache
	resp, err := c.Respond(context.Background(), s, req, nil)
	if err != nil || len(resp.Items) != 1 {
		t.Fatalf("provider.Client.Respond = %+v, %v", resp, err)
	}
	paths, _ := f.seen()
	if len(paths) != 2 {
		t.Fatalf("provider.Client.Respond: the provider saw %v, want the refused request and its retry", paths)
	}
	if f.request(t, 0)["prompt_cache_key"] != id {
		t.Errorf("provider.Request.CacheID: the first request lacks prompt_cache_key")
	}
	if _, ok := f.request(t, 1)["prompt_cache_key"]; ok {
		t.Errorf("provider.CacheState.Unsupported: the retry still carries prompt_cache_key")
	}
	states := cache.States()
	if len(states) != 1 {
		t.Fatalf("provider.Cache.States = %v", states)
	}
	for _, st := range states {
		if st.ID != id || !st.Unsupported {
			t.Errorf("provider.CacheState after a refused key: %+v, want the identity and Unsupported", st)
		}
	}
}

// The body a request renders (build/context.go measures it, build/records.go sizes it): the model,
// the policy text, the conversation, the tools, a forced tool and the cache identity.
func TestProviderClientBodyOfARequest(t *testing.T) {
	var _ func(*provider.Client, provider.Request) map[string]any = (*provider.Client).Body
	req := provider.Request{
		Role: chatgptRole, Instructions: "policy", Input: []any{user("hello")}, CacheID: "conversation-0123456789abcdef",
		Tools:      []any{map[string]any{"type": "function", "name": "finish", "parameters": map[string]any{"type": "object"}}},
		ToolChoice: "finish", Cache: provider.NewCache(nil),
		Context: map[string]any{"phase": "build"},
	}
	var _ provider.Role = req.Role
	var _ string = req.Instructions
	var _ []any = req.Input
	var _ []any = req.Tools
	var _ string = req.ToolChoice
	var _ string = req.CacheID
	var _ *provider.Cache = req.Cache
	var _ map[string]any = req.Context

	c := provider.NewClient(fixtureVersion)
	body := c.Body(req)
	if body["model"] != "gpt-6.1-sol" || body["instructions"] != "policy" || body["prompt_cache_key"] != "conversation-0123456789abcdef" {
		t.Errorf("provider.Client.Body: model %v, instructions %v, prompt_cache_key %v", body["model"], body["instructions"], body["prompt_cache_key"])
	}
	if _, ok := body["prompt_cache_options"]; ok {
		t.Errorf("provider.Client.Body sends prompt_cache_options")
	}
	input, _ := body["input"].([]any)
	if len(input) != 1 || !reflect.DeepEqual(input[0], user("hello")) {
		t.Errorf("provider.Request.Input: body input %v", body["input"])
	}
	tools, _ := body["tools"].([]any)
	if len(tools) != 1 || tools[0].(map[string]any)["name"] != "finish" {
		t.Errorf("provider.Request.Tools: body tools %v", body["tools"])
	}
	if got := jsonText(t, body["tool_choice"]); got != `{"name":"finish","type":"function"}` {
		t.Errorf("provider.Request.ToolChoice: tool_choice %s", got)
	}
	req.ToolChoice = ""
	if _, forced := c.Body(req)["tool_choice"].(map[string]any); forced {
		t.Errorf("provider.Request.ToolChoice: a tool is forced without a ToolChoice")
	}
	if !provider.SupportsToolChoice(chatgptRole) {
		t.Errorf("provider.Request.ToolChoice: a ChatGPT role cannot force a tool")
	}
}

// A contract message is its stable authority and the dynamic evidence behind it, two input_text
// blocks (build/builder.go, material.go, executor_native.go).
func TestProviderContractBlocks(t *testing.T) {
	var _ func(string, string) []any = provider.ContractBlocks
	want := []any{map[string]any{"type": "input_text", "text": "stable"}, map[string]any{"type": "input_text", "text": "dynamic"}}
	if got := provider.ContractBlocks("stable", "dynamic"); !reflect.DeepEqual(got, want) {
		t.Errorf("provider.ContractBlocks = %v", got)
	}
	if got := provider.ContractBlocks("", ""); len(got) != 2 {
		t.Errorf("provider.ContractBlocks of empty texts = %v", got)
	}
}

// The run log records tool arguments and results as a digest (build/builder.go,
// executor_native.go): the SHA-256 of a string's bytes, of any other value's JSON with its keys
// sorted, and nothing for nil.
func TestProviderFingerprint(t *testing.T) {
	var _ func(any) *provider.Digest = provider.Fingerprint
	d := provider.Fingerprint("abc")
	if d == nil || jsonText(t, d) != `{"sha256":"ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad","bytes":3}` {
		t.Errorf("provider.Fingerprint(abc) = %s", jsonText(t, d))
	}
	if got := provider.Fingerprint(nil); got != nil {
		t.Errorf("provider.Fingerprint(nil) = %+v", got)
	}
	a := provider.Fingerprint(map[string]any{"b": 1, "a": 2})
	b := provider.Fingerprint(map[string]any{"a": 2, "b": 1})
	if a == nil || b == nil || *a != *b || jsonText(t, a) != jsonText(t, provider.Fingerprint(`{"a":2,"b":1}`)) {
		t.Errorf("provider.Fingerprint of an object: %s and %s", jsonText(t, a), jsonText(t, b))
	}
	parts := []any{map[string]any{"type": "input_text", "text": "meta"}}
	if got := provider.Fingerprint(parts); got == nil || jsonText(t, got) != jsonText(t, provider.Fingerprint(`[{"text":"meta","type":"input_text"}]`)) {
		t.Errorf("provider.Fingerprint of typed parts = %s", jsonText(t, got))
	}
}

// Before any request the Build checks that a role has a usable login (build/build.go,
// builder.go): a refusal is a provider/login error with exit 4, and only the providers the roles use
// are checked.
func TestProviderRequireLogin(t *testing.T) {
	var _ func(string, []provider.Role) error = provider.RequireLogin

	home := t.TempDir()
	err := provider.RequireLogin(home, []provider.Role{chatgptRole})
	e, ok := sys.AsError(err)
	if !ok || e.Class != "provider" || e.Reason != "login" || e.Exit != 4 {
		t.Fatalf("provider.RequireLogin without a login = %v", err)
	}
	seedChatGPT(t, home)
	if err := provider.RequireLogin(home, []provider.Role{chatgptRole, {Model: "gpt-6.1-sol"}}); err != nil {
		t.Errorf("provider.RequireLogin with a ChatGPT login: %v", err)
	}
	if err := provider.RequireLogin(home, nil); err != nil {
		t.Errorf("provider.RequireLogin of no roles: %v", err)
	}
	err = provider.RequireLogin(home, []provider.Role{chatgptRole, {Provider: "anthropic", Model: "claude-sonnet-5-5"}})
	if e, ok := sys.AsError(err); !ok || e.Class != "provider" || e.Reason != "login" || e.Exit != 4 {
		t.Errorf("provider.RequireLogin of a role that has no login = %v", err)
	}
}

// sseUsage answers like sse with this usage block, the response's model and no other field.
func sseUsage(model string, usage map[string]any, items ...map[string]any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("X-Request-Id", "req-fixture")
		for i, it := range items {
			frame(w, map[string]any{"type": "response.output_item.done", "output_index": i, "item": it})
		}
		frame(w, map[string]any{"type": "response.completed", "response": map[string]any{"id": "resp-1", "model": model, "usage": usage}})
	}
}

// Respond posts the request to the provider the session's loopback seam names and returns the
// validated answer kogen-build reads: the items, the effective model, the operation id and the
// analytical usage; each attempt is reported to the observer with the request's context merged.
func TestProviderRespondAnswersAndReportsAttempts(t *testing.T) {
	var _ func(*provider.Client, context.Context, *sys.Session, provider.Request, provider.Observer) (*provider.Response, error) = (*provider.Client).Respond
	var _ func(string) *provider.Client = provider.NewClient
	var _ interface{ Attempt(map[string]any) } = provider.Observer(nil)
	var _ provider.Observer = interface{ Attempt(map[string]any) }(nil)

	item := callItem("1", "read_file", map[string]any{"path": "a"})
	usage := map[string]any{
		"input_tokens": 11, "input_tokens_details": map[string]any{"cached_tokens": 3, "cache_write_tokens": 2},
		"output_tokens": 7, "output_tokens_details": map[string]any{"reasoning_tokens": 2},
	}
	c, s, f, _ := chatgptWorld(t, sseUsage("gpt-6.1-sol-2026", usage, textItem("hello back"), item), sse(textItem("no write")))
	rec := &attempts{}
	req := chatgptRequest("conversation-0123456789abcdef")
	req.Context = map[string]any{"phase": "build", "role": "build", "run_id": "run-1", "slug": "k-consumes", "lineage": nil, "pass": 2, "repair": 0}
	req.Tools = []any{map[string]any{"type": "function", "name": "read_file", "parameters": map[string]any{"type": "object"}}}
	resp, err := c.Respond(context.Background(), s, req, rec)
	if err != nil {
		t.Fatal(err)
	}
	paths, _ := f.seen()
	if !reflect.DeepEqual(paths, []string{"/v1/responses"}) {
		t.Fatalf("provider.Client.Respond: the provider saw %v", paths)
	}
	if got := f.heads[0].Get("Authorization"); got != "Bearer fake-access-token" {
		t.Errorf("provider.Client.Respond: Authorization %q: the saved login is not used", got)
	}
	sent := f.request(t, 0)
	if sent["model"] != "gpt-6.1-sol" || sent["instructions"] != "policy" || sent["prompt_cache_key"] != "conversation-0123456789abcdef" || len(sent["tools"].([]any)) != 1 {
		t.Errorf("provider.Client.Respond sent %v", sent)
	}
	var _ []map[string]any = resp.Items
	var _ string = resp.Model
	var _ string = resp.OperationID
	var _ provider.KnownUsage = resp.Known
	if len(resp.Items) != 2 || resp.Items[0]["type"] != "message" || resp.Items[1]["call_id"] != "call-1" || resp.Items[1]["name"] != "read_file" {
		t.Errorf("provider.Response.Items = %v", resp.Items)
	}
	if resp.Model != "gpt-6.1-sol-2026" {
		t.Errorf("provider.Response.Model = %q, want the model the provider reported", resp.Model)
	}
	if !strings.HasPrefix(resp.OperationID, "operation-") {
		t.Errorf("provider.Response.OperationID = %q", resp.OperationID)
	}
	k := resp.Known
	var _ *int = k.Write
	if k.Input == nil || *k.Input != 11 || k.Cached == nil || *k.Cached != 3 || k.Output == nil || *k.Output != 7 || k.Reasoning == nil || *k.Reasoning != 2 {
		t.Errorf("provider.Response.Known = %+v", k)
	}
	if k.Write == nil || *k.Write != 2 {
		t.Errorf("provider.KnownUsage.Write = %v, want the 2 cache-write tokens the provider reported", k.Write)
	}
	records := rec.records()
	if len(records) != 1 {
		t.Fatalf("provider.Client.Respond: the observer saw %d attempt records", len(records))
	}
	for k, want := range map[string]any{"phase": "build", "role": "build", "run_id": "run-1", "slug": "k-consumes", "pass": 2, "repair": 0, "conversation_id": "conversation-0123456789abcdef"} {
		if records[0][k] != want {
			t.Errorf("provider.Request.Context: attempt record %q = %v, want %v", k, records[0][k], want)
		}
	}
	if _, ok := records[0]["lineage"]; !ok {
		t.Errorf("provider.Request.Context: the attempt record has no lineage key: %v", records[0])
	}

	// A response that does not report a cache write has no write count.
	resp, err = c.Respond(context.Background(), s, chatgptRequest("conversation-0123456789abcdef"), nil)
	if err != nil || resp.Known.Write != nil || len(resp.Items) != 1 {
		t.Errorf("provider.KnownUsage.Write of a usage without a cache write: %+v, %v", resp, err)
	}
}

// A provider failure is a *sys.Error of class provider with exit 4, which build's stop turns into a
// context_exhausted stop when the conversation no longer fits; a request without a login sends nothing.
func TestProviderRespondFailures(t *testing.T) {
	c, s, f, _ := chatgptWorld(t, failure(400, "context_length_exceeded", "", "too long"), refusal)
	_, err := c.Respond(context.Background(), s, chatgptRequest("conversation-0123456789abcdef"), nil)
	e, ok := sys.AsError(err)
	if !ok || e.Class != "provider" || e.Reason != "context_too_long" || e.Exit != 4 || e.Detail != "the conversation no longer fits gpt-6.1-sol's context" {
		t.Fatalf("provider.Client.Respond of a context overflow = %v", err)
	}
	if fmt.Sprint(e.ProviderFailure["http_status"]) != "400" || e.ProviderFailure["code"] != "context_length_exceeded" {
		t.Errorf("sys.Error.ProviderFailure of a provider failure = %v", e.ProviderFailure)
	}
	_, err = c.Respond(context.Background(), s, chatgptRequest("conversation-0123456789abcdef"), nil)
	if e, ok := sys.AsError(err); !ok || e.Class != "provider" || e.Exit != 4 {
		t.Errorf("provider.Client.Respond of a refused request = %v", err)
	}
	if paths, _ := f.seen(); len(paths) != 2 {
		t.Errorf("provider.Client.Respond: a refusal is not retried: the provider saw %v", paths)
	}

	nobody := newFakeProvider(t, sse(textItem("never")))
	s2, _, _ := newSession(t, nobody.env()...)
	c2 := provider.NewClient(fixtureVersion)
	c2.Sleep = noWait
	if _, err := c2.Respond(context.Background(), s2, chatgptRequest("conversation-0123456789abcdef"), nil); err == nil {
		t.Errorf("provider.Client.Respond without a login succeeded")
	}
	if paths, _ := nobody.seen(); len(paths) != 0 {
		t.Errorf("provider.Client.Respond: a request without a login sent %v", paths)
	}
}

// Client.ResponseBudget lowers the budget of one Respond call (build/build.go passes it to the
// budget-expiry naming): zero keeps the package's, a value outside 1s..30m is refused before anything
// is sent, and a call that runs past its budget fails as provider/unavailable naming it.
func TestProviderClientResponseBudget(t *testing.T) {
	var _ time.Duration = provider.NewClient(fixtureVersion).ResponseBudget
	if got := provider.NewClient("kogen-build").ResponseBudget; got != 0 {
		t.Errorf("provider.NewClient: ResponseBudget = %s, want the package default (zero)", got)
	}
	c, s, f, _ := chatgptWorld(t)
	for _, budget := range []time.Duration{time.Second - 1, 30*time.Minute + 1, -time.Second} {
		c.ResponseBudget = budget
		_, err := c.Respond(context.Background(), s, chatgptRequest("conversation-0123456789abcdef"), nil)
		e, ok := sys.AsError(err)
		if !ok || e.Class != "provider" || e.Reason != "refused" || e.Detail != "response budget must be zero or between 1s and 30m0s" {
			t.Errorf("provider.Client.ResponseBudget %s: %v", budget, err)
		}
	}
	if paths, _ := f.seen(); len(paths) != 0 {
		t.Errorf("provider.Client.ResponseBudget: a refused budget sent %v", paths)
	}

	for _, budget := range []time.Duration{10 * time.Minute, 0} {
		var mu sync.Mutex
		now := time.Now()
		stall := func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			w.(http.Flusher).Flush()
			mu.Lock()
			now = now.Add(30*time.Minute + time.Second)
			mu.Unlock()
			panic(http.ErrAbortHandler)
		}
		client, s, f, _ := chatgptWorld(t, stall, stall, stall, stall)
		client.Now = func() time.Time { mu.Lock(); defer mu.Unlock(); return now }
		client.ResponseBudget = budget
		_, err := client.Respond(context.Background(), s, chatgptRequest("conversation-0123456789abcdef"), nil)
		want := "ChatGPT response wait exceeded 30 minutes"
		if budget != 0 {
			want = "ChatGPT response wait exceeded " + budget.String()
		}
		e, ok := sys.AsError(err)
		if !ok || e.Class != "provider" || e.Reason != "unavailable" || e.Exit != 4 || e.Detail != want {
			t.Errorf("provider.Client.ResponseBudget %s: %v, want %q", budget, err, want)
		}
		if paths, _ := f.seen(); len(paths) == 0 {
			t.Errorf("provider.Client.ResponseBudget %s: nothing was sent", budget)
		}
	}
}

// An interrupted session ends a request with the *sys.Interrupted (build/build.go concludes with
// its exit code), and a route that cannot force a tool refuses a request that names one before
// anything is sent.
func TestProviderRespondInterruptAndUnsupportedToolChoice(t *testing.T) {
	c, s, f, _ := chatgptWorld(t, sse(textItem("never")))
	s.Interrupt("SIGTERM")
	_, err := c.Respond(context.Background(), s, chatgptRequest("conversation-0123456789abcdef"), nil)
	var interrupted *sys.Interrupted
	if !errors.As(err, &interrupted) || interrupted.ExitCode() != 143 {
		t.Errorf("provider.Client.Respond of an interrupted session returned %v", err)
	}
	if paths, _ := f.seen(); len(paths) != 0 {
		t.Errorf("provider.Client.Respond sent %v after an interrupt", paths)
	}

	f = newFakeProvider(t)
	s, _, home := newSession(t, f.env()...)
	dir := filepath.Join(home, ".kogen", "credentials")
	writeFile(t, filepath.Join(dir, "anthropic.json"), jsonText(t, map[string]any{"provider": "anthropic", "api_key": "sk-ant-FIXTURE-KEY"}), 0o600)
	if err := os.Chmod(filepath.Join(home, ".kogen"), 0o700); err != nil {
		t.Fatal(err)
	}
	finish := []any{map[string]any{"type": "function", "name": "finish", "parameters": map[string]any{"type": "object"}}}
	anthropic := provider.Role{Provider: "anthropic", Model: "claude-sonnet-5-5", Effort: "high"}
	req := provider.Request{Role: anthropic, Instructions: "p", Input: []any{user("hi")}, Tools: finish, ToolChoice: "finish", CacheID: "x"}
	_, err = provider.NewClient(fixtureVersion).Respond(context.Background(), s, req, nil)
	e, ok := sys.AsError(err)
	if !ok || e.Class != "provider" || e.Reason != "unsupported_capability" {
		t.Errorf("provider.Request.ToolChoice on a route that cannot force a tool: %v", err)
	}
	if paths, _ := f.seen(); len(paths) != 0 {
		t.Errorf("provider.Request.ToolChoice: a refused request sent %v", paths)
	}
}
