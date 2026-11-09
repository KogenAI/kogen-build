package build

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/KogenAI/kogen-review/review"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-executor/executor"
	"github.com/KogenAI/kogen-executor/executor/claudecode"
	"github.com/KogenAI/kogen-executor/testkit/clauderuntime"
)

// The frozen `build --context` diagnostic reaches the builder's first prompt and every
// independent reviewer, whichever executors serve the build and review roles.

const builderSentinel = "builder-claim-sentinel"

// extCombo is one pairing of builder and reviewer executors: native, or the claude-code fake.
type extCombo struct {
	name                string
	extBuild, extReview bool
}

var extCombos = []extCombo{
	{"native/native", false, false},
	{"native/external", false, true},
	{"external/native", true, false},
	{"external/external", true, true},
}

// useRoles selects claude-code for the build and/or review role, commits the configuration and
// approves again. Neither external role leaves the project as it is.
func (e *env) useRoles(extBuild, extReview bool) {
	e.t.Helper()
	if !extBuild && !extReview {
		return
	}
	if extBuild {
		e.setRole("build", "  build:\n    executor: claude-code\n    model: claude-sonnet-5-5\n    effort: high\n")
	}
	if extReview {
		e.setRole("review", "  review:\n    executor: claude-code\n    model: claude-opus-5-5\n    effort: xhigh\n")
	}
	e.run("git", "commit", "-qm", "claude-code roles")
	e.approve()
}

