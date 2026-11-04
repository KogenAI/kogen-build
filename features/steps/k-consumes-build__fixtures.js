'use strict';
// Known-good fixtures of the k-consumes-build observer controls (k-consumes-build__control.js):
// every use of kogen-build's code at HEAD, by suite and kind, from which the control writes a Go
// test file whose one test, TestRefs, only references each use (it asserts nothing), and the
// consumes/README.md inventory that maps each use to TestRefs. Data only: no step definition.
//
// Kinds: value (a function, variable or constant: _ = pkg.Name), type (var _ *pkg.Type), member
// (a field or method of a named non-interface type), iface (a method of an interface type) and
// generic ([use, Go statement] with type arguments).
const suites = {
  'kogen-coordinator': {
    imports: [
      'github.com/KogenAI/kogen-coordinator/durable',
      'github.com/KogenAI/kogen-coordinator/git',
      'github.com/KogenAI/kogen-coordinator/project',
      'github.com/KogenAI/kogen-coordinator/route',
    ],
    value: [
      'durable.Sync git.AcquireBuild git.BuildLockPath git.LockOwner git.New git.Open',
      'git.StagedEquivalent project.ApprovalPath project.Baseline project.Checkout',
      'project.ConfigPath project.EncodeJSON project.FeaturePath project.Hash project.IntentDir',
      'project.IntentMDPath project.LoadConfig project.LoadIntent project.ModelPath',
      'project.ParseGherkin project.ParseStages project.ReadApproval project.ReadIntent',
      'project.RequireIntentDir project.SourceMessage project.SourceRefFromMap',
      'project.ValidateSource project.WritePackageFacts route.StageCommit',
      'route.StageCommitFailed route.StageLanded route.StageReview route.StageReviewOnly',
      'route.StageReviewRetry route.StageReviewVerify',
    ],
    type: [
      'git.CommitSpec git.Repo project.Acceptance project.Approval project.Check',
      'project.CheckoutFacts project.Config project.Intent project.ModelSpec project.Role',
      'project.Scenario project.SourceRef project.Stage',
    ],
    member: [
      'git.CommitSpec.MessageFile git.CommitSpec.Slug git.CommitSpec.Start git.CommitSpec.Title',
      'git.CommitSpec.Tree git.Lock.Release git.Repo.CanCommit git.Repo.Changed',
      'git.Repo.ChangedPaths git.Repo.CheckCommitMsg git.Repo.Commit git.Repo.Dir git.Repo.Head',
      'git.Repo.IndexPath git.Repo.OK git.Repo.ObjectFormat git.Repo.OperationInProgress',
      'git.Repo.Resolve git.Repo.RestoreIndex git.Repo.RestoreTree git.Repo.SaveIndex',
      'git.Repo.Tree git.Restored.Change git.Restored.Path project.Acceptance.Run',
      'project.Acceptance.StepsDir project.Approval.Base project.Approval.SHA256',
      'project.Approval.SourceProvenance project.Check.Name project.Check.Run',
      'project.CheckoutFacts.ID project.Config.Acceptance project.Config.Checks',
      'project.Config.Fix project.Config.Models project.Config.Protect project.Config.Setup',
      'project.Intent.Acceptance project.Intent.Model project.Intent.Slug project.Intent.Text',
      'project.Intent.Title project.ModelSpec.Invariants project.ModelSpec.None',
      'project.ModelSpec.Path project.ModelSpec.Samples project.ModelSpec.Seed',
      'project.ModelSpec.Steps project.Role.Effort project.Role.Executor project.Role.External',
      'project.Role.Model project.Role.Provider project.SourceRef.ContractSHA256',
      'project.SourceRef.Path project.SourceRef.SHA256 project.Stage.Checks project.Stage.Goal',
      'project.Stage.ID project.Stage.Title',
    ],
    iface: [
    ],
    generic: [
      ['route.Cycle', '_ = route.Cycle[int, int]'],
      ['route.Resume', '_ = route.Resume[int, int]'],
      ['route.Steps', 'var _ route.Steps[int, int]'],
    ],
  },
  'kogen-executor': {
    imports: [
      'github.com/KogenAI/kogen-executor/agent',
      'github.com/KogenAI/kogen-executor/core',
      'github.com/KogenAI/kogen-executor/executor',
      'github.com/KogenAI/kogen-executor/thinking',
    ],
    value: [
      'agent.ApplyBuildReview agent.AsAny agent.BudgetEvent agent.Git agent.NameBudgetExpiry',
      'agent.Prompt agent.TestingRules agent.WithOriginHint core.Count core.DiagnosticText',
      'core.Display core.EncodeJSON core.HashHex core.InspectString core.MaterialText',
      'core.ProviderRole core.ResponseTokens core.ResumeRefused core.ShellDefinitions',
      'core.ZeroTokens executor.Contract executor.EndBlocker executor.EndFinish',
      'executor.EndNoCall executor.Fail executor.Native executor.PreventWriteTool',
      'executor.ReasonDeadline executor.ReasonInterrupted executor.ReasonProtocol',
      'executor.ReasonUnavailable executor.ReasonUsageLimit executor.ResumeExact',
      'executor.RoleBuild executor.RoleReview executor.RoundLimit executor.UsageComplete',
      'executor.UsageProvider thinking.Strip',
    ],
    type: [
      'core.ExecutorState core.ExecutorUsage core.ReviewRecord core.Tokens',
      'executor.BlockerClaim executor.Brief executor.Capabilities executor.Descriptor',
      'executor.Executor executor.Handle executor.Input executor.RoundResult executor.Session',
      'executor.Sink executor.Task executor.Usage',
    ],
    member: [
      'core.ExecutorState.Name core.ExecutorState.Rounds core.ExecutorState.SessionID',
      'core.ExecutorState.Usage core.ExecutorState.Version core.ExecutorUsage.Cached',
      'core.ExecutorUsage.CostUSD core.ExecutorUsage.Input core.ExecutorUsage.Output',
      'core.ExecutorUsage.Plus core.ExecutorUsage.Reasoning core.ExecutorUsage.Source',
      'core.ExecutorUsage.State core.ReviewRecord.Findings core.ReviewRecord.N',
      'core.ReviewRecord.Tree core.ReviewRecord.Verdict core.Tokens.Cached core.Tokens.Input',
      'core.Tokens.Line core.Tokens.Output core.Tokens.Plus core.Tokens.Reasoning',
      'core.Tokens.Total executor.BlockerClaim.Evidence executor.BlockerClaim.Paths',
      'executor.BlockerClaim.Reason executor.Brief.Dynamic executor.Brief.Stable',
      'executor.Capabilities.Blocker executor.Capabilities.Continuation',
      'executor.Capabilities.Images executor.Capabilities.MaxRounds',
      'executor.Capabilities.Prevention executor.Capabilities.Resume',
      'executor.Capabilities.Roles executor.Capabilities.Usage executor.Descriptor.Capabilities',
      'executor.Descriptor.Contract executor.Descriptor.For executor.Descriptor.Name',
      'executor.Descriptor.Validate executor.Descriptor.ValidateRole',
      'executor.Descriptor.Version executor.Handle.Executor executor.Handle.Resume',
      'executor.Handle.SessionID executor.Handle.Version executor.Input.Feedback',
      'executor.RoleCapabilities.Blocker executor.RoleCapabilities.MaxRounds',
      'executor.RoundResult.Blocker executor.RoundResult.Claim executor.RoundResult.Ending',
      'executor.RoundResult.Handle executor.RoundResult.Usage executor.Task.Approval',
      'executor.Task.Brief executor.Task.Deadline executor.Task.Dir executor.Task.Effort',
      'executor.Task.Excluded executor.Task.Home executor.Task.MaxRounds executor.Task.Model',
      'executor.Task.Protected executor.Task.Role executor.Task.Root executor.Task.RunID',
      'executor.Task.Schema executor.Task.StartTree executor.Usage.Cached executor.Usage.Input',
      'executor.Usage.Model executor.Usage.Output executor.Usage.Reasoning',
      'executor.Usage.Source executor.Usage.State',
    ],
    iface: [
      'executor.Executor.Describe executor.Executor.Start executor.Session.Close',
      'executor.Session.Handle executor.Session.Round',
    ],
    generic: [
      ['core.NonNil', '_ = core.NonNil[string]'],
    ],
  },
  'kogen-loop': {
    imports: [
      'kogen-loop/api',
      'kogen-loop/loop',
      'kogen-loop/provider',
      'kogen-loop/sys',
      'kogen-loop/tools',
    ],
    value: [
      'api.Library loop.Run loop.SumUsage loop.UsageOf provider.ContractBlocks',
      'provider.Fingerprint provider.MaxCacheID provider.NewCache provider.NewCacheID',
      'provider.NewClient provider.RequireLogin provider.ValidCacheStates sys.AsError',
      'sys.EnableSupervisor sys.Fail sys.MaybeSupervise sys.New tools.Cut tools.DecodeArguments',
      'tools.Definition tools.Deliver tools.ImageEntries tools.InputBudgetBytes tools.Measure',
      'tools.OutputItem tools.OutputReserveTokens tools.ReadEvent tools.ReadFileTool',
      'tools.Replay',
    ],
    type: [
      'api.Usage loop.Handled loop.Hooks loop.Request loop.Round provider.Cache',
      'provider.CacheState provider.Client provider.Request provider.Response provider.Role',
      'sys.Error sys.Interrupted sys.Options sys.Result sys.Session tools.Call tools.ImageEntry',
      'tools.Measurement tools.Output tools.Tools',
    ],
    member: [
      'api.Usage.Cached api.Usage.Input api.Usage.Output api.Usage.Reasoning api.Usage.State',
      'loop.Handled.Handled loop.Handled.Output loop.Hooks.AfterCall loop.Hooks.AfterResponse',
      'loop.Hooks.BeforeCall loop.Hooks.BeforeRequest loop.Hooks.HandleCall loop.Request.Input',
      'loop.Request.Instructions loop.Request.Owner loop.Request.Tools loop.Result.Input',
      'loop.Result.Response loop.Result.Terminal loop.Round.Definitions loop.Round.Instructions',
      'loop.Round.Owner loop.Round.Role loop.Round.Terminal loop.Round.Tools',
      'provider.Cache.States provider.CacheState.ID provider.CacheState.Response',
      'provider.CacheState.Unsupported provider.CacheState.Valid provider.Client.Body',
      'provider.Client.Respond provider.Client.ResponseBudget provider.KnownUsage.Write',
      'provider.Request.Cache provider.Request.CacheID provider.Request.Context',
      'provider.Request.Input provider.Request.Instructions provider.Request.Role',
      'provider.Request.ToolChoice provider.Request.Tools provider.Response.Items',
      'provider.Response.Known provider.Response.Model provider.Response.OperationID',
      'sys.Error.Class sys.Error.Detail sys.Error.Exit sys.Error.Lines',
      'sys.Error.ProviderFailure sys.Error.Reason sys.Error.Render sys.Interrupted.ExitCode',
      'sys.Interrupted.Signal sys.Options.Env sys.Options.KillGroupOnExit sys.Result.Output',
      'sys.Result.Status sys.Session.Context sys.Session.Env sys.Session.Err sys.Session.Getenv',
      'sys.Session.Out sys.Session.Progress sys.Session.Puts sys.Session.Run sys.Session.Stdout',
      'sys.Session.WatchSignals tools.Call.Args tools.Call.Arguments tools.Call.CallID',
      'tools.Call.Item tools.Call.Name tools.ImageEntry.Meta tools.Measurement.Fits',
      'tools.Measurement.InputBudgetBytes tools.Measurement.RenderedBytes tools.Output.Text',
      'tools.Output.Typed tools.Output.Value tools.Tools.Enabled tools.Tools.Excluded',
      'tools.Tools.Protected tools.Tools.Repo tools.Tools.Root tools.Tools.Session',
    ],
    iface: [
    ],
    generic: [
    ],
  },
  'kogen-review': {
    imports: [
      'github.com/KogenAI/kogen-review/review',
    ],
    value: [
      'review.DecodeSession review.File review.InvalidRecord review.NewExternal',
    ],
    type: [
      'review.Candidate review.Checkpoint review.Commands review.Done review.Env',
      'review.Evidence review.Independent review.Input review.Intent review.Invalid',
      'review.MaterialSet review.Request review.Result review.Reviewer review.Run',
      'review.Session review.Submitted review.Tools review.Verdict',
    ],
    member: [
      'review.Candidate.Builder review.Candidate.Excluded review.Candidate.Protected',
      'review.Candidate.Repo review.Candidate.Root review.Candidate.Start review.Candidate.Tree',
      'review.Checkpoint.Base review.Checkpoint.Pass review.Checkpoint.Session',
      'review.Checkpoint.Stage review.Commands.Acceptance review.Commands.Checks',
      'review.Commands.Fix review.Done.Call review.Done.Result review.Evidence.BaselineFailing',
      'review.Evidence.Commands review.Evidence.Diagnostic review.Evidence.Pass',
      'review.Input.Candidate review.Input.Checkpoint review.Input.Evidence',
      'review.Input.Executor review.Input.Intent review.Input.Last review.Input.Number',
      'review.Input.Run review.Input.Tools review.Intent.Approval review.Intent.Plan',
      'review.Intent.Role review.Intent.Slug review.Intent.Source review.Intent.Text',
      'review.Invalid.Base review.Invalid.ForcedModel review.Invalid.ForcedRoute',
      'review.Invalid.NoToolReminder review.Invalid.Pass review.Invalid.Reason',
      'review.Invalid.RejectedVerdict review.Invalid.Reopened review.Invalid.Repaired',
      'review.Invalid.Review review.MaterialSet.Authority review.MaterialSet.Checks',
      'review.MaterialSet.Files review.MaterialSet.Source review.Request.ForceTool',
      'review.Request.Input review.Request.Instructions review.Request.Prefill',
      'review.Request.Retain review.Request.Role review.Request.Tools review.Result.Invalid',
      'review.Result.Pass review.Result.Record review.Run.Dir review.Run.ID',
      'review.Session.Attempts review.Session.Encode review.Session.Input review.Session.Role',
      'review.Session.Tools review.Session.Tree review.Session.Usage review.Session.Verdict',
      'review.Submitted.Findings review.Submitted.Pass review.Submitted.Tree',
      'review.Submitted.Verdict review.Tools.Home review.Tools.Native review.Tools.Record',
      'review.Tools.Round review.Verdict.Findings review.Verdict.Verdict',
    ],
    iface: [
      'review.Reviewer.Resume review.Reviewer.Review',
    ],
    generic: [
    ],
  },
  'kogen-verification': {
    imports: [
      'github.com/KogenAI/kogen-verification/verify',
    ],
    value: [
      'verify.Cap verify.Diagnostic verify.EnvFailure verify.Fix verify.NotRunNotice',
      'verify.RequireTools verify.RunWatched verify.Verify',
    ],
    type: [
      'verify.CommandResult verify.Input verify.Pass verify.Readiness verify.Stage verify.Tools',
    ],
    member: [
      'verify.Acceptance.Output verify.Acceptance.Problems verify.Acceptance.Scenarios',
      'verify.CommandResult.Command verify.CommandResult.Name verify.CommandResult.Output',
      'verify.CommandResult.Status verify.Input.Config verify.Input.Excluded',
      'verify.Input.Intact verify.Input.Intent verify.Input.Log verify.Input.MaintainedModels',
      'verify.Input.N verify.Input.Repo verify.Input.Root verify.Input.RunDir',
      'verify.Input.Stage verify.Input.Start verify.Input.Tools',
      'verify.MaintainedModelResult.Path verify.MaintainedModelResult.Result',
      'verify.ModelResult.Problems verify.ModelResult.State verify.Pass.Acceptance',
      'verify.Pass.Checks verify.Pass.ChecksLine verify.Pass.Clean verify.Pass.Complete',
      'verify.Pass.Failing verify.Pass.Feedback verify.Pass.Green verify.Pass.Legacy',
      'verify.Pass.MaintainedModels verify.Pass.Model verify.Pass.Moved verify.Pass.N',
      'verify.Pass.NotRun verify.Pass.Setup verify.Pass.Tree verify.Pass.Valid',
      'verify.Readiness.Config verify.Readiness.Log verify.Readiness.Repo verify.Readiness.Root',
      'verify.Readiness.Setup verify.Readiness.Tools verify.ScenarioResult.Detail',
      'verify.ScenarioResult.Line verify.ScenarioResult.Name verify.ScenarioResult.Status',
      'verify.Stage.Checks verify.Stage.ID verify.Tools.Cucumber verify.Tools.Node',
      'verify.Tools.Quint verify.Tools.Require',
    ],
    iface: [
    ],
    generic: [
    ],
  },
};

