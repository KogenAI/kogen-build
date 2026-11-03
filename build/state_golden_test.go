package build

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-verification/verify"
)

// The state goldens pin the exact bytes of the persisted Build state under testdata/state:
//
//	testdata/state/fresh/checkpoint.json          the checkpoint of a fresh run at its first builder request
//	testdata/state/attempts/checkpoint.json       the checkpoint after several builder rounds and verification passes
//	testdata/state/review-approve/checkpoint.json the checkpoint of a run whose independent review approved
//	testdata/state/review-approve/review-1.json   the review record the real reviewer writes for an approval
//	testdata/state/review-changes/checkpoint.json the checkpoint of a run whose independent review asked for changes
//	testdata/state/review-changes/review-1.json   the review record the real reviewer writes for a changes verdict
//	testdata/state/resumed/checkpoint.json        the checkpoint of the attempts run resumed as a new run
//
// They pin the format that existing runs and resumes read, so they must not change without
// deliberate intent. A checkpoint golden is exactly what (*Run).PublishCheckpoint writes for the
// envelope (*Attempt).Save publishes, from a synthetic State with fixed values only; a review
// record golden is the file the real reviewer writes in a Build against the scripted provider.
// Without -update each test compares its bytes with the golden exactly and writes nothing; with
// it (`go test ./... -update`) each test rewrites its golden from the current code.

const (
	stateGoldenRoot      = "/kogen-golden/checkout"
	stateGoldenStart     = "1111111111111111111111111111111111111111"
	stateGoldenStartTree = "2222222222222222222222222222222222222222"
	stateGoldenTree1     = "3333333333333333333333333333333333333333"
	stateGoldenTree2     = "4444444444444444444444444444444444444444"
	stateGoldenCandidate = "5555555555555555555555555555555555555555"
	stateGoldenRunID     = "a1b2c3d4"
	stateGoldenResumedID = "e5f6a7b8"
	stateGoldenT0        = int64(1792000000000)
	stateGoldenResumedT0 = int64(1792000600000)
)

const stateGoldenIntent = `# Preserve the golden fixture

Slug: fixture
Model: not applicable — no model for this fixture

Acceptance:
- .kogen/intents/fixture/acceptance.feature

## Tests
- the fixture stays as it is: the acceptance scenario (existing)

## Source comparison
- addressed: preserve app.txt fixture behavior; source request; contract Tests and acceptance.feature.

## Feasibility
- none
`

// stateGoldenCheck compares the bytes a test produced with the golden at path (relative to the
// package), or with -update rewrites the golden from them.
func stateGoldenCheck(t *testing.T, path string, got []byte) {
	t.Helper()
	file := filepath.FromSlash(path)
	if *update {
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		writeGoldenFile(t, file, got)
		return
	}
	want, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("%s: cannot read the golden (run `go test ./... -update` to create it): %v", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s differs from the bytes the code persists now (run `go test ./... -update` only for a deliberate change):\n got: %s\nwant: %s", path, got, want)
	}
}

// stateGoldenMust fails the test naming the golden when producing its bytes failed.
func stateGoldenMust(t *testing.T, path string, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: producing the golden's bytes: %v", path, err)
	}
}

func stateGoldenRun(id string) *Run {
	dir := stateGoldenRoot + "/.kogen/local/runs/" + id
	return &Run{ID: id, Dir: dir, LogPath: dir + "/log.jsonl"}
}

func stateGoldenConfig() *project.Config {
	return &project.Config{
		Checks:     []project.Check{{Name: "lint", Run: "test -f app.txt"}},
		Acceptance: &project.Acceptance{Run: "sh accept.sh", StepsDir: "features/steps"},
		Models: map[string]project.Role{
			"build":  {Provider: "openai", Model: "golden-build", Effort: "medium"},
			"review": {Provider: "openai", Model: "golden-review", Effort: "high"},
		},
	}
}

func stateGoldenReviewer() project.Role {
	return stateGoldenConfig().Models["review"]
}

func stateGoldenUser(text string) map[string]any {
	return map[string]any{"type": "message", "role": "user", "content": []any{map[string]any{"type": "input_text", "text": text}}}
}