// touchSource changes or deletes the diagnostic's original source file: it is a label, not
// something the Build may read again.
func touchSource(t *testing.T, path, how string) {
	t.Helper()
	switch how {
	case "changed":
		writeFile(t, path, "the original source was edited after the Build began\n", 0o644)
	case "deleted":
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
}

// runCombo runs a complete Build of the fixture with the given executors. The native builder
// and reviewer are scripted through the local provider, the external ones through the fake
// executor; both finish with the same candidate. A non-empty source is changed (or deleted)
// by the builder while it works.
func runCombo(t *testing.T, e *env, c extCombo, d *Diagnostic, sourceHow string) (*lineupExec, int) {
	t.Helper()
	e.useRoles(c.extBuild, c.extReview)
	f := newLineupExec(t)
	f.build = func(n int, root string, in executor.Input) (executor.RoundResult, error) {
		if d != nil && sourceHow != "" {
			touchSource(t, d.SourcePath, sourceHow)
		}
		lineupWriteApp(t, root, "candidate\n")
		return executor.RoundResult{Claim: builderSentinel}, nil
	}
	f.review = func(n int, root string, in executor.Input, task executor.Task) (executor.RoundResult, error) {
		return executor.RoundResult{Structured: verdictJSON("approve")}, nil
	}
	if !c.extBuild {
		var script []any
		if d != nil && sourceHow != "" {
			cmd := "rm -f '" + d.SourcePath + "'"
			if sourceHow == "changed" {
				cmd = "printf 'the original source was edited after the Build began\\n' > '" + d.SourcePath + "'"
			}
			script = append(script, "shell", map[string]any{"command": cmd})
		}
		script = append(script, "write_file", map[string]any{"path": "app.txt", "content": "candidate\n"}, "finish", map[string]any{"summary": builderSentinel})
		e.prov.then(e.prov.calls(script...))
	}
	if !c.extReview {
		e.prov.then(submit(e.prov, "approve"))
	}
	start, offset := e.head(), e.prov.count()
	a := e.build(withLineup(f), func(in *Input) { in.Context = d })
	if a.exit != 0 || a.err != nil {
		t.Fatalf("%s: exit %d err %v\n%s", c.name, a.exit, a.err, a.shown())
	}
	eq(t, c.name+" commits", e.commitsSince(start), "1")
	return f, offset
}

// plainText is everything a native request carries as text, in order.
func plainText(req map[string]any) string {
	var all strings.Builder
	for _, it := range requestInput(req) {
		m, _ := it.(map[string]any)
		switch c := m["content"].(type) {
		case string:
			all.WriteString(c)
			all.WriteString("\n")
		case []any:
			for _, b := range c {
				if bm, ok := b.(map[string]any); ok {
					if s, ok := bm["text"].(string); ok {
						all.WriteString(s)
						all.WriteString("\n")
					}
				}
			}
		}
	}
	return all.String()
}

// hasUserMessage reports whether the request carries a user message that is exactly content.
func hasUserMessage(req map[string]any, content string) bool {
	for _, it := range requestInput(req) {
		if m, _ := it.(map[string]any); m["role"] == "user" && m["content"] == content {
			return true
		}
	}
	return false
}

// builderSaw and reviewerSaw return the materials the two roles were given first.
func builderSaw(t *testing.T, e *env, f *lineupExec, c extCombo, offset int) string {
	t.Helper()
	if c.extBuild {
		tasks := f.starts[executor.RoleBuild]
		if len(tasks) != 1 {
			t.Fatalf("%s: %d build tasks", c.name, len(tasks))
		}
		return tasks[0].Brief.Stable + "\n\n" + tasks[0].Brief.Dynamic
	}
	return plainText(e.prov.requests()[offset])
}

func reviewerSaw(t *testing.T, e *env, f *lineupExec, c extCombo) string {
	t.Helper()
	if c.extReview {
		tasks := f.starts[executor.RoleReview]
		if len(tasks) != 1 {
			t.Fatalf("%s: %d review tasks", c.name, len(tasks))
		}
		return tasks[0].Brief.Stable + "\n\n" + tasks[0].Brief.Dynamic
	}
	reqs := e.prov.reviewRequests()
	if len(reqs) == 0 {
		t.Fatalf("%s: the native reviewer made no request", c.name)
	}
	return plainText(reqs[0])
}

// checkDelivery asserts that the reviewer received the exact labelled diagnostic message and
// nothing of the builder's own conversation.
func checkDelivery(t *testing.T, e *env, f *lineupExec, c extCombo, offset int, d *Diagnostic) {
	t.Helper()
	want := d.message()["content"].(string)
	if !strings.Contains(want, "Source: "+d.SourcePath+"\nSHA-256: "+d.SHA256+"\nDiagnostic text follows:\n"+d.Text+"\n") {
		t.Fatalf("%s: the expected message is not the labelled diagnostic: %q", c.name, want)
	}
	review := reviewerSaw(t, e, f, c)
	if strings.Count(review, want) != 1 {
		t.Fatalf("%s: the review has the exact diagnostic message %d times", c.name, strings.Count(review, want))
	}
	if !c.extReview && !hasUserMessage(e.prov.reviewRequests()[0], want) {
		t.Fatalf("%s: the native review does not carry the diagnostic as its own user message", c.name)
	}
	lacks(t, c.name+" review", review, builderSentinel)
	if c.extBuild {
		eq(t, c.name+" builder rounds", len(f.rounds(executor.RoleBuild)), 1)
	}
	build := builderSaw(t, e, f, c, offset)
	if strings.Count(build, want) != 1 {
		t.Fatalf("%s: the builder's first prompt has the exact diagnostic message %d times", c.name, strings.Count(build, want))
	}
}

func TestContextExternalReviewMatrix(t *testing.T) {
	t.Parallel()
	for _, c := range extCombos {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			for _, how := range []string{"", "changed", "deleted"} {
				t.Run("source "+how, func(t *testing.T) {
					t.Parallel()
					e := newEnv(t)
					text := "Earlier candidate failed at line 3 — Δ🙂\r\nnot approval or old green proof\n"
					d := &Diagnostic{Text: text, SourcePath: filepath.Join(e.aux, "context.md"), SHA256: digestOf(text)}
					writeFile(t, d.SourcePath, text, 0o644)
					f, offset := runCombo(t, e, c, d, how)
					checkDelivery(t, e, f, c, offset, d)
				})
			}
			t.Run("recovery handoff", func(t *testing.T) {
				t.Parallel()
				e, id := blocked(t, "printf 'candidate raw\\n' > filtered")
				output := filepath.Join(physical(t, t.TempDir()), "archive")
				if r := e.recover(id, output); r.exit != 0 || r.err != nil {
					t.Fatalf("recover: exit %d\n%s", r.exit, r.shown())
				}
				raw, err := os.ReadFile(filepath.Join(output, "context.md"))
				if err != nil {
					t.Fatal(err)
				}
				text := string(raw)
				contains(t, "handoff", text, "BUILDER-REPORTED and unproven.")
				contains(t, "handoff", text, "archive "+output)
				contains(t, "handoff", text, "blocker.json SHA-256 ")
				contains(t, "handoff", text, "checkpoint.json SHA-256 ")
				d := &Diagnostic{Text: text, SourcePath: filepath.Join(output, "context.md"), SHA256: digestOf(text)}
				if err := d.Validate(); err != nil {
					t.Fatal(err)
				}
				f, offset := runCombo(t, e, c, d, "")
				checkDelivery(t, e, f, c, offset, d)
				// The archive is diagnostic data only: the candidate is the new Build's own.
				lacks(t, "committed filtered", strings.ToLower(e.gitOut("show", "HEAD:filtered")), "candidate raw")
			})
		})
	}
}

