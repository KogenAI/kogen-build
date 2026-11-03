package core

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"

	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-verification/verify"
)

// The shape of a staged Build's durable state (the optional `## Stages` plan). Its validation,
// custody files and gates belong to package build; what the independent review needs is here:
// the plan's progress, the evidence it shows the reviewer and the phase a review saves.

// StageDef is one stage of the approved plan as the checkpoint keeps it.
type StageDef struct {
	ID     string   `json:"id"`
	Title  string   `json:"title"`
	Goal   string   `json:"goal"`
	Checks []string `json:"checks"`
}

// Value is the stage as plain JSON values.
func (d StageDef) Value() map[string]any {
	checks := make([]any, len(d.Checks))
	for i, c := range d.Checks {
		checks[i] = c
	}
	return map[string]any{"id": d.ID, "title": d.Title, "goal": d.Goal, "checks": checks}
}

// The phases of a staged Build: "stage" while scoped stages remain, then "final" (whole
// gates), "review" and "landed".
const (
	PhaseStage  = "stage"
	PhaseFinal  = "final"
	PhaseReview = "review"
	PhaseLanded = "landed"
)

// StageProgress is the durable position of a staged Build: how many stages there are, which
// are verified, which is active (nil after the last) and the phase.
type StageProgress struct {
	Total     int      `json:"total"`
	Completed []string `json:"completed"`
	Active    *string  `json:"active"`
	Phase     string   `json:"phase"`
}

// Clone is the progress with its own list of completed stages.
func (p StageProgress) Clone() StageProgress {
	p.Completed = slices.Clone(p.Completed)
	if p.Completed == nil {
		p.Completed = []string{}
	}
	return p
}

// Value is the progress as plain JSON values.
func (p StageProgress) Value() map[string]any {
	completed := make([]any, len(p.Completed))
	for i, c := range p.Completed {
		completed[i] = c
	}
	var active any
	if p.Active != nil {
		active = *p.Active
	}
	return map[string]any{"total": p.Total, "completed": completed, "active": active, "phase": p.Phase}
}

// StageCustody names a stage's private evidence file and its SHA-256.
type StageCustody struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// StageEvidence is one scoped stage gate's result: the verification pass (checks named
// stage-<id>-<n>), the stage and its position, the approved contract it ran under and the
// private file keeping it.
type StageEvidence struct {
	Pass     verify.Pass
	StageID  string
	Index    int
	Contract string
	Custody  *StageCustody
}

// FileValue is the evidence as the private file holds it (no custody).
func (e StageEvidence) FileValue() (map[string]any, error) {
	raw, err := json.Marshal(e.Pass)
	if err != nil {
		return nil, err
	}
	m, err := DecodeJSONNumbers(raw)
	if err != nil {
		return nil, err
	}
	doc := m.(map[string]any)
	doc["stage_id"], doc["stage_index"], doc["contract"] = e.StageID, json.Number(strconv.Itoa(e.Index)), e.Contract
	return doc, nil
}

func (e StageEvidence) MarshalJSON() ([]byte, error) {
	doc, err := e.FileValue()
	if err != nil {
		return nil, err
	}
	if e.Custody != nil {
		doc["custody"] = e.Custody
	}
	return json.Marshal(doc)
}

func (e *StageEvidence) UnmarshalJSON(data []byte) error {
	var pass verify.Pass
	if err := json.Unmarshal(data, &pass); err != nil {
		return err
	}
	var rest struct {
		StageID  string        `json:"stage_id"`
		Index    int           `json:"stage_index"`
		Contract string        `json:"contract"`
		Custody  *StageCustody `json:"custody"`
	}
	if err := json.Unmarshal(data, &rest); err != nil {
		return err
	}
	*e = StageEvidence{Pass: pass, StageID: rest.StageID, Index: rest.Index, Contract: rest.Contract, Custody: rest.Custody}
	return nil
}

// summary is the evidence as staged navigation lists it.
func (e StageEvidence) summary() map[string]any {
	m := map[string]any{
		"stage_id": e.StageID, "stage_index": e.Index, "tree": e.Pass.Tree, "green": e.Pass.Green,
		"failing": stringList(e.Pass.Failing), "not_run": stringList(e.Pass.NotRun),
	}
	if e.Custody != nil {
		m["custody"] = map[string]any{"path": e.Custody.Path, "sha256": e.Custody.SHA256}
	}
	return m
}

func stringList(list []string) []any {
	out := make([]any, len(list))
	for i, s := range list {
		out[i] = s
	}
	return out
}

// Navigation is the staged plan's progress as the builder and reviewer see it (empty for an
// Intent without stages): the progress, the active stage, and the evidence, which is
// historical for its recorded tree and not current whole-Intent proof.
func (s *State) Navigation() string {
	p := s.StageProgress
	if p == nil {
		return ""
	}
	progress, _ := project.EncodeJSON(p.Value())
	var active *StageDef
	if p.Active != nil {
		for i := range s.StagePlan {
			if s.StagePlan[i].ID == *p.Active {
				active = &s.StagePlan[i]
			}
		}
	}
	var where string
	if active != nil {
		encoded, _ := project.EncodeJSON(active.Value())
		where = fmt.Sprintf("Active stage %d/%d: %s\n", len(p.Completed)+1, p.Total, encoded)
	} else {
		where = "All scoped stages completed. Continue whole-Intent " + p.Phase + ".\n"
	}
	evidence := make([]any, len(s.StageEvidence))
	for i, e := range s.StageEvidence {
		evidence[i] = e.summary()
	}
	history, _ := project.EncodeJSON(evidence)
	return "\nStaged plan progress: " + string(progress) + "\n" + where +
		"Stage evidence is historical for its recorded tree, not current whole-Intent proof.\nHistorical stage evidence: " + string(history) + "\n"
}

// NavigationIn is the navigation the Build would show in the given phase.
func (s *State) NavigationIn(phase string) string {
	if s.StageProgress == nil {
		return ""
	}
	t := *s
	t.SetPhase(phase)
	return t.Navigation()
}

// SetPhase moves a staged Build to a phase (no effect without stages).
func (s *State) SetPhase(phase string) {
	if s.StageProgress != nil {
		p := s.StageProgress.Clone()
		p.Phase = phase
		s.StageProgress = &p
	}
}
