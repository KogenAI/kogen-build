package build

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// update is the one switch that makes the golden tests write their goldens: `go test ./... -update`.
// Without it every golden test compares its output with its golden and writes nothing. It is read
// by tests only; the kogen program and this library have no such flag, setting or variable.
var update = flag.Bool("update", false, "rewrite the golden files of the tests from the current output instead of comparing with them")

// goldenEncoding is how a golden JSON file is written: compact with sorted keys, no HTML
// escaping and no final newline (indent ""), or indented by indent with HTML escaping and a
// final newline.
type goldenEncoding struct{ indent string }

var (
	compactGolden = goldenEncoding{}
	indent1Golden = goldenEncoding{" "}
	indent2Golden = goldenEncoding{"  "}
)

func (g goldenEncoding) encode(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	if g.indent == "" {
		enc.SetEscapeHTML(false)
		if err := enc.Encode(v); err != nil {
			return nil, err
		}
		return bytes.TrimSuffix(b.Bytes(), []byte("\n")), nil
	}
	enc.SetIndent("", g.indent)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// decodeNumbers decodes JSON into any keeping numbers as written.
func decodeNumbers(t *testing.T, raw []byte) any {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

// asJSONValue converts a Go value to the generic JSON form of a golden (numbers kept as written).
func asJSONValue(t *testing.T, v any) any {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return decodeNumbers(t, raw)
}

// writeGoldenFile replaces the file whole: it writes a temporary file in the same folder and
// renames it, so no reader sees a partial file.
func writeGoldenFile(t *testing.T, path string, data []byte) {
	t.Helper()
	perm := os.FileMode(0o644)
	if st, err := os.Stat(path); err == nil {
		perm = st.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		t.Fatal(err)
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if err := os.Chmod(tmp.Name(), perm); err != nil && werr == nil {
		werr = err
	}
	if werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Rename(tmp.Name(), path)
	}
	if werr != nil {
		os.Remove(tmp.Name())
		t.Fatal(werr)
	}
}

// updateGoldenJSON reads the golden JSON file (numbers kept as written), lets edit change the
// values the test compares and writes it back in its encoding.
func updateGoldenJSON(t *testing.T, path string, enc goldenEncoding, edit func(doc any) any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out, err := enc.encode(edit(decodeNumbers(t, raw)))
	if err != nil {
		t.Fatal(err)
	}
	writeGoldenFile(t, path, out)
}

// updateGoldenAfter runs the update from a cleanup of t: after t and all of its parallel
// subtests have finished, and only when none of them failed.
func updateGoldenAfter(t *testing.T, write func()) {
	t.Helper()
	t.Cleanup(func() {
		if !t.Failed() {
			write()
		}
	})
}

// goldenObject is a decoded golden JSON object.
func goldenObject(t *testing.T, v any) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("golden value is %T, want an object", v)
	}
	return m
}

// goldenList is a decoded golden JSON array.
func goldenList(t *testing.T, v any) []any {
	t.Helper()
	l, ok := v.([]any)
	if !ok {
		t.Fatalf("golden value is %T, want an array", v)
	}
	return l
}
