package build

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-review/review"
)

// The real independent reviewer against a scripted provider: the builder's response is the
// first scripted handler, the reviewer's follow.

func useRealReviewer(in *Input)               { in.Reviewer = nil }
func useRealReviewerOnResume(in *ResumeInput) { in.Reviewer = nil }

func (e *env) reviewed(mutate ...func(*Input)) *attempt {
	e.t.Helper()
	return e.build(append([]func(*Input){useRealReviewer}, mutate...)...)
}

func (e *env) resumeReviewed(id string) *attempt {
	e.t.Helper()
	return e.resume(id, useRealReviewerOnResume)
}

func (e *env) startReviewed() (*attempt, <-chan struct{}) {
	e.t.Helper()
	return e.start(useRealReviewer)
}

// reviewRequests are the provider requests of the reviewer, in order.
func (f *fakeProvider) reviewRequests() []map[string]any {
	var out []map[string]any
	for _, req := range f.requests() {
		if text := requestInstructions(req); text == review.ReviewerInstructions() {
			out = append(out, req)
		}
	}
	return out
}

// requestInstructions are the model's instructions: the request's own, or, for a model with
// prompt-cache support, the developer message that opens the input.
func requestInstructions(req map[string]any) string {
	if s, ok := req["instructions"].(string); ok {
		return s
	}
	items, _ := req["input"].([]any)
	if len(items) == 0 {
		return ""
	}
	dev, _ := items[0].(map[string]any)
	blocks, _ := dev["content"].([]any)
	if len(blocks) == 0 {
		return ""
	}
	text, _ := blocks[0].(map[string]any)["text"].(string)
	return text
}

// requestInput is the conversation of a request, without the developer message.
func requestInput(req map[string]any) []any {
	items, _ := req["input"].([]any)
	if _, ok := req["instructions"]; !ok && len(items) > 0 {
		return items[1:]
	}
	return items
}

func requestText(req map[string]any) string {
	b, _ := json.Marshal(requestInput(req))
	return string(b)
}

func submit(f *fakeProvider, verdict string, findings ...map[string]any) handler {
	list := []any{}
	for _, x := range findings {
		list = append(list, x)
	}
	return f.calls("submit_review", map[string]any{"verdict": verdict, "findings": list})
}

func finding(severity, message string) map[string]any {
	return map[string]any{"path": "app.txt", "line": 1, "severity": severity, "message": message}
}

func writeApp(content string) []any {
	return []any{"write_file", map[string]any{"path": "app.txt", "content": content}}
}

func sessionOfRun(t *testing.T, e *env, id string) (map[string]any, map[string]any) {
	t.Helper()
	_, state := e.checkpoint(id)
	cont := state["continuation"].(map[string]any)
	var session map[string]any
	if raw, ok := cont["review"].(map[string]any); ok {
		session = raw
	}
	return cont, session
}

func TestTheReviewerApprovesUnchangedCodeWithoutRepairs(t *testing.T) {
	t.Parallel()
	// ExUnit: test/integration/release_behavior_test.exs: REV-4/REV-5 retries one invalid review verdict and lands after approval (approval of unchanged code)
	// ExUnit: .kogen/intents/reviewer-style-discipline: unchanged correct-code approval succeeds without an edit quota or preference blockers
	e := newEnv(t)
	f := e.prov
	f.then(landScript(f)...)
	f.then(submit(f, "approve", finding("note", "harmless preference")))
	start := e.head()
	a := e.reviewed()
	if a.exit != 0 || a.err != nil {
		t.Fatalf("exit %d err %v\n%s", a.exit, a.err, a.shown())
	}
	id := e.onlyRun()
	contains(t, "stdout", a.out.String(), "  review: approved, 1 note(s)\n  note: app.txt:1: harmless preference\n  repairs: 0 · tokens: 4 ")
	eq(t, "commits", e.gitOut("rev-list", "--count", start+"..HEAD"), "1")
	eq(t, "committed", e.gitOut("show", "HEAD:app.txt"), "candidate")

	reqs := f.reviewRequests()
	if len(reqs) != 1 {
		t.Fatalf("%d review requests", len(reqs))
	}
	req := reqs[0]
	if got := fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Replace(requestInstructions(req), "\n"+review.LimitRaises+"\n", "", 1)))); got != "345eb0281e2330ba1bf3de62528693cf34d7b3963e91bf332231c94492c4d2bb" {
		t.Fatalf("emitted reviewer prompt hash = %s", got)
	}
	// A fresh conversation: one opening message, the reviewer's tools, no builder replay.
	input := requestInput(req)
	if len(input) != 1 {
		t.Fatalf("%d input items", len(input))
	}
	var names []string
	for _, tool := range req["tools"].([]any) {
		names = append(names, tool.(map[string]any)["name"].(string))
	}
	if !slices.Equal(names, []string{"read_file", "view_image", "list_files", "search_text", "write_file", "shell", "submit_review"}) {
		t.Fatalf("tools %v", names)
	}
	contains(t, "instructions", requestInstructions(req), "Delegate mechanically enforceable formatting and style to the existing configured formatters")
	contains(t, "instructions", requestInstructions(req), "Approving unchanged correct code is a successful outcome.")
	blocks := input[0].(map[string]any)["content"].([]any)
	authority := blocks[0].(map[string]any)["text"].(string)
	source := blocks[1].(map[string]any)["text"].(string)
	for _, part := range []string{
		"# Intent\n\n# Preserve the fixture\n", "# acceptance.feature\n\nFeature: Integration fixture\n", "# Project instructions (README.md)\n\n# Fixture project\n",
		"Source provenance: ", "# Testing rules (docs/testing-rules.md)\n\n", "# Protected paths and configured commands\n\nThe approved baseline cannot change: ",
		"Acceptance: ",
	} {
		contains(t, "authority", authority, part)
	}
	for _, part := range []string{
		"# Check results of the final pass\n\nacceptance: passed (2 scenarios)\nmodel: not applicable\n\n# Diff of existing/deleted paths against start HEAD " + short(start),
		"-baseline\n+candidate\n", "# Full text of every added file\n\nAdded files appear here with their mode; changed files have hunks in the diff.\n\n--- .kogen/intents/fixture/acceptance.feature (added; mode 100644)\nFeature: Integration fixture\n",
		"\n--- app.txt (changed; hunks in diff)\n--- features/steps/integration_steps.js (added; mode 100644)\n// steps\n",
	} {
		contains(t, "source", source, part)
	}

	res := e.result(id)
	eq(t, "passes", res["passes"], any(float64(1)))
	eq(t, "reviews", res["reviews"], any(float64(1)))
	review := e.jsonFile(e.runPath(id, "review-1.json"))
	eq(t, "model", review["model"], any(project.DefaultModels()["review"].Model))
	eq(t, "verdict", review["verdict"], any("approve"))
	eq(t, "tree", review["tree"], any(e.gitOut("rev-parse", "HEAD^{tree}")))
	if attempts := review["attempts"].([]any); len(attempts) != 1 || attempts[0].(map[string]any)["verdict"] != "approve" {
		t.Fatalf("attempts %v", attempts)
	}
	usage := review["usage"].(map[string]any)
	eq(t, "review input tokens", usage["input"], any(float64(1)))
	events := eventsNamed(e.events(id), "review")
	if len(events) != 1 || events[0]["attempt"] != float64(1) || events[0]["verdict"] != "approve" {
		t.Fatalf("review events %v", events)
	}
}

func TestAReviewerRepairIsVerifiedAndCommittedWithoutReturningToTheBuilder(t *testing.T) {
	t.Parallel()
	// ExUnit: test/integration/build_boundaries_test.exs: REV-5 reviewer repairs are verified and committed without returning to the builder
	e := newEnv(t)
	f := e.prov
	f.then(landScript(f)...)
	f.then(f.calls(append(writeApp("reviewer repair\n"), "submit_review", map[string]any{"verdict": "approve", "findings": []any{}})...))
	a := e.reviewed()
	if a.exit != 0 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	id := e.onlyRun()
	eq(t, "committed", e.gitOut("show", "HEAD:app.txt"), "reviewer repair")
	res := e.result(id)
	eq(t, "passes", res["passes"], any(float64(2)))
	eq(t, "reviews", res["reviews"], any(float64(1)))
	eq(t, "requests", f.count(), 2)
	review := e.jsonFile(e.runPath(id, "review-1.json"))
	eq(t, "approved tree", review["tree"], any(e.gitOut("rev-parse", "HEAD^{tree}")))
	passes := eventsNamed(e.events(id), "pass")
	if len(passes) != 2 || passes[1]["green"] != true {
		t.Fatalf("passes %v", passes)
	}
	// The reviewer's tool use is in the run log.
	calls := eventsNamed(e.events(id), "review_tool_call")
	if len(calls) != 1 || calls[0]["name"] != "write_file" {
		t.Fatalf("review tool calls %v", calls)
	}
}

func TestFinalVerificationFailuresReturnToTheSameReviewerContext(t *testing.T) {
	t.Parallel()
	// ExUnit: test/integration/build_boundaries_test.exs: REV-5 final verification failures return to the same reviewer context
	e := newEnv(t, func(c *project.Config) { c.Checks = []project.Check{{Name: "tidy", Run: "! grep -qx broken app.txt"}} })
	e.approve()
	f := e.prov
	f.then(landScript(f)...)
	approve := map[string]any{"verdict": "approve", "findings": []any{}}
	f.then(
		f.calls(append(writeApp("broken\n"), "submit_review", approve)...),
		f.calls(append(writeApp("reviewer repaired\n"), "submit_review", approve)...),
	)
	a := e.reviewed()
	if a.exit != 0 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	id := e.onlyRun()
	eq(t, "committed", e.gitOut("show", "HEAD:app.txt"), "reviewer repaired")
	contains(t, "stdout", a.out.String(), "  repairs: 1 · ")
	res := e.result(id)
	eq(t, "passes", res["passes"], any(float64(3)))
	eq(t, "reviews", res["reviews"], any(float64(1)))
	reqs := f.reviewRequests()
	if len(reqs) != 2 {
		t.Fatalf("%d review requests", len(reqs))
	}
	first, second := requestInput(reqs[0]), requestInput(reqs[1])
	if !reflect.DeepEqual(second[:len(first)], first) {
		t.Fatal("the second request does not continue the first conversation")
	}
	text := requestText(reqs[1])
	contains(t, "second request", text, "Final verification of your repairs failed.")
	contains(t, "second request", text, "check tidy (! grep -qx broken app.txt): failed, exit status 1")
	contains(t, "second request", text, "use submit_review changes with a blocking finding at that protected path")
	contains(t, "second request", text, "then call submit_review again")
	contains(t, "second request", text, `"write_file"`)
	review := e.jsonFile(e.runPath(id, "review-1.json"))
	if attempts := review["attempts"].([]any); len(attempts) != 2 {
		t.Fatalf("attempts %v", attempts)
	}
	// The failed pass belongs to the reviewer: no builder round started.
	eq(t, "requests", f.count(), 3)
	eq(t, "rounds", len(eventsNamed(e.events(id), "pass")), 3)
}