// stateGoldenState is the synthetic state of a fresh run at its first builder request.
func stateGoldenState() State {
	return State{
		Root:       stateGoldenRoot,
		Slug:       "fixture",
		Config:     stateGoldenConfig(),
		IntentText: stateGoldenIntent,
		Start:      stateGoldenStart,
		Baseline:   []string{".kogen/intents/fixture/acceptance.feature", ".kogen/intents/fixture/intent.md", ".kogen/project.yaml"},
		BaselineDigests: map[string]string{
			".kogen/intents/fixture/acceptance.feature": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			".kogen/intents/fixture/intent.md":          "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			".kogen/project.yaml":                       "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		},
		Excluded:  []string{},
		Run:       stateGoldenRun(stateGoldenRunID),
		GitIndex:  stateGoldenRoot + "/.git/index",
		StartTree: stateGoldenStartTree,
		Phase:     "building",
		T0:        stateGoldenT0,
		Usage:     zeroTokens(),
		Round:     1,
		Continuation: Continuation{Stage: StageBuilder, Input: []any{
			stateGoldenUser("Build the approved Intent: Preserve the golden fixture."),
		}},
		ApprovalBytes:  encodeBase64([]byte("{\"slug\":\"fixture\"}\n")),
		ModelActiveMS:  0,
		AttemptUsage:   zeroTokens(),
		WorkingContext: map[string]*OwnerContext{"build": {Mandatory: []any{stateGoldenUser("Build the approved Intent.")}, Observations: []any{}, History: []HistoryRef{}}},
		PromptCache:    nil,
	}
}

func stateGoldenGreenPass(n int, tree string) *verify.Pass {
	scenario := func(name string, line int) verify.ScenarioResult {
		return verify.ScenarioResult{Name: name, Line: line, Tags: []string{}, UnexercisedSteps: []verify.StepRecord{}, Status: "passed",
			Steps: []verify.StepRecord{{Keyword: "Then", Text: "the acceptance check passes", Line: line + 1}}}
	}
	return &verify.Pass{N: n, Clean: true, Green: true, Tree: tree,
		Acceptance: &verify.Acceptance{Status: 0, Output: "", Scenarios: []verify.ScenarioResult{scenario("First", 2), scenario("Second", 4)}, Passed: 2},
		Model:      verify.ModelNone,
		Checks:     []verify.CommandResult{{Name: "lint", Command: "test -f app.txt", Status: 0, Output: ""}},
	}
}

func stateGoldenRedPass(n int, tree string) *verify.Pass {
	detail := "expected app.txt to hold green"
	passed := verify.ScenarioResult{Name: "Second", Line: 4, Tags: []string{}, UnexercisedSteps: []verify.StepRecord{}, Status: "passed",
		Steps: []verify.StepRecord{{Keyword: "Then", Text: "the acceptance check passes", Line: 5}}}
	failed := verify.ScenarioResult{Name: "First", Line: 2, Tags: []string{}, UnexercisedSteps: []verify.StepRecord{}, Status: "failed", Detail: &detail,
		Steps: []verify.StepRecord{{Keyword: "Then", Text: "the acceptance check passes", Line: 3}}}
	return &verify.Pass{N: n, Clean: true, Green: false, Tree: tree,
		Acceptance: &verify.Acceptance{Status: 1, Output: "actual-case-red\n", Scenarios: []verify.ScenarioResult{failed, passed}, Passed: 1, Failed: 1},
		Model:      verify.ModelNone,
		Checks:     []verify.CommandResult{{Name: "lint", Command: "test -f app.txt", Status: 0, Output: ""}},
		Failing:    []string{"acceptance"},
	}
}

func stateGoldenAttemptsState() State {
	st := stateGoldenState()
	st.Round, st.Pass = 3, 2
	st.Usage = Tokens{count(3200), count(1800), count(410), nil}
	st.AttemptUsage = Tokens{count(3200), count(1800), count(410), nil}
	st.ModelActiveMS = 42500
	st.UnknownRequests = 1
	st.LastPass = stateGoldenRedPass(2, stateGoldenTree2)
	st.Continuation = Continuation{Stage: StageBuilder, Input: []any{
		stateGoldenUser("Build the approved Intent: Preserve the golden fixture."),
		map[string]any{"type": "function_call", "call_id": "call_1", "name": "write_file", "arguments": "{\"path\":\"app.txt\",\"content\":\"red\\n\"}"},
		map[string]any{"type": "function_call_output", "call_id": "call_1", "output": "wrote app.txt"},
		stateGoldenUser("Verification pass 2 is red: acceptance failed. Fix it and finish again."),
	}}
	return st
}

