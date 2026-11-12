package build

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/KogenAI/kogen-executor/core"
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

// maxCustodyRead is the most bytes read from an approval record or an index copy.
const maxCustodyRead = 64 << 20

// readBounded reads a regular file of at most maxCustodyRead bytes through one open handle.
// The file is opened without blocking and its type checked on the handle, so a named pipe is
// refused instead of waited on. A refusal names the path and says whether the file is oversized,
// not a regular file or unreadable.
func readBounded(path string) ([]byte, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		var pe *fs.PathError
		if errors.As(err, &pe) {
			err = pe.Err
		}
		return nil, fmt.Errorf("%s is unreadable: %v", path, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("%s is unreadable: %v", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	data, err := io.ReadAll(io.LimitReader(f, maxCustodyRead+1))
	if err != nil {
		return nil, fmt.Errorf("%s is unreadable: %v", path, err)
	}
	if len(data) > maxCustodyRead {
		return nil, fmt.Errorf("%s is oversized (more than 64 MiB)", path)
	}
	return data, nil
}
