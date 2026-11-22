package build

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/KogenAI/kogen-coordinator/rules"
	"github.com/KogenAI/kogen-loop/sys"
	"github.com/KogenAI/kogen-verification/verify"
)

const snapToken = "snapshot-synthetic-token"

// snapStudio is a loopback Studio rules endpoint with one rule.
type snapStudio struct {
	srv    *httptest.Server
	mu     sync.Mutex
	status int
	rev    int
	text   string
	count  int
}

func newSnapStudio(t *testing.T, rev int, text string) *snapStudio {
	s := &snapStudio{rev: rev, text: text}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.count++
		if r.Header.Get("Authorization") != "Bearer "+snapToken {
			http.Error(w, "unauthorized", 401)
			return
		}
		if s.status != 0 {
			http.Error(w, "status", s.status)
			return
		}
		fmt.Fprintf(w, `{"product":"p1","repo":%q,"rules":[{"id":"rule-1","revision":%d,"scope":"repo","topic":"tests","text":%q,"owner":"almir","overrides":null}]}`,
			r.URL.Query().Get("repo"), s.rev, s.text)
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *snapStudio) set(status, rev int, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status, s.rev, s.text = status, rev, text
}

func (s *snapStudio) requests() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.count
}

// snapEnv is a project configured for the snapshot's Studio, approved at the configured commit.
func snapEnv(t *testing.T, s *snapStudio) *env {
	t.Helper()
	e := newEnv(t)
	e.extraEnv = append(e.extraEnv, "SNAP_RULES_TOKEN="+snapToken)
	e.write(".kogen/studio-rules.yaml", "version: 1\nurl: "+s.srv.URL+"\nworkspace_id: w1\nproduct_id: p1\nrepo: kogen-build\ntoken_env: SNAP_RULES_TOKEN\non_unavailable: hold\n")
	e.run("git", "add", ".kogen/studio-rules.yaml")
	e.run("git", "commit", "-qm", "studio rules config")
	e.approve()
	return e
}

func snapRequests(e *env) string {
	var reqs []string
	for _, r := range e.prov.rawRequests() {
		reqs = append(reqs, string(r))
	}
	return strings.Join(reqs, "\n")
}

func snapReceipt(t *testing.T, doc map[string]any) map[string]any {
	t.Helper()
	r, _ := doc["studio_rules"].(map[string]any)
	if r == nil {
		t.Fatalf("no studio_rules in %v", doc)
	}
	return r
}

// The applied rules reach the builder request with their id, revision, topic and text, and the
// receipt of result.json and the checkpoint names them without text or token. A regression: a Build
// that ignores the supplied snapshot has neither.
func TestStudioSnapshotReachesRequestAndReceipt(t *testing.T) {
	t.Parallel()
	s := newSnapStudio(t, 3, "Run the focused test before finishing.")
	e := snapEnv(t, s)
	e.prov.then(landScript(e.prov)...)
	if a := e.build(); a.exit != 0 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	req := snapRequests(e)
	for _, want := range []string{studioHeading, "- rule-1 r3 [tests] Run the focused test before finishing."} {
		if !strings.Contains(req, want) {
			t.Fatalf("the builder request lacks %q", want)
		}
	}
	if strings.Contains(req, snapToken) {
		t.Fatal("the token reached a model request")
	}
	id := e.onlyRun()
	_, state := e.checkpoint(id)
	for name, doc := range map[string]map[string]any{"result.json": e.result(id), "checkpoint": state} {
		r := snapReceipt(t, doc)
		raw, _ := json.Marshal(r)
		if r["origin"] != "live" || r["fallback_reason"] != "" || !strings.HasPrefix(fmt.Sprint(r["content_hash"]), "sha256:") {
			t.Fatalf("%s receipt = %s", name, raw)
		}
		rules, _ := r["rules"].([]any)
		if len(rules) != 1 || fmt.Sprint(rules[0]) != "map[id:rule-1 revision:3]" {
			t.Fatalf("%s receipt rules = %s", name, raw)
		}
		if strings.Contains(string(raw), "focused test") || strings.Contains(string(raw), snapToken) {
			t.Fatalf("%s receipt holds rule text or the token: %s", name, raw)
		}
	}
}

