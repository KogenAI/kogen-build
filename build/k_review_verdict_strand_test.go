package build

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/KogenAI/kogen-coordinator/project"
	looptools "github.com/KogenAI/kogen-loop/tools"
)

// Tests of Intent k-review-verdict-strand: a completed green candidate stays reviewable when
// independent reviewers never submit a verdict. Real Build and CLI Resume, temporary Git and the
// scripted provider; nothing here reaches a paid route.

// strandEdits is a reviewer turn that edits the checkout in every way (modification, deletion,
// addition, mode) before the reviewer goes verdictless.
func strandEdits(f *fakeProvider) handler {
	return f.calls("write_file", map[string]any{"path": "app.txt", "content": "reviewer edit\n"},
		"shell", map[string]any{"command": "rm README.md && printf 'scratch\\n' > notes.txt && chmod +x features/steps/integration_steps.js"})
}

// strandBuilder is the builder's one response: a candidate whose pinned source needs many parts.
func strandBuilder(f *fakeProvider) handler {
	return f.calls("write_file", map[string]any{"path": "app.txt", "content": bigFile()}, "finish", map[string]any{"summary": "done"})
}

// forcedBodies selects transmitted forced reminders in the multipart strand fixture, with
// their original indexes among review requests.
func forcedBodies(t *testing.T, f *fakeProvider) (indexes []int, bodies [][]byte) {
	t.Helper()
	for i, body := range f.reviewBodies() {
		var wire map[string]any
		if err := json.Unmarshal(body, &wire); err != nil {
			t.Fatal(err)
		}
		if jsonOf(wire["tool_choice"]) == jsonOf(forcedSubmit) ||
			jsonOf(wire["tool_choice"]) == jsonOf(map[string]any{"type": "function", "name": "read_file"}) {
			indexes, bodies = append(indexes, i), append(bodies, body)
		}
	}
	return indexes, bodies
}

// TestKReviewVerdictStrandReset: a forced submit_review request that comes right after a real
// working-window reset (pinned source, native prefill around it) is transmitted forced; its
// text-only answer is a provider fault naming route and model and the ordinary no-tool
// exhaustion; a fresh independent review of the same green tree then approves and lands.
func TestKReviewVerdictStrandReset(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	f := e.prov
	f.then(strandBuilder(f))
	// Review 1: a large prose-only turn that pushes the reminder request over the ceiling (the
	// reset happens in that very request, which also carries native source prefill) and a
	// text-only answer to it.
	f.then(prose(strings.Repeat("Everything is fine. ", 7_000)), prose("Nothing to add."))
	f.then(repeatHandler(confirmingReviewer(t, e, f, nil), 40)...)
	start := e.head()
	a := e.reviewed()
	if a.exit != 0 || a.err != nil {
		t.Fatalf("exit %d err %v\n%s", a.exit, a.err, a.shown())
	}
	id := e.onlyRun()
	events := e.events(id)
	reqs, bodies := f.reviewRequests(), f.reviewBodies()

	// The forced request really followed a reset and was transmitted forced, within the ceiling.
	forced, forcedBody := forcedBodies(t, f)
	eq(t, "forced requests", len(forced), 1)
	if resetHandoff(t, reqs[forced[0]]) == nil || resetHandoff(t, reqs[forced[0]-1]) != nil {
		t.Fatal("the forced request is not the one that came right after a working-window reset")
	}
	resets := eventsNamed(events, "window_reset")
	if len(resets) == 0 || resets[0]["input_tokens_after"].(float64) >= resets[0]["input_tokens_before"].(float64) || len(forcedBody[0]) > looptools.InputBudgetBytes {
		t.Fatalf("resets %v, forced request %d bytes", resets, len(forcedBody[0]))
	}
	eq(t, "the reset is part of the forced request's body", strings.Contains(string(bodies[forced[0]]), "Local working-window reset"), true)
	eq(t, "tool choice events", len(eventsNamed(events, "review_tool_choice")), 1)
	eq(t, "mode", eventsNamed(events, "review_tool_choice")[0]["mode"], any("api_forced"))
	if len(eventsNamed(events, "review_source_prefill")) < 2 {
		t.Fatalf("no native source prefill progress around the forced turn: %v", eventsNamed(events, "review_source_prefill"))
	}

	// The text-only answer: a fault of this route and model, an ordinary no-tool turn.
	faults := eventsNamed(events, "provider_fault")
	eq(t, "provider faults", len(faults), 1)
	eq(t, "fault", faults[0]["fault"], any("forced_tool_not_called"))
	eq(t, "fault route", faults[0]["route"], any("chatgpt"))
	eq(t, "fault model", faults[0]["model"], any("gpt-6.1-terra"))
	eq(t, "fault tool", faults[0]["tool"], any("submit_review"))
	verdicts := map[float64]map[string]any{}
	for _, ev := range e.reviewVerdicts(id) {
		verdicts[ev["review"].(float64)] = ev
	}
	if verdicts[1] == nil || verdicts[2] == nil {
		t.Fatalf("review events %v", verdicts)
	}
	eq(t, "review 1", verdicts[1]["verdict"], any("invalid"))
	eq(t, "review 1 reason", verdicts[1]["reason"], any("no_tool_reminder_exhausted"))
	eq(t, "review 2", verdicts[2]["verdict"], any("approve"))

	// Review 2 is a fresh conversation of the same green tree; the Build lands once.
	retries := eventsNamed(events, "review_retry")
	eq(t, "retry events", len(retries), 1)
	eq(t, "retry kind", retries[0]["kind"], any("automatic"))
	second := slices.IndexFunc(reqs, func(req map[string]any) bool { return cacheKey(req) != cacheKey(reqs[0]) })
	if second < 0 {
		t.Fatal("review 2 did not start a conversation of its own")
	}
	lacks(t, "review 2", requestText(reqs[second]), "function_call")
	eq(t, "commits", e.commitsSince(start), "1")
	eq(t, "committed", e.gitOut("show", "HEAD:app.txt")+"\n", bigFile())
	res := e.result(id)
	eq(t, "reviews", res["reviews"], any(float64(2)))
	eq(t, "passes: no completed check was rerun", res["passes"], any(float64(1)))
}

