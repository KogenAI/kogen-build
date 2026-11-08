package build

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-coordinator/route"
	"github.com/KogenAI/kogen-executor/agent"
	"github.com/KogenAI/kogen-executor/core"
	"github.com/KogenAI/kogen-executor/executor"
	"github.com/KogenAI/kogen-loop/provider"
	"github.com/KogenAI/kogen-loop/sys"
	"github.com/KogenAI/kogen-loop/tools"
	"github.com/KogenAI/kogen-verification/verify"
)

// openingMessage is the builder's first message (LOOP-2) as the native conversation opens
// with it: the brief as two input_text blocks (the stable authority and the dynamic evidence
// behind it).
func (b *Attempt) openingMessage() ([]any, error) {
	brief, err := b.brief()
	if err != nil {
		return nil, err
	}
	return provider.ContractBlocks(brief.Stable, brief.Dynamic), nil
}

// brief is the task of the builder (LOOP-2), whichever executor does the work: the Intent,
// its acceptance feature and model, the project's README and testing rules, the configured
// commands, the protected paths (the stable authority) and the file list (the dynamic
// evidence behind it).
func (b *Attempt) brief() (executor.Brief, error) {
	modelText := ""
	if t, err := readFile(b.Root, project.ModelPath(b.Slug)); err == nil {
		modelText = "\n# model.qnt\n\n```quint\n" + string(t) + "```\n"
	}
	feature, err := readFile(b.Root, project.FeaturePath(b.Slug))
	if err != nil {
		return executor.Brief{}, err
	}
	readme := "(no README.md)"
	if t, err := readFile(b.Root, "README.md"); err == nil {
		readme = string(t)
		if max := b.deps.BuildBriefMaxReadmeBytes; b.ext == nil && max > 0 && len(t) > max {
			readme = string(t[:max]) + fmt.Sprintf("\n[README.md cut: %d bytes omitted; read_file README.md for the rest.]", len(t)-max)
		}
	}
	var checks []string
	for _, c := range b.Config.Checks {
		checks = append(checks, c.Name+": "+c.Run)
	}
	acceptanceRun := ""
	if b.Config.Acceptance != nil {
		acceptanceRun = b.Config.Acceptance.Run
	}
	var baseline []string
	for _, p := range b.Baseline {
		baseline = append(baseline, "- "+p)
	}
	featurePath := project.FeaturePath(b.Slug)
	stable := project.SourceMessage(b.SourceProvenance) + "\n\n" +
		"# Intent: " + b.intent.Title + "\n\n" +
		"```markdown\n" + b.intent.Text + "```\n\n" +
		"# Acceptance feature (" + featurePath + ")\n\n" +
		"```gherkin\n" + string(feature) + "```\n" +
		modelText + "\n" +
		"# Project instructions (README.md)\n\n" + readme + "\n\n" +
		"# Testing rules (docs/testing-rules.md)\n\n" + agent.Prompt(agent.TestingRules) + "\n\n" +
		b.State.Navigation() + "\n" +
		"# Configured commands\n\n" +
		"setup: " + inspectStrings(b.Config.Setup) + "\n" +
		"fix (run once after each of your rounds): " + inspectStrings(b.Config.Fix) + "\n" +
		"checks: " + strings.Join(checks, "; ") + "\n" +
		"acceptance: " + acceptanceRun + " (runs " + featurePath + ")\n\n" +
		"# Protected paths\n\n" +
		protectedIntro + "\n" + strings.Join(baseline, "\n") + "\n\n" +
		manifestNotice + "\n\n"
	files, err := b.repo.OK("ls-files", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return executor.Brief{}, err
	}
	if max := b.deps.BuildBriefMaxFiles; b.ext == nil && max > 0 {
		lines := strings.SplitAfter(files, "\n")
		if lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		if len(lines) > max {
			files = strings.Join(lines[:max], "") + fmt.Sprintf("[File list cut: %d entries omitted; use shell git ls-files --cached --others --exclude-standard for the rest.]", len(lines)-max)
		}
	}
	return executor.Brief{Stable: stable, Dynamic: b.baselineText() + "# Files\n\n" + files}, nil
}

