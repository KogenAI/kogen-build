package consumes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"kogen-loop/provider"
	"kogen-loop/sys"
)

// TestMain isolates the suite from the caller: the process's Git and Kogen variables and proxies
// are dropped, HOME is the suite's own temporary directory and Git reads no user or system
// configuration. A command run under the supervisor starts this executable again, so
// sys.MaybeSupervise comes first, as in kogen-build's cmd/kogen-build/main.go; commands then
// run under the supervisor as they do in kogen-build.
func TestMain(m *testing.M) {
	sys.MaybeSupervise(os.Args[1:])
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		upper := strings.ToUpper(name)
		if strings.HasPrefix(upper, "GIT_") || strings.HasPrefix(upper, "KOGEN_") || strings.HasSuffix(upper, "_PROXY") {
			os.Unsetenv(name)
		}
	}
	home, err := os.MkdirTemp("", "consumes-kogen-loop-home-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Setenv("HOME", home)
	os.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	os.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	os.Setenv("GIT_TERMINAL_PROMPT", "0")
	sys.EnableSupervisor()
	code := m.Run()
	os.RemoveAll(home)
	os.Exit(code)
}

// hermeticEnv is an explicit environment for a session: a PATH to find git and sh, its own HOME,
// its own Git identity and no user or system Git configuration. It never takes the caller's
// HOME, credentials or KOGEN_* variables.
func hermeticEnv(home string, extra ...string) []string {
	env := []string{
		"PATH=" + os.Getenv("PATH"), "HOME=" + home,
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0",
		"GIT_AUTHOR_NAME=Consumes", "GIT_AUTHOR_EMAIL=consumes@example.invalid",
		"GIT_COMMITTER_NAME=Consumes", "GIT_COMMITTER_EMAIL=consumes@example.invalid",
	}
	return append(env, extra...)
}

// newSession is a session in a fresh root with its own HOME.
func newSession(t *testing.T, extra ...string) (*sys.Session, string, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	return sys.NewWith(hermeticEnv(home, extra...), root), root, home
}

// gitIn runs git in dir with the hermetic environment and fails the test when it fails.
func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "commit.gpgsign=false", "-c", "tag.gpgsign=false", "-c", "core.hooksPath=/dev/null"}, args...)...)
	cmd.Dir = dir
	cmd.Env = hermeticEnv(t.TempDir())
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// repo is a Git repository in a fresh root with one commit holding files.
func repo(t *testing.T, files map[string]string) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gitIn(t, root, "init", "-q")
	for name, text := range files {
		writeFile(t, filepath.Join(root, name), text, 0o644)
	}
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-q", "-m", "init")
	return root
}

func writeFile(t *testing.T, path, text string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), mode); err != nil {
		t.Fatal(err)
	}
}

func jsonText(t testing.TB, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func intp(v int) *int { return &v }

// seedChatGPT writes a ChatGPT login the way kogen-build's tests seed it:
// $HOME/.kogen/credentials/chatgpt.json, mode 0600, in a 0700 .kogen.
func seedChatGPT(t *testing.T, home string) {
	t.Helper()
	dir := filepath.Join(home, ".kogen", "credentials")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(home, ".kogen"), 0o700); err != nil {
		t.Fatal(err)
	}
	b := jsonText(t, map[string]any{
		"client_id": "oaiapp_fake", "access_token": "fake-access-token", "refresh_token": "fake-refresh-token",
		"expires_at": time.Now().Unix() + 86400, "scope": "openid", "email": "integration@example.invalid",
	})
	writeFile(t, filepath.Join(dir, "chatgpt.json"), b, 0o600)
}

// fakeProvider is a loopback provider that answers /v1/responses from a script, one handler per
// request, refuses any other path and keeps the requests it saw.
type fakeProvider struct {
	srv    *httptest.Server
	mu     sync.Mutex
	script []http.HandlerFunc
	paths  []string
	bodies [][]byte
	heads  []http.Header
}