func TestOneInvalidVerdictIsSentBackAndASecondEndsTheBuild(t *testing.T) {
	t.Parallel()
	// ExUnit: test/integration/release_behavior_test.exs: REV-4/REV-5 retries one invalid review verdict and lands after approval
	bad := finding("blocking", "An approve verdict cannot contain a blocking finding.")
	t.Run("retried once", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		f := e.prov
		f.then(landScript(f)...)
		f.then(submit(f, "approve", bad), submit(f, "approve"))
		a := e.reviewed()
		if a.exit != 0 {
			t.Fatalf("exit %d\n%s", a.exit, a.shown())
		}
		reqs := f.reviewRequests()
		eq(t, "review requests", len(reqs), 2)
		contains(t, "retry", requestText(reqs[1]), "invalid verdict:\\nan approve verdict cannot have blocking findings")
		id := e.onlyRun()
		review := e.jsonFile(e.runPath(id, "review-1.json"))
		eq(t, "attempts", len(review["attempts"].([]any)), 1)
	})
	t.Run("problems sent back", func(t *testing.T) {
		cases := map[string]struct {
			args any
			want string
		}{
			"unknown path": {map[string]any{"verdict": "changes", "findings": []any{map[string]any{"path": "nowhere.txt", "line": 3, "severity": "blocking", "message": "x"}}}, "nowhere.txt is not a file of the repository"},
			"no blocker":   {map[string]any{"verdict": "changes", "findings": []any{finding("note", "x")}}, "a changes verdict needs at least one blocking finding"},
			"bad finding":  {map[string]any{"verdict": "approve", "findings": []any{map[string]any{"path": "app.txt", "line": 1.5, "severity": "note", "message": "x"}}}, "each finding needs path (string), line (integer), severity (blocking or note) and message"},
			"no verdict":   {map[string]any{"verdict": "maybe", "findings": []any{}}, "submit_review needs verdict (approve or changes) and a findings list; verdict must be approve or changes but was a string with an unsupported value"},
			"no findings":  {map[string]any{"verdict": "approve"}, "submit_review needs verdict (approve or changes) and a findings list; findings must be a JSON array of objects but was missing"},
		}
		for i, row := range reviewShapeCases() {
			if row.accepted {
				continue
			}
			cases[fmt.Sprintf("matrix %d", i)] = struct {
				args any
				want string
			}{decodeReviewArgs(t, row.args), row.problem}
		}
		for name, tc := range cases {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				e := newEnv(t)
				f := e.prov
				f.then(landScript(f)...)
				f.then(f.calls("submit_review", tc.args), submit(f, "approve"))
				a := e.reviewed()
				eq(t, "exit", a.exit, 0)
				reqs := f.reviewRequests()
				eq(t, "review requests", len(reqs), 2)
				var outputs []string
				for _, raw := range requestInput(reqs[1]) {
					item, _ := raw.(map[string]any)
					if item["type"] == "function_call_output" {
						output, _ := item["output"].(string)
						outputs = append(outputs, output)
					}
				}
				eq(t, "submit_review tool output", outputs, []string{"invalid verdict:\n" + tc.want})
				if strings.Contains(strings.Join(outputs, ""), "PRIVATE_REVIEW_MARKER") {
					t.Fatal("private value disclosed")
				}
			})
		}
	})
	t.Run("a reply without a verdict is invalid, twice ends the review", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		f := e.prov
		f.then(landScript(f)...)
		say := sse(map[string]any{"type": "message", "id": "msg-1", "role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": "All good."}}})
		// Two replies end review 1; its one automatic fresh review (tested elsewhere) says the same.
		f.then(say, say, say, say)
		start := e.head()
		a := e.reviewed()
		eq(t, "exit", a.exit, 1)
		id := e.onlyRun()
		out := a.out.String()
		contains(t, "stdout", out, "failed fixture: review_invalid (Build "+id+")\ncandidate/review_invalid: the reviewer did not return a valid verdict; edits are left in place\n")
		contains(t, "stdout", out, "  still failing: review\n")
		eq(t, "head", e.head(), start)
		reqs := f.reviewRequests()
		eq(t, "review requests", len(reqs), 4)
		contains(t, "retry", requestText(reqs[1]), "Not a valid verdict: the review ended without calling submit_review. Call submit_review.")
		eq(t, "outcome", e.result(id)["outcome"], any("review_invalid"))
		eq(t, "reviews", e.result(id)["reviews"], any(float64(2)))
	})
}

func TestReviewerToolsAreProtectedLikeTheBuilders(t *testing.T) {
	t.Parallel()
	// ExUnit: test/integration/build_boundaries_test.exs: REV-6 reviewer shell edits cannot change the approved baseline
	e := newEnv(t)
	f := e.prov
	f.then(landScript(f)...)
	f.then(f.calls("shell", map[string]any{"command": "printf changed > .kogen/intents/fixture/acceptance.feature"},
		"submit_review", map[string]any{"verdict": "approve", "findings": []any{}}))
	start := e.head()
	a := e.reviewed()
	eq(t, "exit", a.exit, 1)
	contains(t, "stdout", a.shown(), "candidate/baseline_changed")
	eq(t, "head", e.head(), start)
	eq(t, "outcome", e.result(e.onlyRun())["outcome"], any("baseline_changed"))
	// write_file refuses a protected path.
	e2 := newEnv(t)
	f2 := e2.prov
	f2.then(landScript(f2)...)
	f2.then(f2.calls("write_file", map[string]any{"path": ".kogen/intents/fixture/acceptance.feature", "content": "x"}), submit(f2, "approve"))
	a2 := e2.reviewed()
	eq(t, "exit", a2.exit, 0)
	contains(t, "refusal", requestText(f2.reviewRequests()[1]), "refused")
	eq(t, "feature", e2.read(project.FeaturePath(testSlug)), testFeature)
}

func TestAnInterruptedReviewResumesInTheSameReviewerConversation(t *testing.T) {
	t.Parallel()
	// ExUnit: test/integration/build_boundaries_test.exs: resuming pending reviewer verification keeps repairs with the reviewer
	e := newEnv(t)
	f := e.prov
	f.then(landScript(f)...)
	opened := make(chan struct{})
	f.then(f.calls("read_file", map[string]any{"path": "app.txt"}), hang(opened))
	a, done := e.startReviewed()
	<-opened
	a.s.Interrupt("SIGINT")
	<-done
	eq(t, "exit", a.exit, 130)
	id := e.onlyRun()
	cont, session := sessionOfRun(t, e, id)
	eq(t, "stage", cont["stage"], any("review"))
	if session == nil || len(session["input"].([]any)) < 3 {
		t.Fatalf("session %v", session)
	}
	res := e.result(id)
	eq(t, "unknown requests", res["unknown_requests"], any(float64(1)))
	facts := e.jsonFile(filepath.Join(e.root, ".kogen/local/facts", id+".json"))
	eq(t, "resume status", facts["resume_status"], any("potentially_available"))
	eq(t, "requests so far", len(f.reviewRequests()), 2)

	f.replace(submit(f, "approve"))
	r := e.resumeReviewed(id)
	if r.exit != 0 {
		t.Fatalf("exit %d\n%s", r.exit, r.shown())
	}
	contains(t, "stdout", r.out.String(), "landed fixture ")
	reqs := f.reviewRequests()
	eq(t, "review requests", len(reqs), 3)
	if !reflect.DeepEqual(requestInput(reqs[1]), requestInput(reqs[2])) {
		t.Fatalf("the resumed request is not the interrupted one:\n%s\n%s", requestText(reqs[1]), requestText(reqs[2]))
	}
	if len(requestInput(reqs[2])) < 3 {
		t.Fatal("the read was not part of the conversation")
	}
	ids := e.runIDs()
	eq(t, "runs", len(ids), 2)
	child := ids[0]
	if child == id {
		child = ids[1]
	}
	cr := e.result(child)
	eq(t, "reviews", cr["reviews"], any(float64(1)))
	eq(t, "resumed_from", cr["resumed_from"], any(id))
	review := e.jsonFile(e.runPath(child, "review-1.json"))
	eq(t, "verdict", review["verdict"], any("approve"))
	// Builder requests: one. The reviewer's usage is cumulative.
	eq(t, "total requests", f.count(), 4)
	eq(t, "review input tokens", review["usage"].(map[string]any)["input"], any(float64(2)))
}

func TestAnInterruptedTargetedRepairReportsPendingReviewAndChangedVerification(t *testing.T) {
	t.Parallel()
	// ExUnit: test/integration/build_boundaries_test.exs: REV-5 interrupted targeted repair reports pending review and changed verification
	aux := t.TempDir()
	mutated := filepath.Join(aux, "check-mutated")
	e := newEnv(t, func(c *project.Config) {
		c.Checks = []project.Check{{Name: "mutate", Run: "if grep -qx 'reviewer repair' app.txt && ! test -f '" + mutated + "'; then printf 'check mutation\\n' > app.txt; touch '" + mutated + "'; fi"}}
	})
	e.approve()
	f := e.prov
	f.then(landScript(f)...)
	waiting := filepath.Join(aux, "reviewer-waiting")
	f.then(
		f.calls(append(writeApp("reviewer repair\n"), "submit_review", map[string]any{"verdict": "approve", "findings": []any{}})...),
		f.calls("shell", map[string]any{"command": "touch '" + waiting + "'; sleep 600"}),
	)
	a, done := e.startReviewed()
	t.Cleanup(func() { a.s.Interrupt("SIGINT"); <-done })
	waitForRunning(t, waiting, done, a)
	a.s.Interrupt("SIGINT")
	<-done
	eq(t, "exit", a.exit, 130)
	id := e.onlyRun()
	res := e.result(id)
	eq(t, "outcome", res["outcome"], any("interrupted"))
	still := []string{}
	for _, s := range res["still_failing"].([]any) {
		still = append(still, s.(string))
	}
	slices.Sort(still)
	if !slices.Equal(still, []string{"review", "verification"}) {
		t.Fatalf("still failing %v", still)
	}
	eq(t, "app.txt", e.read("app.txt"), "reviewer repair\n")
	review := e.jsonFile(e.runPath(id, "review-1.json"))
	if attempts := review["attempts"].([]any); len(attempts) != 1 || attempts[0].(map[string]any)["verdict"] != "approve" {
		t.Fatalf("attempts %v", attempts)
	}
	contains(t, "second request", requestText(f.reviewRequests()[0]), "# Check results")
	// The shell call is an uncertain tool boundary: it cannot be resumed.
	r := e.resumeReviewed(id)
	eq(t, "resume exit", r.exit, 1)
	contains(t, "resume", r.shown(), "candidate/resume_refused: Build "+id+": uncertain tool boundary")
}