// stateGoldenReviewedState is the state after the independent review n=1 returned verdict on the
// green pass, with the reviewer's own session saved in the continuation.
func stateGoldenReviewedState(t *testing.T, path, verdict string, findings []map[string]any) State {
	t.Helper()
	pass := stateGoldenGreenPass(1, stateGoldenTree1)
	session := reviewSession{
		Role:     stateGoldenReviewer(),
		Input:    []any{stateGoldenUser("Review the candidate tree " + stateGoldenTree1 + "."), map[string]any{"type": "function_call", "call_id": "call_r1", "name": "submit_review", "arguments": "{}"}},
		Usage:    Tokens{count(900), count(300), count(60), count(20)},
		Attempts: []reviewAttempt{{Pass: pass.N, Tree: stateGoldenTree1, Verdict: verdict, Findings: findings}},
		Verdict:  &reviewVerdict{Verdict: verdict, Findings: findings},
		Tree:     stateGoldenTree1,
	}
	raw, err := session.encode()
	stateGoldenMust(t, path, err)
	st := stateGoldenState()
	st.Round, st.Pass, st.Review = 1, 1, 1
	st.Usage = Tokens{count(2100), count(900), count(250), count(0)}
	st.AttemptUsage = st.Usage
	st.ModelActiveMS = 18000
	st.LastPass = pass
	st.LastReview = &ReviewRecord{N: 1, Verdict: verdict, Findings: findings, Tree: stateGoldenTree1}
	st.Continuation = Continuation{Stage: StageReviewVerify, Pass: pass, Base: pass, Review: raw}
	return st
}

func stateGoldenEnvelope(state json.RawMessage) Checkpoint {
	return Checkpoint{
		Version:   4,
		Identity:  stateGoldenRoot + ":16777230:398539521",
		Candidate: stateGoldenCandidate,
		Index:     encodeBase64([]byte("DIRC golden index")),
		Approval:  encodeBase64([]byte("{\"slug\":\"fixture\"}\n")),
		State:     state,
	}
}

// stateGoldenPublish publishes st in the envelope Save writes and returns the file's bytes.
func stateGoldenPublish(t *testing.T, path string, st *State, doc Checkpoint) []byte {
	t.Helper()
	state, err := encodeJSON(st)
	stateGoldenMust(t, path, err)
	doc.State = state
	run := &Run{Dir: t.TempDir()}
	stateGoldenMust(t, path, run.PublishCheckpoint(doc))
	got, err := os.ReadFile(filepath.Join(run.Dir, "checkpoint.json"))
	stateGoldenMust(t, path, err)
	return got
}

func stateGoldenFresh(t *testing.T, path string) []byte {
	st := stateGoldenState()
	return stateGoldenPublish(t, path, &st, stateGoldenEnvelope(nil))
}

func stateGoldenAttempts(t *testing.T, path string) []byte {
	st := stateGoldenAttemptsState()
	return stateGoldenPublish(t, path, &st, stateGoldenEnvelope(nil))
}

func stateGoldenReviewCheckpoint(t *testing.T, path, verdict string, findings []map[string]any) []byte {
	st := stateGoldenReviewedState(t, path, verdict, findings)
	return stateGoldenPublish(t, path, &st, stateGoldenEnvelope(nil))
}

