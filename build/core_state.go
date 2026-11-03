package build

import (
	"encoding/json"

	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-coordinator/route"
	"github.com/KogenAI/kogen-verification/verify"
	"kogen-loop/provider"
)

// The persisted Build state: what a checkpoint records (its bytes are pinned by the state
// goldens). It was kogen-executor's core while kogen-review worked on it; Review now gets a narrow
// input (kogen-review contract.go), so the state is the Build's alone again. The checkpoint
// stages of the routed steps are kogen-coordinator's route stages.

// HistoryRef names an archived working-context window: its path and SHA-256.
type HistoryRef struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// OwnerContext is the working context of one conversation owner ("build" or "review"): the
// mandatory opening messages, the observations kept across resets and the archived windows.
type OwnerContext struct {
	Mandatory    []any        `json:"mandatory"`
	Observations []any        `json:"observations"`
	History      []HistoryRef `json:"history"`
	// InlineMaterial and SourceTree belong to the review owner while the candidate's source
	// still rides inline in its opening message: the material that message was made from and
	// the tree it describes (nil for a message of a checkpoint that predates the binding).
	// Under pressure that source relocates to pinned parts instead of being rebuilt from the
	// checkout. Both are dropped once the source is pinned.
	InlineMaterial *materialSet `json:"inline_material,omitempty"`
	SourceTree     *string      `json:"source_tree,omitempty"`
}

// Continuation stages. The stages that are boundaries whose effects cannot be told after an
// interruption ("startup", "tool", "commit") are not resumable.
const (
	StageStartup       = "startup"
	StageBuilder       = "builder"
	StageUsageLimit    = "builder_usage_limit"
	StageBuilderVerify = "builder_verify"
	StageTool          = "tool"
	StageCommit        = route.StageCommit
	// StageCommitFailed is the checkpoint of a Build whose `git commit` definitely failed with
	// HEAD still at the start: the verified and approved candidate is intact, so an explicit
	// resume finishes the commit alone.
	StageCommitFailed = route.StageCommitFailed
	StageLanded       = route.StageLanded
	StageReview       = route.StageReview
	StageReviewVerify = route.StageReviewVerify
	StageReviewRetry  = route.StageReviewRetry
	// StageReviewOnly is the review-only checkpoint of a green candidate whose independent
	// reviews ended without a valid verdict: it holds the green pass and no reviewer
	// conversation, so an explicit resume starts a fresh review of that exact tree.
	StageReviewOnly = route.StageReviewOnly
)

// Completed is a tool call that ran and the result it gave.
type Completed struct {
	Call   map[string]any `json:"call"`
	Result any            `json:"result"`
}

// Continuation is where a Build can continue from: the stage and what the stage needs.
type Continuation struct {
	Stage string `json:"stage"`
	// Input is the builder's conversation (stages builder, builder_verify and tool).
	Input    []any  `json:"input,omitempty"`
	Feedback string `json:"feedback,omitempty"`
	// Finish is the builder's finish call that opened a builder_verify stage.
	Finish map[string]any `json:"finish,omitempty"`
	// Owner is who owned the tool boundary ("build" or "review").
	Owner     string           `json:"owner,omitempty"`
	Completed *Completed       `json:"completed,omitempty"`
	Response  []map[string]any `json:"response,omitempty"`
	// Request marks a tool boundary that a model request saved (the response is in
	// Response), as opposed to one a tool loop saved.
	Request bool `json:"request,omitempty"`
	// Tree is the tree being committed (stages commit and commit_failed).
	Tree string `json:"tree,omitempty"`
	// Commit is the landed commit (stage landed).
	Commit         string `json:"commit,omitempty"`
	AlreadyPresent bool   `json:"already_present,omitempty"`
	// Pass and Review belong to the review stages: the green pass under review and the
	// reviewer's own session state, which only the review code interprets.
	Pass   *verify.Pass    `json:"pass,omitempty"`
	Review json.RawMessage `json:"review,omitempty"`
	// Base belongs to the review stages: the green pass that reviewer's current session was
	// started from (the pass its Review call was given). It stays the same through the
	// session's rounds and repairs, while Pass may become a later red pass, and it is what a
	// fresh review resumes from when the session itself cannot be resumed (an external
	// reviewer) or ends without a valid verdict.
	Base *verify.Pass `json:"base,omitempty"`
}

