package consumes

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/KogenAI/kogen-coordinator/git"
	"github.com/KogenAI/kogen-executor/agent"
	"kogen-loop/provider"
	"kogen-loop/sys"
	"kogen-loop/tools"
)

// newRepo makes a Git repository with one commit, a.txt, in a temporary directory.
func newRepo(t *testing.T) (*git.Repo, string) {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"-c", "user.name=T", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false", "commit", "-q", "--allow-empty", "-m", "start"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = os.Environ()
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	repo, err := git.Open(dir, os.Environ())
	if err != nil {
		t.Fatal(err)
	}
	return repo, dir
}

func TestAgentGitNil(t *testing.T) {
	var _ func(*git.Repo) tools.Git = agent.Git
	var g tools.Git = agent.Git(nil)
	if g != nil {
		t.Errorf("agent.Git: a nil repository must give a nil tools.Git (tools then uses its own runner), got %#v", g)
	}
}

func TestAgentGitRunsThroughTheRepository(t *testing.T) {
	repo, dir := newRepo(t)
	g := agent.Git(repo)
	if g == nil {
		t.Fatal("agent.Git: a repository gave a nil tools.Git")
	}
	// A command that ran: its stdout and zero status, with no error.
	res, err := g.Run("rev-parse", "--is-inside-work-tree")
	if err != nil || res.Status != 0 || res.Stdout != "true\n" {
		t.Errorf("agent.Git: Run of a succeeding command = %+v, %v; want stdout \"true\\n\", status 0, no error", res, err)
	}
	// A command that ran and failed: its status and stderr in the result, not an error.
	res, err = g.Run("cat-file", "-t", "nosuchobject")
	if err != nil || res.Status == 0 || !strings.Contains(res.Stderr, "nosuchobject") {
		t.Errorf("agent.Git: Run of a failing command = %+v, %v; want a nonzero status, the stderr and no error", res, err)
	}
	// Head and Tree are the repository's own.
	head := g.Head()
	if len(head) < 40 || head != repo.Head() {
		t.Errorf("agent.Git: Head = %q, want the repository's HEAD %q", head, repo.Head())
	}
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "skip"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skip", "x.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tree, err := g.Tree(head, []string{"skip"})
	want, werr := repo.Tree(head, []string{"skip"})
	if err != nil || werr != nil || tree != want || tree == "" {
		t.Fatalf("agent.Git: Tree = %q, %v; the repository's Tree = %q, %v", tree, err, want, werr)
	}
	listing, err := g.Run("ls-tree", "-r", "--name-only", tree)
	if err != nil || listing.Stdout != "new.txt\n" {
		t.Errorf("agent.Git: Tree must hold the untracked file and not the excluded path; ls-tree = %q, %v", listing.Stdout, err)
	}
	if _, err := g.Tree("no-such-rev", nil); err == nil {
		t.Errorf("agent.Git: Tree of a start that is no revision must fail")
	}
}

func TestAgentPromptTestingRules(t *testing.T) {
	var _ string = agent.TestingRules
	var _ func(string) string = agent.Prompt
	if agent.TestingRules != "testing-rules" {
		t.Errorf("agent.TestingRules = %q, want %q", agent.TestingRules, "testing-rules")
	}
	text := agent.Prompt(agent.TestingRules)
	if !strings.HasPrefix(text, "# Testing rules\n\nThese rules decide which tests a change needs") || !strings.HasSuffix(text, "\n") {
		t.Errorf("agent.Prompt: the testing rules must open with their heading and end with a newline, got %q ... %q", text[:min(60, len(text))], text[max(0, len(text)-20):])
	}
	// The builder's brief embeds these bytes: any change to them is a change of the brief.
	sum := sha256.Sum256([]byte(text))
	if got := hex.EncodeToString(sum[:]); got != "631f7ad0ce8283fd8147656e2d1dbc9b8b7dca7e4a32af0686f9cacbedeb817c" || len(text) != 15677 {
		t.Errorf("agent.Prompt: the testing rules are %d bytes with SHA-256 %s; kogen-build's brief relies on 15677 bytes with SHA-256 631f7ad0...b817c", len(text), got)
	}
}

func TestAgentAsAny(t *testing.T) {
	var _ func(...[]map[string]any) []any = agent.AsAny
	a := map[string]any{"name": "a"}
	b := map[string]any{"name": "b"}
	c := map[string]any{"name": "c"}
	got := agent.AsAny([]map[string]any{a, b}, []map[string]any{c})
	if len(got) != 3 || !reflect.DeepEqual(got[0], a) || !reflect.DeepEqual(got[1], b) || !reflect.DeepEqual(got[2], c) {
		t.Errorf("agent.AsAny: want the definitions of every list in order, got %v", got)
	}
	if got := agent.AsAny(); len(got) != 0 {
		t.Errorf("agent.AsAny: no lists must give no definitions, got %v", got)
	}
}

