package build

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/KogenAI/kogen-build/build/core"
)

func itoa(n int) string { return strconv.Itoa(n) }

func readFile(root, rel string) ([]byte, error) { return os.ReadFile(filepath.Join(root, rel)) }

func encodeBase64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

func jsonUnmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }

// shellQuote quotes s for /bin/sh.
func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// short is the first 8 characters of a hash.
func short(s string) string { return s[:min(8, len(s))] }

// fileDigest is the SHA-256 of a file, or "missing".
func fileDigest(root, rel string) string {
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return "missing"
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// inspectStrings renders a list of strings the way Elixir's inspect/1 does.
func inspectStrings(list []string) string {
	parts := make([]string, len(list))
	for i, s := range list {
		parts[i] = core.InspectString(s)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