func newFakeProvider(t *testing.T, script ...http.HandlerFunc) *fakeProvider {
	t.Helper()
	f := &fakeProvider{script: script}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeProvider) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.paths = append(f.paths, r.URL.Path)
	f.bodies = append(f.bodies, body)
	f.heads = append(f.heads, r.Header.Clone())
	var h http.HandlerFunc
	if len(f.paths) <= len(f.script) {
		h = f.script[len(f.paths)-1]
	}
	f.mu.Unlock()
	if r.URL.Path != "/v1/responses" {
		http.NotFound(w, r)
		return
	}
	if h == nil {
		h = refusal
	}
	h(w, r)
}

func (f *fakeProvider) seen() (paths []string, bodies [][]byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.paths...), append([][]byte(nil), f.bodies...)
}

func (f *fakeProvider) request(t *testing.T, i int) map[string]any {
	t.Helper()
	_, bodies := f.seen()
	if i >= len(bodies) {
		t.Fatalf("request %d: the provider saw %d", i, len(bodies))
	}
	var m map[string]any
	if err := json.Unmarshal(bodies[i], &m); err != nil {
		t.Fatalf("request %d is not JSON: %v", i, err)
	}
	return m
}

// env is the session environment that points a client at this provider.
func (f *fakeProvider) env() []string {
	return []string{"KOGEN_PROVIDER_URL=" + f.srv.URL, "KOGEN_AUTH_URL=" + f.srv.URL}
}

func refusal(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(400)
	io.WriteString(w, `{"error":{"code":"invalid_request_error","message":"scripted provider refusal","type":"invalid_request_error"}}`)
}

// sse answers with the frames kogen-build's fakes write: one response.output_item.done frame per
// item, then response.completed with usage (input 11 of which 3 cached, output 7 of which 2
// reasoning).
func sse(items ...map[string]any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("X-Request-Id", "req-fixture")
		for i, it := range items {
			frame(w, map[string]any{"type": "response.output_item.done", "output_index": i, "item": it})
		}
		frame(w, map[string]any{"type": "response.completed", "response": map[string]any{"usage": map[string]any{
			"input_tokens": 11, "input_tokens_details": map[string]any{"cached_tokens": 3},
			"output_tokens": 7, "output_tokens_details": map[string]any{"reasoning_tokens": 2}}}})
	}
}

func frame(w http.ResponseWriter, v any) {
	b, _ := json.Marshal(v)
	io.WriteString(w, "data: "+string(b)+"\n\n")
}

func textItem(text string) map[string]any {
	return map[string]any{"type": "message", "id": "message", "role": "assistant",
		"content": []any{map[string]any{"type": "output_text", "text": text, "annotations": []any{}}}}
}

func callItem(id, name string, args any) map[string]any {
	b, _ := json.Marshal(args)
	return map[string]any{"type": "function_call", "id": "function-" + id, "call_id": "call-" + id, "name": name, "arguments": string(b)}
}

func user(text string) map[string]any { return map[string]any{"role": "user", "content": text} }

// chatgptRole is the role of the ChatGPT requests of this suite.
var chatgptRole = provider.Role{Provider: "chatgpt", Model: "gpt-6.1-sol", Effort: "medium"}

const fixtureVersion = "kogen-build consumes (test)"

// noWait is the Client.Sleep of every request here: backoff and lock waits take no time.
func noWait(context.Context, time.Duration) error { return nil }

// stopped reports whether the process pid is not running: gone, or a zombie that only waits to be
// reaped. A process that is still ending is not stopped.
func stopped(t *testing.T, pid int) bool {
	t.Helper()
	cmd := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid))
	cmd.Env = hermeticEnv(t.TempDir())
	out, _ := cmd.Output()
	state := strings.TrimSpace(string(out))
	return state == "" || strings.HasPrefix(state, "Z")
}

// pidOf reads the process id a command wrote to path.
func pidOf(t *testing.T, path string) int {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		t.Fatalf("%s holds %q", path, b)
	}
	return pid
}