// ---- the first prompt of the real Claude Code adapter ----

// candidateWriter wraps the real Claude Code adapter: the shared fake runtime writes no files, so
// each round first writes the candidate (app.txt) into the task's checkout, as the runtime's former
// script did, and then runs the adapter's own session.
type candidateWriter struct {
	executor.Executor
	t *testing.T
}

func (c candidateWriter) Start(ctx context.Context, task executor.Task, emit executor.Sink) (executor.Session, error) {
	s, err := c.Executor.Start(ctx, task, emit)
	if err != nil {
		return nil, err
	}
	return candidateSession{Session: s, t: c.t, root: task.Root}, nil
}

// FirstPromptBytes forwards the adapter's exact first-prompt size.
func (c candidateWriter) FirstPromptBytes(role string, brief executor.Brief) int {
	if sizer, ok := c.Executor.(executor.PromptSizer); ok {
		return sizer.FirstPromptBytes(role, brief)
	}
	return len(brief.Stable) + len(brief.Dynamic)
}

type candidateSession struct {
	executor.Session
	t    *testing.T
	root string
}

func (s candidateSession) Round(ctx context.Context, in executor.Input) (executor.RoundResult, error) {
	lineupWriteApp(s.t, s.root, "candidate\n")
	return s.Session.Round(ctx, in)
}

// successRound is a successful result line of the fake runtime.
func successRound() string {
	return clauderuntime.ResultLine(map[string]any{"stop_reason": "end_turn", "permission_denials": []any{}})
}

// withClaudeProcess installs the shared fake Claude runtime of kogen-executor (round 1 plays the
// given stream lines after the init line; none means a successful result) and provides the real
// Claude Code adapter on it, wrapped to write the candidate.
func withClaudeProcess(t *testing.T, e *env, round ...string) (func(*Input), *clauderuntime.Fixture) {
	t.Helper()
	if len(round) == 0 {
		round = []string{successRound()}
	}
	dir := physical(t, t.TempDir())
	fx := clauderuntime.Install(t, clauderuntime.Options{
		Home:     filepath.Join(dir, "home"),
		State:    filepath.Join(dir, "state"),
		Rounds:   [][]string{round},
		Checkout: physical(t, e.root),
	})
	return func(in *Input) {
		in.External = func(project.Role) (executor.Executor, error) {
			return candidateWriter{Executor: claudecode.New(claudecode.Options{Home: fx.Home, HookCommand: []string{"/bin/true"}}), t: t}, nil
		}
	}, fx
}

// roundCalls is the number of rounds the fake runtime played.
func roundCalls(t *testing.T, fx *clauderuntime.Fixture) string {
	t.Helper()
	n, _ := os.ReadFile(filepath.Join(fx.State, "calls"))
	return strings.TrimSpace(string(n))
}

