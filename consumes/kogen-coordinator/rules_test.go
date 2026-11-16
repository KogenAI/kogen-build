package consumes

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/KogenAI/kogen-coordinator/rules"
)

// Compile-time pins of the rules API kogen-build uses.
var (
	_ func(context.Context, string, rules.Options) (*rules.Snapshot, error) = rules.Load
	_ error                                                                 = rules.ErrNotConfigured
	_ error                                                                 = (*rules.Error)(nil)
	_ rules.Options                                                         = rules.Options{Getenv: os.Getenv, Log: func(string) {}}
	_ rules.Scope                                                           = rules.Scope{WorkspaceID: "", ProductID: "", Repo: ""}
	_ string                                                                = rules.Rule{}.ID + rules.Rule{}.Topic + rules.Rule{}.Text + string(rules.Rule{}.Revision)
	_ rules.Origin                                                          = rules.Snapshot{}.Origin
	_ string                                                                = rules.Snapshot{}.ContentHash + rules.Snapshot{}.FallbackReason
	_ time.Time                                                             = rules.Snapshot{}.RetrievedAt
	_ rules.Scope                                                           = rules.Snapshot{}.Scope
	_ []rules.Rule                                                          = rules.Snapshot{}.Rules
)

// rulesProject is a project configured for a Studio at url, with the token in TOKEN_VAR.
func rulesProject(t *testing.T, url string) string {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, ".kogen", "studio-rules.yaml")
	check(t, os.MkdirAll(filepath.Dir(path), 0o755))
	check(t, os.WriteFile(path, []byte("version: 1\nurl: "+url+"\nworkspace_id: w1\nproduct_id: p1\nrepo: r1\ntoken_env: TOKEN_VAR\non_unavailable: hold\n"), 0o644))
	return root
}

func rulesStudio(t *testing.T, status *int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if *status != 0 {
			http.Error(w, "status", *status)
			return
		}
		fmt.Fprint(w, `{"product":"p1","repo":"r1","rules":[{"id":"rule-1","revision":3,"scope":"repo","topic":"tests","text":"Run the focused test.","owner":"almir","overrides":null}]}`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func rulesEnv(name string) func(string) string {
	return func(k string) string {
		if k == "TOKEN_VAR" {
			return name
		}
		return ""
	}
}

func TestRulesLoadSnapshotAndCache(t *testing.T) {
	status := 0
	srv := rulesStudio(t, &status)
	root := rulesProject(t, srv.URL)
	var log []string
	opts := rules.Options{Getenv: rulesEnv("token"), Log: func(l string) { log = append(log, l) }}

	live, err := rules.Load(context.Background(), root, opts)
	check(t, err)
	if live.Origin != rules.Live || live.FallbackReason != "" || len(live.Rules) != 1 || len(log) != 1 {
		t.Fatalf("live snapshot = %+v log %v", live, log)
	}
	r := live.Rules[0]
	if r.ID != "rule-1" || string(r.Revision) != "3" || r.Topic != "tests" || r.Text != "Run the focused test." {
		t.Fatalf("live rule = %+v", r)
	}
	if live.Scope != (rules.Scope{WorkspaceID: "w1", ProductID: "p1", Repo: "r1"}) || len(live.ContentHash) != len("sha256:")+64 || live.RetrievedAt.IsZero() {
		t.Fatalf("live provenance = %+v", live)
	}

	status = 503
	cached, err := rules.Load(context.Background(), root, opts)
	check(t, err)
	if cached.Origin != rules.Cached || cached.FallbackReason != "http_503" || cached.ContentHash != live.ContentHash || string(cached.Rules[0].Revision) != "3" {
		t.Fatalf("cached snapshot = %+v", cached)
	}
}

func TestRulesLoadFailuresAreClassified(t *testing.T) {
	status := 0
	srv := rulesStudio(t, &status)
	root := rulesProject(t, srv.URL)
	if _, err := rules.Load(context.Background(), root, rules.Options{Getenv: rulesEnv("token")}); err != nil {
		t.Fatal(err)
	}
	expect := func(what string, err error, class, reason string) {
		t.Helper()
		var e *rules.Error
		if !errors.As(err, &e) || e.Class != class || e.Reason != reason {
			t.Fatalf("%s: error = %v, want %s/%s", what, err, class, reason)
		}
	}
	status = 401
	_, err := rules.Load(context.Background(), root, rules.Options{Getenv: rulesEnv("revoked")})
	expect("denied token never uses the cache", err, rules.ClassRefused, "http_401")
	_, err = rules.Load(context.Background(), root, rules.Options{Getenv: rulesEnv("")})
	expect("missing token variable", err, rules.ClassRefused, "token_missing")

	status = 503
	fresh := rulesProject(t, srv.URL)
	_, err = rules.Load(context.Background(), fresh, rules.Options{Getenv: rulesEnv("token")})
	expect("outage without a cache", err, rules.ClassHold, "http_503")
}

func TestRulesLoadWithoutConfigurationIsNotConfigured(t *testing.T) {
	snap, err := rules.Load(context.Background(), t.TempDir(), rules.Options{Getenv: rulesEnv("token")})
	if snap != nil || !errors.Is(err, rules.ErrNotConfigured) {
		t.Fatalf("snapshot %v error %v", snap, err)
	}
}