// State is everything a Build keeps and a checkpoint records (the `state` of a version 4
// checkpoint).
type State struct {
	Root            string            `json:"root"`
	Slug            string            `json:"slug"`
	Config          *project.Config   `json:"config"`
	IntentText      string            `json:"intent_text"`
	Start           string            `json:"start"`
	Baseline        []string          `json:"baseline"`
	BaselineDigests map[string]string `json:"baseline_digests"`
	Excluded        []string          `json:"excluded"`
	Run             *Run              `json:"run"`
	GitIndex        string            `json:"git_index"`
	StartTree       string            `json:"start_tree"`
	Phase           string            `json:"phase"` // "startup" or "building"
	// T0 is when this run started, in Unix milliseconds.
	T0               int64                          `json:"t0"`
	Usage            Tokens                         `json:"usage"`
	Round            int                            `json:"round"`
	Pass             int                            `json:"pass"`
	Review           int                            `json:"review"`
	Repairs          int                            `json:"repairs"`
	LastPass         *verify.Pass                   `json:"last_pass"`
	LastReview       *ReviewRecord                  `json:"last_review"`
	Blocker          map[string]any                 `json:"blocker"`
	Continuation     Continuation                   `json:"continuation"`
	ApprovalBytes    string                         `json:"approval_bytes"` // base64
	Parent           *string                        `json:"parent"`
	RootRun          *string                        `json:"root_run"`
	ModelActiveMS    int64                          `json:"model_active_ms"`
	UnknownRequests  int                            `json:"unknown_requests"`
	AttemptUsage     Tokens                         `json:"attempt_usage"`
	WorkingContext   map[string]*OwnerContext       `json:"working_context"`
	SourceProvenance *project.SourceRef             `json:"source_provenance"`
	PromptCache      map[string]provider.CacheState `json:"prompt_cache_states"`
	// CacheIDs are the prompt-cache identities of the Build's model conversations, by
	// conversation ("build", "review-<n>"): created when a conversation starts, kept for every
	// later request of it, in resets and in an explicit resume.
	CacheIDs map[string]string `json:"cache_ids,omitempty"`
	// The staged plan, its durable progress and the evidence of each stage gate; all absent
	// for an Intent without a `## Stages` plan.
	StagePlan     []stageDef      `json:"stage_plan,omitempty"`
	StageProgress *StageProgress  `json:"stage_progress,omitempty"`
	StageEvidence []StageEvidence `json:"stage_evidence,omitempty"`
	// Executor identifies the external executor that produced the candidate and what it
	// reported (docs/executor-contract.md); nil for the native builder, so a native checkpoint
	// keeps its bytes. A checkpoint that has it is not resumed at the builder's own stages.
	Executor *ExecutorState `json:"executor,omitempty"`
	// ReviewExecutor is the same for an independent review by an external executor: its own
	// session, rounds and usage, separate from the builder's. Nil for the native reviewer.
	ReviewExecutor *ExecutorState `json:"review_executor,omitempty"`
	// BuildRequestBudget is the enabled total builder request budget and how many builder
	// requests have been attempted against it; nil (and absent from the checkpoint) when off.
	BuildRequestBudget *RequestBudget `json:"build_request_budget,omitempty"`
	// ReviewRetry is the durable lineage of independent reviews that ended without a valid
	// verdict for one exact green tree, and whether that tree's one automatic fresh review is
	// spent. Absent until a review ends invalid, so other checkpoints keep their bytes.
	ReviewRetry *ReviewRetry `json:"review_retry,omitempty"`
	// CheckBaseline is the result of every configured check, in configured order, on the
	// unchanged tree before the builder edited anything (AUT-5): diagnostic evidence kept
	// through every checkpoint, never a waiver. Absent for a Build that collected none.
	CheckBaseline []verify.CommandResult `json:"check_baseline,omitempty"`
	// CheckResults is the Build's ledger of passed checks, keyed by the check, the tree and the
	// environment (checkreuse.go): a whole-Intent pass on the same key reuses the record. At most
	// one record per configured check; absent until a check passed.
	CheckResults []checkResult `json:"check_results,omitempty"`
}

// RequestBudget is a Build's total builder request limit and its cumulative attempt count.
// An attempt is counted at the durable boundary before dispatch and never uncounted.
type RequestBudget struct {
	Limit int `json:"limit"`
	Count int `json:"count"`
}

// ReviewRetry belongs to one exact green candidate tree. Automatic is true once the single
// automatic fresh review of that tree was started: an interruption, a resume or a later
// explicit review-only resume never resets it.
type ReviewRetry struct {
	Tree      string          `json:"tree"`
	Automatic bool            `json:"automatic"`
	Invalid   []InvalidReview `json:"invalid"`
}

// InvalidReview records one review that ended without a valid verdict: its number, why, how
// much of each allowance it consumed, the run that stopped it and what followed ("automatic"
// fresh review, "stop" for an explicit review-only resume).
type InvalidReview struct {
	Review          int    `json:"review"`
	Reason          string `json:"reason"`
	RejectedVerdict int    `json:"rejected_verdicts"`
	NoToolReminder  int    `json:"no_tool_reminders"`
	Run             string `json:"run"`
	Next            string `json:"next"`
}
