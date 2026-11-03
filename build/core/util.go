package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/KogenAI/kogen-executor/agent"
	"kogen-loop/sys"
	"kogen-loop/tools"
)

// NonNil is s, or an empty list for nil (a JSON array, never null).
func NonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// HashHex is the SHA-256 of b in hex.
func HashHex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// DecodeBase64 decodes standard base64.
func DecodeBase64(s string) ([]byte, error) { return base64.StdEncoding.DecodeString(s) }

// ResumeRefused is the stop of a resume that is refused for detail.
func ResumeRefused(detail string) *sys.Error {
	return sys.Fail("candidate", "resume_refused", detail, 1)
}

// DecodeJSONNumbers decodes one JSON value keeping numbers as json.Number.
func DecodeJSONNumbers(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, fmt.Errorf("trailing data")
	}
	return v, nil
}

// Display is a JSON scalar as text, the way string interpolation shows it (nil is empty).
func Display(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case json.Number:
		return x.String()
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'g', -1, 64)
	}
	return fmt.Sprint(v)
}

// InspectString renders a string the way Elixir's inspect/1 does.
func InspectString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	rs := []rune(s)
	for i, r := range rs {
		switch {
		case r == '"':
			b.WriteString(`\"`)
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == 0x1b:
			b.WriteString(`\e`)
		case r == 7:
			b.WriteString(`\a`)
		case r == '\b':
			b.WriteString(`\b`)
		case r == '\f':
			b.WriteString(`\f`)
		case r == '\v':
			b.WriteString(`\v`)
		case r == 0:
			b.WriteString(`\0`)
		case r == '#' && i+1 < len(rs) && rs[i+1] == '{':
			b.WriteString(`\#`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\x%02X`, r)
		case !unicode.IsPrint(r):
			fmt.Fprintf(&b, `\u%04X`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// MaterialText is the text of a text block of a message, if it is one.
func MaterialText(block any) (string, bool) {
	m, ok := block.(map[string]any)
	if !ok {
		return "", false
	}
	text, ok := m["text"].(string)
	return text, ok
}

// DiagnosticText is the text of a diagnostic message (a plain string or a list of text blocks).
func DiagnosticText(item any) string {
	m, _ := item.(map[string]any)
	switch c := m["content"].(type) {
	case string:
		return c
	case []any:
		var parts []string
		for _, block := range c {
			if t, ok := MaterialText(block); ok {
				parts = append(parts, t)
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

// ShellDefinitions are the definitions of the named tools, the shell's with the executing
// tool's own policy.
func ShellDefinitions(t *tools.Tools, names []string) []any {
	defs := agent.Defs(names...)
	for i, name := range names {
		if name == "shell" {
			defs[i], _ = t.Definition(name)
		}
	}
	return agent.AsAny(defs)
}
