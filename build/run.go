// Package build is `kogen intent build` and `kogen intent resume`: the Build lock, approval
// and baseline custody, the builder conversation, verification, the review hook, the verified
// commit, run records (`.kogen/local/runs/<id>`), local facts and checkpoints (chapters
// 03-07, 09).
package build

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"syscall"
	"unicode/utf8"
)

const logLimit = 1 << 20

// Cap is command output as recorded in the run log: at most 1 MiB with the middle cut out.
func Cap(text string) string {
	if len(text) <= logLimit {
		return text
	}
	const marker = "\n[... cut in the run log ...]\n"
	size := (logLimit - len(marker)) / 2
	return validPrefix(text[:size]) + marker + validSuffix(text[len(text)-size:])
}

func validPrefix(s string) string {
	for !validUTF8(s) {
		s = s[:len(s)-1]
	}
	return s
}

func validSuffix(s string) string {
	for !validUTF8(s) {
		s = s[1:]
	}
	return s
}

// fileMode is the permission bits of a file, or false when it cannot be inspected.
func fileMode(path string) (fs.FileMode, fs.FileMode, bool) {
	st, err := os.Lstat(path)
	if err != nil {
		return 0, 0, false
	}
	return st.Mode().Type(), st.Mode().Perm(), true
}

// fileIdentity is `root:device:inode`, the identity of a checkout's folder.
func fileIdentity(root string) (string, error) {
	st, err := os.Stat(root)
	if err != nil {
		return "", err
	}
	sys, ok := st.Sys().(*syscall.Stat_t)
	if !ok {
		return "", errors.New("no device identity")
	}
	return fmt.Sprintf("%s:%d:%d", root, uint64(sys.Dev), uint64(sys.Ino)), nil
}

func validUTF8(s string) bool { return utf8.ValidString(s) }
