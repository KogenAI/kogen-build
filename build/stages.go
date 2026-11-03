package build

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/KogenAI/kogen-coordinator/durable"
	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-executor/core"
	"kogen-loop/sys"
)

// Staged Builds: one approved Intent built in ordered, verified stages (the optional
// `## Stages` plan). Progress is durable and bound to the tree each stage gate verified, the
// stages are gated by their own scoped checks only, and after the last one the whole
// Intent still goes through the full gates, an independent review and one commit. Stage
// evidence is historical for its recorded tree and never delivery.

// stageDefs is the approved plan as the checkpoint keeps it.
func stageDefs(plan []project.Stage) []stageDef {
	if plan == nil {
		return nil
	}
	out := make([]stageDef, len(plan))
	for i, s := range plan {
		out[i] = stageDef{ID: s.ID, Title: s.Title, Goal: s.Goal, Checks: slices.Clone(s.Checks)}
	}
	return out
}

func initialProgress(plan []stageDef) *StageProgress {
	if plan == nil {
		return nil
	}
	first := plan[0].ID
	return &StageProgress{Total: len(plan), Completed: []string{}, Active: &first, Phase: phaseStage}
}

// retainStage writes the evidence as a private, durable file in the run folder (exclusive
// 0600, synced) and returns it with its custody.
func (b *Attempt) retainStage(e StageEvidence) (StageEvidence, error) {
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return e, err
	}
	path := filepath.Join(b.Run.Dir, "stage-"+strconv.Itoa(e.Pass.N)+"-"+hex.EncodeToString(nonce[:])+".json")
	doc, err := e.FileValue()
	if err != nil {
		return e, err
	}
	raw, err := project.EncodeJSON(doc)
	if err != nil {
		return e, err
	}
	raw = append(raw, '\n')
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return e, err
	}
	werr := f.Chmod(0o600)
	if werr == nil {
		_, werr = f.Write(raw)
	}
	if werr == nil {
		werr = durable.Sync(f)
	}
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		return e, werr
	}
	e.Custody = &StageCustody{Path: path, SHA256: core.HashHex(raw)}
	return e, nil
}

// validateStages refuses a Build whose stage progress or evidence no longer fits the
// approved plan or its private custody.
func (b *Attempt) validateStages() error { return b.validateStagesAs(b.Continuation) }

// validateStagesAs is validateStages for the continuation the caller is working under (a
// request's tool boundary is not the builder's own).
func (b *Attempt) validateStagesAs(c Continuation) error {
	st := b.State
	st.Continuation = c
	if !stagesValid(&st) {
		return sys.Fail("candidate", "resume_refused",
			"stage progress/evidence inconsistent with approved plan or private custody", 1)
	}
	return nil
}

var (
	stageFileName = regexp.MustCompile(`^stage-[0-9]+-[a-f0-9]{16}\.json$`)
	runIDName     = regexp.MustCompile(`^[a-f0-9]{8}$`)
)

// stageBound is whether the evidence's file is where, as and what its custody says: under the
// runs folder in a run folder, private (0700 folder, 0600 file), holding exactly the evidence.
func stageBound(s *State, e StageEvidence) bool {
	if e.Custody == nil {
		return false
	}
	path := e.Custody.Path
	runs := filepath.Join(s.Root, ".kogen/local/runs")
	rel, err := filepath.Rel(runs, path)
	if err != nil {
		return false
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) != 2 || !runIDName.MatchString(parts[0]) || !stageFileName.MatchString(parts[1]) ||
		parts[1][:len("stage-"+strconv.Itoa(e.Pass.N)+"-")] != "stage-"+strconv.Itoa(e.Pass.N)+"-" ||
		path != filepath.Join(runs, parts[0], parts[1]) {
		return false
	}
	for _, dir := range []string{s.Root, filepath.Join(s.Root, ".kogen"), filepath.Join(s.Root, ".kogen/local"), runs, filepath.Dir(path)} {
		if st, err := os.Lstat(dir); err != nil || !st.IsDir() {
			return false
		}
	}
	if st, err := os.Lstat(filepath.Dir(path)); err != nil || st.Mode().Perm() != 0o700 {
		return false
	}
	st, err := os.Lstat(path)
	if err != nil || st.Mode().Type() != 0 || st.Mode().Perm() != 0o600 || st.Mode()&fs.ModeSymlink != 0 {
		return false
	}
	raw, err := os.ReadFile(path)
	if err != nil || core.HashHex(raw) != e.Custody.SHA256 {
		return false
	}
	want, err := e.FileValue()
	if err != nil {
		return false
	}
	return sameJSON(raw, want)
}