func TestABlockedReviewStopsAndAnExplicitResumeReopensItUnderTheSameReviewer(t *testing.T) {
	t.Parallel()
	// ExUnit: test/integration/build_boundaries_test.exs: REV-5 unresolved substantive findings stop rather than start another builder round
	// ExUnit: .kogen/intents/reopen-blocked-review: explicit resume replaces terminal changes replay with fresh verification and same-reviewer explicit clearance
	e := newEnv(t)
	f := e.prov
	f.then(landScript(f)...)
	blocker := finding("blocking", "Original unresolved requirement")
	f.then(submit(f, "changes", blocker))
	start := e.head()
	a := e.reviewed()
	eq(t, "exit", a.exit, 1)
	id := e.onlyRun()
	out := a.out.String()
	contains(t, "stdout", out, "failed fixture: review_blocked (Build "+id+")\ncandidate/review_blocked: the reviewer could not resolve a substantive blocker within the approved Intent; edits are left in place\n  review: app.txt:1: Original unresolved requirement\n")
	contains(t, "stdout", out, "  still failing: review\n")
	eq(t, "head", e.head(), start)
	if _, err := os.Stat(project.ApprovalPath(e.root, testSlug)); err != nil {
		t.Fatalf("the approval was removed: %v", err)
	}
	res := e.result(id)
	eq(t, "outcome", res["outcome"], any("review_blocked"))
	eq(t, "reviews", res["reviews"], any(float64(1)))
	cont, _ := sessionOfRun(t, e, id)
	eq(t, "stage", cont["stage"], any("review_verify"))
	original, err := os.ReadFile(e.runPath(id, "checkpoint.json"))
	if err != nil {
		t.Fatal(err)
	}
	index := e.state()["index"]

	// Resume 1: fresh verification, the reviewer keeps the blocker: stops again.
	f.then(submit(f, "changes", blocker))
	r1 := e.resumeReviewed(id)
	eq(t, "exit", r1.exit, 1)
	contains(t, "stdout", r1.out.String(), "candidate/review_blocked: ")
	contains(t, "stdout", r1.out.String(), "  review: app.txt:1: Original unresolved requirement\n")
	eq(t, "head", e.head(), start)
	eq(t, "index", e.state()["index"], index)
	now, _ := os.ReadFile(e.runPath(id, "checkpoint.json"))
	if string(now) != string(original) {
		t.Fatal("the blocked run's checkpoint changed")
	}
	child := e.otherRun(id)
	cr := e.result(child)
	eq(t, "passes", cr["passes"], any(float64(2)))
	eq(t, "reviews", cr["reviews"], any(float64(1)))
	eq(t, "resumed_from", cr["resumed_from"], any(id))
	eq(t, "root_build", cr["root_build"], any(id))
	reqs := f.reviewRequests()
	eq(t, "review requests", len(reqs), 2)
	first, second := requestInput(reqs[0]), requestInput(reqs[1])
	if !reflect.DeepEqual(second[:len(first)], first) {
		t.Fatal("the reopened review is not the same conversation")
	}
	feedback := second[len(second)-1].(map[string]any)["content"].(string)
	for _, part := range []string{
		"The caller explicitly resumed this blocked Build. Continue as the same independent\nreviewer for targeted clearance or scoped repair, not a new whole-change critique.\n",
		"The original blocking finding remains unresolved until your new explicit decision:\n[{",
		"Original unresolved requirement",
		"Earlier green evidence and approval do not clear this blocker. Fresh verification:\nacceptance: passed (2 scenarios)\nmodel: not applicable\n",
		"edits still require ordinary external exact verification before commit.\n",
	} {
		contains(t, "feedback", feedback, part)
	}
	cont2, session2 := sessionOfRun(t, e, child)
	eq(t, "stage", cont2["stage"], any("review_verify"))
	if v := session2["verdict"].(map[string]any); v["verdict"] != "changes" {
		t.Fatalf("verdict %v", v)
	}

	// Resume 2: the reviewer clears the blocker; the unchanged green tree commits.
	f.then(submit(f, "approve"))
	r2 := e.resumeReviewed(child)
	if r2.exit != 0 {
		t.Fatalf("exit %d\n%s", r2.exit, r2.shown())
	}
	contains(t, "stdout", r2.out.String(), "landed fixture ")
	eq(t, "commits", e.gitOut("rev-list", "--count", start+"..HEAD"), "1")
	last := e.otherRun(id, child)
	lr := e.result(last)
	eq(t, "passes", lr["passes"], any(float64(3)))
	eq(t, "reviews", lr["reviews"], any(float64(1)))
	eq(t, "root_build", lr["root_build"], any(id))
	review := e.jsonFile(e.runPath(last, "review-1.json"))
	attempts := review["attempts"].([]any)
	if len(attempts) != 3 || attempts[0].(map[string]any)["verdict"] != "changes" || attempts[1].(map[string]any)["verdict"] != "changes" || attempts[2].(map[string]any)["verdict"] != "approve" {
		t.Fatalf("attempts %v", attempts)
	}
	eq(t, "builder requests", f.count()-len(f.reviewRequests()), 1)
	if _, err := os.Stat(project.ApprovalPath(e.root, testSlug)); !os.IsNotExist(err) {
		t.Fatal("the approval stayed after the commit")
	}
}

// blockedReviewOfAnExternalRepair stops a Build at a blocking review verdict and returns its id.
func blockedReviewOfAnExternalRepair(t *testing.T) (*env, string, string) {
	e := newEnv(t)
	f := e.prov
	f.then(landScript(f)...)
	f.then(submit(f, "changes", finding("blocking", "Original unresolved requirement")))
	start := e.head()
	a := e.reviewed()
	eq(t, "exit", a.exit, 1)
	return e, e.onlyRun(), start
}

func TestAnExternallyRepairedBlockedReviewIsVerifiedFreshAndAsksTheSameReviewer(t *testing.T) {
	t.Parallel()
	for _, approve := range []bool{true, false} {
		name := "maintained"
		if approve {
			name = "cleared"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e, id, start := blockedReviewOfAnExternalRepair(t)
			f := e.prov
			e.write("app.txt", "repaired\n")
			e.write("extra.txt", "new untracked file\n")
			if approve {
				f.then(submit(f, "approve"))
			} else {
				f.then(submit(f, "changes", finding("blocking", "Original unresolved requirement")))
			}
			r := e.resumeReviewed(id)
			if !approve {
				eq(t, "exit", r.exit, 1)
				contains(t, "stdout", r.out.String(), "candidate/review_blocked: ")
				eq(t, "head", e.head(), start)
				eq(t, "app.txt", e.read("app.txt"), "repaired\n")
				eq(t, "extra.txt", e.read("extra.txt"), "new untracked file\n")
				cont, session := sessionOfRun(t, e, e.otherRun(id))
				eq(t, "stage", cont["stage"], any("review_verify"))
				if v := session["verdict"].(map[string]any); v["verdict"] != "changes" {
					t.Fatalf("verdict %v", v)
				}
			}
			reqs := f.reviewRequests()
			eq(t, "review requests", len(reqs), 2)
			feedback := requestInput(reqs[1])
			text := feedback[len(feedback)-1].(map[string]any)["content"].(string)
			contains(t, "feedback", text, "Original unresolved requirement")
			contains(t, "feedback", text, "repaired")
			contains(t, "feedback", text, "Fresh verification:\nacceptance: passed")
			res := e.result(e.otherRun(id))
			eq(t, "passes", res["passes"], any(float64(2)))
			if approve {
				if r.exit != 0 {
					t.Fatalf("exit %d\n%s", r.exit, r.shown())
				}
				eq(t, "commits", e.gitOut("rev-list", "--count", start+"..HEAD"), "1")
				eq(t, "committed app", e.gitOut("show", "HEAD:app.txt"), "repaired")
			}
		})
	}
}

func TestARepairThatChangesCustodyOfABlockedReviewIsStillRefusedAsDrift(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		prep func(e *env)
		want string
	}{
		{"protected file", func(e *env) { e.write(stepsFile, "// changed\n") }, "baseline changed: " + stepsFile},
		{"staged index", func(e *env) { e.gitOut("add", "app.txt") }, "index/staged custody changed"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			e, id, start := blockedReviewOfAnExternalRepair(t)
			f := e.prov
			before := f.count()
			e.write("app.txt", "repaired\n")
			c.prep(e)
			r := e.resumeReviewed(id)
			eq(t, "exit", r.exit, 1)
			contains(t, "stdout", r.shown(), "candidate/resume_refused: Build "+id+": "+c.want)
			eq(t, "requests", f.count(), before)
			eq(t, "runs", len(e.runIDs()), 1)
			eq(t, "head", e.head(), start)
		})
	}
}

func (e *env) otherRun(except ...string) string {
	e.t.Helper()
	for _, id := range e.runIDs() {
		if !slices.Contains(except, id) {
			return id
		}
	}
	e.t.Fatal("no other run")
	return ""
}

