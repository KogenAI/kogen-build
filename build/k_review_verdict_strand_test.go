package build

import (
	"encoding/json"
	"net/http"
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
//
// The forced tool follows the pinned unread parts the reset handoff reports (unread_parts):
// read_file while any remain unread, submit_review once none do. Both states run: a candidate
// whose pinned parts the reviewer never reads leaves some unread, a small one it reads first
// leaves none.
func TestKReviewVerdictStrandReset(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		readAll  bool // the reviewer reads every pinned part before the long prose turn
		wantRead bool // some pinned parts are then still unread at the reset
	}{
		{"unread pinned parts remain", false, true},
		{"every pinned part was read", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			strandReset(t, tc.readAll, tc.wantRead)
		})
	}
}

// reviewResetHandoff is the kogen-review working-context reset message a request carries, or
// nil. It states how many pinned parts remain unread as unread_parts.
func reviewResetHandoff(t *testing.T, req map[string]any) map[string]any {
	t.Helper()
	const header = "Kogen review working-context reset\n"
	for _, it := range requestInput(req) {
		m, _ := it.(map[string]any)
		s, ok := m["content"].(string)
		if !ok || !strings.HasPrefix(s, header) {
			continue
		}
		var doc map[string]any
		if err := json.Unmarshal([]byte(strings.TrimPrefix(s, header)), &doc); err != nil {
			t.Fatal(err)
		}
		return doc
	}
	return nil
}

// resetText is the text of every reset handoff a request carries (Build's and the review's), in
// order, or "".
func resetText(t *testing.T, req map[string]any) string {
	t.Helper()
	var out string
	for _, it := range requestInput(req) {
		m, _ := it.(map[string]any)
		if c, ok := m["content"].(string); ok && (strings.HasPrefix(c, "Local working-window reset\n") || strings.HasPrefix(c, "Kogen review working-context reset\n")) {
			out += c + "\n"
		}
	}
	return out
}

// afterReset reports whether a request carries a working-window reset handoff, Build's or the
// review's own.
func afterReset(t *testing.T, req map[string]any) bool {
	t.Helper()
	return resetHandoff(t, req) != nil || reviewResetHandoff(t, req) != nil
}

// unreadParts is the unread_parts count the reset handoff of a request reports: the review's own
// handoff states it as unread_parts, Build's carries the unread pinned parts of its source
// delivery facts. ok is false when the request carries no such count.
func unreadParts(t *testing.T, req map[string]any) (n int, ok bool) {
	t.Helper()
	if doc := reviewResetHandoff(t, req); doc != nil {
		n, ok := doc["unread_parts"].(float64)
		return int(n), ok
	}
	delivery, _ := resetHandoff(t, req)["review_delivery"].(map[string]any)
	source, _ := delivery["source"].(map[string]any)
	unread, isList := source["unread"].([]any)
	if source["current"] != true || !isList {
		return 0, false
	}
	return len(unread), true
}

// forcedToolFor is the tool the review contract forces for a reminder: read_file while pinned
// parts remain unread, submit_review once none do.
func forcedToolFor(unread int) string {
	if unread > 0 {
		return "read_file"
	}
	return "submit_review"
}

// unreadLeft reports whether the confirming reviewer would still read a pinned part in answer
// to req: one that neither the request's window nor the reset's native delivery facts show as
// delivered.
func unreadLeft(t *testing.T, req map[string]any, e *env) bool {
	t.Helper()
	known, _ := inWindow(req)
	for path := range prefilledParts(t, req) {
		known[path] = true
	}
	parts := latestPinnedParts(e.root)
	if reset := resetHandoff(t, req); reset != nil {
		if d, ok := reset["review_delivery"].(map[string]any); ok {
			if source, ok := d["source"].(map[string]any); ok && source["current"] == true {
				unread := map[int]bool{}
				for _, n := range source["unread"].([]any) {
					unread[int(n.(float64))] = true
				}
				for i, p := range parts {
					if !unread[i+1] {
						known[p] = true
					}
				}
			}
		}
	}
	return slices.ContainsFunc(parts, func(p string) bool { return !known[p] })
}

