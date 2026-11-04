package consumes

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestMain isolates the suite from the caller: no inherited GIT_* or KOGEN_* variable, a HOME of
// its own, no global or system Git configuration and no Git prompt.
func TestMain(m *testing.M) {
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "GIT_") || strings.HasPrefix(name, "KOGEN_") {
			os.Unsetenv(name)
		}
	}
	home, err := os.MkdirTemp("", "consumes-executor-home-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	os.Setenv("HOME", home)
	os.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	os.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	os.Setenv("GIT_TERMINAL_PROMPT", "0")
	code := m.Run()
	os.RemoveAll(home)
	os.Exit(code)
}