func TestAReopenedReviewSeesFreshRedResultsAndStaleApprovalCannotClearIt(t *testing.T) {
	t.Parallel()
	// ExUnit: .kogen/intents/reopen-blocked-review: complete fresh verification is required and stale green/approval cannot override unresolved, fresh-red or setup-failed blockers
	flag := filepath.Join(t.TempDir(), "gate-red")
	e := newEnv(t, func(c *project.Config) {
		c.Checks = []project.Check{{Name: "gate", Run: "test ! -f '" + flag + "'"}}
	})
	e.approve()
	f := e.prov
	f.then(landScript(f)...)
	f.then(submit(f, "changes", finding("blocking", "Original unresolved requirement")))
	start := e.head()
	a := e.reviewed()
	eq(t, "exit", a.exit, 1)
	id := e.onlyRun()
	writeFile(t, flag, "red", 0o644)
	f.then(submit(f, "changes", finding("blocking", "Still unresolved")))
	r := e.resumeReviewed(id)
	eq(t, "exit", r.exit, 1)
	out := r.out.String()
	contains(t, "stdout", out, "  review: app.txt:1: Still unresolved\n")
	contains(t, "stdout", out, "  still failing: gate, review\n")
	eq(t, "head", e.head(), start)
	child := e.otherRun(id)
	reqs := f.reviewRequests()
	feedback := requestInput(reqs[1])
	text := feedback[len(feedback)-1].(map[string]any)["content"].(string)
	contains(t, "feedback", text, "Verification pass 2 is not green.")
	contains(t, "feedback", text, "check gate (test ! -f '"+flag+"'): failed, exit status 1")
	lacks(t, "feedback", text, "acceptance: passed")
	res := e.result(child)
	eq(t, "passes", res["passes"], any(float64(2)))

	// With the failure gone, an approval after fresh green verification lands.
	os.Remove(flag)
	f.then(submit(f, "approve"))
	r2 := e.resumeReviewed(child)
	if r2.exit != 0 {
		t.Fatalf("exit %d\n%s", r2.exit, r2.shown())
	}
	eq(t, "commits", e.gitOut("rev-list", "--count", start+"..HEAD"), "1")
	eq(t, "passes", e.result(e.otherRun(id, child))["passes"], any(float64(3)))
}

func TestABlockedRetryThatFailsAtTheContinuedRequestKeepsFreshEvidenceAndTheSavedReviewer(t *testing.T) {
	t.Parallel()
	// ExUnit: test/integration/build_boundaries_test.exs: blocked #{retry_outcome} retry provider failure retains fresh evidence and the saved reviewer
	for _, setupRed := range []bool{false, true} {
		name := "green"
		if setupRed {
			name = "setup red"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			aux := t.TempDir()
			trace, red := filepath.Join(aux, "setup-trace"), filepath.Join(aux, "setup-red")
			e := newEnv(t, func(c *project.Config) {
				c.Setup = []string{"echo setup >> '" + trace + "'; if test -f '" + red + "'; then echo retry-setup-failure; exit 23; fi"}
			})
			e.approve()
			f := e.prov
			f.then(landScript(f)...)
			blocker := finding("blocking", "Original unresolved requirement")
			f.then(submit(f, "changes", blocker), failure(400, "invalid_request_error", "continued request failed"), submit(f, "approve"))
			a := e.reviewed()
			eq(t, "exit", a.exit, 1)
			contains(t, "stdout", a.shown(), "review_blocked")
			original := e.onlyRun()
			originalCheckpoint, _ := os.ReadFile(e.runPath(original, "checkpoint.json"))
			writeFile(t, trace, "", 0o644)
			if setupRed {
				writeFile(t, red, "red", 0o644)
			}
			r := e.resumeReviewed(original)
			if r.exit == 0 {
				t.Fatalf("a failed continued request exits 0\n%s", r.shown())
			}
			child := e.otherRun(original)
			cr := e.result(child)
			eq(t, "passes", cr["passes"], any(float64(2)))
			eq(t, "unknown requests", cr["unknown_requests"], any(float64(1)))
			if !slices.Contains(toStrings(cr["still_failing"]), "review") {
				t.Fatalf("still failing %v", cr["still_failing"])
			}
			b, _ := os.ReadFile(e.runPath(child, "checkpoint.json"))
			childCheckpoint := string(b)
			eq(t, "setup runs", readString(t, trace), "setup\n")
			cont, _ := sessionOfRun(t, e, child)
			eq(t, "stage", cont["stage"], any("review"))
			pass := cont["pass"].(map[string]any)
			if setupRed {
				eq(t, "pass green", pass["green"], any(false))
				if !reflect.DeepEqual(pass["not_run"], []any{"acceptance"}) {
					t.Fatalf("not_run %v", pass["not_run"])
				}
				if setup := pass["setup"].([]any); len(setup) != 1 || setup[0].(map[string]any)["status"] != float64(23) {
					t.Fatalf("setup %v", setup)
				}
				os.Remove(red)
			} else {
				eq(t, "pass green", pass["green"], any(true))
			}

			r2 := e.resumeReviewed(child)
			if r2.exit != 0 {
				t.Fatalf("exit %d\n%s", r2.exit, r2.shown())
			}
			final := e.otherRun(original, child)
			fr := e.result(final)
			want := 2.0
			wantTrace := "setup\n"
			if setupRed {
				want, wantTrace = 3, "setup\nsetup\n"
			}
			eq(t, "passes", fr["passes"], any(want))
			eq(t, "setup runs", readString(t, trace), wantTrace)
			eq(t, "unknown requests", fr["unknown_requests"], any(float64(1)))
			eq(t, "usage input", fr["usage"].(map[string]any)["input"], any(cr["usage"].(map[string]any)["input"].(float64)+1))
			if now, _ := os.ReadFile(e.runPath(child, "checkpoint.json")); string(now) != childCheckpoint {
				t.Fatal("the failed run's checkpoint changed")
			}
			eq(t, "reviews", fr["reviews"], any(float64(1)))
			if now, _ := os.ReadFile(e.runPath(original, "checkpoint.json")); string(now) != string(originalCheckpoint) {
				t.Fatal("the original checkpoint changed")
			}
			reqs := f.reviewRequests()
			eq(t, "review requests", len(reqs), 3)
			initial, failed, continued := requestInput(reqs[0]), requestInput(reqs[1]), requestInput(reqs[2])
			if !reflect.DeepEqual(failed, continued) {
				t.Fatal("the resent request differs from the failed one")
			}
			if !reflect.DeepEqual(continued[:len(initial)], initial) {
				t.Fatal("the continued request does not extend the initial conversation")
			}
			contains(t, "continued", requestText(reqs[2]), blocker["message"].(string))
			if setupRed {
				feedback := continued[len(continued)-1].(map[string]any)["content"].(string)
				contains(t, "feedback", feedback, "retry-setup-failure")
				contains(t, "feedback", feedback, "exit status 23")
				contains(t, "feedback", feedback, "not run because an earlier item failed: acceptance")
			}
		})
	}
}

func toStrings(v any) []string {
	var out []string
	for _, x := range v.([]any) {
		out = append(out, x.(string))
	}
	return out
}