// TestContextExternalMalformedRuntimeResult: a malformed round result is refused by the adapter,
// the Build stops after one round and commits nothing.
func TestContextExternalMalformedRuntimeResult(t *testing.T) {
	for _, c := range []struct{ name, line, refusal string }{
		{"truncated json", `{"type":"result","subtype":"success","is_error":false`,
			"executor/protocol: line 2 of the event stream is not a JSON object"},
		{"is_error not a boolean", clauderuntime.ResultLine(map[string]any{"is_error": "false"}),
			"executor/protocol: the result has no boolean is_error flag"},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			e.useRoles(true, false)
			provide, fx := withClaudeProcess(t, e, c.line)
			start := e.head()
			a := e.build(provide)
			if a.exit != 1 || a.err != nil {
				t.Fatalf("exit %d err %v\n%s", a.exit, a.err, a.shown())
			}
			contains(t, "output", a.shown(), c.refusal)
			eq(t, "commits", e.commitsSince(start), "0")
			eq(t, "rounds", roundCalls(t, fx), "1")
		})
	}
}

func TestContextExternalFirstPrompt(t *testing.T) {
	text := "Previous candidate failed at line 3 — \xf0\x9f\x99\x82\nnot approval, a plan or current proof\n"
	text = strings.ToValidUTF8(text, "")
	for _, withContext := range []bool{true, false} {
		t.Run(fmt.Sprintf("context %v", withContext), func(t *testing.T) {
			e := newEnv(t)
			e.useRoles(true, false)
			provide, fx := withClaudeProcess(t, e)
			d := &Diagnostic{Text: text, SourcePath: "/inaccessible/label", SHA256: digestOf(text)}
			if !withContext {
				d = nil
			}
			a := e.build(provide, func(in *Input) { in.Context = d })
			if a.exit != 0 || a.err != nil {
				t.Fatalf("exit %d err %v\n%s", a.exit, a.err, a.shown())
			}
			prompt := fx.Prompt(t, 1)
			contains(t, "prompt", prompt, "# Protected paths")
			if d == nil {
				lacks(t, "prompt", prompt, "Caller diagnostic data")
				return
			}
			want := d.message()["content"].(string)
			if strings.Count(prompt, want) != 1 {
				t.Fatalf("the first prompt has the exact labelled diagnostic %d times:\n%.600s", strings.Count(prompt, want), prompt)
			}
			// It follows the authority and precedes the dynamic evidence, as in the native opening.
			if strings.Index(prompt, "# Protected paths") > strings.Index(prompt, want) {
				t.Fatal("the diagnostic precedes the authority")
			}
			contains(t, "prompt", prompt, want+"\n\n# Files\n\n")
			if n := roundCalls(t, fx); n != "1" {
				t.Fatalf("calls %q: the diagnostic must not depend on a later round", n)
			}
		})
	}
}

// ---- the native bytes and the omitted context ----

// reviewBody is the first request body of the native reviewer, exactly as received.
func reviewBody(t *testing.T, e *env) string {
	t.Helper()
	reqs, bodies := e.prov.requests(), e.prov.rawRequests()
	for i, req := range reqs {
		if text := requestInstructions(req); text == review.ReviewerInstructions() {
			return string(bodies[i])
		}
	}
	t.Fatal("no native review request")
	return ""
}

var conversationID = regexp.MustCompile(`conversation-[0-9a-f]{16}`)

var gitObjectName = regexp.MustCompile(`\b[0-9a-f]{40}\b|(start HEAD )[0-9a-f]{7,40}\b`)

// normalized replaces only what a temporary repository makes nondeterministic: its folders,
// the run, the conversation ids and the Git object names. The prompt text is untouched.
func normalized(e *env, text string) string {
	for _, id := range e.runIDs() {
		text = strings.ReplaceAll(text, id, "<run>")
	}
	for from, to := range map[string]string{e.root: "<root>", e.aux: "<aux>", e.home: "<home>"} {
		text = strings.ReplaceAll(text, from, to)
	}
	text = conversationID.ReplaceAllString(text, "conversation-<id>")
	return gitObjectName.ReplaceAllString(text, "${1}<git-object>")
}

