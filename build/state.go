package build

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"

	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-loop/provider"
	"github.com/KogenAI/kogen-verification/verify"
)

// requiredStateKeys are the fields a checkpoint state must carry.
var requiredStateKeys = []string{
	"root", "slug", "config", "intent_text", "start", "baseline", "baseline_digests", "excluded",
	"run", "git_index", "start_tree", "phase", "t0", "usage", "round", "pass", "review", "repairs",
	"last_pass", "last_review", "blocker", "continuation", "approval_bytes", "parent", "root_run",
	"model_active_ms", "unknown_requests", "attempt_usage", "working_context", "source_provenance",
	"prompt_cache_states",
}

// decodeState reads a version 4 state; ok is false for a missing required field or a value of
// the wrong type.
func decodeState(raw json.RawMessage) (*State, bool) {
	var present map[string]json.RawMessage
	if json.Unmarshal(raw, &present) != nil {
		return nil, false
	}
	for _, k := range requiredStateKeys {
		if _, ok := present[k]; !ok {
			return nil, false
		}
	}
	for _, k := range []string{"usage", "attempt_usage"} {
		var counts map[string]json.RawMessage
		if json.Unmarshal(present[k], &counts) != nil {
			return nil, false
		}
		for _, name := range []string{"input", "cached", "output", "reasoning"} {
			if _, ok := counts[name]; !ok {
				return nil, false
			}
		}
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	st := &State{}
	if dec.Decode(st) != nil {
		return nil, false
	}
	return st, true
}

// sameConfig reports whether two configurations have the same canonical JSON.
func sameConfig(a, b *project.Config) bool {
	if a == nil || b == nil {
		return a == b
	}
	ja, errA := json.Marshal(a)
	jb, errB := json.Marshal(b)
	return errA == nil && errB == nil && bytes.Equal(ja, jb)
}

// stateIntent rebuilds the Intent from the text the Build was started with.
func stateIntent(s *State) (*project.Intent, error) {
	return project.ReadIntent(s.Root, s.Slug, map[string][]byte{project.IntentMDPath(s.Slug): []byte(s.IntentText)})
}

// validContinuation is the shape check of a continuation.
func validContinuation(c *Continuation) bool {
	switch c.Stage {
	case StageBuilder, StageUsageLimit:
		return validItems(c.Input)
	case StageBuilderVerify:
		return validItems(c.Input)
	case StageReview, StageReviewVerify, StageReviewRetry:
		return c.Pass != nil && c.Pass.Valid() && len(c.Review) > 0
	case StageReviewOnly:
		return c.Pass != nil && c.Pass.Valid() && c.Pass.Green && len(c.Review) == 0
	case StageLanded:
		return c.Commit != ""
	case StageCommitFailed:
		return c.Tree != ""
	case StageStartup, StageTool, StageCommit:
		return true
	}
	return false
}

func validItems(items []any) bool {
	for _, it := range items {
		if _, ok := it.(map[string]any); !ok {
			return false
		}
	}
	return true
}

// expectedScenarios parses the Intent's feature for the completeness check of a green pass.
func expectedScenarios(s *State) ([]project.Scenario, bool) {
	feature, err := readFile(s.Root, project.FeaturePath(s.Slug))
	if err != nil {
		return nil, false
	}
	expected, err := project.ParseGherkin(string(feature))
	return expected, err == nil
}

// stateComplete is whether a recorded green pass proves every required gate (VER-7).
func stateComplete(s *State, p *verify.Pass, model *project.ModelSpec) bool {
	expected, ok := expectedScenarios(s)
	return ok && p.Complete(s.Config, expected, model)
}

// validCheckpointState is the structural check of a decoded state against its checkpoint envelope.
func validCheckpointState(s *State, doc *Checkpoint) bool {
	if s.Phase != "startup" && s.Phase != "building" {
		return false
	}
	if s.Root == "" || s.Slug == "" || s.Start == "" || s.GitIndex == "" || s.StartTree == "" || s.ApprovalBytes == "" {
		return false
	}
	if s.Run == nil || s.Run.ID == "" || s.Run.Dir == "" || s.Run.LogPath == "" || s.Config == nil {
		return false
	}
	intent, err := stateIntent(s)
	if err != nil || intent.Title == "" || intent.Model == nil {
		return false
	}
	if s.LastPass != nil {
		if !s.LastPass.Valid() || (s.LastPass.Green && !stateComplete(s, s.LastPass, intent.Model)) {
			return false
		}
	}
	if !validWorkingContext(s) || !validSource(s.SourceProvenance) {
		return false
	}
	if r := s.LastReview; r != nil && !slices.Contains([]string{"approve", "changes", "pending", "invalid"}, r.Verdict) {
		return false
	}
	if !provider.ValidCacheStates(s.PromptCache) || !validCacheIDs(s.CacheIDs) {
		return false
	}
	if !stagesValid(s) {
		return false
	}
	if r := s.ReviewRetry; r != nil && r.Tree == "" {
		return false
	}
	if !validBaseline(s) {
		return false
	}
	if s.Baseline == nil || !slices.Contains(s.Baseline, project.ConfigPath) || s.BaselineDigests == nil {
		return false
	}
	for _, p := range s.Baseline {
		if _, ok := s.BaselineDigests[p]; !ok {
			return false
		}
	}
	for _, n := range []int{s.Round, s.Pass, s.Review, s.Repairs, s.UnknownRequests, int(s.ModelActiveMS)} {
		if n < 0 {
			return false
		}
	}
	for _, t := range []Tokens{s.Usage, s.AttemptUsage} {
		if !validTokens(t) {
			return false
		}
	}
	if x := s.Executor; x != nil && (x.Name == "" || x.Rounds < 0) {
		return false
	}
	c := &s.Continuation
	if !validContinuation(c) {
		return false
	}
	if c.Pass != nil && c.Pass.Green && !stateComplete(s, c.Pass, intent.Model) {
		return false
	}
	if c.Stage == StageLanded {
		if s.LastPass == nil || s.LastReview == nil || !s.LastPass.Green || s.LastReview.Verdict != "approve" || s.LastReview.Tree != s.LastPass.Tree {
			return false
		}
	}
	if c.Stage == StageCommitFailed {
		p, r := s.LastPass, s.LastReview
		if p == nil || r == nil || !p.Green || !p.Clean || r.Verdict != "approve" || p.Tree != c.Tree || r.Tree != c.Tree {
			return false
		}
	}
	if !sameProgress(doc.StageProgress, s.StageProgress) {
		return false
	}
	return doc.Identity != "" && doc.Candidate != "" && doc.Approval == s.ApprovalBytes
}

func sameProgress(a, b *StageProgress) bool {
	if a == nil || b == nil {
		return a == b
	}
	return canonical(a.Value()) == canonical(b.Value())
}

// validCacheIDs is the checkpoint check of the conversation identities: at most
// provider.MaxCacheID characters each.
func validCacheIDs(ids map[string]string) bool {
	for k, id := range ids {
		if k == "" || id == "" || len(id) > provider.MaxCacheID {
			return false
		}
	}
	return true
}

func validSource(r *project.SourceRef) bool {
	return r == nil || (r.Path != "" && r.SHA256 != "" && r.ContractSHA256 != "")
}

func validWorkingContext(s *State) bool {
	if s.WorkingContext == nil {
		return false
	}
	for owner, st := range s.WorkingContext {
		if (owner != "build" && owner != "review") || st == nil || st.Mandatory == nil || st.Observations == nil || st.History == nil {
			return false
		}
		if !validItems(st.Mandatory) {
			return false
		}
		for _, h := range st.History {
			if h.Path == "" || h.SHA256 == "" {
				return false
			}
		}
	}
	return true
}

func trimmed(s string) string { return strings.TrimSpace(s) }

func validTokens(t Tokens) bool {
	for _, n := range []*int{t.Input, t.Cached, t.Output, t.Reasoning} {
		if n != nil && *n < 0 {
			return false
		}
	}
	return true
}