func readString(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestTheReviewerReadsTheFullSourceAndNewFilesAppearOnce(t *testing.T) {
	t.Parallel()
	// ExUnit: test/integration/build_boundaries_test.exs: REV-2 review retains full source without replaying new files twice
	e := newEnv(t)
	before := "outside-diff-sentinel\n" + strings.Repeat("context\n", 20) + "old marker\n"
	after := strings.ReplaceAll(before, "old marker", "new marker")
	e.write("existing-context.txt", before)
	e.write("rename-from.txt", "renamed-source-sentinel\n")
	e.run("git", "add", "existing-context.txt", "rename-from.txt")
	e.run("git", "commit", "--only", "-qm", "Add review context fixture", "--", "existing-context.txt", "rename-from.txt")
	e.approve()
	f := e.prov
	f.then(f.calls(
		"write_file", map[string]any{"path": "new-source.txt", "content": "new-source-sentinel\n"},
		"write_file", map[string]any{"path": "existing-context.txt", "content": after},
		"shell", map[string]any{"command": "chmod 755 new-source.txt && printf '\\000\\001\\002' > new-data.bin && mv rename-from.txt rename-to.txt"},
		"shell", map[string]any{"command": "git init -q new-submodule && git -C new-submodule -c user.name=Fixture -c user.email=fixture@example.test -c commit.gpgsign=false commit --allow-empty -qm 'Nested fixture'"},
		"write_file", map[string]any{"path": "app.txt", "content": "candidate\n"},
		"finish", map[string]any{"summary": "done"},
	))
	f.then(submit(f, "approve"))
	a := e.reviewed()
	if a.exit != 0 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	reqs := f.reviewRequests()
	eq(t, "review requests", len(reqs), 1)
	material := requestText(reqs[0])
	// The text is JSON-escaped in requestText: decode the blocks instead.
	blocks := requestInput(reqs[0])[0].(map[string]any)["content"].([]any)
	material = blocks[0].(map[string]any)["text"].(string) + blocks[1].(map[string]any)["text"].(string)
	count := func(s string) int { return strings.Count(material, s) }
	eq(t, "new-source-sentinel", count("new-source-sentinel"), 1)
	contains(t, "material", material, "100755")
	contains(t, "material", material, "--- new-data.bin (added; mode 100644) (binary)")
	contains(t, "material", material, "rename from rename-from.txt\nrename to rename-to.txt")
	contains(t, "material", material, "--- rename-to.txt (from rename-from.txt) (changed; hunks in diff)")
	lacks(t, "material", material, "--- rename-from.txt (deleted)")
	eq(t, "renamed-source-sentinel", count("renamed-source-sentinel"), 0)
	submodule := strings.TrimSpace(e.run("git", "-C", "new-submodule", "rev-parse", "HEAD"))
	contains(t, "material", material, "--- new-submodule (added; mode 160000)\nSubproject commit "+submodule)
	lacks(t, "material", material, "--- new-submodule (deleted)")
	contains(t, "material", material, "-old marker")
	contains(t, "material", material, "+new marker")
	// A changed file is in the source by its hunks: its full text is read in the checkout.
	lacks(t, "material", material, after)
	eq(t, "outside-diff-sentinel", count("outside-diff-sentinel"), 0)
}

// readPinned reads, one per response, the source parts pinned in the run folders, then approves.
// One handler keeps its place for every request it answers.
func readPinned(e *env, f *fakeProvider) handler {
	next := 0
	return func(w http.ResponseWriter, r *http.Request) {
		parts, _ := filepath.Glob(filepath.Join(e.root, ".kogen/local/runs/*/review-source/*/*.json"))
		slices.Sort(parts)
		if next >= len(parts) {
			submit(f, "approve")(w, r)
			return
		}
		path := parts[next]
		next++
		f.calls("read_file", map[string]any{"path": path})(w, r)
	}
}

func TestTheReviewAndItsResetsCarryOnlyTheFrozenCallerDiagnostic(t *testing.T) {
	t.Parallel()
	// ExUnit: test/integration/build_context_files_test.exs: fresh independent review and its resets retain only frozen caller diagnostics, not builder reasoning
	e := newEnv(t)
	f := e.prov
	f.then(f.calls("finish", map[string]any{"summary": "builder-only-reasoning-sentinel"}))
	// The builder finished with the unchanged file: the baseline is green; the reviewer grows its
	// conversation past the working window three times; its candidate source, still inline in
	// its opening message, is then pinned to private parts it must read before approving.
	for i := 0; i < 3; i++ {
		f.then(f.calls("write_file", map[string]any{"path": ".kogen/local/padding", "content": strings.Repeat("x", 190_000)}))
	}
	f.then(f.calls("read_file", map[string]any{"path": "app.txt"}))
	shared := readPinned(e, f)
	for i := 0; i < 40; i++ {
		f.then(shared)
	}
	text := "  frozen selective reuse Δ🙂\r\nnot approval or old green proof\n"
	d := &Diagnostic{Text: text, SourcePath: "/diagnostic/label.txt", SHA256: digestOf(text)}
	a := e.reviewed(func(in *Input) { in.Context = d })
	if a.exit != 0 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	contains(t, "progress", a.errs.String(), "review local reset")
	parts, _ := filepath.Glob(filepath.Join(e.root, ".kogen/local/runs/*/review-source/*/*.json"))
	if len(parts) == 0 {
		t.Fatal("the inline source was never pinned")
	}
	reqs := f.reviewRequests()
	eq(t, "review requests", len(reqs), 5+len(parts))
	for i, req := range reqs {
		var all strings.Builder
		for _, it := range requestInput(req) {
			m := it.(map[string]any)
			switch c := m["content"].(type) {
			case string:
				all.WriteString(c)
			case []any:
				for _, b := range c {
					all.WriteString(b.(map[string]any)["text"].(string))
				}
			}
		}
		material := all.String()
		if !strings.Contains(material, text) || !strings.Contains(material, "lower-authority data") {
			t.Fatalf("request %d lacks the frozen diagnostic", i+1)
		}
		lacks(t, "request", requestText(req), "builder-only-reasoning-sentinel")
	}
}

// ---- large source: pinned parts ----

// reviewCatalog is the pinned source catalog of a review request.
func reviewCatalog(t *testing.T, req map[string]any) map[string]any {
	t.Helper()
	blocks := requestInput(req)[0].(map[string]any)["content"].([]any)
	text := blocks[1].(map[string]any)["text"].(string)
	_, rest, ok := strings.Cut(text, "# Pinned complete candidate source evidence\n")
	if !ok {
		t.Fatalf("no catalog in %.200s", text)
	}
	line, _, _ := strings.Cut(rest, "\n")
	var catalog map[string]any
	if err := json.Unmarshal([]byte(line), &catalog); err != nil {
		t.Fatal(err)
	}
	return catalog
}

// pinnedPartFiles are the part files kogen-build's materialPrepare pinned for review n of the
// checkout's run (<run dir>/review-source/<n>-<candidate tree>/<part>.json), ordered by part
// number. A scripted reviewer takes the paths it reads from them, not from anything the
// reviewer's requests say about the parts.
func pinnedPartFiles(root string, review int) []string {
	files, _ := filepath.Glob(filepath.Join(root, ".kogen/local/runs/*/review-source", itoa(review)+"-*", "*.json"))
	slices.SortFunc(files, func(a, b string) int { return partNumber(a) - partNumber(b) })
	return files
}

// partNumber is the n of a part file <n>.json.
func partNumber(path string) int {
	n, _ := strconv.Atoi(strings.TrimSuffix(filepath.Base(path), ".json"))
	return n
}

// latestPinnedReview is the highest review number that has pinned parts in the checkout's run:
// the review a request being answered belongs to, as kogen-build pins a review's parts before
// its first request.
func latestPinnedReview(root string) int {
	dirs, _ := filepath.Glob(filepath.Join(root, ".kogen/local/runs/*/review-source/*"))
	latest := 0
	for _, dir := range dirs {
		number, _, _ := strings.Cut(filepath.Base(dir), "-")
		if n, err := strconv.Atoi(number); err == nil {
			latest = max(latest, n)
		}
	}
	return latest
}

// latestPinnedParts are the part files of the review being answered.
func latestPinnedParts(root string) []string { return pinnedPartFiles(root, latestPinnedReview(root)) }

// prefilledNumbers are the part numbers of a review that kogen-build's native prefill messages
// carried so far, from the run's review_source_prefill events.
func prefilledNumbers(events []map[string]any, review int) map[int]bool {
	out := map[int]bool{}
	for _, ev := range eventsNamed(events, "review_source_prefill") {
		if ev["review"] != float64(review) {
			continue
		}
		for _, n := range ev["parts"].([]any) {
			out[int(n.(float64))] = true
		}
	}
	return out
}

// materialEventsOf are the review_material events of a review: one for each verdict it
// accepted, with the number of its pinned parts a completed conversation had delivered.
func materialEventsOf(events []map[string]any, review int) []map[string]any {
	var out []map[string]any
	for _, ev := range eventsNamed(events, "review_material") {
		if ev["review"] == float64(review) {
			out = append(out, ev)
		}
	}
	return out
}

// bigFile is a candidate whose diff alone, a changed file's text no longer being repeated in the
// review source, does not fit the reviewer's working window and is pinned as private parts.
func bigFile() string { return strings.Repeat("0123456789abcdef-line\n", 28_000) }

// TestLargeSourceIsDeliveredAsPinnedPartsAndMustBeReadBeforeApproval: a candidate whose review
// source does not fit the reviewer's window is pinned as private part files that together hold
// the whole source; a reviewer that reads every part approves, and the candidate lands.
func TestLargeSourceIsDeliveredAsPinnedPartsAndMustBeReadBeforeApproval(t *testing.T) {
	t.Parallel()
	// ExUnit: test/integration/review_material_test.exs: exact UTF8 source delivery survives reset and damaged custody refuses
	e := newEnv(t)
	f := e.prov
	content := bigFile()
	f.then(f.calls("write_file", map[string]any{"path": "app.txt", "content": content}, "finish", map[string]any{"summary": "done"}))
	// One handler keeps the reading's progress for every request.
	shared := readThen(e, f, "approve")
	for i := 0; i < 30; i++ {
		f.then(shared)
	}
	a := e.reviewed()
	if a.exit != 0 {
		t.Fatalf("exit %d\n%s\n%s", a.exit, a.shown(), a.errs)
	}
	id := e.onlyRun()
	tree := e.gitOut("rev-parse", "HEAD^{tree}")
	parts := pinnedPartFiles(e.root, 1)
	if len(parts) < 20 {
		t.Fatalf("%d parts", len(parts))
	}
	var source strings.Builder
	for n, p := range parts {
		eq(t, "part path", p, filepath.Join(e.root, ".kogen/local/runs", id, "review-source", "1-"+tree, itoa(n+1)+".json"))
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if st, _ := os.Stat(p); st.Mode().Perm() != 0o600 {
			t.Fatalf("mode %v", st.Mode().Perm())
		}
		if st, _ := os.Stat(filepath.Dir(p)); st.Mode().Perm() != 0o700 {
			t.Fatalf("folder mode %v", st.Mode().Perm())
		}
		var doc struct {
			Input        []any
			ReviewSource struct {
				Review, Part, Parts int
				Text                string
			} `json:"review_source"`
		}
		if err := json.Unmarshal(data, &doc); err != nil || len(doc.Input) != 0 {
			t.Fatalf("part %d: %v", n+1, err)
		}
		eq(t, "review", doc.ReviewSource.Review, 1)
		eq(t, "part number", doc.ReviewSource.Part, n+1)
		eq(t, "parts", doc.ReviewSource.Parts, len(parts))
		source.WriteString(doc.ReviewSource.Text)
	}
	contains(t, "source", source.String(), "# Diff of existing/deleted paths against start HEAD "+short(e.gitOut("rev-parse", "HEAD^")))
	contains(t, "source", source.String(), "+"+strings.ReplaceAll(strings.TrimSuffix(content, "\n"), "\n", "\n+"))

	// Every request fits the working window kogen-build keeps.
	for _, req := range f.requests() {
		raw, _ := json.Marshal(req)
		if len(raw) > 524_288+4096 {
			t.Fatalf("a request of %d bytes", len(raw))
		}
	}
	evidence := eventsNamed(e.events(id), "review_source_evidence")
	if len(evidence) != 1 || evidence[0]["parts"] != float64(len(parts)) {
		t.Fatalf("evidence %v", evidence)
	}
	review := e.jsonFile(e.runPath(id, "review-1.json"))
	eq(t, "verdict", review["verdict"], any("approve"))
	eq(t, "committed", e.gitOut("show", "HEAD:app.txt")+"\n", content)
}

// TestApprovalNeedsEveryPinnedPartDeliveredInACompletedContinuation: a scripted reviewer that
// approves while a pinned part is undelivered (a read and an approval in one response deliver
// nothing) is refused, the refusal names that part, and the candidate does not land while the
// part is unread.
func TestApprovalNeedsEveryPinnedPartDeliveredInACompletedContinuation(t *testing.T) {
	t.Parallel()
	// ExUnit: test/integration/review_material_test.exs: exact UTF8 source delivery survives reset and damaged custody refuses (delivery before approval)
	e := newEnv(t)
	f := e.prov
	f.then(f.calls("write_file", map[string]any{"path": "app.txt", "content": bigFile()}, "finish", map[string]any{"summary": "done"}))
	var manual, broken string
	f.then(func(w http.ResponseWriter, r *http.Request) {
		// Kogen's own prefill carries the unread parts from the front; its log shows which parts
		// the first request carried. P is the first, in path order, of those it did not: the
		// refusal lists unread parts in path order.
		carried := prefilledNumbers(e.events(e.onlyRun()), 1)
		parts := latestPinnedParts(e.root)
		slices.Sort(parts)
		for _, p := range parts {
			if !carried[partNumber(p)] {
				manual = p
				break
			}
		}
		if manual == "" {
			broken = "kogen-build's prefill delivered every pinned part to the first request"
			submit(f, "approve")(w, r)
			return
		}
		// A read and an approval in one response deliver nothing.
		f.calls("read_file", map[string]any{"path": manual}, "submit_review", map[string]any{"verdict": "approve", "findings": []any{}})(w, r)
	})
	f.then(repeatHandler(submit(f, "approve"), 8)...)
	start := e.head()
	a := e.reviewed()
	if broken != "" {
		t.Fatalf("the fixture is broken: %s", broken)
	}
	id := e.onlyRun()
	events := e.events(id)
	// The refusal reached the reviewer and names P: in what Kogen returned for that
	// submit_review call, or in the problems recorded for the invalid review.
	var call string
	for _, req := range f.reviewRequests() {
		for _, raw := range requestInput(req) {
			if item, _ := raw.(map[string]any); item["type"] == "function_call" && item["name"] == "submit_review" && call == "" {
				call, _ = item["call_id"].(string)
			}
		}
	}
	named := false
	for _, req := range f.reviewRequests() {
		for _, raw := range requestInput(req) {
			if item, _ := raw.(map[string]any); item["type"] == "function_call_output" && item["call_id"] == call && call != "" {
				named = named || strings.Contains(fmt.Sprint(item["output"]), manual)
			}
		}
	}
	for _, ev := range eventsNamed(events, "review") {
		problems, _ := ev["problems"].([]any)
		for _, p := range problems {
			named = named || strings.Contains(fmt.Sprint(p), manual)
		}
	}
	if !named {
		t.Fatalf("the refusal of the approval does not name the undelivered part %s", manual)
	}

	// No accepted verdict came while a part was undelivered, and the candidate did not land with one.
	for _, ev := range eventsNamed(events, "review_material") {
		eq(t, "parts read at an accepted verdict of review "+fmt.Sprint(ev["review"]), ev["parts_read"], ev["parts"])
	}
	switch a.exit {
	case 1:
		contains(t, "stdout", a.out.String(), "failed fixture: review_invalid (Build "+id+")\ncandidate/review_invalid: the reviewer did not return a valid verdict; edits are left in place\n")
		eq(t, "head", e.head(), start)
		if _, err := os.Stat(e.runPath(id, "review-1.json")); !os.IsNotExist(err) {
			t.Fatal("an invalid review has a review record")
		}
	case 0:
		// A later approval may land, once P has been delivered.
		n := int(e.result(id)["reviews"].(float64))
		eq(t, "approved review", e.jsonFile(e.runPath(id, "review-"+itoa(n)+".json"))["verdict"], any("approve"))
		material := materialEventsOf(events, n)
		if len(material) == 0 || material[len(material)-1]["parts_read"] != float64(len(pinnedPartFiles(e.root, n))) {
			t.Fatalf("landed with undelivered parts: %v", material)
		}
	default:
		t.Fatalf("exit %d\n%s\n%s", a.exit, a.shown(), a.errs)
	}
}

func TestSmallSourceIsDeliveredInline(t *testing.T) {
	t.Parallel()
	// ExUnit: test/integration/review_material_test.exs: small source remains inline without new custody
	e := newEnv(t)
	f := e.prov
	f.then(landScript(f)...)
	f.then(submit(f, "approve"))
	a := e.reviewed()
	eq(t, "exit", a.exit, 0)
	id := e.onlyRun()
	if _, err := os.Stat(filepath.Join(e.root, ".kogen/local/runs", id, "review-source")); !os.IsNotExist(err) {
		t.Fatal("source custody was created")
	}
	if n := len(eventsNamed(e.events(id), "review_source_evidence")); n != 0 {
		t.Fatalf("%d evidence events", n)
	}
}

func TestTheReviewerHasItsOwnImageAccess(t *testing.T) {
	t.Parallel()
	// ExUnit: .kogen/intents/native-image-evidence: resume frozen Build pixels and use fresh reviewer access (the reviewer's own view_image read and delivery)
	e := newEnv(t)
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for i := 0; i < 4; i++ {
		img.Set(i%2, i/2, color.RGBA{R: uint8(60 * i), G: 120, B: 200, A: 255})
	}
	var pixels bytes.Buffer
	if err := png.Encode(&pixels, img); err != nil {
		t.Fatal(err)
	}
	shot := ".kogen/local/shot.png"
	writeFile(t, filepath.Join(e.root, shot), pixels.String(), 0o644)
	f := e.prov
	f.then(landScript(f)...)
	f.then(f.calls("view_image", map[string]any{"path": shot}), submit(f, "approve"))
	a := e.reviewed()
	if a.exit != 0 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	id := e.onlyRun()
	events := e.events(id)
	reads := eventsNamed(events, "image_read")
	if len(reads) != 1 || reads[0]["owner"] != "review" || reads[0]["status"] != "read" {
		t.Fatalf("image_read %v", reads)
	}
	deliveries := eventsNamed(events, "image_delivery")
	if len(deliveries) != 1 || deliveries[0]["owner"] != "review" || deliveries[0]["status"] != "completed" {
		t.Fatalf("image_delivery %v", deliveries)
	}
	eq(t, "sha256", deliveries[0]["sha256"], reads[0]["sha256"])
	// The pixels reach the reviewer's own continuation, never the builder's conversation.
	reqs := f.reviewRequests()
	eq(t, "review requests", len(reqs), 2)
	contains(t, "second request", requestText(reqs[1]), "input_image")
	lacks(t, "first request", requestText(reqs[0]), "input_image")
}

func TestAnInterruptedVerificationOfAReviewerRepairResumesWithoutANewRequest(t *testing.T) {
	t.Parallel()
	// ExUnit: test/integration/build_boundaries_test.exs: resuming pending reviewer verification keeps repairs with the reviewer
	paused := filepath.Join(t.TempDir(), "paused")
	e := newEnv(t, func(c *project.Config) {
		c.Checks = []project.Check{{Name: "pause", Run: "if grep -qx 'reviewer repair' app.txt && ! test -f '" + paused + "'; then touch '" + paused + "'; sleep 600; fi"}}
	})
	f := e.prov
	f.then(landScript(f)...)
	f.then(f.calls(append(writeApp("reviewer repair\n"), "submit_review", map[string]any{"verdict": "approve", "findings": []any{}})...))
	a, done := e.startReviewed()
	t.Cleanup(func() { a.s.Interrupt("SIGINT"); <-done })
	waitForRunning(t, paused, done, a)
	a.s.Interrupt("SIGINT")
	<-done
	eq(t, "exit", a.exit, 130)
	id := e.onlyRun()
	contains(t, "stdout", a.out.String(), "  still failing: review\n")
	cont, session := sessionOfRun(t, e, id)
	eq(t, "stage", cont["stage"], any("review_verify"))
	if v := session["verdict"].(map[string]any); v["verdict"] != "approve" {
		t.Fatalf("verdict %v", v)
	}
	original := e.result(id)
	requests := f.count()
	r := e.resumeReviewed(id)
	if r.exit != 0 {
		t.Fatalf("exit %d\n%s", r.exit, r.shown())
	}
	contains(t, "stdout", r.out.String(), "resuming Build "+id)
	contains(t, "stdout", r.out.String(), "landed fixture ")
	eq(t, "requests", f.count(), requests)
	eq(t, "committed", e.gitOut("show", "HEAD:app.txt"), "reviewer repair")
	if now := e.result(id); !reflect.DeepEqual(now, original) {
		t.Fatal("the interrupted run's result changed")
	}
	child := e.otherRun(id)
	eq(t, "reviews", e.result(child)["reviews"], any(float64(1)))
	review := e.jsonFile(e.runPath(child, "review-1.json"))
	eq(t, "verdict", review["verdict"], any("approve"))
}

func TestAReopenedReviewersResetsPreserveTheUnresolvedBlockerAndCannotAuthorizeACommit(t *testing.T) {
	t.Parallel()
	// ExUnit: test/integration/build_boundaries_test.exs: reopened reviewer resets preserve the unresolved blocker and cannot authorize commit
	e := newEnv(t)
	f := e.prov
	f.then(landScript(f)...)
	blocker := finding("blocking", "Unresolved exact requirement")
	verdict := submit(f, "changes", blocker)
	f.then(verdict)
	start := e.head()
	a := e.reviewed()
	eq(t, "exit", a.exit, 1)
	id := e.onlyRun()
	for i := 0; i < 3; i++ {
		f.then(f.calls("write_file", map[string]any{"path": ".kogen/local/padding", "content": strings.Repeat("x", 190_000)}))
	}
	f.then(verdict)
	index := e.state()["index"]
	r := e.resumeReviewed(id)
	eq(t, "exit", r.exit, 1)
	contains(t, "stdout", r.out.String(), "review_blocked")
	eq(t, "head", e.head(), start)
	eq(t, "index", e.state()["index"], index)
	var reset map[string]any
	for _, req := range f.reviewRequests() {
		for _, it := range requestInput(req) {
			m, _ := it.(map[string]any)
			if s, ok := m["content"].(string); ok && strings.HasPrefix(s, "Local working-window reset\n") {
				reset = map[string]any{}
				if err := json.Unmarshal([]byte(strings.TrimPrefix(s, "Local working-window reset\n")), &reset); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	if reset == nil {
		t.Fatal("no reset reached the reviewer")
	}
	eq(t, "owner", reset["owner"], any("review"))
	findings := reset["findings"].([]any)
	if len(findings) != 1 || findings[0].(map[string]any)["message"] != "Unresolved exact requirement" {
		t.Fatalf("findings %v", findings)
	}
	child := e.otherRun(id)
	_, state := e.checkpoint(child)
	last := state["last_review"].(map[string]any)
	eq(t, "verdict", last["verdict"], any("changes"))
	eq(t, "stage", state["continuation"].(map[string]any)["stage"], any("review_verify"))
	if _, err := os.Stat(project.ApprovalPath(e.root, testSlug)); err != nil {
		t.Fatal("the approval was removed")
	}
}

func TestAResumedFittedReviewPinsItsSourceWhenTheConversationOutgrowsTheWindow(t *testing.T) {
	t.Parallel()
	// Intent review-refine-working-room: resume a fitted inline Review with room for investigation and final repair
	e := newEnv(t)
	f := e.prov
	f.then(f.calls("finish", map[string]any{"summary": "done"}), refused())
	a := e.reviewed()
	eq(t, "first exit", a.exit, 4)
	id := e.onlyRun()
	_, state := e.checkpoint(id)
	owner := state["working_context"].(map[string]any)["review"].(map[string]any)
	if owner["inline_material"] == nil || owner["source_tree"] == nil {
		t.Fatalf("the saved review context lacks its inline material: %v", owner)
	}
	for i := 0; i < 3; i++ {
		f.then(f.calls("write_file", map[string]any{"path": ".kogen/local/padding", "content": strings.Repeat("x", 190_000)}))
	}
	f.then(f.calls("read_file", map[string]any{"path": "app.txt"}))
	shared := readPinned(e, f)
	for i := 0; i < 40; i++ {
		f.then(shared)
	}
	r := e.resumeReviewed(id)
	if r.exit != 0 {
		t.Fatalf("exit %d\n%s\n%s", r.exit, r.shown(), r.errs.String())
	}
	parts, _ := filepath.Glob(filepath.Join(e.root, ".kogen/local/runs/*/review-source/*/*.json"))
	if len(parts) == 0 {
		t.Fatal("the inline source was never pinned after the resume")
	}
}

func TestAResumedFittedReviewOfALargerSourcePinsItAndDeliversIt(t *testing.T) {
	t.Parallel()
	// Intent review-refine-working-room: resume a fitted inline Review with room for investigation and final repair (suite parameters)
	e := newEnv(t)
	e.write("README.md", "# Fixture project\n"+strings.Repeat("r", 61000)+"\n")
	e.write("docs/development.md", strings.Repeat("d", 49000)+"\n")
	e.write("seed-knowledge.txt", strings.Repeat("k", 32000)+"\n")
	e.write("pad.txt", strings.Repeat("p", 22000))
	e.run("git", "add", "README.md", "docs/development.md", "seed-knowledge.txt", "pad.txt")
	e.run("git", "commit", "-qm", "Seed maintained documents")
	e.approve()
	f := e.prov
	f.then(
		f.calls("shell", map[string]any{"command": "printf '\\nreview-edit\\n' >> README.md; printf '\\nreview-edit\\n' >> docs/development.md; cp seed-knowledge.txt knowledge_live.ex"}),
		f.calls("finish", map[string]any{"summary": "candidate complete"}),
		failure(503, "server_error", "local scripted overload"),
	)
	a := e.reviewed()
	if a.exit == 0 {
		t.Fatal("the first run should stop")
	}
	id := e.onlyRun()
	for n := 1; n <= 8; n++ {
		f.then(f.calls("read_file", map[string]any{"path": "pad.txt"}, "read_file", map[string]any{"path": "pad.txt"}, "read_file", map[string]any{"path": "pad.txt"}))
	}
	f.then(f.calls("read_file", map[string]any{"path": "pad.txt"}))
	shared := readPinned(e, f)
	for i := 0; i < 60; i++ {
		f.then(shared)
	}
	r := e.resumeReviewed(id)
	parts, _ := filepath.Glob(filepath.Join(e.root, ".kogen/local/runs/*/review-source/*/*.json"))
	if r.exit != 0 || len(parts) == 0 {
		t.Fatalf("exit %d, %d parts\n%s\n%s", r.exit, len(parts), r.shown(), r.errs.String())
	}
}

// reviewShapeCases covers the accepted set as well as shape and inner-validation failures.
func reviewShapeCases() []struct {
	args     string
	accepted bool
	problem  string
} {
	return []struct {
		args     string
		accepted bool
		problem  string
	}{
		{`{"verdict":"approve","findings":[]}`, true, ""},
		{`{"verdict":"approve","findings":[{"path":"app.txt","line":1,"severity":"note","message":"PRIVATE_REVIEW_MARKER"}]}`, true, ""},
		{`{"verdict":"changes","findings":[{"path":"app.txt","line":1,"severity":"blocking","message":"PRIVATE_REVIEW_MARKER"}]}`, true, ""},
		{`{"verdict":"approve","findings":"[]"}`, false, "submit_review needs verdict (approve or changes) and a findings list; findings must be a JSON array of objects but was a string"},
		{`{"verdict":"approve","findings":"[{\"path\":\"app.txt\",\"line\":1,\"severity\":\"note\",\"message\":\"PRIVATE_REVIEW_MARKER\"}]"}`, false, "submit_review needs verdict (approve or changes) and a findings list; findings must be a JSON array of objects but was a string"},
		{`{"verdict":"approve"}`, false, "submit_review needs verdict (approve or changes) and a findings list; findings must be a JSON array of objects but was missing"},
		{`{"verdict":"approve","findings":null}`, false, "submit_review needs verdict (approve or changes) and a findings list; findings must be a JSON array of objects but was null"},
		{`{"verdict":"approve","findings":{"private":"PRIVATE_REVIEW_MARKER"}}`, false, "submit_review needs verdict (approve or changes) and a findings list; findings must be a JSON array of objects but was an object"},
		{`{"verdict":"approve","findings":true}`, false, "submit_review needs verdict (approve or changes) and a findings list; findings must be a JSON array of objects but was a boolean"},
		{`{"verdict":"approve","findings":12345}`, false, "submit_review needs verdict (approve or changes) and a findings list; findings must be a JSON array of objects but was a number"},
		{`{"findings":[]}`, false, "submit_review needs verdict (approve or changes) and a findings list; verdict must be approve or changes but was missing"},
		{`{"verdict":null,"findings":[]}`, false, "submit_review needs verdict (approve or changes) and a findings list; verdict must be approve or changes but was null"},
		{`{"verdict":12345,"findings":[]}`, false, "submit_review needs verdict (approve or changes) and a findings list; verdict must be approve or changes but was a number"},
		{`{"verdict":false,"findings":[]}`, false, "submit_review needs verdict (approve or changes) and a findings list; verdict must be approve or changes but was a boolean"},
		{`{"verdict":{"private":"PRIVATE_REVIEW_MARKER"},"findings":[]}`, false, "submit_review needs verdict (approve or changes) and a findings list; verdict must be approve or changes but was an object"},
		{`{"verdict":["PRIVATE_REVIEW_MARKER"],"findings":[]}`, false, "submit_review needs verdict (approve or changes) and a findings list; verdict must be approve or changes but was an array"},
		{`{"verdict":"PRIVATE_REVIEW_MARKER","findings":[]}`, false, "submit_review needs verdict (approve or changes) and a findings list; verdict must be approve or changes but was a string with an unsupported value"},
		{`{}`, false, "submit_review needs verdict (approve or changes) and a findings list; verdict must be approve or changes but was missing; findings must be a JSON array of objects but was missing"},
		{`{"verdict":"PRIVATE_REVIEW_MARKER","findings":"PRIVATE_REVIEW_MARKER"}`, false, "submit_review needs verdict (approve or changes) and a findings list; verdict must be approve or changes but was a string with an unsupported value; findings must be a JSON array of objects but was a string"},
		{`{"verdict":"changes","findings":[]}`, false, "a changes verdict needs at least one blocking finding"},
		{`{"verdict":"approve","findings":[{"path":"app.txt","line":1,"severity":"blocking","message":"PRIVATE_REVIEW_MARKER"}]}`, false, "an approve verdict cannot have blocking findings"},
		{`{"verdict":"approve","findings":[{"path":"app.txt","line":1.5,"severity":"note","message":"PRIVATE_REVIEW_MARKER"}]}`, false, "each finding needs path (string), line (integer), severity (blocking or note) and message"},
	}
}

func decodeReviewArgs(t *testing.T, text string) map[string]any {
	t.Helper()
	var args map[string]any
	dec := json.NewDecoder(strings.NewReader(text))
	dec.UseNumber()
	if err := dec.Decode(&args); err != nil {
		t.Fatal(err)
	}
	return args
}

func TestReviewValidateAcceptanceSet(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "app.txt"), []byte("candidate"), 0600); err != nil {
		t.Fatal(err)
	}
	a := &Attempt{State: State{Root: root}}
	for i, tc := range reviewShapeCases() {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			verdict, problems, err := validateReview(a, decodeReviewArgs(t, tc.args), nil)
			if err != nil {
				t.Fatal(err)
			}
			eq(t, "accepted", verdict != nil, tc.accepted)
			eq(t, "diagnostic", strings.Join(problems, "; "), tc.problem)
			if strings.Contains(strings.Join(problems, ""), "PRIVATE_REVIEW_MARKER") {
				t.Fatal("private value disclosed")
			}
		})
	}
}

