package build

import (
	"testing"

	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-loop/provider"
)

func TestProviderRoleKeepsTheProviderFields(t *testing.T) {
	t.Parallel()
	got := providerRole(project.Role{Provider: "anthropic", Model: "m", Effort: "high", BaseURL: "http://127.0.0.1:1", Executor: "claude-code"})
	if want := (provider.Role{Provider: "anthropic", Model: "m", Effort: "high", BaseURL: "http://127.0.0.1:1"}); got != want {
		t.Errorf("providerRole = %+v, want %+v", got, want)
	}
}
