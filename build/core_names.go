package build

import (
	"github.com/KogenAI/kogen-build/build/core"
)

// The persisted Build state, the run record and the review contract live in build/core, which
// build and build/review share; build re-exports them under their own names.
type (
	Tokens        = core.Tokens
	ReviewRecord  = core.ReviewRecord
	HistoryRef    = core.HistoryRef
	OwnerContext  = core.OwnerContext
	Completed     = core.Completed
	Continuation  = core.Continuation
	State         = core.State
	RequestBudget = core.RequestBudget
	ReviewRetry   = core.ReviewRetry
	InvalidReview = core.InvalidReview
	ExecutorState = core.ExecutorState
	ExecutorUsage = core.ExecutorUsage
	StageProgress = core.StageProgress
	StageCustody  = core.StageCustody
	StageEvidence = core.StageEvidence
	Run           = core.Run
	Approved      = core.Approved
	RequestSpec   = core.RequestSpec

	stageDef          = core.StageDef
	materialSet       = core.MaterialSet
	materialSegment   = core.MaterialSegment
	materialHunkRange = core.MaterialHunkRange
	materialFileText  = core.MaterialFileText
	invalidReview     = core.InvalidEnd
)

// Continuation stages.
const (
	StageStartup       = core.StageStartup
	StageBuilder       = core.StageBuilder
	StageUsageLimit    = core.StageUsageLimit
	StageBuilderVerify = core.StageBuilderVerify
	StageTool          = core.StageTool
	StageCommit        = core.StageCommit
	StageCommitFailed  = core.StageCommitFailed
	StageLanded        = core.StageLanded
	StageReview        = core.StageReview
	StageReviewVerify  = core.StageReviewVerify
	StageReviewRetry   = core.StageReviewRetry
	StageReviewOnly    = core.StageReviewOnly
)

// The phases of a staged Build.
const (
	phaseStage  = core.PhaseStage
	phaseFinal  = core.PhaseFinal
	phaseReview = core.PhaseReview
	phaseLanded = core.PhaseLanded
)

// StartRun creates a new run folder (0700) with a fresh 8-hex id under root.
func StartRun(root string) (*Run, error) { return core.StartRun(root) }