// readThen reads the pinned parts of the review it answers in batches, then submits the verdict.
func readThen(e *env, f *fakeProvider, verdict string, fs ...map[string]any) handler {
	next := map[int]int{}
	return func(w http.ResponseWriter, r *http.Request) {
		review := latestPinnedReview(e.root)
		parts := pinnedPartFiles(e.root, review)
		if next[review] >= len(parts) {
			submit(f, verdict, fs...)(w, r)
			return
		}
		var pairs []any
		for _, p := range parts[next[review]:min(next[review]+15, len(parts))] {
			pairs = append(pairs, "read_file", map[string]any{"path": p})
		}
		next[review] += 15
		f.calls(pairs...)(w, r)
	}
}

// TestChangesWhilePartsAreUnreadIsSentBackToRead: a changes verdict with a blocking finding
// given while pinned parts are unread never lets the candidate land with a part undelivered.
// Either the reviewer is sent back to read and its later approval comes after every part was
// delivered, or the review ends blocked.
func TestChangesWhilePartsAreUnreadIsSentBackToRead(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	f := e.prov
	start := e.head()
	f.then(f.calls("write_file", map[string]any{"path": "app.txt", "content": bigFile()}, "finish", map[string]any{"summary": "done"}))
	f.then(submit(f, "changes", finding("blocking", "the parts were not delivered")))
	reader := readThen(e, f, "approve")
	for i := 0; i < 30; i++ {
		f.then(reader)
	}
	a := e.reviewed()
	id := e.onlyRun()
	switch a.exit {
	case 0:
		res := e.result(id)
		n := int(res["reviews"].(float64))
		eq(t, "approved review", e.jsonFile(e.runPath(id, "review-"+itoa(n)+".json"))["verdict"], any("approve"))
		if e.head() == start {
			t.Fatal("the Build exited 0 without a commit")
		}
		material := materialEventsOf(e.events(id), n)
		parts := len(pinnedPartFiles(e.root, n))
		if parts == 0 || len(material) == 0 || material[len(material)-1]["parts_read"] != float64(parts) {
			t.Fatalf("landed before the %d parts were delivered: %v", parts, material)
		}
	case 1:
		eq(t, "head", e.head(), start)
		if outcome := e.result(id)["outcome"]; outcome != "review_blocked" && outcome != "review_invalid" {
			t.Fatalf("outcome %v", outcome)
		}
	default:
		t.Fatalf("exit %d\n%s\n%s", a.exit, a.shown(), a.errs)
	}
}

