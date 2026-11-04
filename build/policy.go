package build

import (
	"embed"
	"fmt"

	"github.com/KogenAI/kogen-loop/tools"
)

// The builder's model-facing policy, owned here: its instructions (prompts/builder.md), its
// terminal tools and the structured submission an external builder finishes with. Every byte is
// the one the builders have always been given (testdata/prompts_golden.json,
// testdata/defs_golden.json, and SubmissionSchema the Claude Code adapter's former BuildSchema).

//go:embed prompts/*.md
var promptFiles embed.FS

// The builder's terminal tools.
const (
	FinishTool        = "finish"
	ReportBlockerTool = "report_blocker"
)

// SubmissionSchema is the JSON Schema of an external builder's structured final result, which
// the Build hands its executor as the task's schema (executor.Task.Schema): an optional summary
// and an optional blocker, the executor contract's builder-reported blocker claim
// (executor.BlockerClaim: paths, reason, evidence).
const SubmissionSchema = `{"type":"object","properties":{"summary":{"type":"string"},"blocker":{"type":"object","properties":{"paths":{"type":"array","items":{"type":"string"}},"reason":{"type":"string"},"evidence":{"type":"string"}},"required":["paths","reason","evidence"],"additionalProperties":false}},"additionalProperties":false}`

func promptFile(name string) string {
	b, err := promptFiles.ReadFile("prompts/" + name + ".md")
	if err != nil {
		panic(fmt.Sprintf("build: unknown prompt %q", name))
	}
	return string(b)
}

// BuilderInstructions are the native builder's instructions, byte for byte what the model
// receives.
func BuilderInstructions() string { return promptFile("builder") }

// BuilderToolDefinitions returns the model-facing definition of each named tool, in order: the
// generic file, search and command tools come from kogen-loop/tools, finish and report_blocker
// are the builder's own. A name that is neither panics: the names are fixed by the program.
func BuilderToolDefinitions(names ...string) []map[string]any {
	out := make([]map[string]any, len(names))
	for i, name := range names {
		out[i] = builderDefinition(name)
	}
	return out
}

func builderDefinition(name string) map[string]any {
	if d, ok := tools.Definition(name); ok {
		return d
	}
	switch name {
	case ReportBlockerTool:
		return toolFunction(name,
			"Stop with a BUILDER-REPORTED protected-baseline contradiction. Supply concrete current protected file paths, the contradiction, and observed command/step/error evidence. Ordinary implementation failures or expected missing-feature red require repair, not a blocker. This never authorizes protected writes or skips a successful gate.",
			map[string]any{"paths": policyStringList(), "reason": policyString(), "evidence": policyString()}, "paths", "reason", "evidence")
	case FinishTool:
		return toolFunction(name,
			"Say the implementation is complete. Kogen then runs the fixers and every check, and a reviewer reviews the code; failures come back to you.",
			map[string]any{"summary": policyString()}, "summary")
	}
	panic(fmt.Sprintf("build: unknown tool %q", name))
}

func toolFunction(name, description string, props map[string]any, required ...string) map[string]any {
	req := make([]any, len(required))
	for i, r := range required {
		req[i] = r
	}
	return map[string]any{
		"type":        "function",
		"name":        name,
		"description": description,
		"parameters": map[string]any{
			"type":                 "object",
			"properties":           props,
			"required":             req,
			"additionalProperties": false,
		},
		"strict": false,
	}
}

func policyString() map[string]any { return map[string]any{"type": "string"} }
func policyStringList() map[string]any {
	return map[string]any{"type": "array", "items": policyString()}
}
