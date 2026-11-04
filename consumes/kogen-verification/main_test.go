package consumes

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain gives every test of the suite one isolated process environment: no inherited GIT_*
// or KOGEN_* variable, a HOME of the suite's own, and no global or system Git configuration.
func TestMain(m *testing.M) {
	for _, kv := range os.Environ() {
		if k, _, _ := strings.Cut(kv, "="); strings.HasPrefix(k, "GIT_") || strings.HasPrefix(k, "KOGEN_") {
			os.Unsetenv(k)
		}
	}
	home, err := os.MkdirTemp("", "consumes-verification-home-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Setenv("HOME", home)
	os.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	os.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	os.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	os.Setenv("GIT_TERMINAL_PROMPT", "0")
	code := m.Run()
	os.RemoveAll(home)
	os.Exit(code)
}
