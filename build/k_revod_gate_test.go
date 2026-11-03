package build

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KogenAI/kogen-review/review"
)

// revodGateEmptySourceControls are 60 clean control cells whose source fits inline (no pinned
// source): an approval with no reads is a valid verdict and no source custody exists. One real
// Build of such a candidate lands without any read.
func revodGateEmptySourceControls(t *testing.T) {
	b, _, tree := materialAttempt(t)
	for cell := 0; cell < 60; cell++ {
		b.Review, b.WorkingContext = cell+1, map[string]*OwnerContext{}
		source := fmt.Sprintf("# Check results of the final pass\n\nacceptance: passed (%d scenarios)\n\n", cell%7) +
			strings.Repeat(fmt.Sprintf("--- cell%02d.txt\nclean control %d\n", cell, cell), 1+cell*37)
		input, err := b.materialPrepare(tree, testMaterial(source), materialRole(), review.ReviewerInstructions(), nil, nil, false)
		if err != nil {
			t.Fatal(err)
		}
		if b.WorkingContext["review"].InlineMaterial == nil || len(b.WorkingContext["review"].History) != 0 {
			t.Fatalf("cell %d pinned its source", cell)
		}
		if verdict, problems, err := validateReview(b, map[string]any{"verdict": "approve", "findings": []any{}}, input); err != nil || verdict == nil {
			t.Fatalf("cell %d approval refused: %v %v", cell, problems, err)
		}
	}
	if _, err := os.Stat(filepath.Join(b.Run.Dir, "review-source")); !os.IsNotExist(err) {
		t.Fatal("a control cell created source custody")
	}

	e := newEnv(t)
	f := e.prov
	f.then(f.calls("write_file", map[string]any{"path": "app.txt", "content": "small clean change\n"}, "finish", map[string]any{"summary": "done"}))
	f.then(submit(f, "approve"))
	start := e.head()
	if a := e.reviewed(); a.exit != 0 || e.head() == start {
		t.Fatalf("control not landed: %d %s", a.exit, a.shown())
	}
	events := e.events(e.onlyRun())
	if len(eventsNamed(events, "review_source_evidence")) != 0 || len(eventsNamed(events, "review_material")) != 0 {
		t.Fatal("the inline control pinned source")
	}
	eq(t, "review requests", len(f.reviewRequests()), 1)
}
