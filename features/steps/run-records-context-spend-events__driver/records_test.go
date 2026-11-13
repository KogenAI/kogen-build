//go:build kogen_run_records_acceptance

package build

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/KogenAI/kogen-executor/core"
	"github.com/KogenAI/kogen-loop/tools"
)

func rrExpected(t *testing.T) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal([]byte(os.Getenv("RR_EXPECT")), &v); err != nil {
		t.Fatalf("%s: invalid feature expectations: %v", t.Name(), err)
	}
	return v
}

func rrResponse(id string, items ...map[string]any) handler {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for i, it := range items {
			fmt.Fprintf(w, "data: %s\n\n", jsonText(map[string]any{"type": "response.output_item.done", "output_index": i, "item": it}))
		}
		fmt.Fprintf(w, "data: %s\n\n", jsonText(map[string]any{"type": "response.completed", "response": map[string]any{"id": id, "usage": map[string]any{"input_tokens": 1, "output_tokens": 1}}}))
	}
}
func rrFixture(t *testing.T) (*env, []map[string]any) {
	t.Helper()
	e := newEnv(t)
	e.prov.then(rrResponse("rr-response-1", e.prov.call("read_file", map[string]any{"path": "app.txt"}), e.prov.call("read_file", map[string]any{"path": "app.txt"})))
	e.prov.then(rrResponse("rr-response-2", e.prov.call("write_file", map[string]any{"path": "app.txt", "content": "candidate\n"}), e.prov.call("finish", map[string]any{"summary": "done"})))
	a := e.build()
	if a.exit != 0 {
		t.Fatalf("native scripted Build exit %d: %s %s", a.exit, a.shown(), a.errs)
	}
	return e, e.events(e.onlyRun())
}
func rrBlob(t *testing.T, dir string, ref any) []byte {
	t.Helper()
	m, ok := ref.(map[string]any)
	if !ok {
		t.Fatalf("missing blob reference: %v", ref)
	}
	hash, _ := m["sha256"].(string)
	if len(hash) != 64 {
		t.Fatalf("invalid SHA-256 %q", hash)
	}
	data, err := os.ReadFile(filepath.Join(dir, "blobs", hash))
	if err != nil {
		t.Fatalf("blob %s: %v", hash, err)
	}
	if core.HashHex(data) != hash || m["bytes"] != float64(len(data)) {
		t.Fatalf("blob digest/length mismatch: %v", m)
	}
	st, err := os.Stat(filepath.Join(dir, "blobs", hash))
	if err != nil || st.Mode().Perm() != 0600 {
		t.Fatalf("blob permissions: %v %v", st, err)
	}
	return data
}
func rrItems(t *testing.T, dir string, ref any, want []any) {
	t.Helper()
	refs, ok := ref.([]any)
	if !ok || len(refs) != len(want) {
		t.Fatalf("ordered item references: got %v want %d", ref, len(want))
	}
	for i, it := range want {
		var got any
		if err := json.Unmarshal(rrBlob(t, dir, refs[i]), &got); err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(it)
		var normalized any
		json.Unmarshal(raw, &normalized)
		if !reflect.DeepEqual(got, normalized) {
			t.Fatalf("item %d content mismatch", i)
		}
	}
}
func TestKogenRunRecordsCustody(t *testing.T) {
	e, events := rrFixture(t)
	expected := rrExpected(t)
	dir := filepath.Join(e.root, ".kogen/local/runs", e.onlyRun())
	reqs := eventsNamed(events, "request")
	wire := e.prov.requests()
	if float64(len(reqs)) != expected["requests"] || len(wire) != len(reqs) {
		t.Fatalf("scripted request count: %d %d", len(reqs), len(wire))
	}
	for i, ev := range reqs {
		rrItems(t, dir, ev["items"], wire[i]["input"].([]any))
	}
	calls := eventsNamed(events, "tool_call")
	if float64(len(calls)) != expected["calls"] {
		t.Fatalf("tool count: %d", len(calls))
	}
	for _, ev := range calls {
		args := rrBlob(t, dir, ev["arguments"])
		var decoded map[string]any
		if json.Unmarshal(args, &decoded) != nil {
			t.Fatalf("arguments not original JSON: %s", args)
		}
		result := rrBlob(t, dir, ev["result"])
		if ev["name"] == "read_file" && strings.TrimSuffix(string(result), "\n") != expected["read_result"] {
			t.Fatalf("read result lost: %s", result)
		}
	}
	if !reflect.DeepEqual(calls[0]["arguments"], calls[1]["arguments"]) || !reflect.DeepEqual(calls[0]["result"], calls[1]["result"]) {
		t.Fatal("identical tool content did not deduplicate")
	}
	entries, err := os.ReadDir(filepath.Join(dir, "blobs"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(dir, "blobs", entry.Name()))
		if err != nil || entry.Name() != core.HashHex(data) {
			t.Fatalf("non-addressed or duplicate blob %s", entry.Name())
		}
	}
	st, _ := os.Stat(filepath.Join(dir, "blobs"))
	if st.Mode().Perm() != 0700 {
		t.Fatal("blob directory not private")
	}
	// Authentication belongs to transport, not to request item content or persisted records.
	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, secret := range []string{"fake-access-token", "fake-refresh-token", "Authorization: Bearer"} {
			if strings.Contains(string(data), secret) {
				return fmt.Errorf("credential leaked into %s", path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
func TestKogenRunRecordsIdentity(t *testing.T) {
	e, events := rrFixture(t)
	expected := rrExpected(t)
	dir := filepath.Join(e.root, ".kogen/local/runs", e.onlyRun())
	reqs := eventsNamed(events, "request")
	calls := eventsNamed(events, "tool_call")
	seen := map[string]bool{}
	for i, ev := range reqs {
		if ev["response_id"] != expected["response_ids"].([]any)[i] {
			t.Fatalf("response identity: %v", ev["response_id"])
		}
		refs, ok := ev["response_items"].([]any)
		if !ok || len(refs) != 2 {
			t.Fatalf("response item references: %v", ev["response_items"])
		}
		for _, ref := range refs {
			var item map[string]any
			json.Unmarshal(rrBlob(t, dir, ref), &item)
			id, _ := item["call_id"].(string)
			if id == "" || item["id"] == "" || seen[id] {
				t.Fatalf("response call identity: %v", item)
			}
			seen[id] = true
			found := false
			for _, call := range calls {
				if call["call_id"] == id {
					found = true
					if call["name"] != item["name"] || string(rrBlob(t, dir, call["arguments"])) != item["arguments"] {
						t.Fatal("tool call linkage mismatch")
					}
				}
			}
			if !found {
				t.Fatalf("no executed call for %s", id)
			}
		}
	}
	if len(seen) != len(calls) {
		t.Fatal("unlinked tool calls")
	}
	// The following request must carry the first response's calls and outputs with their IDs.
	input := e.prov.requests()[1]["input"].([]any)
	for _, call := range calls[:2] {
		n := 0
		for _, v := range input {
			m := v.(map[string]any)
			if m["call_id"] == call["call_id"] {
				n++
			}
		}
		if n != 2 {
			t.Fatalf("request replay linkage %v count %d", call["call_id"], n)
		}
	}
}
func rrKinds(items []any) map[string]any {
	out := map[string]any{}
	for _, v := range items {
		m := v.(map[string]any)
		kind, _ := m["type"].(string)
		if kind == "" {
			kind, _ = m["role"].(string)
		}
		raw, _ := core.EncodeJSON(v)
		n, _ := out[kind].(float64)
		out[kind] = n + float64(len(raw))
	}
	return out
}
func TestKogenRunRecordsAttribution(t *testing.T) {
	// Exercise actual native scripted-provider requests as well as exact local attribution below.
	expected := rrExpected(t)
	for _, refusal := range []bool{false, true} {
		e := newEnv(t)
		count, size := 3, 240000
		if refusal {
			count, size = 1, 600000
		}
		for i := 0; i < count; i++ {
			e.prov.then(e.prov.calls("shell", map[string]any{"command": "printf tiny", "padding": strings.Repeat("p", size)}))
		}
		e.prov.then(e.prov.calls("write_file", map[string]any{"path": "app.txt", "content": "candidate\n"}, "finish", map[string]any{"summary": "done"}))
		before := e.head()
		a := e.build()
		name := "window_reset"
		if refusal {
			name = "context_budget_exceeded"
			if a.exit == 0 || e.head() != before {
				t.Fatal("native refusal must stop without committing")
			}
		} else if a.exit != 0 {
			t.Fatalf("native reset fixture exit %d: %s", a.exit, a.errs)
		}
		found := eventsNamed(e.events(e.onlyRun()), name)
		if len(found) != 1 {
			t.Fatalf("native %s count %d", name, len(found))
		}
		data, ok := found[0]["breakdown"].(map[string]any)
		if !ok || data["before_bytes"].(float64) <= expected["ceiling_bytes"].(float64) {
			t.Fatalf("native %s missing overflow attribution: %v", name, found)
		}
	}

	for _, refusal := range []bool{false, true} {
		t.Run(fmt.Sprint(refusal), func(t *testing.T) {
			b, e, _ := windowAttempt(t)
			opening := map[string]any{"role": "user", "content": "mandatory authority"}
			input := []any{opening, map[string]any{"type": "message", "role": "assistant", "content": "earlier observation"}, map[string]any{"type": "function_call", "call_id": "old", "name": "read_file", "arguments": "{}"}, map[string]any{"type": "function_call_output", "call_id": "old", "output": strings.Repeat("x", 600000)}, map[string]any{"type": "function_call", "call_id": "new", "name": "read_file", "arguments": "{}"}, map[string]any{"type": "function_call_output", "call_id": "new", "output": "new output"}}
			if refusal {
				opening["content"] = strings.Repeat("authority", 70000)
			}
			role := defaultRole()
			before, err := tools.Measure(b.bodyOf(role, "policy", input, nil, "build"), providerRole(role))
			if err != nil {
				t.Fatal(err)
			}
			reduced, err := b.prepareContext("build", role, "policy", input, nil)
			eventName := "window_reset"
			if refusal {
				eventName = "context_budget_exceeded"
				if err == nil || !strings.Contains(err.Error(), "context_budget_exceeded") || !strings.Contains(err.Error(), fmt.Sprintf("ceiling %.0f, output reserve %.0f tokens", expected["ceiling_bytes"], expected["output_reserve_tokens"])) {
					t.Fatalf("expected budget refusal: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			events := eventsNamed(e.events(b.Run.ID), eventName)
			if len(events) != 1 {
				t.Fatalf("%s count %d", eventName, len(events))
			}
			breakdown, ok := events[0]["breakdown"].(map[string]any)
			if !ok {
				t.Fatalf("%s missing breakdown", eventName)
			}
			if breakdown["before_bytes"] != float64(before.RenderedBytes) {
				t.Fatalf("%s before bytes: %v", eventName, breakdown)
			}
			after, ok := breakdown["after_bytes"].(float64)
			if !ok || after <= 0 {
				t.Fatalf("%s after bytes missing", eventName)
			}
			summary, ok := breakdown["summary_bytes"].(float64)
			if !ok || summary <= 0 {
				t.Fatalf("%s summary bytes missing", eventName)
			}
			if !refusal {
				measured, _ := tools.Measure(b.bodyOf(role, "policy", reduced, nil, "build"), providerRole(role))
				if after != float64(measured.RenderedBytes) || after >= float64(before.RenderedBytes) || after > 524288 {
					t.Fatalf("reset measured bytes: %v", breakdown)
				}
				if summary != float64(len(reduced[1].(map[string]any)["content"].(string))) {
					t.Fatal("summary bytes mismatch")
				}
				if !reflect.DeepEqual(breakdown["mandatory_bytes_by_kind"], rrKinds(reduced[:2])) || !reflect.DeepEqual(breakdown["optional_bytes_by_kind"], rrKinds(reduced[2:])) {
					t.Fatalf("item kind attribution mismatch: %v", breakdown)
				}
			} else {
				mandatory, ok := breakdown["mandatory_bytes_by_kind"].(map[string]any)
				if !ok || mandatory["user"].(float64) < 630000 {
					t.Fatalf("refusal mandatory attribution: %v", breakdown)
				}
				if !reflect.DeepEqual(breakdown["optional_bytes_by_kind"], rrKinds(input[4:])) {
					t.Fatalf("refusal optional attribution: %v", breakdown)
				}
				if _, err := os.Stat(filepath.Join(b.Run.Dir, "working-context")); !os.IsNotExist(err) {
					t.Fatal("refusal published archive")
				}
			}
		})
	}
}
