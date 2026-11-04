package consumes

import (
	"os"
	"strings"
	"testing"
)

// TestMain isolates the suite from its caller: no inherited GIT_* or KOGEN_* variable, no global
// or system Git configuration, no terminal prompt, and a HOME of the suite's own. Every Git
// command and every producer call that takes an environment inherits exactly this.
func TestMain(m *testing.M) {
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(k, "GIT_") || strings.HasPrefix(k, "KOGEN_") {
			os.Unsetenv(k)
		}
	}
	home, err := os.MkdirTemp("", "kogen-build-consumes-review-home-")
	if err != nil {
		panic(err)
	}
	os.Setenv("HOME", home)
	os.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	os.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	os.Setenv("GIT_TERMINAL_PROMPT", "0")
	code := m.Run()
	os.RemoveAll(home)
	os.Exit(code)
}