function names(lines) {
  return lines.join(' ').split(' ').filter(Boolean);
}

// refs is each [use, Go statement naming it] of suite s.
function refs(s) {
  const d = suites[s];
  const member = (u, iface) => {
    const parts = u.split('.');
    const type = parts.slice(0, 2).join('.');
    return [u, `_ = func(v ${iface ? '' : '*'}${type}) any { return v.${parts.slice(2).join('.')} }`];
  };
  return [
    ...names(d.value).map((u) => [u, `_ = ${u}`]),
    ...names(d.type).map((u) => [u, `var _ *${u}`]),
    ...names(d.member).map((u) => member(u, false)),
    ...names(d.iface).map((u) => member(u, true)),
    ...d.generic,
  ];
}

// refsTest is the Go test file of suite s.
function refsTest(s) {
  const imports = suites[s].imports.map((p) => `\t"${p}"\n`).join('');
  const body = refs(s).map(([, r]) => `\t${r}\n`).join('');
  return `package consumes\n\nimport (\n\t"testing"\n\n${imports})\n\nfunc TestRefs(t *testing.T) {\n${body}}\n`;
}

// readme is consumes/README.md with the inventory of every suite.
function readme() {
  let text = '# Consumes\n\nKnown-good observer control fixture.\n\n## Inventory\n';
  for (const s of Object.keys(suites)) {
    text += `\n### ${s}\n\n| Use | Tests |\n| --- | --- |\n`;
    text += refs(s).map(([u]) => `| \`${u}\` | TestRefs |\n`).join('');
  }
  return text;
}

module.exports = { suites: Object.keys(suites), refsTest, readme };