// An outage uses the verified cache and the Build runs with origin cached and the reason.
func TestStudioSnapshotOutageUsesTheCache(t *testing.T) {
	t.Parallel()
	s := newSnapStudio(t, 3, "Run the focused test before finishing.")
	e := snapEnv(t, s)
	// A live load of the producer fills the verified cache a later outage falls back to.
	if _, err := rules.Load(context.Background(), e.root, rules.Options{Getenv: func(string) string { return snapToken }}); err != nil {
		t.Fatal(err)
	}

	s.set(503, 3, "")
	e.prov.then(landScript(e.prov)...)
	if a := e.build(); a.exit != 0 {
		t.Fatalf("outage with a cache: exit %d\n%s", a.exit, a.shown())
	}
	id := e.runIDs()[len(e.runIDs())-1]
	if r := snapReceipt(t, e.result(id)); r["origin"] != "cached" || r["fallback_reason"] != "http_503" {
		t.Fatalf("outage receipt = %v", r)
	}

}

// A denied token never reuses the cache, and a missing token variable is named; neither reaches a provider.
func TestStudioSnapshotRefusalsNeverUseTheCache(t *testing.T) {
	t.Parallel()
	s := newSnapStudio(t, 3, "Run the focused test before finishing.")
	e := snapEnv(t, s)
	if _, err := rules.Load(context.Background(), e.root, rules.Options{Getenv: func(string) string { return snapToken }}); err != nil {
		t.Fatal(err)
	}
	e.extraEnv = []string{"SNAP_RULES_TOKEN=revoked"}
	before := e.prov.count()
	a := e.build()
	if a.exit == 0 || !strings.Contains(fmt.Sprint(a.err), "rules/refused") || !strings.Contains(fmt.Sprint(a.err), "http_401") || e.prov.count() != before {
		t.Fatalf("revoked token: exit %d err %v provider requests %d", a.exit, a.err, e.prov.count()-before)
	}
	e.extraEnv = nil
	a = e.build()
	if a.exit == 0 || !strings.Contains(fmt.Sprint(a.err), "rules/refused") || !strings.Contains(fmt.Sprint(a.err), "token_missing") {
		t.Fatalf("missing token variable: exit %d err %v", a.exit, a.err)
	}
}

// A resumed Build keeps the recorded snapshot: no Studio request, the same receipt in result.json
// and the checkpoint.
func TestStudioSnapshotResumeKeepsRecordedSnapshot(t *testing.T) {
	t.Parallel()
	s := newSnapStudio(t, 3, "Run the focused test before finishing.")
	e := snapEnv(t, s)
	e.prov.then(landScript(e.prov)...)
	session := json.RawMessage(`{"input":[{"role":"user","content":"review"}]}`)
	stop := &scriptedReviewer{review: func(ctx context.Context, b *Attempt, pass verify.Pass) (Approved, error) {
		b.Review++
		b.LastReview = &ReviewRecord{N: b.Review, Verdict: "pending", Findings: []map[string]any{}}
		if err := b.Save(Continuation{Stage: StageReview, Pass: &pass, Review: session}); err != nil {
			return Approved{}, err
		}
		return Approved{}, sys.Fail("provider", "unavailable", "the reviewer's provider is down", 4)
	}}
	if a := e.build(func(in *Input) { in.Reviewer = stop }); a.exit != 4 {
		t.Fatalf("first exit %d\n%s", a.exit, a.shown())
	}
	id := e.onlyRun()
	first := s.requests()
	s.set(0, 4, "Run every test twice.")
	done := &scriptedReviewer{resume: func(ctx context.Context, b *Attempt) (Approved, error) {
		return approve(b, *b.Continuation.Pass), nil
	}}
	if r := e.resume(id, func(in *ResumeInput) { in.Reviewer = done }); r.exit != 0 {
		t.Fatalf("resume exit %d\n%s", r.exit, r.shown())
	}
	if s.requests() != first {
		t.Fatalf("the resumed Build asked Studio %d more time(s)", s.requests()-first)
	}
	_, state := e.checkpoint(id)
	for name, doc := range map[string]map[string]any{"result.json": e.result(id), "checkpoint": state} {
		rules, _ := snapReceipt(t, doc)["rules"].([]any)
		if len(rules) != 1 || fmt.Sprint(rules[0]) != "map[id:rule-1 revision:3]" {
			t.Fatalf("%s receipt after resume: %v", name, rules)
		}
	}
}

// stopInReview stops a configured Build inside its review and returns its id.
func stopInReview(t *testing.T, e *env) string {
	t.Helper()
	e.prov.then(landScript(e.prov)...)
	session := json.RawMessage(`{"input":[{"role":"user","content":"review"}]}`)
	stop := &scriptedReviewer{review: func(ctx context.Context, b *Attempt, pass verify.Pass) (Approved, error) {
		b.Review++
		b.LastReview = &ReviewRecord{N: b.Review, Verdict: "pending", Findings: []map[string]any{}}
		if err := b.Save(Continuation{Stage: StageReview, Pass: &pass, Review: session}); err != nil {
			return Approved{}, err
		}
		return Approved{}, sys.Fail("provider", "unavailable", "the reviewer's provider is down", 4)
	}}
	if a := e.build(func(in *Input) { in.Reviewer = stop }); a.exit != 4 {
		t.Fatalf("first exit %d\n%s", a.exit, a.shown())
	}
	return e.onlyRun()
}