// openings are the first request or prompt of the builder and of the reviewer, as the roles
// receive them: native requests are the bodies the provider received, reduced by
// kogenOwnedRequest to what Kogen owns (without the pinned loop's generic tool definitions and
// envelope fields).
func openings(t *testing.T, c extCombo, d *Diagnostic) map[string]string {
	t.Helper()
	e := newEnvIn(t, physical(t, t.TempDir()), physical(t, t.TempDir()), physical(t, t.TempDir()),
		[]string{"GIT_AUTHOR_DATE=2026-10-22T00:00:00Z", "GIT_COMMITTER_DATE=2026-10-22T00:00:00Z"})
	f, offset := runCombo(t, e, c, d, "")
	out := map[string]string{}
	if c.extBuild {
		task := f.starts[executor.RoleBuild][0]
		out["builder.stable"], out["builder.dynamic"] = task.Brief.Stable, task.Brief.Dynamic
	} else {
		out["builder"] = string(e.prov.rawRequests()[offset])
	}
	if c.extReview {
		task := f.starts[executor.RoleReview][0]
		out["reviewer.stable"], out["reviewer.dynamic"] = task.Brief.Stable, task.Brief.Dynamic
	} else {
		out["reviewer"] = reviewBody(t, e)
	}
	for k, v := range out {
		out[k] = normalized(e, v)
		if k == "builder" || k == "reviewer" {
			out[k] = kogenOwnedRequest(t, out[k])
		}
		if k == "reviewer" {
			out[k] = withoutReviewerWording(t, out[k])
		}
		out[k] = withoutLineupFile(out[k])
	}
	return out
}

// reviewerInstructionsPlaceholder and reviewerToolPlaceholder stand for the wording kogen-review owns.
const reviewerInstructionsPlaceholder = "<review.ReviewerInstructions()>"

var reviewerToolPlaceholder = map[string]any{"review_tool_definition": review.SubmitReviewTool}

// withoutReviewerWording is a native reviewer request (compact JSON from kogenOwnedRequest) with the
// wording kogen-review owns replaced by placeholders: the instructions, which must equal
// review.ReviewerInstructions(), and the submit_review definition, which must equal
// review.ReviewerToolDefinitions(review.SubmitReviewTool)[0] as a decoded JSON value. The tools keep
// their order and nothing else changes; the request is encoded again with sorted keys.
func withoutReviewerWording(t *testing.T, wire string) string {
	t.Helper()
	var v map[string]any
	dec := json.NewDecoder(strings.NewReader(wire))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		t.Fatalf("reviewer request JSON: %v", err)
	}
	if got, _ := v["instructions"].(string); got != review.ReviewerInstructions() {
		t.Fatalf("the reviewer's instructions are not review.ReviewerInstructions()")
	}
	v["instructions"] = reviewerInstructionsPlaceholder
	exported, err := json.Marshal(review.ReviewerToolDefinitions(review.SubmitReviewTool)[0])
	if err != nil {
		t.Fatal(err)
	}
	want := decodeNumbers(t, exported)
	tools, _ := v["tools"].([]any)
	found := false
	for i, tool := range tools {
		if m, _ := tool.(map[string]any); m["name"] == review.SubmitReviewTool {
			if !reflect.DeepEqual(tool, want) {
				t.Fatalf("the %s definition differs from review.ReviewerToolDefinitions(%s)[0]:\n got %v\nwant %v", review.SubmitReviewTool, review.SubmitReviewTool, tool, want)
			}
			tools[i] = reviewerToolPlaceholder
			found = true
		}
	}
	if !found {
		t.Fatalf("the reviewer's request offers no %s tool", review.SubmitReviewTool)
	}
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		t.Fatal(err)
	}
	return strings.TrimSuffix(out.String(), "\n")
}

