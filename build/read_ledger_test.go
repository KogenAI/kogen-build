package build

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/KogenAI/kogen-executor/core"
)

// With Deps.ReadLedger on, a working-window reset re-opens the conversation with the mandatory
// opening, the reset message and one read_ledger message; the newest exchange is not carried.

func ledgerRead(id, path, output string) []any {
	return []any{
		map[string]any{"type": "function_call", "call_id": id, "name": "read_file", "arguments": jsonText(map[string]any{"path": path})},
		map[string]any{"type": "function_call_output", "call_id": id, "output": output},
	}
}

func ledgerAssistant() any {
	return map[string]any{"type": "message", "role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": "working"}}}
}

func ledgerMessageOf(t *testing.T, doc readLedger) any {
	t.Helper()
	raw, err := core.EncodeJSON(doc)
	if err != nil {
		t.Fatal(err)
	}
	return map[string]any{"role": "user", "content": string(raw)}
}

// ledgerReset runs one builder reset of input with the option on and returns the request it
// re-opens with: the mandatory opening, the reset message and the ledger.
func ledgerReset(t *testing.T, b *Attempt, input []any) ([]any, readLedger) {
	t.Helper()
	b.deps.ReadLedger = true
	got, err := b.prepareContext("build", defaultRole(), "policy", input, nil)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "opening, reset and ledger only", len(got), 3)
	eq(t, "the reset is second", strings.HasPrefix(got[1].(map[string]any)["content"].(string), resetMarker), true)
	ledger, ok := ledgerOf(got[2])
	eq(t, "the last message is the ledger", ok, true)
	return got, ledger
}

func TestReadLedgerEntriesFollowTheReadsOfTheArchivedInput(t *testing.T) {
	t.Parallel()
	b, _, _ := windowAttempt(t)
	long := strings.Repeat("Δ", 100)
	numbered := "7\t  \n8\t   first line   \n9\tsecond\nplain text line\n10\tlast\n"
	input := []any{map[string]any{"role": "user", "content": "authority"}, ledgerAssistant()}
	input = append(input, ledgerRead("a", "./dir/a b.txt", numbered)...)
	input = append(input, ledgerRead("b", "long.txt", "1\t"+long+"\n2\tx\n")...)
	input = append(input, ledgerRead("c", "plain.txt", "no numbered lines\n12 not a tab\n")...)
	input = append(input, ledgerRead("d", "e.txt", "error: no such file")...)
	input = append(input, ledgerRead("e", "r.txt", "refused: outside")...)
	input = append(input, ledgerRead("f", "n.txt", "not delivered: pinned review source part 3 did not fit")...)
	input = append(input, ledgerRead("g", "blank.txt", "3\t\n4\t \t\n")...)
	input = append(input, ledgerRead("h", "./dir/a b.txt", numbered)...)
	// Not reads: another tool, no path, an unanswered call, a non-text output.
	input = append(input,
		map[string]any{"type": "function_call", "call_id": "w", "name": "write_file", "arguments": jsonText(map[string]any{"path": "w.txt", "content": "x"})},
		map[string]any{"type": "function_call_output", "call_id": "w", "output": "1\tok"},
		map[string]any{"type": "function_call", "call_id": "n", "name": "read_file", "arguments": "{}"},
		map[string]any{"type": "function_call_output", "call_id": "n", "output": "1\tx"},
		map[string]any{"type": "function_call", "call_id": "u", "name": "read_file", "arguments": jsonText(map[string]any{"path": "u.txt"})},
		map[string]any{"type": "function_call", "call_id": "i", "name": "read_file", "arguments": jsonText(map[string]any{"path": "i.png"})},
		map[string]any{"type": "function_call_output", "call_id": "i", "output": []any{map[string]any{"type": "input_image"}}},
	)
	input = append(input, budgetPad("pad", 600_000)...)

	_, ledger := ledgerReset(t, b, input)
	sum := func(s string) string { return core.HashHex([]byte(s)) }
	want := []ledgerEntry{
		{Tool: "read_file", Path: "./dir/a b.txt", StartLine: 7, EndLine: 10, SHA256: sum(numbered), Note: "first line"},
		{Tool: "read_file", Path: "long.txt", StartLine: 1, EndLine: 2, SHA256: sum("1\t" + long + "\n2\tx\n"), Note: strings.Repeat("Δ", 80)},
		{Tool: "read_file", Path: "plain.txt", StartLine: 0, EndLine: 0, SHA256: sum("no numbered lines\n12 not a tab\n"), Note: ""},
		{Tool: "read_file", Path: "blank.txt", StartLine: 3, EndLine: 4, SHA256: sum("3\t\n4\t \t\n"), Note: ""},
		{Tool: "read_file", Path: "./dir/a b.txt", StartLine: 7, EndLine: 10, SHA256: sum(numbered), Note: "first line"},
	}
	eq(t, "entries", ledger.Entries, want)
	eq(t, "dropped", ledger.Dropped, 0)
	eq(t, "type", ledger.Type, readLedgerType)

	// The message is the compact JSON of exactly these keys.
	b2, _, _ := windowAttempt(t)
	got2, _ := ledgerReset(t, b2, input)
	var doc map[string]any
	if err := json.Unmarshal([]byte(got2[2].(map[string]any)["content"].(string)), &doc); err != nil {
		t.Fatal(err)
	}
	eq(t, "keys", len(doc), 3)
	eq(t, "entry keys", len(doc["entries"].([]any)[0].(map[string]any)), 6)
}