// stateGoldenResumedBytes resumes the attempts checkpoint bytes the way resumeLocked does: a new
// run whose parent and root are the attempts run, a new start time and no attempt usage.
func stateGoldenResumedBytes(t *testing.T, path string, attempts []byte) []byte {
	loaded, why := decodeCheckpoint(attempts, nil)
	if loaded == nil {
		stateGoldenMust(t, path, &stateGoldenError{"the attempts checkpoint does not decode: " + why})
	}
	st := loaded.State
	parent := st.Run.ID
	root := parent
	st.Run = stateGoldenRun(stateGoldenResumedID)
	st.Parent, st.RootRun = &parent, &root
	st.T0 = stateGoldenResumedT0
	st.AttemptUsage = zeroTokens()
	return stateGoldenPublish(t, path, st, loaded.Doc)
}

type stateGoldenError struct{ text string }

func (e *stateGoldenError) Error() string { return e.text }

func TestStateGoldenFreshCheckpoint(t *testing.T) {
	const path = "testdata/state/fresh/checkpoint.json"
	stateGoldenCheck(t, path, stateGoldenFresh(t, path))
}

func TestStateGoldenAttemptsCheckpoint(t *testing.T) {
	const path = "testdata/state/attempts/checkpoint.json"
	stateGoldenCheck(t, path, stateGoldenAttempts(t, path))
}

func TestStateGoldenReviewApproveCheckpoint(t *testing.T) {
	const path = "testdata/state/review-approve/checkpoint.json"
	stateGoldenCheck(t, path, stateGoldenReviewCheckpoint(t, path, "approve", []map[string]any{}))
}

func TestStateGoldenReviewChangesCheckpoint(t *testing.T) {
	const path = "testdata/state/review-changes/checkpoint.json"
	findings := []map[string]any{{"path": "app.txt", "line": 1, "severity": "blocking", "message": "app.txt does not hold the requested content"}}
	stateGoldenCheck(t, path, stateGoldenReviewCheckpoint(t, path, "changes", findings))
}

// TestStateGoldenResumedCheckpoint resumes the attempts checkpoint bytes produced in this very
// test, not the attempts golden, so a drifted attempts golden does not change this one.
func TestStateGoldenResumedCheckpoint(t *testing.T) {
	const path = "testdata/state/resumed/checkpoint.json"
	attempts := stateGoldenAttempts(t, path)
	stateGoldenCheck(t, path, stateGoldenResumedBytes(t, path, attempts))
}

// stateGoldenReviewRecord runs a reviewed Build against the scripted provider with the real
// reviewer and returns the review-1.json it wrote. The acceptance command is reached through the
// environment, so no temporary path is recorded in the candidate tree and the record is the same
// in every run.
func stateGoldenReviewRecord(t *testing.T, path, verdict string, findings ...map[string]any) []byte {
	t.Helper()
	dates := []string{"GIT_AUTHOR_DATE=2026-10-22T00:00:00Z", "GIT_COMMITTER_DATE=2026-10-22T00:00:00Z"}
	e := newEnvIn(t, physical(t, t.TempDir()), physical(t, t.TempDir()), physical(t, t.TempDir()), dates, func(c *project.Config) {
		c.Acceptance.Run = `sh "$KOGEN_GOLDEN_ACCEPT"`
	})
	e.extraEnv = []string{"KOGEN_GOLDEN_ACCEPT=" + filepath.Join(e.aux, "accept.sh")}
	f := e.prov
	f.then(landScript(f)...)
	f.then(submit(f, verdict, findings...))
	a := e.reviewed()
	wantExit := 0
	if verdict == "changes" {
		wantExit = 1
	}
	if a.exit != wantExit {
		t.Fatalf("%s: the reviewed Build exited %d, want %d: %s %s", path, a.exit, wantExit, a.shown(), a.errs)
	}
	got, err := os.ReadFile(e.runPath(e.onlyRun(), "review-1.json"))
	stateGoldenMust(t, path, err)
	return got
}

func TestStateGoldenReviewApproveRecord(t *testing.T) {
	t.Parallel()
	const path = "testdata/state/review-approve/review-1.json"
	stateGoldenCheck(t, path, stateGoldenReviewRecord(t, path, "approve"))
}

func TestStateGoldenReviewChangesRecord(t *testing.T) {
	t.Parallel()
	const path = "testdata/state/review-changes/review-1.json"
	stateGoldenCheck(t, path, stateGoldenReviewRecord(t, path, "changes", finding("blocking", "app.txt does not hold the requested content")))
}
