package consumes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	gitpkg "github.com/KogenAI/kogen-coordinator/git"
	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-executor/core"
	"github.com/KogenAI/kogen-executor/executor"
	"github.com/KogenAI/kogen-loop/provider"
	"github.com/KogenAI/kogen-loop/sys"
	"github.com/KogenAI/kogen-loop/tools"
	"github.com/KogenAI/kogen-review/review"
	"github.com/KogenAI/kogen-verification/verify"
)

// eq fails the test when got differs from want; item names the inventory item checked.
func eq[T comparable](t testing.TB, item string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %#v, want %#v", item, got, want)
	}
}

// deepEq is eq for values that are not comparable.
func deepEq(t testing.TB, item string, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s: got %#v, want %#v", item, got, want)
	}
}

// wantError fails unless err is a *sys.Error of the class and reason; it returns the error.
func wantError(t testing.TB, item string, err error, class, reason string) *sys.Error {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: no error, want %s/%s", item, class, reason)
	}
	e, ok := sys.AsError(err)
	if !ok {
		t.Fatalf("%s: error %v is not a *sys.Error, want %s/%s", item, err, class, reason)
	}
	if e.Class != class || e.Reason != reason {
		t.Errorf("%s: error %s/%s (%s), want %s/%s", item, e.Class, e.Reason, e.Detail, class, reason)
	}
	return e
}

func writeFile(t testing.TB, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// git runs a Git command in dir with the suite's isolated environment.
func git(t testing.TB, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// reviewRole is the review model of the fixture's configuration.
var reviewRole = project.Role{Provider: "chatgpt", Model: "fixture-review-model", Effort: "medium"}

const (
	otherText = "candidate, edited after the green pass\n"
	feature   = "Feature: Fixture\n  Scenario: First\n    Then it works\n"
)

// fixture is a saved candidate in a temporary Git checkout: the start commit holds app.txt,
// deleted.txt, skip.txt and the approved Intent's feature; the candidate (green) changed app.txt
// and deleted deleted.txt; other is the candidate after a further edit of app.txt.
type fixture struct {
	root, start, green, other, dir string
	repo                           *gitpkg.Repo
}

func newFixture(t testing.TB) *fixture {
	t.Helper()
	root := t.TempDir()
	git(t, root, "init", "-q", "-b", "main")
	writeFile(t, filepath.Join(root, ".gitignore"), ".kogen/local/\n")
	writeFile(t, filepath.Join(root, "app.txt"), "start\n")
	writeFile(t, filepath.Join(root, "deleted.txt"), "gone\n")
	writeFile(t, filepath.Join(root, "skip.txt"), "skip\n")
	writeFile(t, filepath.Join(root, project.FeaturePath("fixture")), feature)
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "-m", "start")
	f := &fixture{root: root, start: git(t, root, "rev-parse", "HEAD"), repo: gitpkg.New(root, nil)}
	writeFile(t, filepath.Join(root, "app.txt"), "candidate\n")
	if err := os.Remove(filepath.Join(root, "deleted.txt")); err != nil {
		t.Fatal(err)
	}
	var err error
	if f.green, err = f.repo.Tree(f.start, nil); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "app.txt"), otherText)
	if f.other, err = f.repo.Tree(f.start, nil); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "app.txt"), "candidate\n")
	f.dir = filepath.Join(root, ".kogen/local/runs/fixture-run")
	if err := os.MkdirAll(f.dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return f
}

// greenPass is the green pass of the candidate a review is given.
func (f *fixture) greenPass() verify.Pass {
	return verify.Pass{N: 1, Clean: true, Green: true, Tree: f.green}
}

// input is the review Input of the candidate: review number 1 of greenPass.
func (f *fixture) input() review.Input {
	return review.Input{
		Candidate: review.Candidate{Root: f.root, Repo: f.repo, Start: f.start, Tree: f.green},
		Intent:    review.Intent{Slug: "fixture", Text: "# Fixture intent\n", Role: reviewRole},
		Evidence:  review.Evidence{Pass: f.greenPass()},
		Tools: review.Tools{
			Native: func(enabled []string) *tools.Tools { return &tools.Tools{Root: f.root, Enabled: enabled} },
			Record: func(tools.Call, tools.Output) map[string]any { return nil },
			Round: func(ctx context.Context, sess executor.Session, in executor.Input) (executor.RoundResult, error) {
				return sess.Round(ctx, in)
			},
			Home: os.Getenv("HOME"),
		},
		Number: 1,
		Run:    review.Run{ID: "fixture-run", Dir: f.dir},
	}
}