func TestReadLedgerEmptyHasAnEntriesArray(t *testing.T) {
	t.Parallel()
	b, _, _ := windowAttempt(t)
	input := append([]any{map[string]any{"role": "user", "content": "authority"}, ledgerAssistant()}, budgetPad("pad", 600_000)...)
	got, ledger := ledgerReset(t, b, input)
	eq(t, "entries", len(ledger.Entries), 0)
	eq(t, "content", got[2].(map[string]any)["content"], any(`{"type":"read_ledger","dropped":0,"entries":[]}`))
}

func TestReadLedgerSecondResetCarriesEarlierEntriesFirstAndDroppedIsCumulative(t *testing.T) {
	t.Parallel()
	b, _, _ := windowAttempt(t)
	input := []any{map[string]any{"role": "user", "content": "authority"}, ledgerAssistant()}
	input = append(input, ledgerRead("a", "first.txt", "1\talpha\n")...)
	input = append(input, budgetPad("pad-1", 600_000)...)
	got, first := ledgerReset(t, b, input)
	eq(t, "first reset", []string{first.Entries[0].Path}, []string{"first.txt"})

	// The re-opened conversation continues, reads again and outgrows the window again.
	reopened := append(slices.Clone(got), ledgerAssistant())
	reopened = append(reopened, ledgerRead("b", "second.txt", "5\tbeta\n")...)
	reopened = append(reopened, budgetPad("pad-2", 600_000)...)
	_, second := ledgerReset(t, b, reopened)
	paths := []string{}
	for _, e := range second.Entries {
		paths = append(paths, e.Path)
	}
	eq(t, "earlier entries first", paths, []string{"first.txt", "second.txt"})
	eq(t, "dropped", second.Dropped, 0)
	eq(t, "two archives", len(b.WorkingContext["build"].History), 2)
}

func TestReadLedgerCapEvictsTheOldestEntriesAndCountsThem(t *testing.T) {
	t.Parallel()
	// A carried ledger of exactly bound+delta bytes: the oldest entries go until it fits.
	for _, c := range []struct {
		name    string
		delta   int
		dropped int
	}{{"below", -1, 0}, {"at", 0, 0}, {"above", 1, 1}} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			carried := readLedger{Type: readLedgerType, Dropped: 5}
			for i := 0; i < 40; i++ {
				carried.Entries = append(carried.Entries, ledgerEntry{
					Tool: "read_file", Path: "dir/" + strings.Repeat("p", 200) + itoa(i+100), StartLine: 1, EndLine: 9,
					SHA256: core.HashHex([]byte(itoa(i))), Note: "n" + itoa(i+100),
				})
			}
			size := func() int {
				raw, _ := core.EncodeJSON(carried)
				return len(raw)
			}
			// Trim the oldest entries to just fit the bound, then pad the last path to the target.
			for size() > readLedgerCap {
				carried.Entries = carried.Entries[1:]
			}
			last := &carried.Entries[len(carried.Entries)-1]
			if readLedgerCap+c.delta < size() {
				t.Fatalf("carried ledger is already %d bytes", size())
			}
			last.Path += strings.Repeat("q", readLedgerCap+c.delta-size())
			eq(t, "carried bytes", size(), readLedgerCap+c.delta)
			kept := len(carried.Entries)

			b, _, _ := windowAttempt(t)
			input := []any{map[string]any{"role": "user", "content": "authority"}, ledgerAssistant(), ledgerMessageOf(t, carried)}
			input = append(input, budgetPad("pad", 600_000)...)
			got, ledger := ledgerReset(t, b, input)
			eq(t, "dropped", ledger.Dropped, 5+c.dropped)
			eq(t, "entries", len(ledger.Entries), kept-c.dropped)
			eq(t, "within the cap", len(got[2].(map[string]any)["content"].(string)) <= readLedgerCap, true)
			if c.dropped > 0 {
				eq(t, "the oldest went", ledger.Entries[0].Path, carried.Entries[1].Path)
			}
		})
	}
}