// A resume whose token is missing, or whose cache is absent or no longer matches the recorded
// receipt, stops with a rules error before any provider, reviewer or Studio request.
func TestStudioSnapshotResumeStopsWhenRecordedRulesCannotBeRecovered(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		break_ func(e *env, s *snapStudio)
		want   []string
	}{
		{"token missing", func(e *env, s *snapStudio) { e.extraEnv = nil }, []string{"rules/refused", "token_missing"}},
		{"cache absent", func(e *env, s *snapStudio) {
			files, _ := filepath.Glob(filepath.Join(e.root, ".kogen/local/studio-rules/*.json"))
			for _, f := range files {
				os.Remove(f)
			}
		}, []string{"rules/hold"}},
		{"cache overwritten by a later snapshot", func(e *env, s *snapStudio) {
			s.set(0, 4, "Run every test twice.")
			if _, err := rules.Load(context.Background(), e.root, rules.Options{Getenv: func(string) string { return snapToken }}); err != nil {
				t.Fatal(err)
			}
		}, []string{"rules/hold", "content_hash differs"}},
		{"cache unverifiable", func(e *env, s *snapStudio) {
			files, _ := filepath.Glob(filepath.Join(e.root, ".kogen/local/studio-rules/*.json"))
			for _, f := range files {
				raw, _ := os.ReadFile(f)
				os.WriteFile(f, []byte(strings.Replace(string(raw), "focused test", "other test", 1)), 0o644)
			}
		}, []string{"rules/hold"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newSnapStudio(t, 3, "Run the focused test before finishing.")
			e := snapEnv(t, s)
			id := stopInReview(t, e)
			c.break_(e, s)
			studio, provider := s.requests(), e.prov.count()
			reviewed := false
			r := e.resume(id, func(in *ResumeInput) {
				in.Reviewer = &scriptedReviewer{resume: func(ctx context.Context, b *Attempt) (Approved, error) {
					reviewed = true
					return approve(b, *b.Continuation.Pass), nil
				}}
			})
			text := fmt.Sprint(r.err) + r.shown()
			if r.exit == 0 || reviewed || s.requests() != studio || e.prov.count() != provider {
				t.Fatalf("exit %d reviewed %v studio requests %d provider requests %d\n%s", r.exit, reviewed, s.requests()-studio, e.prov.count()-provider, text)
			}
			for _, want := range c.want {
				if !strings.Contains(text, want) {
					t.Fatalf("the stop does not name %q\n%s", want, text)
				}
			}
			if strings.Contains(text, snapToken) {
				t.Fatal("the token reached the output")
			}
		})
	}
}

// A finding whose message cites an applied rule id carries that rule and revision; one that cites
// none, or only a longer id, does not, and the written record is a copy.
func TestStudioSnapshotAnnotatesReviewFindings(t *testing.T) {
	t.Parallel()
	b := &Attempt{State: State{StudioRules: &StudioRules{Rules: []StudioRuleRef{
		{ID: "rule-1", Revision: json.RawMessage("3")}, {ID: "rule-2", Revision: json.RawMessage(`"b"`)},
	}}}}
	findings := []map[string]any{
		{"message": "rule-1 says to run the focused test"},
		{"message": "see (rule-2) and rule-1"},
		{"message": "rule-10 is another rule"},
		{"message": "unrelated"},
	}
	record := map[string]any{"verdict": "changes", "findings": findings}
	got := b.annotateReview("review-1.json", record).(map[string]any)["findings"].([]any)
	want := []any{
		map[string]any{"id": "rule-1", "revision": json.RawMessage("3")},
		map[string]any{"id": "rule-2", "revision": json.RawMessage(`"b"`)},
		nil, nil,
	}
	for i, w := range want {
		applied, has := got[i].(map[string]any)["applied_rule"]
		if (w == nil) == has || (has && fmt.Sprint(applied) != fmt.Sprint(w)) {
			t.Fatalf("finding %d applied_rule = %v (present %v), want %v", i, applied, has, w)
		}
	}
	if _, touched := findings[0]["applied_rule"]; touched {
		t.Fatal("the reviewer's own finding was edited")
	}
	if out := b.annotateReview("initial-verification.json", record); fmt.Sprint(out) != fmt.Sprint(record) {
		t.Fatal("a file that is not a review record was annotated")
	}
}