// withoutLineupFile is text without `.kogen/models.yaml` in the protected files the prompts name
// and in the sorted list of the project's tracked files: the reverse of lineupFileForms and
// trackedLineupFileForms. Whether the coordinator lists the lineup file is not what these bytes
// assert, so the golden, the output and what -update records all go through it; no other byte changes.
func withoutLineupFile(text string) string {
	for _, forms := range [][][2]string{lineupFileForms, trackedLineupFileForms} {
		for _, f := range forms {
			text = strings.ReplaceAll(text, f[1], f[0])
		}
	}
	return text
}

// diagnosticOf is the diagnostic every native-bytes case supplies.
func diagnosticOf() *Diagnostic {
	text := "Earlier candidate failed at line 3 — Δ🙂\r\nnot approval or old green proof\n"
	return &Diagnostic{Text: text, SourcePath: "/diagnostic/context.md", SHA256: digestOf(text)}
}

// sameBytes fails with the first difference of two texts.
func sameBytes(t *testing.T, name, got, want string) {
	t.Helper()
	if got == want {
		return
	}
	i := 0
	for i < len(got) && i < len(want) && got[i] == want[i] {
		i++
	}
	cut := func(s string) string { return fmt.Sprintf("%q", s[max(0, i-80):min(len(s), i+80)]) }
	t.Fatalf("%s differs at byte %d (%d bytes, want %d):\n got ...%s\nwant ...%s", name, i, len(got), len(want), cut(got), cut(want))
}

// TestContextExternalNativeBytes: the native opening requests (with and without the
// diagnostic) and the omitted-context requests of every executor pairing are byte for byte
// what they were before the diagnostic reached external roles (testdata/context-external-golden.json,
// recorded from the unfixed Build); only the folders, run, conversation and Git object names are normalized,
// and a native request is compared without the pinned loop's generic tool definitions and envelope fields.
// The golden holds those normalized parts per pairing and context; it is frozen and must not
// change without deliberate intent. Whether `.kogen/models.yaml` is a member of the coordinator's
// protected files is not compared: it is removed from the golden and from the output alike, and
// from what -update records. `go test ./... -update` rewrites it from the current output
// (a Git diff to review like any other change); the external builder's "with context" subtests,
// whose expectation derives from stored entries, then derive it from this run's own output.
//
// The reviewer's instructions, the `submit_review` definition and the external review task are
// kogen-review's: they are compared with its exports (review.ReviewerInstructions(),
// review.ReviewerToolDefinitions(review.SubmitReviewTool), review.LimitRaises) and not stored in the
// golden. The native reviewer part holds placeholders for the first two; an external reviewer's
// parts are not stored: they are the native reviewer's authority, diagnostic and review source
// followed by the review task, which carries review.LimitRaises.
func TestContextExternalNativeBytes(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("testdata/context-external-golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden map[string]map[string]string
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}
	for _, parts := range golden {
		for part, text := range parts {
			parts[part] = withoutLineupFile(text)
		}
	}
	d := diagnosticOf()
	// With -update, each subtest that owns a golden entry records its output under the entry's
	// key, and the entries are written once all subtests have finished.
	var recorded sync.Map
	if *update {
		updateGoldenAfter(t, func() {
			updateGoldenJSON(t, "testdata/context-external-golden.json", indent2Golden, func(doc any) any {
				entries := goldenObject(t, doc)
				recorded.Range(func(key, parts any) bool {
					entries[key.(string)] = asJSONValue(t, parts)
					return true
				})
				return entries
			})
		})
	}
	// nativeReviewer is the native reviewer part a subtest derives an external reviewer's expectation
	// from: the golden's, or with -update this run's own.
	nativeReviewer := func(t *testing.T, withContext bool) string {
		t.Helper()
		dd, name := (*Diagnostic)(nil), "native/native without context"
		if withContext {
			dd, name = d, "native/native with context"
		}
		if *update {
			return openings(t, extCombos[0], dd)["reviewer"]
		}
		return golden[name]["reviewer"]
	}
	// check compares the parts of one subtest with the golden entry, or records them. The parts of
	// an external reviewer are not in the golden: they are checked against the native reviewer's.
	check := func(t *testing.T, key string, c extCombo, withContext bool, got map[string]string) {
		t.Helper()
		own := map[string]string{}
		for part, text := range got {
			if !c.extReview {
				own[part] = text
			} else if part != "reviewer.stable" && part != "reviewer.dynamic" {
				own[part] = text
			}
		}
		if *update {
			recorded.Store(key, own)
		} else {
			want := golden[key]
			eq(t, "parts", len(own), len(want))
			for part, text := range own {
				sameBytes(t, part, text, want[part])
			}
		}
		if c.extReview {
			sameExternalReviewer(t, got, nativeReviewer(t, withContext), withContext)
		}
	}
	for _, c := range extCombos {
		t.Run(c.name+" without context", func(t *testing.T) {
			t.Parallel()
			check(t, c.name+" without context", c, false, openings(t, c, nil))
		})
		if c.extBuild {
			continue
		}
		t.Run(c.name+" with context", func(t *testing.T) {
			t.Parallel()
			check(t, c.name+" with context", c, true, openings(t, c, d))
		})
	}
	// An external builder gets exactly the omitted-context brief plus the diagnostic message,
	// and its reviewer, native or external, exactly what the native builder's reviewer gets.
	for _, c := range extCombos {
		if !c.extBuild {
			continue
		}
		t.Run(c.name+" with context", func(t *testing.T) {
			t.Parallel()
			plain, got := golden[c.name+" without context"], openings(t, c, d)
			if *update {
				// The golden may not be rewritten yet: derive the expectation from the output.
				plain = openings(t, c, nil)
			}
			sameBytes(t, "builder.stable", got["builder.stable"], withoutLineupFile(plain["builder.stable"]+d.message()["content"].(string)))
			sameBytes(t, "builder.dynamic", got["builder.dynamic"], plain["builder.dynamic"])
			if c.extReview {
				return
			}
			sameBytes(t, "reviewer", got["reviewer"], nativeReviewer(t, true))
		})
	}
}