func TestReadLedgerReviewDefersUndeliveredPartReadsAndCarriesNothingElse(t *testing.T) {
	t.Parallel()
	s := budgetPinned(t, batchHeadroom)
	b, role := s.b, s.role
	b.deps.ReadLedger = true
	got1, err := b.prepareContext("review", role, "policy", s.input(t, 1_000), nil)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "first reset: the opening, the reset, the ledger", len(got1), len(s.opening)+2)
	first, _ := ledgerOf(got1[len(got1)-1])
	eq(t, "first ledger entries", len(first.Entries), budgetReads)
	unreadBefore, err := b.materialUnread(got1)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "unread before", len(unreadBefore), len(s.parts)-budgetReads)

	batch, full := s.batch(t, batchFrom, batchSize)
	input := slices.Concat(got1, batch)
	if budgetMeasure(t, b, role, input).Fits {
		t.Fatal("the input is not over budget")
	}
	got, err := b.prepareContext("review", role, "policy", input, nil)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "the opening is kept", reflect.DeepEqual(got[:len(s.opening)], s.opening), true)
	eq(t, "opening, reset and ledger only", len(got), len(s.opening)+2)
	ledger, ok := ledgerOf(got[len(got)-1])
	eq(t, "ledger", ok, true)
	// The earlier ledger's entries stay; the deferred reads were never delivered, so are no entries.
	eq(t, "entries", ledger.Entries, first.Entries)

	reset := handoffOf(t, got)
	var parts []any
	for i := 0; i < batchSize; i++ {
		parts = append(parts, float64(batchFrom+i+1))
	}
	deferred := reset["deferred_reads"].(map[string]any)
	eq(t, "deferred parts", deferred["parts"].([]any), parts)
	source := reset["review_delivery"].(map[string]any)["source"].(map[string]any)
	eq(t, "delivered", source["delivered"], any(float64(budgetReads)))
	for _, part := range parts {
		if !slices.Contains(source["unread"].([]any), part) {
			t.Fatalf("deferred part %v is not listed unread", part)
		}
	}

	// The archive holds the not-delivered notices, never the part texts; delivery stays as before.
	state := b.WorkingContext["review"]
	archive := state.History[len(state.History)-1]
	archiveOK(t, archive)
	data, err := os.ReadFile(archive.Path)
	if err != nil {
		t.Fatal(err)
	}
	for i, text := range full {
		if strings.Contains(string(data), jsonText(text)) {
			t.Fatalf("the archive holds the full text of deferred part %d", batchFrom+i+1)
		}
	}
	unread, err := b.materialUnread(got)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "unread after", unread, unreadBefore)
	for _, path := range batchPaths(s, 0, budgetReads) {
		if slices.Contains(unread, path) {
			t.Fatalf("delivered %s is unread", path)
		}
	}
	// The caller's input is never changed.
	eq(t, "caller output", batchOutputs(t, input, batchSize), full)
}

func TestReadLedgerReviewDeliveryNoteStatesThatNoPartsAreAppended(t *testing.T) {
	t.Parallel()
	s := prefillScenario(t)
	input := slices.Concat(s.opening, s.reads(t))
	const old = "Kogen also appends unread pinned parts as native source data messages; they count once a completed response follows them. "
	const now = "After a working-window reset Kogen appends no pinned parts to this review: read the unread parts with read_file; " +
		"the read_ledger message lists the reads this conversation made, and every pinned part can be read again. "
	const rest = "Read unread pinned parts in batches of about 10 read_file calls per response, not one per response; " +
		"use a smaller batch only after a native not-delivered notice shows a batch did not fit. " +
		"A not-delivered read is not counted and must be read again."

	off, err := s.b.reviewDelivery(input, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	contains(t, "off note", off["note"].(string), old+rest)
	s.b.deps.ReadLedger = true
	on, err := s.b.reviewDelivery(input, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	contains(t, "on note", on["note"].(string), now+rest)
	eq(t, "the old sentence is gone", strings.Contains(on["note"].(string), old), false)
	eq(t, "only the sentence differs", strings.Replace(on["note"].(string), now, old, 1), off["note"].(string))
}

func TestReadLedgerPrefillAppendsNothingAfterAReset(t *testing.T) {
	t.Parallel()
	s := prefillScenario(t)
	reset := map[string]any{"role": "user", "content": resetMarker + "{}"}
	input := slices.Concat(s.opening, s.reads(t))

	// Before a reset the prefill is as before, on or off.
	s.b.deps.ReadLedger = true
	out, appended := prefillNext(t, s, input)
	eq(t, "before a reset: appended", appended != nil, true)
	_ = out

	withReset := slices.Concat([]any{s.opening[0], reset}, s.opening[1:], s.reads(t))
	s.b.deps.ReadLedger = false
	_, appended = prefillNext(t, s, withReset)
	eq(t, "off: appended after a reset", appended != nil, true)
	s.b.deps.ReadLedger = true
	out, appended = prefillNext(t, s, withReset)
	eq(t, "on: nothing after a reset", appended == nil && len(out) == len(withReset), true)
}
