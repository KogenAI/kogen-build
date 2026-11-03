package build

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/KogenAI/kogen-executor/core"
	agent "kogen-loop/tools"
)

// The read ledger is the continuation a native conversation is re-opened with after a
// working-window reset when Deps.ReadLedger is on: instead of the newest exchange it carries
// which files were read (path, range, hash and a short note), and the model re-reads on demand.
// It follows kogen-loop's L-LEDGER continuation message; the pinned kogen-loop no longer has it.
const (
	// readLedgerType is the "type" of the ledger message's JSON text.
	readLedgerType = "read_ledger"
	// readLedgerCap is the most UTF-8 bytes the ledger message's content may have.
	readLedgerCap = 8192
	// readLedgerNote is the most Unicode scalars of an entry's note.
	readLedgerNote = 80
	// resetMarker opens the content of a working-window reset message.
	resetMarker = "Local working-window reset\n"
)

// ledgerEntry is one read_file read: the path as the model spelled it, the numbered lines it
// returned, the SHA-256 of the output and the first meaningful line.
type ledgerEntry struct {
	Tool      string `json:"tool"`
	Path      string `json:"path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	SHA256    string `json:"sha256"`
	Note      string `json:"note"`
}

// readLedger is the ledger message: entries oldest first and how many older ones were dropped.
type readLedger struct {
	Type    string        `json:"type"`
	Dropped int           `json:"dropped"`
	Entries []ledgerEntry `json:"entries"`
}

// ledgerOf is the ledger a message carries; false for any other item.
func ledgerOf(item any) (readLedger, bool) {
	m, _ := item.(map[string]any)
	text, ok := m["content"].(string)
	if !ok || m["role"] != "user" || !strings.HasPrefix(text, `{`) {
		return readLedger{}, false
	}
	var l readLedger
	if jsonUnmarshal([]byte(text), &l) != nil || l.Type != readLedgerType {
		return readLedger{}, false
	}
	return l, true
}

// ledgerEntryOf is the entry of a read_file call whose output is the given output; false when
// the output is not a delivered read.
func ledgerEntryOf(path, output string) (ledgerEntry, bool) {
	for _, refused := range []string{"error:", "refused:", "not delivered:"} {
		if strings.HasPrefix(output, refused) {
			return ledgerEntry{}, false
		}
	}
	e := ledgerEntry{Tool: "read_file", Path: path, SHA256: core.HashHex([]byte(output))}
	first := true
	for _, line := range strings.Split(output, "\n") {
		number, text, ok := strings.Cut(line, "\t")
		if !ok || number == "" || strings.Trim(number, "0123456789") != "" {
			continue
		}
		n, err := strconv.Atoi(number)
		if err != nil {
			continue
		}
		if first {
			e.StartLine, first = n, false
		}
		e.EndLine = n
		if e.Note == "" {
			e.Note = noteOf(text)
		}
	}
	return e, true
}

// noteOf is the text trimmed of white space and cut to its first readLedgerNote scalars.
func noteOf(text string) string {
	text = strings.TrimSpace(text)
	for i, count := 0, 0; i < len(text); count++ {
		if count == readLedgerNote {
			return text[:i]
		}
		_, size := utf8.DecodeRuneInString(text[i:])
		i += size
	}
	return text
}

// readLedgerMessage builds the ledger of the input a reset archives, in input order: the
// entries of a ledger message in it at its position, and one entry per read_file call with a
// delivered output. The oldest entries are dropped until the content fits readLedgerCap. It
// returns the message and the ledger it carries.
func readLedgerMessage(input []any) (map[string]any, readLedger, error) {
	ledger := readLedger{Type: readLedgerType, Entries: []ledgerEntry{}}
	for i, item := range input {
		if carried, ok := ledgerOf(item); ok {
			ledger.Dropped = carried.Dropped
			ledger.Entries = append(ledger.Entries, carried.Entries...)
			continue
		}
		call, _ := item.(map[string]any)
		if call["type"] != "function_call" || call["name"] != "read_file" {
			continue
		}
		arguments, _ := call["arguments"].(string)
		path, ok := agent.DecodeArguments(arguments)["path"].(string)
		if !ok {
			continue
		}
		for _, later := range input[i+1:] {
			answer, _ := later.(map[string]any)
			if answer["type"] != "function_call_output" || answer["call_id"] != call["call_id"] {
				continue
			}
			if output, ok := answer["output"].(string); ok {
				if entry, ok := ledgerEntryOf(path, output); ok {
					ledger.Entries = append(ledger.Entries, entry)
				}
			}
			break
		}
	}
	for {
		raw, err := core.EncodeJSON(ledger)
		if err != nil {
			return nil, ledger, err
		}
		if len(raw) <= readLedgerCap || len(ledger.Entries) == 0 {
			return map[string]any{"role": "user", "content": string(raw)}, ledger, nil
		}
		ledger.Entries = ledger.Entries[1:]
		ledger.Dropped++
	}
}

// resetCarriesWindow reports whether the input carries a working-window reset message.
func resetCarriesWindow(input []any) bool {
	for _, item := range input {
		m, _ := item.(map[string]any)
		if text, ok := m["content"].(string); ok && m["role"] == "user" && strings.HasPrefix(text, resetMarker) {
			return true
		}
	}
	return false
}

// contentBytes is the UTF-8 bytes of the content of the items: string content, or the text
// blocks of block content.
func contentBytes(items []any) int {
	total := 0
	for _, text := range mandatoryTexts(items) {
		total += len(text)
	}
	return total
}
