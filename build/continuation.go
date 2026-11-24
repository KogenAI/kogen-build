package build

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/KogenAI/kogen-coordinator/git"
	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-executor/agent"
	"github.com/KogenAI/kogen-executor/core"
	"github.com/KogenAI/kogen-loop/sys"
)

// ContinuationBinding is caller-supplied, non-secret dispatch evidence, not authorization.
type ContinuationBinding struct {
	Host            string `json:"host"`
	Claim           string `json:"claim"`
	Attempt         string `json:"attempt"`
	RequestRevision string `json:"request_revision"`
	RequestHash     string `json:"request_hash"`
}

// ContinuationPlan is a model-free observation of native resume custody and remaining work.
// Saved identities remain available even when current validation refuses continuation.
type ContinuationPlan struct {
	BuildID               string              `json:"build_id"`
	Slug                  string              `json:"slug"`
	Root                  string              `json:"root"`
	CheckoutIdentity      string              `json:"checkout_identity"`
	SavedCheckoutIdentity string              `json:"saved_checkout_identity"`
	CandidateTree         string              `json:"candidate_tree"`
	CurrentCandidateTree  string              `json:"current_candidate_tree"`
	Start                 string              `json:"start"`
	ApprovalSHA256        string              `json:"approval_sha256"`
	CurrentApprovalSHA256 string              `json:"current_approval_sha256"`
	ContractSHA256        string              `json:"contract_sha256"`
	CheckpointSHA256      string              `json:"checkpoint_sha256"`
	Stage                 string              `json:"stage"`
	HeldReason            string              `json:"held_reason"`
	Binding               ContinuationBinding `json:"binding"`
	RequiredRoles         []string            `json:"required_roles"`
	RequiredChecks        []string            `json:"required_checks"`
}

// privateResumeRepo allows Git to compute trees without writing even loose objects in the
// checkout. All objects needed to inspect a refused candidate disappear with this view.
func privateResumeRepo(repo *git.Repo) ([]string, func(), error) {
	objects, err := repo.OK("rev-parse", "--git-path", "objects")
	if err != nil {
		return nil, nil, err
	}
	objects = strings.TrimSpace(objects)
	if !filepath.IsAbs(objects) {
		objects = filepath.Join(repo.Dir, objects)
	}
	dir, err := os.MkdirTemp("", "kogen-continuation-objects-")
	if err != nil {
		return nil, nil, err
	}
	return []string{"GIT_OBJECT_DIRECTORY=" + dir, "GIT_ALTERNATE_OBJECT_DIRECTORIES=" + objects}, func() { _ = os.RemoveAll(dir) }, nil
}

func continuationDetail(err error) string {
	if err == nil {
		return ""
	}
	if e, ok := sys.AsError(err); ok {
		return e.Detail
	}
	return err.Error()
}

func (in ResumeInput) observeContinuation(ctx context.Context, s *sys.Session, repo *git.Repo, binding ContinuationBinding, gitEnv []string) (ContinuationPlan, resumeEvidence, error) {
	p := ContinuationPlan{BuildID: in.ID, Root: repo.Dir, Binding: binding, RequiredRoles: []string{}, RequiredChecks: []string{}}
	evidence := resumeEvidence{gitEnv: gitEnv}
	err := resumePreflight(ctx, s, in, repo, &evidence)
	p.HeldReason = continuationDetail(err)
	p.CheckoutIdentity, _ = fileIdentity(repo.Dir)
	if evidence.raw != nil {
		p.CheckpointSHA256 = core.HashHex(evidence.raw)
	}
	b := evidence.b
	if b == nil {
		return p, evidence, err
	}
	p.Slug, p.Start, p.Stage = b.Slug, b.Start, b.Continuation.Stage
	p.SavedCheckoutIdentity, p.CandidateTree = evidence.doc.Identity, evidence.doc.Candidate
	p.CurrentCandidateTree, _ = snapshotTreeEnv(repo, b.Start, nil, gitEnv)
	saved, _ := decodeBase64(evidence.doc.Approval)
	p.ApprovalSHA256 = core.HashHex(saved)
	var approval struct {
		SHA256 string `json:"sha256"`
	}
	_ = json.Unmarshal(saved, &approval)
	p.ContractSHA256 = approval.SHA256
	if raw, e := readBounded(project.ApprovalPath(repo.Dir, b.Slug)); e == nil {
		p.CurrentApprovalSHA256 = core.HashHex(raw)
	}
	p.RequiredRoles, p.RequiredChecks = b.continuationObligations()
	return p, evidence, err
}