func strandReset(t *testing.T, readAll, wantUnread bool) {
	e := newEnv(t)
	f := e.prov
	f.then(strandBuilder(f))
	// Review 1: optionally reads every pinned part first (the confirming reviewer's reads, a few
	// at a time across its windows), then a large prose-only turn that pushes the reminder request
	// over the ceiling (the reset happens in that very request, which also carries native source
	// prefill) and a text-only answer to it. Review 2 then confirms and approves.
	confirm := confirmingReviewer(t, e, f, nil)
	stage := 0
	f.then(repeatHandler(func(w http.ResponseWriter, r *http.Request) {
		reqs := f.reviewRequests()
		if stage == 0 && readAll && !unreadLeft(t, reqs[len(reqs)-1], e) {
			stage = 1
		} else if stage == 0 && !readAll {
			stage = 1
		}
		switch stage {
		case 0:
			confirm(w, r)
		case 1:
			stage = 2
			prose(strings.Repeat("Everything is fine. ", 7_000))(w, r)
		case 2:
			stage = 3
			prose("Nothing to add.")(w, r)
		default:
			confirm(w, r)
		}
	}, 120)...)
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
	if !afterReset(t, reqs[forced[0]]) || (forced[0] > 0 && resetText(t, reqs[forced[0]]) == resetText(t, reqs[forced[0]-1])) {
		t.Fatal("the forced request is not the one that came right after a working-window reset")
	}
	// Build's own reset (window_reset) or the review's (review_window_reset) shrank the window.
	shrank := func(evs []map[string]any, before, after string) bool {
		return len(evs) > 0 && evs[0][after].(float64) < evs[0][before].(float64)
	}
	resets, reviewResets := eventsNamed(events, "window_reset"), eventsNamed(events, "review_window_reset")
	if !(shrank(resets, "input_tokens_before", "input_tokens_after") || shrank(reviewResets, "bytes_before", "bytes_after")) || len(forcedBody[0]) > looptools.InputBudgetBytes {
		t.Fatalf("resets %v, review resets %v, forced request %d bytes", resets, reviewResets, len(forcedBody[0]))
	}
	eq(t, "the reset is part of the forced request's body", strings.Contains(string(bodies[forced[0]]), "working-window reset") || strings.Contains(string(bodies[forced[0]]), "working-context reset"), true)

	// The forced tool follows the unread pinned parts the reset handoff of the forced request
	// reports, not a fixed tool.
	unread, ok := unreadParts(t, reqs[forced[0]])
	if !ok {
		t.Fatal("the forced request reports no unread_parts count in its reset handoff")
	}
	eq(t, "some pinned parts are unread", unread > 0, wantUnread)
	wantTool := forcedToolFor(unread)
	eq(t, "forced tool_choice", jsonOf(reqs[forced[0]]["tool_choice"]), jsonOf(map[string]any{"type": "function", "name": wantTool}))
	eq(t, "tool choice events", len(eventsNamed(events, "review_tool_choice")), 1)
	eq(t, "mode", eventsNamed(events, "review_tool_choice")[0]["mode"], any("api_forced"))
	eq(t, "tool choice event tool", eventsNamed(events, "review_tool_choice")[0]["tool"], any(wantTool))
	if len(eventsNamed(events, "review_source_prefill")) < 2 {
		t.Fatalf("no native source prefill progress around the forced turn: %v", eventsNamed(events, "review_source_prefill"))
	}

	// The text-only answer: a fault of this route and model, an ordinary no-tool turn.
	faults := eventsNamed(events, "provider_fault")
	eq(t, "provider faults", len(faults), 1)
	eq(t, "fault", faults[0]["fault"], any("forced_tool_not_called"))
	eq(t, "fault route", faults[0]["route"], any("chatgpt"))
	eq(t, "fault model", faults[0]["model"], any("gpt-6.1-terra"))
	eq(t, "fault tool", faults[0]["tool"], any(wantTool))
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