func TestAgentApplyBuildReview(t *testing.T) {
	var _ func(*provider.Client) *provider.Client = agent.ApplyBuildReview
	if got := agent.ApplyBuildReview(nil); got != nil {
		t.Errorf("agent.ApplyBuildReview: nil client must stay nil, got %v", got)
	}
	c := &provider.Client{}
	if got := agent.ApplyBuildReview(c); got != c || c.ResponseBudget != 10*time.Minute {
		t.Errorf("agent.ApplyBuildReview: must return the same client with a 10-minute response budget, got %p (client %p) budget %v", got, c, c.ResponseBudget)
	}
}

func TestAgentBudgetEvent(t *testing.T) {
	var _ func(*provider.Client) map[string]any = agent.BudgetEvent
	// The recorded budgets: the library's 30 minutes (no client, or a client that sets none),
	// Build and Review's 10 minutes, and any other duration as Go renders it.
	custom := &provider.Client{ResponseBudget: 90 * time.Second}
	for _, tc := range []struct {
		name   string
		client *provider.Client
		ms     int64
		text   string
	}{
		{"nil client", nil, 1800000, "30 minutes"},
		{"zero budget", &provider.Client{}, 1800000, "30 minutes"},
		{"build and review", agent.ApplyBuildReview(&provider.Client{}), 600000, "10 minutes"},
		{"custom", custom, 90000, "1m30s"},
	} {
		want := map[string]any{"event": "response_budget", "budget_ms": tc.ms, "budget": tc.text}
		if got := agent.BudgetEvent(tc.client); !reflect.DeepEqual(got, want) {
			t.Errorf("agent.BudgetEvent: %s = %#v, want %#v", tc.name, got, want)
		}
	}
}

func TestAgentNameBudgetExpiry(t *testing.T) {
	var _ func(error, time.Duration) error = agent.NameBudgetExpiry
	const budget = 10 * time.Minute
	expired := &sys.Error{Class: "provider", Reason: "unavailable", Detail: "chatgpt response wait exceeded 10m0s", Exit: 4, Lines: []string{"line"}}
	got, ok := agent.NameBudgetExpiry(expired, budget).(*sys.Error)
	if !ok || got.Detail != "chatgpt response wait exceeded 10 minutes" || got.Class != "provider" || got.Reason != "unavailable" || got.Exit != 4 || !slices.Equal(got.Lines, []string{"line"}) {
		t.Errorf("agent.NameBudgetExpiry: the expiry of the 10-minute budget must be renamed and keep class, reason, exit and lines, got %#v", got)
	}
	if expired.Detail != "chatgpt response wait exceeded 10m0s" {
		t.Errorf("agent.NameBudgetExpiry: must not change the error it is given, got detail %q", expired.Detail)
	}
	unchanged := []struct {
		name   string
		err    error
		budget time.Duration
	}{
		{"another budget", expired, 5 * time.Minute},
		{"another class", &sys.Error{Class: "executor", Reason: "unavailable", Detail: "x response wait exceeded 10m0s"}, budget},
		{"another reason", &sys.Error{Class: "provider", Reason: "refused", Detail: "x response wait exceeded 10m0s"}, budget},
		{"another detail", &sys.Error{Class: "provider", Reason: "unavailable", Detail: "x response wait exceeded 10m0s later"}, budget},
		{"not a sys.Error", os.ErrClosed, budget},
	}
	for _, tc := range unchanged {
		if got := agent.NameBudgetExpiry(tc.err, tc.budget); got != tc.err {
			t.Errorf("agent.NameBudgetExpiry: %s must be returned unchanged, got %#v", tc.name, got)
		}
	}
	if got := agent.NameBudgetExpiry(nil, budget); got != nil {
		t.Errorf("agent.NameBudgetExpiry: a nil error must stay nil, got %#v", got)
	}
}

func TestAgentWithOriginHint(t *testing.T) {
	var _ func(*sys.Error) *sys.Error = agent.WithOriginHint
	const relogin = "log in again for that origin with: kogen provider login openai-compatible"
	refused := &sys.Error{Class: "provider", Reason: "refused", Detail: "the saved key belongs to https://a.example", Lines: []string{"first"}, Exit: 3}
	got := agent.WithOriginHint(refused)
	if got == refused || got.Class != "provider" || got.Reason != "refused" || got.Detail != refused.Detail || got.Exit != 3 || !slices.Equal(got.Lines, []string{"first", relogin}) {
		t.Errorf("agent.WithOriginHint: the origin refusal must gain the relogin line after its own lines, got %#v", got)
	}
	if !slices.Equal(refused.Lines, []string{"first"}) {
		t.Errorf("agent.WithOriginHint: must not change the error it is given, got lines %v", refused.Lines)
	}
	if again := agent.WithOriginHint(got); again != got {
		t.Errorf("agent.WithOriginHint: an error that already names the command must be returned as it is")
	}
	for name, e := range map[string]*sys.Error{
		"nil":            nil,
		"another class":  {Class: "executor", Reason: "refused", Detail: "key belongs to x"},
		"another reason": {Class: "provider", Reason: "unavailable", Detail: "key belongs to x"},
		"another detail": {Class: "provider", Reason: "refused", Detail: "something else"},
	} {
		if got := agent.WithOriginHint(e); got != e {
			t.Errorf("agent.WithOriginHint: %s must be returned unchanged, got %#v", name, got)
		}
	}
}
