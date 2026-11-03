package review

import (
	"strconv"
	"strings"

	"github.com/KogenAI/kogen-executor/agent"
)

// The reviewer's instructions in the on_demand mode: the complete-source rule of the parts mode
// is replaced by the hunk-covering reading rule; everything else is the reviewer prompt verbatim.
const (
	PartsRule    = "Read every pinned part through read_file before approving; a pointer alone is not inspection."
	OnDemandRule = "With an on_demand catalog, read every part its required_parts lists (the parts covering changed hunks) " +
		"completely through read_file before any verdict, then judge the candidate on the evidence read; request any other " +
		"part by id when a claim depends on it, and nothing beyond that is required. A pointer alone is not inspection. " +
		"Unread required parts make a verdict invalid, never a finding: a blocking finding states a concrete candidate defect."
	partsRound    = "issue all pinned-part reads and needed context lookups together"
	onDemandRound = "issue all required pinned-part reads and needed context lookups together"
)

// Prompt is the native reviewer's instructions: the reviewer prompt, with the on_demand
// reading rule in that mode.
func Prompt(onDemand bool) string {
	text := agent.Prompt(agent.Reviewer)
	if !onDemand {
		return text
	}
	text = strings.Replace(text, PartsRule, OnDemandRule, 1)
	return strings.Replace(text, partsRound, onDemandRound, 1)
}

// File is the name of the record of review n in the run folder.
func File(n int) string { return "review-" + strconv.Itoa(n) + ".json" }
