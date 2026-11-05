'use strict';
// The Go acceptance harness of k-review-reopen-after-external-repair, as text. The step file
// (k-review-reopen-after-external-repair__steps.js) writes it to a temporary file and overlays it
// into ./build as build/k_review_reopen_acceptance_test.go for one test binary; it is not one of
// the package's own tests and this module registers no step.
module.exports.HARNESS = String.raw`package build

// The acceptance harness of the Intent k-review-reopen-after-external-repair. Its step file overlays it
// into this package for one test binary; it is not one of the package's own tests. Each mode
// (KRR_MODE) makes a Build whose independent review ends with a blocking verdict, changes the
// checkout outside the Build, resumes the Build and prints one observation line, "KRR <json>".
// With KRR_SIMULATE=1 the checkpoint is told the repaired candidate before the resume, so that the
// reopening the review already does can be observed through the same code.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-review/review"
)

const krrFinding = "Original unresolved requirement"

// krrCase is one blocked Build and what happens to it before and at its resume.
type krrCase struct {
	previous bool         // the blocked checkpoint is a version 3 one
	repair   string       // the app.txt content written outside the Build after the block
	verdict  string       // the reopened reviewer's decision; "" when no reviewer request is scripted
	stale    bool         // the checkpoint records a reviewer approval of the blocked tree
	custody  func(e *env) // a custody change made together with the repair
	version  any          // the checkpoint version written before the resume (nil: unchanged)
}

func TestKReviewReopenAcceptance(t *testing.T) {
	mode := os.Getenv("KRR_MODE")
	simulate := os.Getenv("KRR_SIMULATE") == "1"
	cases := map[string]krrCase{
		"current-fixed":          {repair: "repaired\n", verdict: "approve"},
		"current-unfixed":        {repair: "still wrong\n", verdict: "changes"},
		"current-stale-approval": {repair: "repaired\n", verdict: "changes", stale: true},
		"unsupported-version":    {repair: "repaired\n", version: 2},
		"previous-fixed":         {previous: true, repair: "repaired\n", verdict: "approve"},
		"previous-unfixed":       {previous: true, repair: "still wrong\n", verdict: "changes"},
		"previous-custody": {previous: true, repair: "repaired\n", custody: func(e *env) {
			writeFile(e.t, filepath.Join(e.root, "accept.sh"), "#!/bin/sh\nexit 0\n", 0o755)
		}},
	}
	var obs map[string]any
	if mode == "current-custody" {
		obs = map[string]any{
			"protected": krrRun(t, krrCase{repair: "repaired\n", custody: func(e *env) { e.write(stepsFile, "// changed\n") }}, simulate),
			"index":     krrRun(t, krrCase{repair: "repaired\n", custody: func(e *env) { e.run("git", "add", "app.txt") }}, simulate),
		}
	} else if c, ok := cases[mode]; ok {
		obs = krrRun(t, c, simulate)
	} else {
		t.Fatalf("unknown KRR_MODE %q", mode)
	}
	obs["mode"] = mode
	b, err := json.Marshal(obs)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("KRR " + string(b))
}

func krrRun(t *testing.T, c krrCase, simulate bool) map[string]any {
	t.Helper()
	log := filepath.Join(t.TempDir(), "verified.log")
	var e *env
	var id string
	var saved []any // the blocked reviewer's first request (version 4)
	blocked := map[string]any{}
	if c.previous {
		e, id = newLegacy76Env(t, "review_retry_verify", func(o *legacy76Options) {
			o.state = func(m *legacyMap) {
				// The terminal blocking verdict: stage review_verify with the changes verdict.
				legacyPut(legacyDigMap(t, m, latom("continuation")), latom("stage"), latom("review_verify"))
				legacyPut(legacyDigMap(t, m, latom("last_review")), latom("verdict"), "changes")
			}
		})
		raw, err := os.ReadFile(e.runPath(id, "checkpoint.json"))
		if err != nil {
			t.Fatal(err)
		}
		blocked["version"] = 3
		if loaded, err := legacyDecode(raw); err != nil {
			blocked["decoded"] = err.Error()
		} else {
			blocked["stage"] = loaded.State.Continuation.Stage
			if sess, err := review.DecodeSession(loaded.State.Continuation.Review); err == nil && sess.Verdict != nil {
				blocked["verdict"] = sess.Verdict.Verdict
			}
		}
	} else {
		e = newEnv(t, func(cfg *project.Config) {
			cfg.Checks = []project.Check{{Name: "record", Run: "cat app.txt >> '" + log + "'"}}
		})
		e.approve()
		f := e.prov
		f.then(landScript(f)...)
		f.then(submit(f, "changes", finding("blocking", krrFinding)))
		a := e.reviewed()
		id = e.onlyRun()
		if reqs := f.reviewRequests(); len(reqs) > 0 {
			saved = requestInput(reqs[0])
		}
		blocked["version"] = 4
		blocked["exit"] = a.exit
		blocked["stop"] = krrStop(a.shown())
		cont, session := sessionOfRun(t, e, id)
		blocked["stage"] = cont["stage"]
		if v, ok := session["verdict"].(map[string]any); ok {
			blocked["verdict"] = v["verdict"]
		}
	}
	f := e.prov
	start := e.head()

	e.write("app.txt", c.repair)
	if c.custody != nil {
		c.custody(e)
	}
	if c.stale || c.version != nil || simulate {
		doc, state := e.checkpoint(id)
		if c.stale {
			cont := state["continuation"].(map[string]any)
			tree := cont["pass"].(map[string]any)["tree"]
			state["last_review"] = map[string]any{"n": 1, "verdict": "approve", "findings": []any{}, "tree": tree}
			session := cont["review"].(map[string]any)
			attempts, _ := session["attempts"].([]any)
			session["attempts"] = append([]any{map[string]any{"pass": 1, "tree": tree, "verdict": "approve", "findings": []any{}}}, attempts...)
		}
		if c.version != nil {
			doc["version"] = c.version
		}
		if simulate {
			tree, err := e.gitRepo().Tree(start, nil)
			if err != nil {
				t.Fatal(err)
			}
			doc["candidate"] = tree
		}
		e.writeCheckpoint(id, doc)
	}

	var mu sync.Mutex
	var decision map[string]any
	if c.verdict != "" {
		var findings []map[string]any
		if c.verdict == "changes" {
			findings = append(findings, finding("blocking", krrFinding))
		}
		answer := submit(f, c.verdict, findings...)
		f.then(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			decision = krrAtDecision(e, id, start, log, c.previous)
			mu.Unlock()
			answer(w, r)
		})
	}
	before := f.count()
	r := e.resume(id, func(in *ResumeInput) { in.Reviewer = nativeReviewer() })
	shown := r.shown()

	obs := map[string]any{"blocked": blocked, "exit": r.exit, "stop": krrStop(shown), "refusal": krrRefusal(shown, id)}
	runs := e.runIDs()
	child := ""
	for _, x := range runs {
		if x != id {
			child = x
		}
	}
	obs["new_runs"] = len(runs) - 1
	reqs := f.requests()[before:]
	obs["requests"] = len(reqs)
	if len(reqs) > 0 {
		text := lastUserText(reqs[0])
		obs["finding_returned"] = strings.Contains(text, krrFinding)
		obs["repair_delivered"] = strings.Contains(text, "-candidate\n+"+c.repair)
		if c.previous {
			obs["same_reviewer"] = krrInOrder(legacyTexts(reqs[0]), "Preserve the fixture", "Final verification passed.", krrFinding)
		} else {
			input := requestInput(reqs[0])
			obs["same_reviewer"] = len(saved) > 0 && len(input) > len(saved) && reflect.DeepEqual(input[:len(saved)], saved)
		}
	}
	mu.Lock()
	obs["at_decision"] = decision
	mu.Unlock()
	obs["outcome"], obs["child_stage"], obs["child_verdict"] = nil, nil, nil
	if child != "" {
		if res, err := krrJSON(e.runPath(child, "result.json")); err == nil {
			obs["outcome"] = res["outcome"]
		}
		if doc, err := krrJSON(e.runPath(child, "checkpoint.json")); err == nil {
			state, _ := doc["state"].(map[string]any)
			cont, _ := state["continuation"].(map[string]any)
			obs["child_stage"] = cont["stage"]
			if session, ok := cont["review"].(map[string]any); ok {
				if v, ok := session["verdict"].(map[string]any); ok {
					obs["child_verdict"] = v["verdict"]
				}
			}
		}
	}
	commits, _ := strconv.Atoi(e.commitsSince(start))
	obs["commits"] = commits
	obs["committed_app"] = nil
	if commits > 0 {
		obs["committed_app"] = e.gitOut("show", "HEAD:app.txt")
	}
	obs["app_txt"] = e.read("app.txt")
	_, err := os.Stat(project.ApprovalPath(e.root, testSlug))
	obs["approval_kept"] = err == nil
	if !c.previous {
		obs["verified"] = krrLines(log)
	}
	return obs
}

// krrAtDecision is what the resumed Build had verified when the reopened reviewer was asked for
// its decision: the green and the tree of the resumed run's latest pass, and (version 4) every
// app.txt content the record check has seen.
func krrAtDecision(e *env, id, start, log string, previous bool) map[string]any {
	out := map[string]any{}
	for _, x := range e.runIDs() {
		if x == id {
			continue
		}
		doc, err := krrJSON(e.runPath(x, "checkpoint.json"))
		if err != nil {
			out["error"] = err.Error()
			continue
		}
		state, _ := doc["state"].(map[string]any)
		pass, _ := state["last_pass"].(map[string]any)
		var excluded []string
		for _, p := range asSlice(state["excluded"]) {
			if s, ok := p.(string); ok {
				excluded = append(excluded, s)
			}
		}
		repaired, err := e.gitRepo().Tree(start, excluded)
		out["pass_green"] = pass["green"]
		out["pass_tree_is_repaired"] = err == nil && pass["tree"] == repaired
	}
	if !previous {
		out["verified"] = krrLines(log)
	}
	return out
}

func asSlice(v any) []any { s, _ := v.([]any); return s }

func krrJSON(path string) (map[string]any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	err = json.Unmarshal(b, &m)
	return m, err
}

func krrLines(path string) []string {
	b, _ := os.ReadFile(path)
	out := []string{}
	for _, l := range strings.Split(strings.TrimSuffix(string(b), "\n"), "\n") {
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

var krrStopRe = regexp.MustCompile("(?m)^((?:candidate|environment|internal)/[a-z_]+): ")
var krrRefusalRe = regexp.MustCompile("(?m)^candidate/resume_[a-z_]+: (.*)$")

// krrStop is the first error class/kind line of the output ("" when there is none).
func krrStop(shown string) string {
	if m := krrStopRe.FindStringSubmatch(shown); m != nil {
		return m[1]
	}
	return ""
}

// krrRefusal is the resume refusal's message line with the Build id masked ("" when none).
func krrRefusal(shown, id string) string {
	if m := krrRefusalRe.FindStringSubmatch(shown); m != nil {
		return strings.ReplaceAll(m[1], "Build "+id+":", "Build ID:")
	}
	return ""
}

// krrInOrder: each part is in one of the texts, in order.
func krrInOrder(texts []string, parts ...string) bool {
	at := 0
	for _, p := range parts {
		for at < len(texts) && !strings.Contains(texts[at], p) {
			at++
		}
		if at == len(texts) {
			return false
		}
	}
	return true
}
`;