// TestAReviewerThatNeverReadsIsStopped: a reviewer that only ever answers changes with a
// blocking finding and never reads a pinned part is stopped by the review itself, before the
// scripted answers run out: the Build exits 1 with HEAD unchanged and the review invalid or blocked.
func TestAReviewerThatNeverReadsIsStopped(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	f := e.prov
	f.then(f.calls("write_file", map[string]any{"path": "app.txt", "content": bigFile()}, "finish", map[string]any{"summary": "done"}))
	for i := 0; i < 40; i++ {
		f.then(submit(f, "changes", finding("blocking", "x")))
	}
	start := e.head()
	a := e.reviewed()
	eq(t, "exit", a.exit, 1)
	eq(t, "head", e.head(), start)
	if outcome := e.result(e.onlyRun())["outcome"]; outcome != "review_invalid" && outcome != "review_blocked" {
		t.Fatalf("outcome %v", outcome)
	}
}

// TestChangesAfterEveryPartWasReadStaysBlocked: a changes verdict with a blocking finding given
// after the reviewer read every pinned part blocks the review.
func TestChangesAfterEveryPartWasReadStaysBlocked(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	f := e.prov
	f.then(f.calls("write_file", map[string]any{"path": "app.txt", "content": bigFile()}, "finish", map[string]any{"summary": "done"}))
	reader := readThen(e, f, "changes", finding("blocking", "real defect"))
	for i := 0; i < 30; i++ {
		f.then(reader)
	}
	start := e.head()
	a := e.reviewed()
	eq(t, "exit", a.exit, 1)
	contains(t, "stdout", a.out.String(), "review_blocked")
	eq(t, "head", e.head(), start)
}