// drive is the builder's conversation (LOOP-1..7), whichever executor runs it: a round (the
// executor's requests and tool calls), and after the builder finishes (or answers without a
// tool call) the fixers and a verification pass; a red pass goes back to the same session as
// feedback, a green one to review and commit. newRound starts a numbered round before the
// round. An external executor's round is a claim like any other: Kogen's baseline and HEAD
// detection runs after it, and its round cap is enforced here.
func (b *Attempt) drive(ctx context.Context, sess executor.Session, in executor.Input, newRound bool) error {
	rounds := 0
	if b.ext != nil && b.Executor != nil {
		rounds = b.Executor.Rounds
	}
	for {
		if newRound {
			if err := b.facts.update(b, "building", ""); err != nil {
				return err
			}
			b.Round++
			if err := b.Track(); err != nil {
				return err
			}
			b.s.Progress(fmt.Sprintf("kogen: building (round %d)", b.Round))
		}
		previous := b.Continuation
		if b.ext != nil {
			if err := b.Save(Continuation{Stage: StageTool, Owner: "build"}); err != nil {
				return err
			}
		}
		start, err := b.checkRepo().Tree(b.Start, b.Excluded)
		if err != nil {
			return err
		}
		b.noEdit.roundStart, b.noEdit.wrote = start, false
		res, err := b.runRound(ctx, sess, in)
		if err != nil {
			if e, ok := sys.AsError(err); ok && e.Class == "executor" && e.Reason == executor.ReasonUsageLimit && b.ext != nil {
				if saveErr := b.Save(Continuation{Stage: StageUsageLimit, Feedback: in.Feedback}); saveErr != nil {
					return saveErr
				}
			} else if b.ext != nil {
				if previous.Stage == StageUsageLimit {
					previous = Continuation{Stage: StageBuilder}
				}
				b.Continuation = previous
			}
			return err
		}
		b.blockerNote = ""
		b.noEdit.rounds++
		if b.ext != nil {
			rounds++
			if err := b.afterExternalRound(res, rounds); err != nil {
				return err
			}
		}
		feedback, landed, err := b.roundEnd(ctx)
		if err != nil || landed {
			return err
		}
		if b.blockerNote != "" {
			feedback = b.blockerNote + "\n\n" + feedback
		}
		if b.ext != nil && b.ext.maxRounds > 0 && rounds >= b.ext.maxRounds {
			return executor.RoundLimit(b.ext.maxRounds)
		}
		in, newRound = executor.Input{Feedback: feedback}, true
	}
}

// afterRound is the conversation after a round's verification: the feedback answers the
// finish call, or is a user message when the builder ended without one.
func (b *Attempt) afterRound(input []any, finish map[string]any, feedback string) []any {
	if finish != nil {
		return append(slices.Clone(input), tools.OutputItem(finish["call_id"], tools.Cut(feedback)))
	}
	return append(slices.Clone(input), map[string]any{"role": "user", "content": feedback})
}

// reportBlocker handles the builder's report_blocker call (LOOP-7). A valid report (exactly
// nonempty unique current protected paths, a reason and observed evidence) stops the Build
// as BUILDER-REPORTED build_blocked, provided HEAD and the protected bytes are still the
// original and one of its paths is named by a failure of the latest verification pass of the
// current tree; anything else is feedback and grants nothing.
func (b *Attempt) reportBlocker(args map[string]any, call tools.Call) (string, error) {
	if !validBlocker(args, b.Baseline) {
		return invalidBlockerMessage, nil
	}
	if err := b.blockerCustody(); err != nil {
		return "", err
	}
	if msg, err := b.unbackedBlocker(args); err != nil || msg != "" {
		return msg, err
	}
	return "", b.stopBlocked(args, func() {
		b.Run.Event(map[string]any{
			"event": "tool_call", "call_id": call.CallID(), "name": "report_blocker",
			"arguments": provider.Fingerprint(call.Arguments), "result": "BUILDER-REPORTED build_blocked",
		})
	})
}

const invalidBlockerMessage = "invalid blocker: supply exactly nonempty unique current protected paths, reason and observed evidence strings; " +
	"no write permission is granted; continue ordinary implementation repairs"

// stopBlocked is the part of a valid blocker report shared by the native tool and external
// executors: the guards, the record, the events and the build_blocked stop. It always returns
// an error. recorded runs once the blocker is tracked, before the builder_blocker event.
func (b *Attempt) stopBlocked(args map[string]any, recorded func()) error {
	if err := b.blockerCustody(); err != nil {
		return err
	}
	blocker := map[string]any{}
	for k, v := range args {
		blocker[k] = v
	}
	blocker["classification"] = "BUILDER-REPORTED"
	b.Blocker = blocker
	if err := b.Track(); err != nil {
		return err
	}
	recorded()
	b.Run.Event(map[string]any{"event": "builder_blocker", "blocker": blocker})
	var paths []string
	for _, p := range args["paths"].([]any) {
		paths = append(paths, p.(string))
	}
	e := sys.Fail("candidate", "build_blocked",
		"BUILDER-REPORTED protected-baseline contradiction; edits are left in place; this report is not independently proven", 1)
	e.Lines = []string{
		"paths: " + strings.Join(paths, ", "),
		"reason: " + tools.Cut(args["reason"].(string)),
		"evidence: " + tools.Cut(args["evidence"].(string)),
	}
	return e
}