// TestKReviewVerdictStrandSafety: recovery of the exact green candidate and its truthful
// refusals. An unchanged candidate after a discarded first review stays eligible; an edited,
// committed or restaged candidate is refused for what it actually is, with the checkout left as
// the reviewer left it; a changed stopped candidate refuses an explicit resume naming the
// observed and green trees and the changed paths; substantive verdicts are not verdictless.
func TestKReviewVerdictStrandSafety(t *testing.T) {
	t.Parallel()
	// stopWith runs two verdictless reviews in this process; second is review 2's first turn,
	// which may touch the checkout.
	stopWith := func(t *testing.T, first, second func(f *fakeProvider) handler) (*env, string, string) {
		e := newEnv(t)
		f := e.prov
		f.then(landScript(f)...)
		if first != nil {
			f.then(first(f))
		}
		f.then(prose("a"), prose("b"))
		if second != nil {
			f.then(second(f))
		}
		f.then(prose("c"), prose("d"))
		start := e.head()
		a := e.reviewed()
		wantStop(t, a, "candidate/review_invalid")
		return e, e.onlyRun(), start
	}
	lastOnly := func(e *env, id string) map[string]any {
		only := eventsNamed(e.events(id), "review_only")
		return only[len(only)-1]
	}

	t.Run("an unchanged candidate after discarded edits stays reviewable", func(t *testing.T) {
		t.Parallel()
		e, id, _ := stopWith(t, strandEdits, nil)
		only := eventsNamed(e.events(id), "review_only")
		eq(t, "review-only events", len(only), 2)
		eq(t, "second eligible: the unchanged second reviewer caused no false drift", only[1]["eligible"], any(true))
		eq(t, "discard events", len(eventsNamed(e.events(id), "review_edits_discarded")), 1)
		eq(t, "stage", e.checkpointStage(id), StageReviewOnly)
		eq(t, "candidate", e.read("app.txt"), "candidate\n")
		e.prov.then(submit(e.prov, "approve"))
		if r := e.resumeReviewed(id); r.exit != 0 {
			t.Fatalf("resume exit %d\n%s", r.exit, r.shown())
		}
	})

	t.Run("a later invalid review's edits are left in place and a resume discards them", func(t *testing.T) {
		t.Parallel()
		e, id, start := stopWith(t, nil, func(f *fakeProvider) handler {
			return f.calls("write_file", map[string]any{"path": "app.txt", "content": "reviewer edit\n"})
		})
		only := lastOnly(e, id)
		green := eventsNamed(e.events(id), "review_only")[0]["tree"].(string)
		eq(t, "eligible", only["eligible"], any(true))
		eq(t, "tree is the green tree", only["tree"], any(green))
		eq(t, "next", only["next"], any("stop"))
		eq(t, "stage", e.checkpointStage(id), StageReviewOnly)
		if treeOfCandidate(e) == green {
			t.Fatal("the stopped candidate is not the edited one")
		}
		eq(t, "the reviewer's edit is left in place", e.read("app.txt"), "reviewer edit\n")
		eq(t, "discard events", len(eventsNamed(e.events(id), "review_edits_discarded")), 0)
		eq(t, "head", e.head(), start)
		if _, err := os.Stat(filepath.Join(e.root, ".kogen/local/runs", id, "index-at-start")); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(project.ApprovalPath(e.root, testSlug)); err != nil {
			t.Fatal("approval was not preserved")
		}

		// A change after the stop is still refused as drift, naming trees and paths, and is not
		// restored.
		e.write("app.txt", "drifted\n")
		observed := treeOfCandidate(e)
		requests := e.prov.count()
		r := e.resumeReviewed(id)
		if r.exit != 1 {
			t.Fatalf("resume exit %d\n%s", r.exit, r.shown())
		}
		contains(t, "drift", r.shown(), "candidate contents/modes or unrelated paths changed: app.txt")
		contains(t, "drift", r.shown(), "observed tree "+short(observed))
		contains(t, "drift", r.shown(), "green tree "+short(green))
		eq(t, "the drifted candidate is not restored", e.read("app.txt"), "drifted\n")
		eq(t, "provider requests", e.prov.count(), requests)
		e.write("app.txt", "reviewer edit\n")

		// The unchanged stopped candidate resumes: the edits are discarded and one fresh review
		// of the green tree approves it.
		e.prov.then(submit(e.prov, "approve"))
		r = e.resumeReviewed(id)
		if r.exit != 0 {
			t.Fatalf("resume exit %d\n%s", r.exit, r.shown())
		}
		contains(t, "stdout", r.shown(), "discarded review 2's uncommitted edits, which are not part of that green candidate: app.txt")
		eq(t, "committed app.txt", e.gitOut("show", "HEAD:app.txt"), "candidate")
	})

	t.Run("a moved HEAD is reported as HEAD, not as drift of contents", func(t *testing.T) {
		t.Parallel()
		e, id, start := stopWith(t, nil, func(f *fakeProvider) handler {
			return f.calls("write_file", map[string]any{"path": "app.txt", "content": "reviewer edit\n"},
				"shell", map[string]any{"command": "git commit -q -am reviewer-commit"})
		})
		why := lastOnly(e, id)["why"].(string)
		eq(t, "eligible", lastOnly(e, id)["eligible"], any(false))
		contains(t, "why", why, "HEAD moved")
		lacks(t, "why", why, "candidate changed")
		eq(t, "commits", e.commitsSince(start), "1")
		eq(t, "the reviewer's commit stays", e.gitOut("log", "-1", "--format=%s"), "reviewer-commit")
	})

	t.Run("a restaged index is reported as the index, not as drift of contents", func(t *testing.T) {
		t.Parallel()
		e, id, start := stopWith(t, nil, func(f *fakeProvider) handler {
			return f.calls("shell", map[string]any{"command": "printf 'x\\n' > staged.txt && git add staged.txt"})
		})
		only := lastOnly(e, id)
		eq(t, "eligible", only["eligible"], any(false))
		contains(t, "why", only["why"].(string), "staged index changed")
		lacks(t, "why", only["why"].(string), "candidate changed")
		eq(t, "head", e.head(), start)
		eq(t, "the reviewer's file is left in place", e.read("staged.txt"), "x\n")
	})

	t.Run("an explicit resume of a changed or inconsistent stopped candidate names the actual fact", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		id, _ := e.stopNatively()
		doc, state := e.checkpoint(id)
		green := state["continuation"].(map[string]any)["pass"].(map[string]any)["tree"].(string)
		raw, err := os.ReadFile(e.runPath(id, "checkpoint.json"))
		if err != nil {
			t.Fatal(err)
		}
		refused := func(name string, want ...string) {
			t.Helper()
			before, requests := e.snapshot(), e.prov.count()
			r := e.resumeReviewed(id)
			if r.exit != 1 || !strings.Contains(r.shown(), "candidate/") {
				t.Fatalf("%s: exit %d, want a refusal\n%s", name, r.exit, r.shown())
			}
			for _, w := range want {
				contains(t, name, r.shown(), w)
			}
			eq(t, name+": provider requests", e.prov.count(), requests)
			after := e.snapshot()
			eq(t, name+": files", after.files, before.files)
			eq(t, name+": index", after.index, before.index)
			eq(t, name+": HEAD", after.head, before.head)
			eq(t, name+": approval", after.approval, before.approval)
			eq(t, name+": runs", after.runs, before.runs)
			eq(t, name+": no commit", e.commitsSince(before.head), "0")
		}

		e.write("app.txt", "drifted\n")
		observed := treeOfCandidate(e)
		refused("changed candidate", "app.txt", "observed tree "+short(observed), "green tree "+short(green))
		eq(t, "the edited candidate is not restored", e.read("app.txt"), "drifted\n")
		e.write("app.txt", "candidate\n")

		// The checkpoint's own pass names another tree than the unchanged candidate: the refusal
		// reports both trees and why the paths cannot be listed, not a claim of changed contents.
		other := strings.Repeat("0", 40)
		state["continuation"].(map[string]any)["pass"].(map[string]any)["tree"] = other
		doc["state"] = state
		e.writeCheckpoint(id, doc)
		refused("another pass's tree", "observed tree", "green tree "+short(other), "cannot be inspected")
		if err := os.WriteFile(e.runPath(id, "checkpoint.json"), raw, 0o600); err != nil {
			t.Fatal(err)
		}
		eq(t, "saved green tree", green, treeOfCandidate(e))

		// Everything restored: the same checkpoint resumes.
		e.prov.then(submit(e.prov, "approve"))
		if r := e.resumeReviewed(id); r.exit != 0 {
			t.Fatalf("control resume exit %d\n%s", r.exit, r.shown())
		}
	})

	t.Run("a blocking or rejected verdict is not a missing verdict", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		f := e.prov
		f.then(landScript(f)...)
		f.then(submit(f, "changes", finding("blocking", "app.txt is wrong")))
		wantStop(t, e.reviewed(), "candidate/review_blocked")
		id := e.onlyRun()
		for _, name := range []string{"review_retry", "review_only", "review_allowance"} {
			eq(t, name+" events", len(eventsNamed(e.events(id), name)), 0)
		}
		eq(t, "review requests", len(f.reviewRequests()), 1)

		// A rejected verdict spends only its own allowance, bounded at one send-back.
		e = newEnv(t)
		f = e.prov
		f.then(landScript(f)...)
		bad := finding("blocking", "An approve verdict cannot contain a blocking finding.")
		f.then(submit(f, "approve", bad), submit(f, "approve", bad), prose("a"), prose("b"))
		wantStop(t, e.reviewed(), "candidate/review_invalid")
		id = e.onlyRun()
		verdicts := e.reviewVerdicts(id)
		eq(t, "review 1 reason", verdicts[0]["reason"], any("rejected_verdict_exhausted"))
		eq(t, "review 1 counters", counters(verdicts[0]), [2]float64{1, 0})
		eq(t, "review requests", len(f.reviewRequests()), 4)
	})
}
