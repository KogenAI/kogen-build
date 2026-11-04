package consumes

import (
	"reflect"
	"testing"

	"github.com/KogenAI/kogen-executor/thinking"
)

func TestThinkingStrip(t *testing.T) {
	var _ func([]any) []any = thinking.Strip
	text := map[string]any{"type": "text", "text": "answer"}
	use := map[string]any{"type": "tool_use", "id": "t1", "name": "shell"}
	mixed := map[string]any{"role": "assistant", "id": "m1", "anthropic_message": map[string]any{
		"role": "assistant",
		"content": []any{
			map[string]any{"type": "thinking", "thinking": "hmm", "signature": "sig"},
			text,
			map[string]any{"type": "redacted_thinking", "data": "xx"},
			use,
		},
	}}
	onlyThinking := map[string]any{"role": "assistant", "content": "plain", "anthropic_message": map[string]any{
		"content": []any{map[string]any{"type": "thinking", "thinking": "x", "signature": "s"}},
	}}
	untouched := []any{
		map[string]any{"role": "user", "content": "hello"},
		map[string]any{"type": "function_call", "name": "shell"},
		"not a map",
		nil,
	}
	in := append(append([]any{}, untouched[:2]...), mixed, onlyThinking, untouched[2], untouched[3])
	before := reflect.ValueOf(mixed["anthropic_message"]).Len()

	out := thinking.Strip(in)
	if len(out) != len(in) {
		t.Fatalf("thinking.Strip: got %d items, want %d", len(out), len(in))
	}
	// Other items stay as they are.
	for i, j := range []int{0, 1, 4, 5} {
		if !reflect.DeepEqual(out[j], in[j]) {
			t.Errorf("thinking.Strip: item %d (%d of the untouched) changed: %v", j, i, out[j])
		}
	}
	// Thinking and redacted_thinking blocks go; the other blocks and the item's other keys stay.
	wantMixed := map[string]any{"role": "assistant", "id": "m1", "anthropic_message": map[string]any{
		"role": "assistant", "content": []any{text, use},
	}}
	if !reflect.DeepEqual(out[2], wantMixed) {
		t.Errorf("thinking.Strip: message with thinking = %v, want %v", out[2], wantMixed)
	}
	// A message left without blocks loses anthropic_message and renders from its plain content.
	wantPlain := map[string]any{"role": "assistant", "content": "plain"}
	if !reflect.DeepEqual(out[3], wantPlain) {
		t.Errorf("thinking.Strip: message of only thinking = %v, want %v", out[3], wantPlain)
	}
	// The items passed in are not mutated.
	if got := mixed["anthropic_message"].(map[string]any)["content"].([]any); len(got) != 4 || reflect.ValueOf(mixed["anthropic_message"]).Len() != before {
		t.Errorf("thinking.Strip: must not mutate its input, message content now %v", got)
	}
	if _, ok := onlyThinking["anthropic_message"]; !ok {
		t.Errorf("thinking.Strip: must not mutate its input, the message lost anthropic_message")
	}
	if got := thinking.Strip(nil); len(got) != 0 {
		t.Errorf("thinking.Strip: no items must give none, got %v", got)
	}
}