// sameExternalReviewer asserts an external reviewer's parts against the native reviewer's request
// (nativeWire, as withoutReviewerWording leaves it). reviewer.dynamic is the review source; reviewer.stable
// begins with the authority, then with context the diagnostic message and a blank line, and the
// rest is kogen-review's review task, which must carry review.LimitRaises once; its other bytes are not compared.
func sameExternalReviewer(t *testing.T, got map[string]string, nativeWire string, withContext bool) {
	t.Helper()
	var native struct {
		Input []struct {
			Content json.RawMessage `json:"content"`
		} `json:"input"`
	}
	if err := json.Unmarshal([]byte(nativeWire), &native); err != nil {
		t.Fatalf("native reviewer request JSON: %v", err)
	}
	var blocks []struct {
		Text string `json:"text"`
	}
	wantMessages := 1
	if withContext {
		wantMessages = 2
	}
	if len(native.Input) != wantMessages || json.Unmarshal(native.Input[0].Content, &blocks) != nil || len(blocks) != 2 {
		t.Fatalf("the native reviewer's input is not the authority and the review source (with context %v)", withContext)
	}
	authority, source := blocks[0].Text, blocks[1].Text
	prefix := authority + "\n"
	if withContext {
		var diagnostic string
		if err := json.Unmarshal(native.Input[1].Content, &diagnostic); err != nil {
			t.Fatalf("the native reviewer's diagnostic message: %v", err)
		}
		prefix = authority + "\n" + diagnostic + "\n" + "\n"
	}
	sameBytes(t, "reviewer.dynamic", got["reviewer.dynamic"], source)
	stable := got["reviewer.stable"]
	sameBytes(t, "reviewer.stable", stable[:min(len(stable), len(prefix))], prefix)
	task := stable[len(prefix):]
	if task == "" {
		t.Fatal("reviewer.stable has no review task after the authority")
	}
	if n := strings.Count(task, review.LimitRaises); n != 1 {
		t.Fatalf("the review task carries review.LimitRaises %d times, want once", n)
	}
}