// sameJSON is whether the encoded document and the value are the same parsed JSON.
func sameJSON(raw []byte, want any) bool {
	got, err := core.DecodeJSONNumbers(raw)
	if err != nil {
		return false
	}
	wanted, err := json.Marshal(want)
	if err != nil {
		return false
	}
	reread, err := core.DecodeJSONNumbers(wanted)
	return err == nil && canonical(got) == canonical(reread)
}

// stagesValid is the consistency of a Build's stage state with its approved Intent: no plan
// means no stage state at all; a plan means progress that fits the plan, the continuation
// owner and the recorded evidence (each green stage's clean, complete, scoped evidence in
// order, bound to its private file and to this approval).
func stagesValid(s *State) bool {
	plan, err := project.ParseStages(s.IntentText)
	if err != nil {
		return false
	}
	if plan == nil {
		return s.StageProgress == nil && s.StagePlan == nil && len(s.StageEvidence) == 0
	}
	return progressValid(s, stageDefs(plan))
}

func phaseValid(s *State, phase string) bool {
	c := s.Continuation
	switch c.Stage {
	case StageStartup, StageBuilder, StageBuilderVerify:
		return phase == phaseStage || phase == phaseFinal
	case StageTool:
		// A tool boundary the builder's own loop saved; the one a request saved names no
		// stage owner and is not a valid staged state.
		if c.Request {
			return false
		}
		switch c.Owner {
		case "build":
			return phase == phaseStage || phase == phaseFinal
		case "review":
			return phase == phaseReview
		}
		return false
	case StageReview, StageReviewVerify, StageReviewRetry, StageReviewOnly, StageCommit, StageCommitFailed:
		return phase == phaseReview
	case StageLanded:
		return phase == phaseLanded
	}
	return false
}

func progressValid(s *State, plan []stageDef) bool {
	p := s.StageProgress
	if p == nil || !phaseValid(s, p.Phase) || p.Total != len(plan) || !slices.EqualFunc(s.StagePlan, plan, stageEqual) {
		return false
	}
	ids := make([]string, len(plan))
	for i, d := range plan {
		ids[i] = d.ID
	}
	done := len(p.Completed)
	if done > len(ids) || !slices.Equal(p.Completed, ids[:done]) {
		return false
	}
	switch {
	case done < len(ids):
		if p.Active == nil || *p.Active != ids[done] || p.Phase != phaseStage {
			return false
		}
	default:
		if p.Active != nil || !slices.Contains([]string{phaseFinal, phaseReview, phaseLanded}, p.Phase) {
			return false
		}
	}
	var greens []string
	for _, e := range s.StageEvidence {
		if e.Pass.Green {
			greens = append(greens, e.StageID)
		}
	}
	if !slices.Equal(core.NonNil(greens), core.NonNil(p.Completed)) {
		return false
	}
	index := 1
	for _, e := range s.StageEvidence {
		if e.Index != index {
			return false
		}
		if e.Pass.Green {
			index++
		}
	}
	if index != done+1 {
		return false
	}
	for _, e := range s.StageEvidence {
		if e.Index < 1 || e.Index > len(plan) {
			return false
		}
		stage := plan[e.Index-1]
		if stage.ID != e.StageID || e.Contract != s.ApprovalBytes || !stageBound(s, e) || e.Pass.Tree == "" {
			return false
		}
		if e.Pass.Green && !scopedGreen(s, e, stage) {
			return false
		}
	}
	return true
}

func stageEqual(a, b stageDef) bool {
	return a.ID == b.ID && a.Title == b.Title && a.Goal == b.Goal && slices.Equal(a.Checks, b.Checks)
}

// scopedGreen is whether a green stage record really is a clean pass of exactly the stage's
// setup and checks, all successful.
func scopedGreen(s *State, e StageEvidence, stage stageDef) bool {
	p := e.Pass
	if !p.Clean || len(p.Moved) != 0 || len(p.Failing) != 0 || len(p.NotRun) != 0 || s.Config == nil {
		return false
	}
	if len(p.Checks) != len(stage.Checks) || len(p.Setup) != len(s.Config.Setup) {
		return false
	}
	for i, c := range stage.Checks {
		if p.Checks[i].Command != c || p.Checks[i].Status != 0 {
			return false
		}
	}
	for i, c := range s.Config.Setup {
		if p.Setup[i].Command != c || p.Setup[i].Status != 0 {
			return false
		}
	}
	return true
}