// blockerCustody is the check that the protected bytes and HEAD are still the original, which
// every well-formed claim passes before its backing is looked at.
func (b *Attempt) blockerCustody() error {
	if err := b.BaselineIntact(); err != nil {
		return err
	}
	return b.headIntact()
}

func validBlocker(args map[string]any, baseline []string) bool {
	if len(args) != 3 {
		return false
	}
	for _, k := range []string{"evidence", "paths", "reason"} {
		if _, ok := args[k]; !ok {
			return false
		}
	}
	paths, ok := args["paths"].([]any)
	if !ok || len(paths) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, p := range paths {
		s, ok := p.(string)
		if !ok || !slices.Contains(baseline, s) || seen[s] {
			return false
		}
		seen[s] = true
	}
	for _, k := range []string{"reason", "evidence"} {
		if s, ok := args[k].(string); !ok || strings.TrimSpace(s) == "" {
			return false
		}
	}
	return true
}

// roundEnd runs the fixers once, then passes and reviews until the builder has work again
// (feedback) or the Build landed.
func (b *Attempt) roundEnd(ctx context.Context) (feedback string, landed bool, err error) {
	if err := verify.Fix(ctx, b.s, b.verifyInput(b.Pass)); err != nil {
		return "", false, err
	}
	stage := ""
	if b.StageProgress != nil && b.StageProgress.Phase == phaseStage {
		stage = b.StagePlan[len(b.StageProgress.Completed)].ID
	}
	notice, check, err := b.noEditRound(stage)
	if err != nil || !check {
		return notice, false, err
	}
	if stage != "" {
		return b.stageGate(ctx)
	}
	return b.verifyAndReview(ctx, true)
}

// stageGate is the scoped gate of the active stage after a builder round: a pass of the
// stage's own checks only, recorded as durable private evidence. A green gate advances the
// plan and the next stage's navigation answers the builder's finish call; the last one
// leaves the whole-Intent gates, independent review and commit. A red gate keeps the builder
// at the same stage.
func (b *Attempt) stageGate(ctx context.Context) (string, bool, error) {
	p := b.StageProgress.Clone()
	index := len(p.Completed) + 1
	stage := b.StagePlan[index-1]
	b.s.Progress(fmt.Sprintf("kogen: stage %d/%d: %s", index, p.Total, stage.ID))
	pass, err := b.verifyStage(ctx, stage)
	if err != nil {
		return "", false, err
	}
	evidence, err := b.retainStage(StageEvidence{Pass: pass, StageID: stage.ID, Index: index, Contract: b.ApprovalBytes})
	if err != nil {
		return "", false, err
	}
	b.StageEvidence = append(slices.Clone(b.StageEvidence), evidence)
	b.Run.Event(map[string]any{
		"event": "stage_pass", "stage_id": stage.ID, "stage_index": index, "tree": pass.Tree,
		"green": pass.Green, "failing": core.NonNil(pass.Failing), "not_run": core.NonNil(pass.NotRun),
	})
	if pass.Green {
		b.s.Progress("kogen: stage " + stage.ID + " gate passed")
	} else {
		b.s.Progress("kogen: stage " + stage.ID + " gate failed")
	}
	if err := b.hostOrBaseStop(pass); err != nil {
		return "", false, err
	}
	if b.ext != nil {
		if err := b.externalStalled(stage.ID, pass); err != nil {
			return "", false, err
		}
	}
	b.remember(stage.ID, pass)
	editedNotice := b.unmovedNotice(stage.ID, pass)
	if !pass.Green {
		b.Repairs++
		if err := b.Track(); err != nil {
			return "", false, err
		}
		return withNotice(editedNotice, "Stage "+stage.ID+" failed; remain at this stage.\n"+b.PassFeedback(pass, "finish")), false, nil
	}
	p.Completed = append(p.Completed, stage.ID)
	p.Active, p.Phase = nil, phaseFinal
	if index < len(b.StagePlan) {
		next := b.StagePlan[index].ID
		p.Active, p.Phase = &next, phaseStage
	}
	b.StageProgress = &p
	if err := b.Track(); err != nil {
		return "", false, err
	}
	if p.Active == nil {
		// Every scoped stage is verified: the whole Intent's own gates, review and commit.
		if err := b.Save(b.Continuation); err != nil {
			return "", false, err
		}
		return b.verifyAndReview(ctx, true)
	}
	// The verified milestone and its navigation are durable before the next request.
	feedback := "Stage " + stage.ID + " passed. Continue the next stage. " + b.State.Navigation()
	c := b.Continuation
	next := b.afterRound(c.Input, c.Finish, feedback)
	if err := b.Save(Continuation{Stage: StageBuilder, Input: next}); err != nil {
		return "", false, err
	}
	return feedback, false, nil
}

