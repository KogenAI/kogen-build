package build

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
)

type progressBuffer struct {
	mu   sync.Mutex
	text strings.Builder
}

func (b *progressBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.text.Write(p)
}
func (b *progressBuffer) String() string { b.mu.Lock(); defer b.mu.Unlock(); return b.text.String() }
func lastBuilding(s string) string {
	var last string
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, "kogen: building (round 1)") {
			last = line
		}
	}
	return last
}

func TestCompletedToolsProgressDuringNativeRound(t *testing.T) {
	for _, mode := range []string{"two", "none"} {
		t.Run(mode, func(t *testing.T) {
			e := newEnv(t)
			e.credentials()
			s, _, _ := e.session()
			out := &progressBuffer{}
			s.Stdout = out
			s.Stderr = out
			var snapshots []string
			capture := func(next handler) handler {
				return func(w http.ResponseWriter, r *http.Request) { snapshots = append(snapshots, out.String()); next(w, r) }
			}
			if mode == "two" {
				writeFile(t, e.root+"/progress-input.txt", "TOOL_RESULT_PRIVATE_7081", 0600)
				e.run("git", "add", "progress-input.txt")
				e.run("git", "commit", "-qm", "input")
				e.approve()
				e.prov.then(capture(e.prov.calls("read_file", map[string]any{"path": "progress-input.txt"})), capture(e.prov.calls("shell", map[string]any{"command": "printf TOOL_RESULT_PRIVATE_7081; exit 7"})), capture(refused()))
			} else {
				e.prov.then(capture(refused()))
			}
			_, _ = Build(context.Background(), s, Input{Deps: e.deps(), Project: e.root, Slug: testSlug})
			if len(snapshots) == 0 || lastBuilding(snapshots[0]) != "kogen: building (round 1)" {
				t.Fatalf("initial: %q", snapshots)
			}
			if mode == "two" {
				if len(snapshots) != 3 {
					t.Fatalf("snapshots: %q", snapshots)
				}
				a, b := lastBuilding(snapshots[1]), lastBuilding(snapshots[2])
				if !strings.Contains(a, "read_file progress-input.txt ok") || !strings.Contains(b, "shell failed") || strings.Contains(b, "progress-input.txt") || strings.Contains(b, "read_file") {
					t.Fatalf("progress: %q then %q", a, b)
				}
			} else if len(snapshots) != 1 {
				t.Fatalf("snapshots: %q", snapshots)
			}
			if strings.Contains(out.String(), "TOOL_RESULT_PRIVATE_7081") {
				t.Fatal("tool result leaked")
			}
		})
	}
}

func TestExternalCompletedToolsProgress(t *testing.T) {
	e := newEnv(t)
	e.useExecutor("")
	f := newFakeExecutor(t, externalWritesApp("candidate\n", harnessUsage(1, 0, 1, 0)))
	f.events = []map[string]any{
		{"event": "executor_tool", "name": "Read", "arguments": map[string]any{"file_path": "input.txt"}, "result_error": false, "result": "PRIVATE_RESULT"},
		{"event": "executor_tool", "name": "Bash", "result_error": true, "result": "PRIVATE_RESULT"},
		{"event": "executor_tool", "name": "Pending", "unanswered": true},
	}
	a := e.build(withExecutor(f, nil))
	if a.exit != 0 {
		t.Fatalf("build: %v %s", a.err, a.errs)
	}
	text := a.errs.String()
	if !strings.Contains(text, "Read input.txt ok") || !strings.Contains(text, "Bash failed") || strings.Contains(text, "PRIVATE_RESULT") || strings.Contains(text, "Pending") {
		t.Fatalf("progress: %s", text)
	}
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Bash failed") && strings.Contains(line, "input.txt") {
			t.Fatalf("stale path: %s", line)
		}
	}
}