func (b *Attempt) continuationObligations() ([]string, []string) {
	roles, checks := []string{}, []string{}
	switch b.Continuation.Stage {
	case StageLanded, StageCommitFailed:
		return roles, checks
	case StageBuilder, StageBuilderVerify, StageUsageLimit:
		roles = append(roles, "build", "review")
	case StageReview, StageReviewVerify, StageReviewRetry, StageReviewOnly:
		// Resumed review routing can reverify and review again, but cannot restart the builder.
		roles = append(roles, "review")
	}
	// Match the native ledger without decideReuse's run events or reordering.
	tree, err := snapshotTreeEnv(b.repo, b.Start, b.Excluded, b.observationEnv)
	toolchain := toolchainIdentity(b.commandEnv())
	for _, c := range b.Config.Checks {
		if err == nil {
			if _, ok := b.ledgerFind(c, tree, checkEnvironment(toolchain, c)); ok {
				continue
			}
		}
		checks = append(checks, c.Run)
	}
	return roles, checks
}

// PrepareContinuation observes the exact native refusal path without running models,
// admission, or creating a run. Binding is only echoed evidence.
func (in ResumeInput) PrepareContinuation(ctx context.Context, s *sys.Session, binding ContinuationBinding) (ContinuationPlan, error) {
	repo, err := git.OpenReadOnly(in.Project, s.Env)
	if err != nil {
		return ContinuationPlan{BuildID: in.ID, Binding: binding, RequiredRoles: []string{}, RequiredChecks: []string{}, HeldReason: continuationDetail(err)}, err
	}
	view, cleanup, err := privateResumeRepo(repo)
	if err != nil {
		return ContinuationPlan{BuildID: in.ID, Binding: binding, RequiredRoles: []string{}, RequiredChecks: []string{}, HeldReason: continuationDetail(err)}, err
	}
	defer cleanup()
	p, _, err := in.observeContinuation(ctx, s, repo.Observing(), binding, view)
	return p, err
}

// ExecuteContinuation rechecks evidence and calls the trusted worker's fresh fenced
// admission under the ordinary Build lock. Neither a plan nor a binding grants authority.
func (in ResumeInput) ExecuteContinuation(ctx context.Context, s *sys.Session, plan ContinuationPlan, fresh ContinuationBinding, admit func(context.Context, ContinuationPlan, ContinuationBinding) error) (int, error) {
	var b *Attempt
	err := in.executeContinuation(ctx, s, plan, fresh, admit, &b)
	exit, resultErr := conclude(s, &b, err)
	recordOutcome(in.Outcome, b, err)
	return exit, resultErr
}

func (in ResumeInput) executeContinuation(ctx context.Context, s *sys.Session, plan ContinuationPlan, fresh ContinuationBinding, admit func(context.Context, ContinuationPlan, ContinuationBinding) error, out **Attempt) error {
	repo, err := git.Open(in.Project, s.Env)
	if err != nil {
		return err
	}
	lock, err := git.AcquireBuild(repo.Dir, s)
	if err != nil {
		return err
	}
	defer lock.Release()
	view, cleanup, err := privateResumeRepo(repo)
	if err != nil {
		return err
	}
	defer cleanup()
	observed, evidence, err := in.observeContinuation(ctx, s, repo.Observing(), fresh, view)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(plan, observed) {
		return refuse(in.ID, "continuation plan or dispatch binding changed", "")
	}
	if admit == nil {
		return refuse(in.ID, "fresh fenced admission required", "")
	}
	if err := admit(ctx, observed, fresh); err != nil {
		return refuse(in.ID, "continuation admission: "+err.Error(), "")
	}
	agent.ApplyBuildReview(in.Deps.Client)
	return continueResume(ctx, s, in, repo, evidence, out)
}

// observedChanges uses the same native diff format while temporary objects are in custody.
func (b *Attempt) observedChanges(a, c string) ([]string, error) {
	if len(b.observationEnv) == 0 {
		return b.repo.Changed(a, c)
	}
	out, err := b.repo.OKEnv(b.observationEnv, "diff", "--name-only", "-z", "--no-renames", a, c, "--")
	paths := []string{}
	for _, p := range strings.Split(out, "\x00") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths, err
}