// verifyStage runs the stage's scoped pass: numbered like any pass, but never recorded as the
// Build's last pass (a milestone is historical evidence, not whole-Intent proof).
func (b *Attempt) verifyStage(ctx context.Context, stage stageDef) (verify.Pass, error) {
	b.Pass++
	if err := b.Track(); err != nil {
		return verify.Pass{}, err
	}
	if err := b.facts.update(b, "verifying", ""); err != nil {
		return verify.Pass{}, err
	}
	in := b.verifyInput(b.Pass)
	in.Stage = &verify.Stage{ID: stage.ID, Checks: stage.Checks}
	pass, err := verify.Verify(ctx, b.s, in)
	if err != nil {
		return verify.Pass{}, err
	}
	return pass, b.Track()
}

// withNotice puts a notice, when there is one, and a blank line before feedback.
func withNotice(notice, feedback string) string {
	if notice == "" {
		return feedback
	}
	return notice + "\n\n" + feedback
}

// verifyAndReview is the whole-Intent pass, review and commit, routed by kogen-coordinator's
// route (steps). A builder pass (after a builder round) is also checked for a repeated host or
// base failure; a pass that only restarts verification after a drifted review is not. Red
// feedback that goes back to the builder ends with the finish reminder.
func (b *Attempt) verifyAndReview(ctx context.Context, builderPass bool) (string, bool, error) {
	feedback, landed, err := route.Cycle(ctx, steps{b}, builderPass)
	if err != nil || landed {
		return feedback, landed, err
	}
	return feedback + "\n\n" + finishReminder, false, nil
}

// ---- resume of a conversation stage ----

// builderResumeRefusal is why the builder's own stage cannot be resumed ("" when it can). Only
// the native builder's session resumes: an external executor's unfinished session cannot
// (resume capability: none), while at a review stage that builder has finished.
func (b *Attempt) builderResumeRefusal() string {
	x := b.Executor
	if x == nil || x.Name == executor.Native || !slices.Contains([]string{StageBuilder, StageBuilderVerify}, b.Continuation.Stage) {
		return ""
	}
	return "Builds by executor " + x.Name + " cannot be resumed in this version (resume capability: none); " +
		"inspect the preserved candidate or start a fresh Build"
}

// resumeStage continues the Build from its checkpointed stage: the native builder's
// conversation, or a routed step (kogen-coordinator's route.Resume), never an external
// executor's builder session.
func (b *Attempt) resumeStage(ctx context.Context) error {
	if why := b.builderResumeRefusal(); why != "" {
		return core.ResumeRefused(why)
	}
	if err := provider.RequireLogin(b.s.Getenv("HOME"), buildRoles(b.Config)); err != nil {
		return err
	}
	c := b.Continuation
	switch c.Stage {
	case StageUsageLimit:
		brief, err := b.brief()
		if err != nil {
			return err
		}
		brief.Dynamic += fmt.Sprintf("\n# Fresh conversation continuation\n\nThis is a fresh executor conversation, not restored provider history. Continue interrupted logical round %d. The candidate files include partial work; previous verification is historical and tree-bound.\n\nPrevious verification feedback:\n%s\n", b.Round, c.Feedback)
		b.s.Progress(fmt.Sprintf("kogen: fresh conversation (provider history not restored), continuing round %d", b.Round))
		sess, err := b.startBuilder(ctx, brief)
		if err != nil {
			return err
		}
		defer sess.Close()
		return b.drive(ctx, sess, executor.Input{Feedback: c.Feedback}, false)
	case StageBuilder:
		return b.drive(ctx, b.nativeSession(c.Input, nil, nil), executor.Input{}, false)
	case StageBuilderVerify:
		sess := b.nativeSession(nil, c.Input, c.Finish)
		feedback, landed, err := b.roundEnd(ctx)
		if err != nil || landed {
			return err
		}
		return b.drive(ctx, sess, executor.Input{Feedback: feedback}, true)
	default: // the routed steps: review_only, commit_failed and the review stages
		return route.Resume(ctx, steps{b}, c.Stage)
	}
}

// toolProgress writes only the completed operation's identity and outcome, never its result.
func (b *Attempt) toolProgress(name, path string, failed bool) {
	status := "ok"
	if failed {
		status = "failed"
	}
	line := fmt.Sprintf("kogen: building (round %d) — %s", b.Round, name)
	if path != "" {
		line += " " + path
	}
	b.s.Progress(line + " " + status)
}