const deliveredHead = "Your verdict was not applied: the candidate changed after its contents were delivered to you, other than through your own Kogen tool calls: app.txt.\n" +
	"The exact diff of these changes against the contents delivered to you:\n\n" +
	"diff --git a/app.txt b/app.txt\n"

const deliveredGreen = "The new tree passed checks but has not been reviewed. Inspect these changes and submit_review for the current contents; do not restart the whole critique."

// lastUserText is the last user message with text content in a native reviewer request.
func lastUserText(req map[string]any) string {
	input := requestInput(req)
	for i := len(input) - 1; i >= 0; i-- {
		m, _ := input[i].(map[string]any)
		if s, ok := m["content"].(string); m["role"] == "user" && ok {
			return s
		}
	}
	return ""
}

// outsideThenApprove is a reviewer reply that approves while another writer changes app.txt.
func outsideThenApprove(e *env, f *fakeProvider, content string) handler {
	approve := submit(f, "approve")
	return func(w http.ResponseWriter, r *http.Request) {
		writeFile(e.t, filepath.Join(e.root, "app.txt"), content, 0o644)
		approve(w, r)
	}
}

func TestANativeApprovalOfContentsChangedWhileItsRequestWasInFlightIsNotApplied(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	f := e.prov
	f.then(landScript(f)...)
	f.then(outsideThenApprove(e, f, "changed outside\n"))
	f.then(submit(f, "changes", finding("blocking", "app.txt now says changed outside")))
	start := e.head()
	a := e.reviewed()
	wantStop(t, a, "candidate/review_blocked")
	eq(t, "HEAD", e.head(), start)
	eq(t, "commits", e.commitsSince(start), "0")
	reqs := f.reviewRequests()
	eq(t, "review requests", len(reqs), 2)
	feedback := lastUserText(reqs[1])
	if !strings.HasPrefix(feedback, deliveredHead) || !strings.HasSuffix(feedback, "@@ -1 +1 @@\n-candidate\n+changed outside\n\n"+deliveredGreen) {
		t.Fatalf("feedback %q", feedback)
	}
	contains(t, "progress", a.errs.String(), "kogen: review 1: the candidate changed after its contents were delivered; returning the changes to the reviewer\n")
	lacks(t, "progress", a.errs.String(), "candidate changed after approval")
}

func TestANativeReviewersFreshApprovalCommitsTheDeliveredContents(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	f := e.prov
	f.then(landScript(f)...)
	f.then(outsideThenApprove(e, f, "changed outside\n"))
	f.then(submit(f, "approve"))
	start := e.head()
	a := e.reviewed()
	if a.exit != 0 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	eq(t, "commits", e.commitsSince(start), "1")
	eq(t, "committed", e.gitOut("show", "HEAD:app.txt"), "changed outside")
	eq(t, "review requests", len(f.reviewRequests()), 2)
	eq(t, "repairs", e.result(e.onlyRun())["repairs"], any(float64(1)))
}

func TestANativeReviewersOwnShellRepairIsBoundAndLandsInOneRequest(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	f := e.prov
	f.then(landScript(f)...)
	f.then(f.calls("shell", map[string]any{"command": "printf 'reviewer shell repair\\n' > app.txt"},
		"submit_review", map[string]any{"verdict": "approve", "findings": []any{}}))
	start := e.head()
	a := e.reviewed()
	if a.exit != 0 {
		t.Fatalf("exit %d\n%s", a.exit, a.shown())
	}
	eq(t, "commits", e.commitsSince(start), "1")
	eq(t, "committed", e.gitOut("show", "HEAD:app.txt"), "reviewer shell repair")
	eq(t, "review requests", len(f.reviewRequests()), 1)
}

// A change by another writer after the reviewer's own edit is not the reviewer's: the seen tree
// follows only the edit.
func TestAnOutsideChangeAfterTheReviewersOwnEditIsNotApproved(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	f := e.prov
	f.then(landScript(f)...)
	approve := submit(f, "approve")
	f.then(f.calls(writeApp("reviewer repair\n")...))
	f.then(func(w http.ResponseWriter, r *http.Request) {
		writeFile(t, filepath.Join(e.root, "app.txt"), "changed outside\n", 0o644)
		approve(w, r)
	})
	f.then(submit(f, "changes", finding("blocking", "stop")))
	start := e.head()
	a := e.reviewed()
	wantStop(t, a, "candidate/review_blocked")
	eq(t, "HEAD", e.head(), start)
	reqs := f.reviewRequests()
	eq(t, "review requests", len(reqs), 3)
	if feedback := lastUserText(reqs[2]); !strings.HasPrefix(feedback, deliveredHead) || !strings.Contains(feedback, "@@ -1 +1 @@\n-reviewer repair\n+changed outside\n") {
		t.Fatalf("feedback %q", feedback)
	}
}

// An explicit retry of a blocked native review delivers the changes of the checkout since the
// reviewer's contents, and the verified tree is then the seen one.
func TestARetryOfABlockedReviewDeliversCheckoutChangesAndBindsTheVerifiedTree(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	f := e.prov
	f.then(landScript(f)...)
	f.then(submit(f, "changes", finding("blocking", "Original unresolved requirement")))
	start := e.head()
	a := e.reviewed()
	eq(t, "exit", a.exit, 1)
	id := e.onlyRun()
	e.write("app.txt", "changed after the block\n")
	doc, _ := e.checkpoint(id)
	tree, err := e.gitRepo().Tree(start, nil)
	if err != nil {
		t.Fatal(err)
	}
	doc["candidate"] = tree
	e.writeCheckpoint(id, doc)
	f.then(submit(f, "approve"))
	r := e.resumeReviewed(id)
	if r.exit != 0 {
		t.Fatalf("exit %d\n%s\n%s", r.exit, r.shown(), r.errs.String())
	}
	// Approved in the one request: the verified tree is the seen one.
	eq(t, "committed", e.gitOut("show", "HEAD:app.txt"), "changed after the block")
	reqs := f.reviewRequests()
	eq(t, "review requests", len(reqs), 2)
	feedback := lastUserText(reqs[1])
	if !strings.HasPrefix(feedback, deliveredHead) || !strings.Contains(feedback, "@@ -1 +1 @@\n-candidate\n+changed after the block\n") ||
		!strings.Contains(feedback, "\nThe caller explicitly resumed this blocked Build.") {
		t.Fatalf("feedback %q", feedback)
	}
}

// A review resumed at the stage review keeps its saved seen tree: the reviewer's own edit stays
// bound, and a change made outside before the resume is not approved.
func TestAResumedNativeReviewKeepsItsSeenTree(t *testing.T) {
	t.Parallel()
	interrupted := func(t *testing.T) (*env, *fakeProvider, string, string) {
		e := newEnv(t)
		f := e.prov
		f.then(landScript(f)...)
		f.then(f.calls(writeApp("reviewer repair\n")...))
		f.then(refused())
		start := e.head()
		a := e.reviewed()
		eq(t, "exit", a.exit, 4)
		id := e.onlyRun()
		cont, session := sessionOfRun(t, e, id)
		eq(t, "stage", cont["stage"], any("review"))
		if session["seen_tree"] == nil || session["seen_tree"] == "" {
			t.Fatalf("the saved session has no seen tree: %v", session)
		}
		return e, f, id, start
	}

	t.Run("an own edit stays bound", func(t *testing.T) {
		t.Parallel()
		e, f, id, start := interrupted(t)
		f.then(submit(f, "approve"))
		if r := e.resumeReviewed(id); r.exit != 0 {
			t.Fatalf("exit %d\n%s\n%s", r.exit, r.shown(), r.errs.String())
		}
		eq(t, "commits", e.commitsSince(start), "1")
		eq(t, "committed", e.gitOut("show", "HEAD:app.txt"), "reviewer repair")
		eq(t, "review requests", len(f.reviewRequests()), 3)
	})

	t.Run("an outside change before the resume is not approved", func(t *testing.T) {
		t.Parallel()
		e, f, id, start := interrupted(t)
		e.write("app.txt", "changed outside\n")
		doc, _ := e.checkpoint(id)
		tree, err := e.gitRepo().Tree(start, nil)
		if err != nil {
			t.Fatal(err)
		}
		doc["candidate"] = tree
		e.writeCheckpoint(id, doc)
		f.then(submit(f, "approve"), submit(f, "changes", finding("blocking", "stop")))
		wantStop(t, e.resumeReviewed(id), "candidate/review_blocked")
		eq(t, "commits", e.commitsSince(start), "0")
		reqs := f.reviewRequests()
		eq(t, "review requests", len(reqs), 4)
		if feedback := lastUserText(reqs[3]); !strings.HasPrefix(feedback, deliveredHead) || !strings.Contains(feedback, "-reviewer repair\n+changed outside\n") {
			t.Fatalf("feedback %q", feedback)
		}
	})
}
