package build

import (
	"reflect"
	"testing"

	"github.com/KogenAI/kogen-review/review"
)

func TestForcedBodiesUnreadReminders(t *testing.T) {
	bodies := [][]byte{
		[]byte(`{"tool_choice":"auto"}`),
		[]byte(`{"tool_choice":{"type":"function","name":"submit_review"},"marker":1}`),
		[]byte(`{"tool_choice":{"type":"function","name":"read_file"},"marker":2}`),
	}
	f := &fakeProvider{bodies: bodies}
	for range bodies {
		f.reqs = append(f.reqs, map[string]any{"instructions": review.ReviewerInstructions()})
	}
	indexes, got := forcedBodies(t, f)
	if !reflect.DeepEqual(indexes, []int{1, 2}) || !reflect.DeepEqual(got, bodies[1:]) {
		t.Fatalf("forcedBodies = %v, %q; want indexes [1 2] and original bodies %q", indexes, got, bodies[1:])
	}
}