// fakeEnv is a review's Env as a test scripts it: the model's answers and the verification
// passes. It logs every call the reviewer makes of its caller, in order, and keeps their arguments.
type fakeEnv struct {
	t   testing.TB
	dir string

	answers  []*provider.Response // one per Request, in order
	verifies []verify.Pass        // what Verify returns, in order
	unread   []string             // Unread's answer
	failOn   map[string]error     // an error a method returns instead of acting

	calls     []string
	requests  []review.Request
	prepares  []prepared
	prefills  [][]any
	saves     []review.Checkpoint
	boundary  []boundary
	reviewed  []core.ReviewRecord
	repairs   int
	tracks    int
	facts     []string
	events    []map[string]any
	progress  []string
	feedbacks []string
	executor  []*core.ExecutorState
	delivered int
}

type prepared struct {
	tree         string
	material     review.MaterialSet
	role         project.Role
	instructions string
	defs         []any
	diagnostic   []any
}

type boundary struct {
	input []any
	done  *review.Done
}

func (h *fakeEnv) log(name string) error {
	h.calls = append(h.calls, name)
	return h.failOn[name]
}

func (h *fakeEnv) Request(ctx context.Context, r review.Request) (*provider.Response, []any, error) {
	if err := h.log("Request"); err != nil {
		return nil, nil, err
	}
	h.requests = append(h.requests, r)
	if len(h.answers) == 0 {
		h.t.Fatal("no scripted answer left for a model request")
	}
	resp := h.answers[0]
	h.answers = h.answers[1:]
	return resp, r.Input, nil
}

func (h *fakeEnv) Prepare(tree string, m review.MaterialSet, role project.Role, instructions string, defs, diagnostic []any) ([]any, error) {
	if err := h.log("Prepare"); err != nil {
		return nil, err
	}
	h.prepares = append(h.prepares, prepared{tree, m, role, instructions, defs, diagnostic})
	return []any{map[string]any{"role": "user", "content": m.Authority + "\n" + m.Source}}, nil
}

func (h *fakeEnv) Unread([]any) ([]string, error) { return h.unread, h.log("Unread") }
func (h *fakeEnv) Delivered([]any) error          { h.delivered++; return h.log("Delivered") }

func (h *fakeEnv) Prefill(role project.Role, instructions string, defs, input []any) ([]any, error) {
	h.prefills = append(h.prefills, []any{role, instructions, defs, input})
	return append(input[:len(input):len(input)], map[string]any{"role": "user", "content": "prefilled"}), h.log("Prefill")
}

func (h *fakeEnv) Custody() error { return h.log("Custody") }

func (h *fakeEnv) Verify(context.Context) (verify.Pass, error) {
	if err := h.log("Verify"); err != nil {
		return verify.Pass{}, err
	}
	if len(h.verifies) == 0 {
		h.t.Fatal("Verify was not expected")
	}
	p := h.verifies[0]
	h.verifies = h.verifies[1:]
	return p, nil
}

func (h *fakeEnv) Feedback(pass verify.Pass, completion string) string {
	h.log("Feedback")
	s := "fixture feedback of pass " + itoa(pass.N) + "; call " + completion
	h.feedbacks = append(h.feedbacks, s)
	return s
}

func (h *fakeEnv) BaselineIntact() error { return h.log("BaselineIntact") }

func (h *fakeEnv) Comparison() []string {
	h.log("Comparison")
	return []string{"comparison line 1", "comparison line 2"}
}

func (h *fakeEnv) Save(c review.Checkpoint) error {
	h.saves = append(h.saves, c)
	return h.log("Save")
}

func (h *fakeEnv) Boundary(input []any, done *review.Done) error {
	h.boundary = append(h.boundary, boundary{input, done})
	return h.log("Boundary")
}

func (h *fakeEnv) Reviewed(r core.ReviewRecord) error {
	h.reviewed = append(h.reviewed, r)
	return h.log("Reviewed")
}

func (h *fakeEnv) Repaired() error { h.repairs++; return h.log("Repaired") }

func (h *fakeEnv) Executor(x *core.ExecutorState) {
	h.log("Executor")
	h.executor = append(h.executor, x)
}

func (h *fakeEnv) Track() error { h.tracks++; return h.log("Track") }

func (h *fakeEnv) Facts(phase string) error {
	h.facts = append(h.facts, phase)
	return h.log("Facts")
}

