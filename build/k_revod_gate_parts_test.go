package build

import (
	"strings"
	"testing"

	"github.com/KogenAI/kogen-review/review"
)

// The default parts mode is not changed by the on_demand hunk gate (k-revod-gate): a long
// parts-mode review (pinned parts, reads, approval) lands, and no on_demand policy reaches
// any reviewer request. The requests' layout, counts and bytes belong to kogen-review and are
// not asserted here.

// revodGatePartsRequests runs a long parts-mode review (pinned parts, reads and approval) and
// returns every reviewer request body as the provider received it.
func revodGatePartsRequests(t *testing.T) []string {
	t.Helper()
	e := newEnv(t)
	f := e.prov
	f.then(f.calls("write_file", map[string]any{"path": "app.txt", "content": bigFile()}, "finish", map[string]any{"summary": "done"}))
	reader := confirmingReviewer(t, e, f, nil)
	for i := 0; i < 40; i++ {
		f.then(reader)
	}
	start := e.head()
	a := e.reviewed()
	if a.exit != 0 || e.head() == start {
		t.Fatalf("not landed: %d %s %s", a.exit, a.shown(), a.errs)
	}
	var bodies []string
	for i, raw := range f.rawRequests() {
		if requestInstructions(f.requests()[i]) == review.ReviewerInstructions() {
			bodies = append(bodies, string(raw))
		}
	}
	return bodies
}

// TestKRevodGatePartsUnchanged: the long parts-mode review lands and no reviewer request
// carries the on_demand policy.
func TestKRevodGatePartsUnchanged(t *testing.T) {
	t.Parallel()
	for i, body := range revodGatePartsRequests(t) {
		for _, s := range []string{"required_parts", "required_unread", "on_demand", "hunk-covering", "covering changed hunks"} {
			if strings.Contains(body, s) {
				t.Errorf("parts-mode review request %d carries %q", i+1, s)
			}
		}
	}
	t.Run("empty-source controls", revodGateEmptySourceControls)
}
