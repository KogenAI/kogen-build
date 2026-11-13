package build

import (
	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-executor/core"
	"github.com/KogenAI/kogen-loop/provider"
)

// providerRole is a configured role as the provider needs it.
func providerRole(r project.Role) provider.Role {
	return provider.Role{Provider: r.Provider, Model: r.Model, Effort: r.Effort, BaseURL: r.BaseURL}
}

// responseTokens is what one response adds to the totals. Anthropic reports no reasoning
// count and may omit any other, so its totals come from the validated analytical usage; the
// other routes' compatible totals are always integers.
func responseTokens(role project.Role, resp *provider.Response) core.Tokens {
	if role.Provider == "anthropic" {
		k := resp.Known
		return core.Tokens{Input: k.Input, Cached: k.Cached, Output: k.Output, Reasoning: k.Reasoning}
	}
	u := resp.Usage
	return core.Tokens{Input: core.Count(u.Input), Cached: core.Count(u.Cached), Output: core.Count(u.Output), Reasoning: core.Count(u.Reasoning)}
}