func (h *fakeEnv) Progress(line string) { h.log("Progress"); h.progress = append(h.progress, line) }
func (h *fakeEnv) Event(e map[string]any) {
	h.log("Event")
	h.events = append(h.events, e)
}

func (h *fakeEnv) Write(name string, data any) error {
	if err := h.log("Write"); err != nil {
		return err
	}
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(h.dir, name), b, 0o600)
}

func (h *fakeEnv) Err() error { return h.log("Err") }

var _ review.Env = (*fakeEnv)(nil)

func newEnv(t testing.TB, f *fixture) *fakeEnv {
	return &fakeEnv{t: t, dir: f.dir}
}

// called is how many times the reviewer called the named Env method.
func (h *fakeEnv) called(name string) int {
	n := 0
	for _, c := range h.calls {
		if c == name {
			n++
		}
	}
	return n
}

// stages are the stages a review checkpointed, consecutive repeats merged, with "tool" for each
// Boundary call.
func (h *fakeEnv) stages() []string {
	var out []string
	save, bound := 0, 0
	for _, c := range h.calls {
		s := ""
		switch c {
		case "Save":
			s = h.saves[save].Stage
			save++
		case "Boundary":
			s = "tool"
			bound++
		default:
			continue
		}
		if len(out) == 0 || out[len(out)-1] != s {
			out = append(out, s)
		}
	}
	return out
}

// eventsNamed are the run log's events of one kind.
func (h *fakeEnv) eventsNamed(name string) []map[string]any {
	var out []map[string]any
	for _, e := range h.events {
		if e["event"] == name {
			out = append(out, e)
		}
	}
	return out
}

// submitReview is a model answer calling submit_review with args.
func submitReview(args string) *provider.Response {
	return &provider.Response{Items: []map[string]any{{
		"type": "function_call", "call_id": "call-submit", "name": "submit_review", "arguments": args,
	}}, Usage: provider.Usage{Input: 10, Output: 2}}
}

// callTool is a model answer calling a native tool.
func callTool(name, args string) *provider.Response {
	return &provider.Response{Items: []map[string]any{{
		"type": "function_call", "call_id": "call-" + name, "name": name, "arguments": args,
	}}, Usage: provider.Usage{Input: 10, Output: 2}}
}

// prose is a model answer without a tool call.
func prose(text string) *provider.Response {
	return &provider.Response{Items: []map[string]any{{
		"type": "message", "role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": text}},
	}}, Usage: provider.Usage{Input: 10, Output: 2}}
}

// fakeExecutor is an external executor serving the review role: one session per Start, whose
// rounds return the scripted results in order.
type fakeExecutor struct {
	t       testing.TB
	results []executor.RoundResult
	tasks   []executor.Task
	inputs  []executor.Input
	starts  int
	// sessionID overrides the id of every session ("": review-session-<n>).
	sessionID string
}

func (x *fakeExecutor) Describe() executor.Descriptor { return externalDescriptor() }

func (x *fakeExecutor) Start(ctx context.Context, task executor.Task, emit executor.Sink) (executor.Session, error) {
	x.starts++
	x.tasks = append(x.tasks, task)
	id := "review-session-" + itoa(x.starts)
	if x.sessionID != "" {
		id = x.sessionID
	}
	return &fakeSession{x: x, id: id}, nil
}

type fakeSession struct {
	x  *fakeExecutor
	id string
}

func (s *fakeSession) Handle() executor.Handle {
	return executor.Handle{Role: executor.RoleReview, Executor: "fixture-executor", Version: "1.0.0", SessionID: s.id}
}

func (s *fakeSession) Close() error { return nil }

func (s *fakeSession) Round(ctx context.Context, in executor.Input) (executor.RoundResult, error) {
	s.x.inputs = append(s.x.inputs, in)
	if len(s.x.results) == 0 {
		s.x.t.Fatal("no scripted executor round left")
	}
	r := s.x.results[0]
	s.x.results = s.x.results[1:]
	r.Handle = s.Handle()
	return r, nil
}

func externalDescriptor() executor.Descriptor {
	return executor.Descriptor{Name: "fixture-executor", Contract: "1", Version: "1.0.0",
		RoleCapabilities: map[string]executor.RoleCapabilities{executor.RoleReview: {}}}
}

// structured is an executor round that finished with the structured submission text.
func structured(text string) executor.RoundResult {
	return executor.RoundResult{Ending: executor.EndFinish, Structured: json.RawMessage(text)}
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// readJSON decodes a JSON file of the run folder.
func readJSON(t testing.TB, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func chmod(path string, mode os.FileMode) error { return os.Chmod(path, mode) }

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
