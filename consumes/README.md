# Consumes suites

kogen-build keeps `contract/` (what it provides) and states, as tests, what it relies on from
each Kogen dependency: one suite per producer, at the version `go.mod` pins.

| Suite | Producer module | Pins |
| --- | --- | --- |
| `consumes/kogen-coordinator/` | `github.com/KogenAI/kogen-coordinator` | `durable`, `git`, `project` and `route`: the Git custody, the project files, the Intent and approval formats and the verify → review → repair → commit routing |
| `consumes/kogen-executor/` | `github.com/KogenAI/kogen-executor` | `agent`, `core`, `executor` and `thinking`: the executor contract, the persisted token, review and executor-state records and the builder's agent helpers |
| `consumes/kogen-loop/` | `github.com/KogenAI/kogen-loop` | `api`, `loop`, `provider`, `sys` and `tools`: the model loop, the provider client and its cache, the session, errors and exit codes, and the native tools |
| `consumes/kogen-review/` | `github.com/KogenAI/kogen-review` | `review`: the independent review's input, its `Env`, sessions, verdicts and material |
| `consumes/kogen-verification/` | `github.com/KogenAI/kogen-verification` | `verify`: the verification pass, its check and acceptance results and its tools |

Each suite is one Go package of `_test.go` files, written only against the producer's exported
API. It never imports a package of kogen-build, and imports another Kogen dependency only where a
producer signature needs its values. Each test pins the types and signatures of the items it names
at compile time (with local `var _ T = x` statements and method expressions, interfaces with their
complete method sets) and asserts, through real calls, the results kogen-build's call sites rely
on. Items that cannot run offline (a real provider, a real external executor, the supervisor
process, a login) are pinned by signature and by the offline outcome kogen-build relies on.

## Running

```sh
go test -race -count=1 ./consumes/...
```

They are also part of `go test ./...`.

## Isolation

Every suite's `TestMain` clears the inherited `GIT_*` and `KOGEN_*` variables and sets a `HOME` of
its own under a temporary directory, `GIT_CONFIG_GLOBAL=/dev/null`, `GIT_CONFIG_NOSYSTEM=1` and
`GIT_TERMINAL_PROMPT=0`, and every subprocess and producer call taking an environment gets that
environment. Files live under `t.TempDir()`; no test reads kogen-build's sources or `testdata/`,
the module cache or the caller's `KOGEN_NODE`, `KOGEN_QUINT` and `KOGEN_CUCUMBER` tools. A provider
exchange uses only an in-process transport (an `httptest` loopback server or an
`http.RoundTripper`). The tests share no mutable state, so they pass under
`-race -count=1 -shuffle=on` and `-trimpath`.

## How coverage was checked

A use is every identifier in kogen-build's own non-test Go code (the packages `build` and
`cmd/kogen-build`; not `contract/`, `consumes/` or `features/`) that go/types resolves (`Uses` or
`Selections`) to an exported package-level object of a package of a producer's module, named
`pkg.Name`, or to a method or field of one of its named types, named `pkg.Type.Member` by the type
that declares the member. Import aliases, type aliases and re-exports (`build/core_names.go`) and
members reached through returned values and struct fields are therefore counted. The uses were
listed per dependency and package with this go/types listing (563 at HEAD: 112 kogen-coordinator,
139 kogen-executor, 137 kogen-loop, 104 kogen-review, 71 kogen-verification) and compared with
the identifiers each suite's tests name inside their bodies, using the same go/types resolution.
The inventory below holds one row per use, under its suite: the use and the tests whose bodies name
it (the tests that assert it, and any that only arrange another assertion with it).

## Inventory

### kogen-coordinator

| Use | Tests |
| --- | --- |
| `durable.Sync` | TestDurableSyncFlushesAndReportsFailure |
| `git.AcquireBuild` | TestBuildLock, TestBuildLockOfAProcessThatNoLongerRunsIsRemoved |
| `git.BuildLockPath` | TestBuildLock, TestBuildLockOfAProcessThatNoLongerRunsIsRemoved |
| `git.CommitSpec` | TestCommitFailureLeavesHeadAndTheIndexRestorable, TestCommitLandsTheReviewedTree, TestCommitUndoesACommitAHookChanged |
| `git.CommitSpec.MessageFile` | TestCommitFailureLeavesHeadAndTheIndexRestorable, TestCommitLandsTheReviewedTree, TestCommitUndoesACommitAHookChanged |
| `git.CommitSpec.Slug` | TestCommitFailureLeavesHeadAndTheIndexRestorable, TestCommitLandsTheReviewedTree, TestCommitUndoesACommitAHookChanged |
| `git.CommitSpec.Start` | TestCommitFailureLeavesHeadAndTheIndexRestorable, TestCommitLandsTheReviewedTree, TestCommitUndoesACommitAHookChanged |
| `git.CommitSpec.Title` | TestCommitFailureLeavesHeadAndTheIndexRestorable, TestCommitLandsTheReviewedTree, TestCommitUndoesACommitAHookChanged |
| `git.CommitSpec.Tree` | TestCommitFailureLeavesHeadAndTheIndexRestorable, TestCommitLandsTheReviewedTree, TestCommitUndoesACommitAHookChanged |
| `git.Lock.Release` | TestBuildLock, TestBuildLockOfAProcessThatNoLongerRunsIsRemoved |
| `git.LockOwner` | TestBuildLock, TestBuildLockOfAProcessThatNoLongerRunsIsRemoved |
| `git.New` | TestCanCommitNeedsAConfiguredIdentity, TestChangedBetweenTrees, TestChangedPaths, TestCheckCommitMsgRunsTheCommitMsgHook, TestHeadAndResolve, TestIndexCustody, TestNewMakesARepoWithoutChecking, TestOKReturnsTheStdoutOfASuccessfulCommand, TestObjectFormat, TestOperationInProgress, TestRestoreTreePutsBackTheDifferingPaths, TestStagedEquivalentIgnoresOnlyTheStatCache, TestTreeIsTheCandidate |
| `git.Open` | TestOpenFindsTheCheckoutRoot |
| `git.Repo` | TestCanCommitNeedsAConfiguredIdentity, TestChangedBetweenTrees, TestChangedPaths, TestCheckCommitMsgRunsTheCommitMsgHook, TestCommitLandsTheReviewedTree, TestHeadAndResolve, TestIndexCustody, TestNewMakesARepoWithoutChecking, TestOKReturnsTheStdoutOfASuccessfulCommand, TestObjectFormat, TestOpenFindsTheCheckoutRoot, TestOperationInProgress, TestRestoreTreePutsBackTheDifferingPaths, TestTreeIsTheCandidate |
| `git.Repo.CanCommit` | TestCanCommitNeedsAConfiguredIdentity |
| `git.Repo.Changed` | TestChangedBetweenTrees |
| `git.Repo.ChangedPaths` | TestChangedPaths |
| `git.Repo.CheckCommitMsg` | TestCheckCommitMsgRunsTheCommitMsgHook |
| `git.Repo.Commit` | TestCommitFailureLeavesHeadAndTheIndexRestorable, TestCommitLandsTheReviewedTree, TestCommitUndoesACommitAHookChanged |
| `git.Repo.Dir` | TestNewMakesARepoWithoutChecking, TestOpenFindsTheCheckoutRoot |
| `git.Repo.Head` | TestCommitFailureLeavesHeadAndTheIndexRestorable, TestCommitLandsTheReviewedTree, TestCommitUndoesACommitAHookChanged, TestHeadAndResolve, TestTreeIsTheCandidate |
| `git.Repo.IndexPath` | TestIndexCustody |
| `git.Repo.OK` | TestOKReturnsTheStdoutOfASuccessfulCommand |
| `git.Repo.ObjectFormat` | TestObjectFormat, TestStagedEquivalentIgnoresOnlyTheStatCache |
| `git.Repo.OperationInProgress` | TestOperationInProgress |
| `git.Repo.Resolve` | TestChangedBetweenTrees, TestHeadAndResolve, TestRestoreTreePutsBackTheDifferingPaths |
| `git.Repo.RestoreIndex` | TestCommitFailureLeavesHeadAndTheIndexRestorable, TestIndexCustody |
| `git.Repo.RestoreTree` | TestRestoreTreePutsBackTheDifferingPaths |
| `git.Repo.SaveIndex` | TestCommitFailureLeavesHeadAndTheIndexRestorable, TestIndexCustody |
| `git.Repo.Tree` | TestNewMakesARepoWithoutChecking, TestRestoreTreePutsBackTheDifferingPaths, TestTreeIsTheCandidate |
| `git.Restored.Change` | TestRestoreTreePutsBackTheDifferingPaths |
| `git.Restored.Path` | TestRestoreTreePutsBackTheDifferingPaths |
| `git.StagedEquivalent` | TestStagedEquivalentIgnoresOnlyTheStatCache |
| `project.Acceptance` | TestConfigJSONIsTheCheckpointFormat, TestLoadConfigReadsTheProjectConfiguration |
| `project.Acceptance.Run` | TestConfigJSONIsTheCheckpointFormat, TestLoadConfigDefaults, TestLoadConfigReadsTheProjectConfiguration |
| `project.Acceptance.StepsDir` | TestConfigJSONIsTheCheckpointFormat, TestLoadConfigDefaults, TestLoadConfigReadsTheProjectConfiguration |
| `project.Approval` | TestReadApproval |
| `project.Approval.Base` | TestReadApproval |
| `project.Approval.SHA256` | TestReadApproval |
| `project.Approval.SourceProvenance` | TestReadApproval |
| `project.ApprovalPath` | TestIntentPaths, TestReadApproval |
| `project.Baseline` | TestBaselineAndHash, TestWritePackageFacts |
| `project.Check` | TestConfigJSONIsTheCheckpointFormat, TestLoadConfigReadsTheProjectConfiguration |
| `project.Check.Name` | TestConfigJSONIsTheCheckpointFormat, TestLoadConfigReadsTheProjectConfiguration |
| `project.Check.Run` | TestConfigJSONIsTheCheckpointFormat, TestLoadConfigReadsTheProjectConfiguration |
| `project.Checkout` | TestCheckoutFacts, TestWritePackageFacts |
| `project.CheckoutFacts` | TestCheckoutFacts |
| `project.CheckoutFacts.ID` | TestCheckoutFacts, TestWritePackageFacts |
| `project.Config` | TestBaselineAndHash, TestConfigJSONIsTheCheckpointFormat, TestLoadConfigReadsTheProjectConfiguration, TestWritePackageFacts |
| `project.Config.Acceptance` | TestConfigJSONIsTheCheckpointFormat, TestLoadConfigDefaults, TestLoadConfigReadsTheProjectConfiguration |
| `project.Config.Checks` | TestConfigJSONIsTheCheckpointFormat, TestLoadConfigDefaults, TestLoadConfigReadsTheProjectConfiguration |
| `project.Config.Fix` | TestConfigJSONIsTheCheckpointFormat, TestLoadConfigDefaults, TestLoadConfigReadsTheProjectConfiguration |
| `project.Config.Models` | TestConfigJSONIsTheCheckpointFormat, TestLoadConfigDefaults, TestLoadConfigReadsTheProjectConfiguration |
| `project.Config.Protect` | TestBaselineAndHash, TestConfigJSONIsTheCheckpointFormat, TestLoadConfigDefaults, TestLoadConfigReadsTheProjectConfiguration |
| `project.Config.Setup` | TestConfigJSONIsTheCheckpointFormat, TestLoadConfigDefaults, TestLoadConfigReadsTheProjectConfiguration |
| `project.ConfigPath` | TestLoadConfigDefaults, TestLoadConfigErrors, TestLoadConfigReadsTheProjectConfiguration |
| `project.EncodeJSON` | TestEncodeJSONBytes, TestWritePackageFacts |
| `project.FeaturePath` | TestIntentPaths |
| `project.Hash` | TestBaselineAndHash, TestWritePackageFacts |
| `project.Intent` | TestLoadIntentAndReadIntent |
| `project.Intent.Acceptance` | TestLoadIntentAndReadIntent |
| `project.Intent.Model` | TestIntentModelSpec, TestLoadIntentAndReadIntent |
| `project.Intent.Slug` | TestLoadIntentAndReadIntent |
| `project.Intent.Text` | TestLoadIntentAndReadIntent |
| `project.Intent.Title` | TestLoadIntentAndReadIntent |
| `project.IntentDir` | TestIntentPaths |
| `project.IntentMDPath` | TestIntentModelSpec, TestIntentPaths, TestLoadIntentAndReadIntent, TestWritePackageFacts |
| `project.LoadConfig` | TestBaselineAndHash, TestLoadConfigDefaults, TestLoadConfigErrors, TestLoadConfigReadsTheProjectConfiguration, TestWritePackageFacts |
| `project.LoadIntent` | TestIntentModelSpec, TestLoadIntentAndReadIntent |
| `project.ModelPath` | TestIntentModelSpec, TestIntentPaths |
| `project.ModelSpec` | TestIntentModelSpec, TestLoadIntentAndReadIntent |
| `project.ModelSpec.Invariants` | TestIntentModelSpec |
| `project.ModelSpec.None` | TestIntentModelSpec, TestLoadIntentAndReadIntent |
| `project.ModelSpec.Path` | TestIntentModelSpec |
| `project.ModelSpec.Samples` | TestIntentModelSpec |
| `project.ModelSpec.Seed` | TestIntentModelSpec |
| `project.ModelSpec.Steps` | TestIntentModelSpec |
| `project.ParseGherkin` | TestParseGherkinCountsTheScenarios |
| `project.ParseStages` | TestParseStages |
| `project.ReadApproval` | TestReadApproval |
| `project.ReadIntent` | TestLoadIntentAndReadIntent |
| `project.RequireIntentDir` | TestRequireIntentDir |
| `project.Role` | TestConfigJSONIsTheCheckpointFormat, TestLoadConfigReadsTheProjectConfiguration |
| `project.Role.Effort` | TestConfigJSONIsTheCheckpointFormat, TestLoadConfigDefaults, TestLoadConfigReadsTheProjectConfiguration |
| `project.Role.Executor` | TestConfigJSONIsTheCheckpointFormat, TestLoadConfigReadsTheProjectConfiguration |
| `project.Role.External` | TestLoadConfigDefaults, TestLoadConfigReadsTheProjectConfiguration |
| `project.Role.Model` | TestConfigJSONIsTheCheckpointFormat, TestLoadConfigDefaults, TestLoadConfigReadsTheProjectConfiguration |
| `project.Role.Provider` | TestConfigJSONIsTheCheckpointFormat, TestLoadConfigReadsTheProjectConfiguration |
| `project.Scenario` | TestParseGherkinCountsTheScenarios |
| `project.SourceMessage` | TestSourceReferences |
| `project.SourceRef` | TestSourceReferences, TestValidateSourceCustody |
| `project.SourceRef.ContractSHA256` | TestSourceReferences, TestValidateSourceCustody |
| `project.SourceRef.Path` | TestSourceReferences, TestValidateSourceCustody |
| `project.SourceRef.SHA256` | TestSourceReferences, TestValidateSourceCustody |
| `project.SourceRefFromMap` | TestSourceReferences |
| `project.Stage` | TestParseStages |
| `project.Stage.Checks` | TestParseStages |
| `project.Stage.Goal` | TestParseStages |
| `project.Stage.ID` | TestParseStages |
| `project.Stage.Title` | TestParseStages |
| `project.ValidateSource` | TestSourceReferences, TestValidateSourceCustody |
| `project.WritePackageFacts` | TestWritePackageFacts |
| `route.Cycle` | TestCycleRoutesTheCandidate |
| `route.Resume` | TestResumeContinuesAStoppedBuild |
| `route.StageCommit` | TestStageNamesAreThePersistedOnes |
| `route.StageCommitFailed` | TestResumeContinuesAStoppedBuild, TestStageNamesAreThePersistedOnes |
| `route.StageLanded` | TestStageNamesAreThePersistedOnes |
| `route.StageReview` | TestResumeContinuesAStoppedBuild, TestStageNamesAreThePersistedOnes |
| `route.StageReviewOnly` | TestResumeContinuesAStoppedBuild, TestStageNamesAreThePersistedOnes |
| `route.StageReviewRetry` | TestResumeContinuesAStoppedBuild, TestStageNamesAreThePersistedOnes |
| `route.StageReviewVerify` | TestResumeContinuesAStoppedBuild, TestStageNamesAreThePersistedOnes |
| `route.Steps` | TestCycleRoutesTheCandidate, TestResumeContinuesAStoppedBuild, TestStepsMethodSet |

### kogen-executor

| Use | Tests |
| --- | --- |
| `agent.ApplyBuildReview` | TestAgentApplyBuildReview, TestAgentBudgetEvent |
| `agent.AsAny` | TestAgentAsAny |
| `agent.BudgetEvent` | TestAgentBudgetEvent |
| `agent.Git` | TestAgentGitNil, TestAgentGitRunsThroughTheRepository |
| `agent.NameBudgetExpiry` | TestAgentNameBudgetExpiry |
| `agent.Prompt` | TestAgentPromptTestingRules |
| `agent.TestingRules` | TestAgentPromptTestingRules |
| `agent.WithOriginHint` | TestAgentWithOriginHint |
| `core.Count` | TestCoreExecutorStateJSON, TestCoreExecutorUsagePlus, TestCoreResponseTokens, TestCoreTokens |
| `core.DiagnosticText` | TestCoreDiagnosticText |
| `core.Display` | TestCoreDisplay |
| `core.EncodeJSON` | TestCoreEncodeJSON |
| `core.ExecutorState` | TestCoreExecutorStateJSON |
| `core.ExecutorState.Name` | TestCoreExecutorStateJSON |
| `core.ExecutorState.Rounds` | TestCoreExecutorStateJSON |
| `core.ExecutorState.SessionID` | TestCoreExecutorStateJSON |
| `core.ExecutorState.Usage` | TestCoreExecutorStateJSON |
| `core.ExecutorState.Version` | TestCoreExecutorStateJSON |
| `core.ExecutorUsage` | TestCoreExecutorStateJSON, TestCoreExecutorUsagePlus |
| `core.ExecutorUsage.Cached` | TestCoreExecutorStateJSON, TestCoreExecutorUsagePlus |
| `core.ExecutorUsage.CostUSD` | TestCoreExecutorStateJSON, TestCoreExecutorUsagePlus |
| `core.ExecutorUsage.Input` | TestCoreExecutorStateJSON, TestCoreExecutorUsagePlus |
| `core.ExecutorUsage.Output` | TestCoreExecutorStateJSON, TestCoreExecutorUsagePlus |
| `core.ExecutorUsage.Plus` | TestCoreExecutorUsagePlus |
| `core.ExecutorUsage.Reasoning` | TestCoreExecutorStateJSON, TestCoreExecutorUsagePlus |
| `core.ExecutorUsage.Source` | TestCoreExecutorStateJSON, TestCoreExecutorUsagePlus |
| `core.ExecutorUsage.State` | TestCoreExecutorStateJSON, TestCoreExecutorUsagePlus |
| `core.HashHex` | TestCoreHashHex |
| `core.InspectString` | TestCoreInspectString |
| `core.MaterialText` | TestCoreMaterialText |
| `core.NonNil` | TestCoreNonNil, TestCoreReviewRecordJSON |
| `core.ProviderRole` | TestCoreProviderRole |
| `core.ResponseTokens` | TestCoreResponseTokens |
| `core.ResumeRefused` | TestCoreResumeRefused |
| `core.ReviewRecord` | TestCoreReviewRecordJSON |
| `core.ReviewRecord.Findings` | TestCoreReviewRecordJSON |
| `core.ReviewRecord.N` | TestCoreReviewRecordJSON |
| `core.ReviewRecord.Tree` | TestCoreReviewRecordJSON |
| `core.ReviewRecord.Verdict` | TestCoreReviewRecordJSON |
| `core.ShellDefinitions` | TestCoreShellDefinitions |
| `core.Tokens` | TestCoreResponseTokens, TestCoreTokens |
| `core.Tokens.Cached` | TestCoreResponseTokens, TestCoreTokens |
| `core.Tokens.Input` | TestCoreResponseTokens, TestCoreTokens |
| `core.Tokens.Line` | TestCoreTokens |
| `core.Tokens.Output` | TestCoreResponseTokens, TestCoreTokens |
| `core.Tokens.Plus` | TestCoreTokens |
| `core.Tokens.Reasoning` | TestCoreResponseTokens, TestCoreTokens |
| `core.Tokens.Total` | TestCoreTokens |
| `core.ZeroTokens` | TestCoreTokens |
| `executor.BlockerClaim` | TestExecutorDrivenThroughTheContract, TestExecutorRoundResultFields |
| `executor.BlockerClaim.Evidence` | TestExecutorDrivenThroughTheContract, TestExecutorRoundResultFields |
| `executor.BlockerClaim.Paths` | TestExecutorDrivenThroughTheContract, TestExecutorRoundResultFields |
| `executor.BlockerClaim.Reason` | TestExecutorDrivenThroughTheContract, TestExecutorRoundResultFields |
| `executor.Brief` | TestExecutorDrivenThroughTheContract, TestExecutorTaskFields |
| `executor.Brief.Dynamic` | TestExecutorDrivenThroughTheContract, TestExecutorTaskFields |
| `executor.Brief.Stable` | TestExecutorDrivenThroughTheContract, TestExecutorTaskFields |
| `executor.Capabilities` | TestExecutorDescriptorFields, TestExecutorDescriptorFor, TestExecutorDrivenThroughTheContract |
| `executor.Capabilities.Blocker` | TestExecutorDescriptorFields, TestExecutorDescriptorFor, TestExecutorDrivenThroughTheContract |
| `executor.Capabilities.Continuation` | TestExecutorDescriptorFields, TestExecutorDescriptorFor |
| `executor.Capabilities.Images` | TestExecutorDescriptorFields, TestExecutorDescriptorFor |
| `executor.Capabilities.MaxRounds` | TestExecutorDescriptorFields, TestExecutorDescriptorFor, TestExecutorDrivenThroughTheContract |
| `executor.Capabilities.Prevention` | TestExecutorDescriptorFields, TestExecutorDescriptorFor |
| `executor.Capabilities.Resume` | TestExecutorDescriptorFields, TestExecutorDescriptorFor, TestExecutorDrivenThroughTheContract |
| `executor.Capabilities.Roles` | TestExecutorDescriptorFields, TestExecutorDescriptorFor, TestExecutorDrivenThroughTheContract |
| `executor.Capabilities.Usage` | TestExecutorDescriptorFields, TestExecutorDescriptorFor |
| `executor.Contract` | TestExecutorConstants, TestExecutorDescriptorFields, TestExecutorDescriptorFor, TestExecutorDrivenThroughTheContract |
| `executor.Descriptor` | TestExecutorDescriptorFields, TestExecutorDescriptorFor, TestExecutorDescriptorValidate, TestExecutorDrivenThroughTheContract, TestExecutorInterfaceMethodSets |
| `executor.Descriptor.Capabilities` | TestExecutorDescriptorFields, TestExecutorDescriptorFor, TestExecutorDrivenThroughTheContract |
| `executor.Descriptor.Contract` | TestExecutorDescriptorFields, TestExecutorDescriptorFor, TestExecutorDescriptorValidate, TestExecutorDrivenThroughTheContract |
| `executor.Descriptor.For` | TestExecutorDescriptorFields, TestExecutorDescriptorFor, TestExecutorDescriptorValidate |
| `executor.Descriptor.Name` | TestExecutorDescriptorFields, TestExecutorDescriptorFor, TestExecutorDrivenThroughTheContract |
| `executor.Descriptor.Validate` | TestExecutorDescriptorFields, TestExecutorDescriptorValidate, TestExecutorDrivenThroughTheContract |
| `executor.Descriptor.ValidateRole` | TestExecutorDescriptorValidate |
| `executor.Descriptor.Version` | TestExecutorDescriptorFields, TestExecutorDescriptorFor, TestExecutorDrivenThroughTheContract |
| `executor.EndBlocker` | TestExecutorConstants, TestExecutorDrivenThroughTheContract, TestExecutorRoundResultFields |
| `executor.EndFinish` | TestExecutorConstants, TestExecutorDrivenThroughTheContract |
| `executor.EndNoCall` | TestExecutorConstants |
| `executor.Executor` | TestExecutorDrivenThroughTheContract, TestExecutorInterfaceMethodSets |
| `executor.Executor.Describe` | TestExecutorDrivenThroughTheContract, TestExecutorInterfaceMethodSets |
| `executor.Executor.Start` | TestExecutorDrivenThroughTheContract, TestExecutorInterfaceMethodSets |
| `executor.Fail` | TestExecutorDrivenThroughTheContract, TestExecutorFail |
| `executor.Handle` | TestExecutorInterfaceMethodSets, TestExecutorRoundResultFields |
| `executor.Handle.Executor` | TestExecutorDrivenThroughTheContract, TestExecutorRoundResultFields |
| `executor.Handle.Resume` | TestExecutorDrivenThroughTheContract, TestExecutorRoundResultFields |
| `executor.Handle.SessionID` | TestExecutorDrivenThroughTheContract, TestExecutorRoundResultFields |
| `executor.Handle.Version` | TestExecutorDrivenThroughTheContract, TestExecutorRoundResultFields |
| `executor.Input` | TestExecutorDrivenThroughTheContract, TestExecutorInterfaceMethodSets, TestExecutorTaskFields |
| `executor.Input.Feedback` | TestExecutorDrivenThroughTheContract, TestExecutorTaskFields |
| `executor.Native` | TestExecutorConstants, TestExecutorDescriptorFields, TestExecutorRoundResultFields |
| `executor.PreventWriteTool` | TestExecutorConstants, TestExecutorDescriptorFields |
| `executor.ReasonDeadline` | TestExecutorConstants, TestExecutorFail |
| `executor.ReasonInterrupted` | TestExecutorConstants, TestExecutorDrivenThroughTheContract, TestExecutorFail |
| `executor.ReasonProtocol` | TestExecutorConstants, TestExecutorFail |
| `executor.ReasonUnavailable` | TestExecutorConstants, TestExecutorDrivenThroughTheContract, TestExecutorFail |
| `executor.ReasonUsageLimit` | TestExecutorConstants, TestExecutorFail |
| `executor.ResumeExact` | TestExecutorConstants, TestExecutorDescriptorFields, TestExecutorRoundResultFields |
| `executor.RoleBuild` | TestExecutorConstants, TestExecutorDescriptorFields, TestExecutorDescriptorFor, TestExecutorDescriptorValidate, TestExecutorDrivenThroughTheContract, TestExecutorRoundResultFields, TestExecutorTaskFields |
| `executor.RoleCapabilities.Blocker` | TestExecutorDescriptorFields, TestExecutorDescriptorFor |
| `executor.RoleCapabilities.MaxRounds` | TestExecutorDescriptorFields, TestExecutorDescriptorFor, TestExecutorDescriptorValidate |
| `executor.RoleReview` | TestExecutorConstants, TestExecutorDescriptorFor, TestExecutorDescriptorValidate |
| `executor.RoundLimit` | TestExecutorFail |
| `executor.RoundResult` | TestExecutorDrivenThroughTheContract, TestExecutorInterfaceMethodSets, TestExecutorRoundResultFields |
| `executor.RoundResult.Blocker` | TestExecutorDrivenThroughTheContract, TestExecutorRoundResultFields |
| `executor.RoundResult.Claim` | TestExecutorDrivenThroughTheContract, TestExecutorRoundResultFields |
| `executor.RoundResult.Ending` | TestExecutorDrivenThroughTheContract, TestExecutorRoundResultFields |
| `executor.RoundResult.Handle` | TestExecutorRoundResultFields |
| `executor.RoundResult.Usage` | TestExecutorDrivenThroughTheContract, TestExecutorRoundResultFields |
| `executor.Session` | TestExecutorDrivenThroughTheContract, TestExecutorInterfaceMethodSets |
| `executor.Session.Close` | TestExecutorDrivenThroughTheContract, TestExecutorInterfaceMethodSets |
| `executor.Session.Handle` | TestExecutorDrivenThroughTheContract, TestExecutorInterfaceMethodSets |
| `executor.Session.Round` | TestExecutorDrivenThroughTheContract, TestExecutorInterfaceMethodSets |
| `executor.Sink` | TestExecutorInterfaceMethodSets |
| `executor.Task` | TestExecutorDrivenThroughTheContract, TestExecutorInterfaceMethodSets, TestExecutorTaskFields |
| `executor.Task.Approval` | TestExecutorTaskFields |
| `executor.Task.Brief` | TestExecutorDrivenThroughTheContract, TestExecutorTaskFields |
| `executor.Task.Deadline` | TestExecutorTaskFields |
| `executor.Task.Dir` | TestExecutorDrivenThroughTheContract, TestExecutorTaskFields |
| `executor.Task.Effort` | TestExecutorTaskFields |
| `executor.Task.Excluded` | TestExecutorTaskFields |
| `executor.Task.Home` | TestExecutorTaskFields |
| `executor.Task.MaxRounds` | TestExecutorDrivenThroughTheContract, TestExecutorTaskFields |
| `executor.Task.Model` | TestExecutorTaskFields |
| `executor.Task.Protected` | TestExecutorTaskFields |
| `executor.Task.Role` | TestExecutorDrivenThroughTheContract, TestExecutorTaskFields |
| `executor.Task.Root` | TestExecutorDrivenThroughTheContract, TestExecutorTaskFields |
| `executor.Task.RunID` | TestExecutorDrivenThroughTheContract, TestExecutorTaskFields |
| `executor.Task.Schema` | TestExecutorTaskFields |
| `executor.Task.StartTree` | TestExecutorTaskFields |
| `executor.Usage` | TestCoreExecutorUsagePlus, TestExecutorDrivenThroughTheContract, TestExecutorRoundResultFields |
| `executor.Usage.Cached` | TestCoreExecutorUsagePlus, TestExecutorRoundResultFields |
| `executor.Usage.Input` | TestCoreExecutorUsagePlus, TestExecutorRoundResultFields |
| `executor.Usage.Model` | TestExecutorRoundResultFields |
| `executor.Usage.Output` | TestCoreExecutorUsagePlus, TestExecutorRoundResultFields |
| `executor.Usage.Reasoning` | TestCoreExecutorUsagePlus, TestExecutorRoundResultFields |
| `executor.Usage.Source` | TestCoreExecutorUsagePlus, TestExecutorRoundResultFields |
| `executor.Usage.State` | TestCoreExecutorUsagePlus, TestExecutorDrivenThroughTheContract, TestExecutorRoundResultFields |
| `executor.UsageComplete` | TestCoreExecutorUsagePlus, TestExecutorConstants, TestExecutorDrivenThroughTheContract, TestExecutorRoundResultFields |
| `executor.UsageProvider` | TestExecutorConstants, TestExecutorDescriptorFields, TestExecutorRoundResultFields |
| `thinking.Strip` | TestThinkingStrip |

### kogen-loop

| Use | Tests |
| --- | --- |
| `api.Library` | TestAPILibraryIDNamesTheEngine |
| `api.Usage` | TestLoopUsageOfAndSumUsage |
| `api.Usage.Cached` | TestLoopUsageOfAndSumUsage |
| `api.Usage.Input` | TestLoopUsageOfAndSumUsage |
| `api.Usage.Output` | TestLoopUsageOfAndSumUsage |
| `api.Usage.Reasoning` | TestLoopUsageOfAndSumUsage |
| `api.Usage.State` | TestLoopUsageOfAndSumUsage |
| `loop.Handled` | TestLoopHandledCallIsAnsweredNotRun, TestLoopRunEngineOrderAndHookArguments, TestLoopRunStopsWithTheErrorUnchanged |
| `loop.Handled.Handled` | TestLoopHandledCallIsAnsweredNotRun, TestLoopRunEngineOrderAndHookArguments, TestLoopRunStopsWithTheErrorUnchanged |
| `loop.Handled.Output` | TestLoopHandledCallIsAnsweredNotRun, TestLoopRunEngineOrderAndHookArguments, TestLoopRunStopsWithTheErrorUnchanged |
| `loop.Hooks` | TestLoopHandledCallIsAnsweredNotRun, TestLoopRunEngineOrderAndHookArguments, TestLoopRunReturnsTheInterruptOfAShellCall, TestLoopRunSentInputIsWhatTheRequesterReturned, TestLoopRunStopsTheProcessGroupsOfTheRound, TestLoopRunStopsWithTheErrorUnchanged, TestLoopRunTerminalNamesAreTheRounds |
| `loop.Hooks.AfterCall` | TestLoopRunEngineOrderAndHookArguments, TestLoopRunStopsTheProcessGroupsOfTheRound, TestLoopRunStopsWithTheErrorUnchanged, TestLoopRunTerminalNamesAreTheRounds |
| `loop.Hooks.AfterResponse` | TestLoopRunEngineOrderAndHookArguments, TestLoopRunSentInputIsWhatTheRequesterReturned, TestLoopRunStopsWithTheErrorUnchanged |
| `loop.Hooks.BeforeCall` | TestLoopRunEngineOrderAndHookArguments, TestLoopRunReturnsTheInterruptOfAShellCall, TestLoopRunStopsWithTheErrorUnchanged |
| `loop.Hooks.BeforeRequest` | TestLoopRunEngineOrderAndHookArguments, TestLoopRunSentInputIsWhatTheRequesterReturned, TestLoopRunStopsWithTheErrorUnchanged, TestLoopRunTerminalNamesAreTheRounds |
| `loop.Hooks.HandleCall` | TestLoopHandledCallIsAnsweredNotRun, TestLoopRunEngineOrderAndHookArguments, TestLoopRunStopsWithTheErrorUnchanged |
| `loop.Request` | TestLoopRunEngineOrderAndHookArguments |
| `loop.Request.Input` | TestLoopHandledCallIsAnsweredNotRun, TestLoopRunEngineOrderAndHookArguments |
| `loop.Request.Instructions` | TestLoopRunEngineOrderAndHookArguments |
| `loop.Request.Owner` | TestLoopRunEngineOrderAndHookArguments |
| `loop.Request.Tools` | TestLoopRunEngineOrderAndHookArguments |
| `loop.Result.Input` | TestLoopRunEngineOrderAndHookArguments, TestLoopRunSentInputIsWhatTheRequesterReturned |
| `loop.Result.Response` | TestLoopRunEngineOrderAndHookArguments, TestLoopRunSentInputIsWhatTheRequesterReturned |
| `loop.Result.Terminal` | TestLoopHandledCallIsAnsweredNotRun, TestLoopRunEngineOrderAndHookArguments, TestLoopRunSentInputIsWhatTheRequesterReturned, TestLoopRunStopsTheProcessGroupsOfTheRound, TestLoopRunTerminalNamesAreTheRounds |
| `loop.Round` | TestLoopRunEngineOrderAndHookArguments, TestLoopRunReturnsTheInterruptOfAShellCall, TestLoopRunStopsTheProcessGroupsOfTheRound |
| `loop.Round.Definitions` | TestLoopRunEngineOrderAndHookArguments, TestLoopRunReturnsTheInterruptOfAShellCall, TestLoopRunStopsTheProcessGroupsOfTheRound |
| `loop.Round.Instructions` | TestLoopRunEngineOrderAndHookArguments, TestLoopRunReturnsTheInterruptOfAShellCall, TestLoopRunStopsTheProcessGroupsOfTheRound |
| `loop.Round.Owner` | TestLoopRunEngineOrderAndHookArguments, TestLoopRunReturnsTheInterruptOfAShellCall, TestLoopRunStopsTheProcessGroupsOfTheRound |
| `loop.Round.Role` | TestLoopRunEngineOrderAndHookArguments, TestLoopRunReturnsTheInterruptOfAShellCall, TestLoopRunStopsTheProcessGroupsOfTheRound |
| `loop.Round.Terminal` | TestLoopRunEngineOrderAndHookArguments, TestLoopRunReturnsTheInterruptOfAShellCall, TestLoopRunStopsTheProcessGroupsOfTheRound, TestLoopRunTerminalNamesAreTheRounds |
| `loop.Round.Tools` | TestLoopRunEngineOrderAndHookArguments, TestLoopRunReturnsTheInterruptOfAShellCall, TestLoopRunStopsTheProcessGroupsOfTheRound |
| `loop.Run` | TestLoopHandledCallIsAnsweredNotRun, TestLoopRunEngineOrderAndHookArguments, TestLoopRunReturnsTheInterruptOfAShellCall, TestLoopRunSentInputIsWhatTheRequesterReturned, TestLoopRunStopsTheProcessGroupsOfTheRound, TestLoopRunStopsWithTheErrorUnchanged, TestLoopRunTerminalNamesAreTheRounds |
| `loop.SumUsage` | TestLoopUsageOfAndSumUsage |
| `loop.UsageOf` | TestLoopUsageOfAndSumUsage |
| `provider.Cache` | TestProviderCacheKeepsConversationStates, TestProviderClientBodyOfARequest |
| `provider.Cache.States` | TestProviderCacheKeepsConversationStates, TestProviderRespondRemembersARefusedCacheKey |
| `provider.CacheState` | TestProviderCacheKeepsConversationStates, TestProviderCacheStateCheckpointForm |
| `provider.CacheState.ID` | TestProviderCacheKeepsConversationStates, TestProviderCacheStateCheckpointForm, TestProviderRespondRemembersARefusedCacheKey |
| `provider.CacheState.Response` | TestProviderCacheStateCheckpointForm |
| `provider.CacheState.Unsupported` | TestProviderCacheKeepsConversationStates, TestProviderCacheStateCheckpointForm, TestProviderRespondRemembersARefusedCacheKey |
| `provider.CacheState.Valid` | TestProviderCacheKeepsConversationStates, TestProviderCacheStateCheckpointForm |
| `provider.Client` | TestProviderClientBodyOfARequest, TestProviderRespondAnswersAndReportsAttempts |
| `provider.Client.Body` | TestProviderCacheKeepsConversationStates, TestProviderClientBodyOfARequest, TestProviderNewCacheIDAndMaxCacheID, TestToolsMeasureAgainstTheInputBudget |
| `provider.Client.Respond` | TestProviderClientResponseBudget, TestProviderRespondAnswersAndReportsAttempts, TestProviderRespondFailures, TestProviderRespondInterruptAndUnsupportedToolChoice, TestProviderRespondRemembersARefusedCacheKey |
| `provider.Client.ResponseBudget` | TestProviderClientResponseBudget |
| `provider.ContractBlocks` | TestProviderContractBlocks |
| `provider.Fingerprint` | TestProviderFingerprint |
| `provider.KnownUsage.Write` | TestProviderRespondAnswersAndReportsAttempts |
| `provider.MaxCacheID` | TestProviderNewCacheIDAndMaxCacheID |
| `provider.NewCache` | TestProviderCacheKeepsConversationStates, TestProviderClientBodyOfARequest, TestProviderRespondRemembersARefusedCacheKey |
| `provider.NewCacheID` | TestProviderCacheKeepsConversationStates, TestProviderNewCacheIDAndMaxCacheID, TestProviderRespondRemembersARefusedCacheKey |
| `provider.NewClient` | TestProviderCacheKeepsConversationStates, TestProviderClientBodyOfARequest, TestProviderClientResponseBudget, TestProviderNewCacheIDAndMaxCacheID, TestProviderRespondAnswersAndReportsAttempts, TestProviderRespondFailures, TestProviderRespondInterruptAndUnsupportedToolChoice, TestToolsMeasureAgainstTheInputBudget |
| `provider.Request` | TestProviderClientBodyOfARequest, TestProviderRespondAnswersAndReportsAttempts, TestProviderRespondInterruptAndUnsupportedToolChoice, TestToolsMeasureAgainstTheInputBudget |
| `provider.Request.Cache` | TestProviderCacheKeepsConversationStates, TestProviderClientBodyOfARequest, TestProviderRespondRemembersARefusedCacheKey |
| `provider.Request.CacheID` | TestProviderClientBodyOfARequest, TestProviderRespondInterruptAndUnsupportedToolChoice, TestToolsMeasureAgainstTheInputBudget |
| `provider.Request.Context` | TestProviderClientBodyOfARequest, TestProviderRespondAnswersAndReportsAttempts |
| `provider.Request.Input` | TestProviderClientBodyOfARequest, TestProviderRespondInterruptAndUnsupportedToolChoice, TestToolsMeasureAgainstTheInputBudget |
| `provider.Request.Instructions` | TestProviderClientBodyOfARequest, TestProviderRespondInterruptAndUnsupportedToolChoice, TestToolsMeasureAgainstTheInputBudget |
| `provider.Request.Role` | TestProviderClientBodyOfARequest, TestProviderRespondInterruptAndUnsupportedToolChoice, TestToolsMeasureAgainstTheInputBudget |
| `provider.Request.ToolChoice` | TestProviderClientBodyOfARequest, TestProviderRespondInterruptAndUnsupportedToolChoice |
| `provider.Request.Tools` | TestProviderClientBodyOfARequest, TestProviderRespondAnswersAndReportsAttempts, TestProviderRespondInterruptAndUnsupportedToolChoice |
| `provider.RequireLogin` | TestProviderRequireLogin |
| `provider.Response` | TestLoopHandledCallIsAnsweredNotRun, TestLoopRunEngineOrderAndHookArguments, TestLoopRunReturnsTheInterruptOfAShellCall, TestLoopRunSentInputIsWhatTheRequesterReturned, TestLoopRunStopsTheProcessGroupsOfTheRound, TestLoopRunStopsWithTheErrorUnchanged, TestLoopRunTerminalNamesAreTheRounds, TestLoopUsageOfAndSumUsage, TestProviderRespondAnswersAndReportsAttempts, TestToolsDeliverReportsWhatBecameOfTheImages |
| `provider.Response.Items` | TestLoopRunEngineOrderAndHookArguments, TestProviderRespondAnswersAndReportsAttempts, TestProviderRespondRemembersARefusedCacheKey |
| `provider.Response.Known` | TestLoopUsageOfAndSumUsage, TestProviderRespondAnswersAndReportsAttempts |
| `provider.Response.Model` | TestLoopRunEngineOrderAndHookArguments, TestProviderRespondAnswersAndReportsAttempts |
| `provider.Response.OperationID` | TestProviderRespondAnswersAndReportsAttempts |
| `provider.Role` | TestLoopRunEngineOrderAndHookArguments, TestProviderClientBodyOfARequest, TestProviderRequireLogin, TestProviderRespondInterruptAndUnsupportedToolChoice, TestToolsMeasureAgainstTheInputBudget |
| `provider.ValidCacheStates` | TestProviderCacheKeepsConversationStates, TestProviderCacheStateCheckpointForm |
| `sys.AsError` | TestProviderClientResponseBudget, TestProviderRequireLogin, TestProviderRespondFailures, TestProviderRespondInterruptAndUnsupportedToolChoice, TestSysFailAndAsError |
| `sys.EnableSupervisor` | TestMain, TestSysSuperviseHandshake |
| `sys.Error` | TestSysErrorRender, TestSysFailAndAsError, TestToolsDeliverReportsWhatBecameOfTheImages |
| `sys.Error.Class` | TestProviderClientResponseBudget, TestProviderRequireLogin, TestProviderRespondFailures, TestProviderRespondInterruptAndUnsupportedToolChoice, TestSysErrorRender, TestSysFailAndAsError, TestToolsDeliverReportsWhatBecameOfTheImages |
| `sys.Error.Detail` | TestProviderClientResponseBudget, TestProviderRespondFailures, TestSysErrorRender, TestSysFailAndAsError, TestToolsDeliverReportsWhatBecameOfTheImages |
| `sys.Error.Exit` | TestProviderClientResponseBudget, TestProviderRequireLogin, TestProviderRespondFailures, TestSysErrorRender, TestSysFailAndAsError, TestToolsDeliverReportsWhatBecameOfTheImages |
| `sys.Error.Lines` | TestSysErrorRender, TestSysFailAndAsError |
| `sys.Error.ProviderFailure` | TestProviderRespondFailures, TestSysErrorRender, TestSysFailAndAsError, TestToolsDeliverReportsWhatBecameOfTheImages |
| `sys.Error.Reason` | TestProviderClientResponseBudget, TestProviderRequireLogin, TestProviderRespondFailures, TestProviderRespondInterruptAndUnsupportedToolChoice, TestSysErrorRender, TestSysFailAndAsError, TestToolsDeliverReportsWhatBecameOfTheImages |
| `sys.Error.Render` | TestSysErrorRender |
| `sys.Fail` | TestSysErrorRender, TestSysFailAndAsError |
| `sys.Interrupted` | TestLoopRunReturnsTheInterruptOfAShellCall, TestProviderRespondInterruptAndUnsupportedToolChoice, TestSysInterruptedExitCodes, TestSysSessionRunIsInterrupted, TestSysSessionWatchSignals |
| `sys.Interrupted.ExitCode` | TestLoopRunReturnsTheInterruptOfAShellCall, TestProviderRespondInterruptAndUnsupportedToolChoice, TestSysInterruptedExitCodes, TestSysSessionRunIsInterrupted, TestSysSessionWatchSignals |
| `sys.Interrupted.Signal` | TestLoopRunReturnsTheInterruptOfAShellCall, TestSysInterruptedExitCodes, TestSysSessionRunIsInterrupted, TestSysSessionWatchSignals |
| `sys.MaybeSupervise` | TestMain, TestSysSuperviseHandshake |
| `sys.New` | TestSysNewTakesTheProcessEnvironment |
| `sys.Options` | TestSysSessionRunIsInterrupted, TestSysSessionRunResultAndOptions, TestSysSuperviseHandshake |
| `sys.Options.Env` | TestSysSessionRunResultAndOptions |
| `sys.Options.KillGroupOnExit` | TestSysSessionRunResultAndOptions |
| `sys.Result` | TestSysSessionRunIsInterrupted, TestSysSessionRunResultAndOptions |
| `sys.Result.Output` | TestSysSessionRunResultAndOptions, TestSysSuperviseHandshake |
| `sys.Result.Status` | TestSysSessionRunResultAndOptions, TestSysSuperviseHandshake |
| `sys.Session` | TestProviderRespondAnswersAndReportsAttempts, TestSysInterruptedExitCodes, TestSysNewTakesTheProcessEnvironment, TestSysSessionOutputAndEnvironment, TestSysSessionRunResultAndOptions, TestSysSessionWatchSignals, TestToolsRunsTheBuildersCalls |
| `sys.Session.Context` | TestSysInterruptedExitCodes, TestSysSessionWatchSignals |
| `sys.Session.Env` | TestSysNewTakesTheProcessEnvironment, TestSysSessionOutputAndEnvironment |
| `sys.Session.Err` | TestSysInterruptedExitCodes, TestSysNewTakesTheProcessEnvironment, TestSysSessionWatchSignals |
| `sys.Session.Getenv` | TestSysNewTakesTheProcessEnvironment, TestSysSessionOutputAndEnvironment |
| `sys.Session.Out` | TestSysSessionOutputAndEnvironment |
| `sys.Session.Progress` | TestSysSessionOutputAndEnvironment |
| `sys.Session.Puts` | TestSysSessionOutputAndEnvironment |
| `sys.Session.Run` | TestSysSessionRunIsInterrupted, TestSysSessionRunResultAndOptions, TestSysSuperviseHandshake |
| `sys.Session.Stdout` | TestSysNewTakesTheProcessEnvironment, TestSysSessionOutputAndEnvironment |
| `sys.Session.WatchSignals` | TestSysSessionWatchSignals |
| `tools.Call` | TestLoopHandledCallIsAnsweredNotRun, TestLoopRunEngineOrderAndHookArguments, TestLoopRunReturnsTheInterruptOfAShellCall, TestLoopRunStopsTheProcessGroupsOfTheRound, TestLoopRunStopsWithTheErrorUnchanged, TestLoopRunTerminalNamesAreTheRounds, TestToolsCallFieldsAndArguments, TestToolsDeliverReportsWhatBecameOfTheImages, TestToolsReadEventOfAnImageAndOfText, TestToolsRunsTheBuildersCalls |
| `tools.Call.Args` | TestLoopRunEngineOrderAndHookArguments, TestToolsCallFieldsAndArguments |
| `tools.Call.Arguments` | TestLoopRunEngineOrderAndHookArguments, TestToolsCallFieldsAndArguments, TestToolsDeliverReportsWhatBecameOfTheImages, TestToolsReadEventOfAnImageAndOfText, TestToolsRunsTheBuildersCalls |
| `tools.Call.CallID` | TestToolsCallFieldsAndArguments |
| `tools.Call.Item` | TestLoopRunEngineOrderAndHookArguments, TestToolsCallFieldsAndArguments, TestToolsDeliverReportsWhatBecameOfTheImages, TestToolsReadEventOfAnImageAndOfText, TestToolsRunsTheBuildersCalls |
| `tools.Call.Name` | TestLoopRunEngineOrderAndHookArguments, TestLoopRunStopsTheProcessGroupsOfTheRound, TestLoopRunTerminalNamesAreTheRounds, TestToolsCallFieldsAndArguments, TestToolsDeliverReportsWhatBecameOfTheImages, TestToolsReadEventOfAnImageAndOfText, TestToolsRunsTheBuildersCalls |
| `tools.Cut` | TestToolsCutBoundsTextWithAMarkerCountingTheOmission |
| `tools.DecodeArguments` | TestToolsCallFieldsAndArguments |
| `tools.Definition` | TestToolsDefinitionsOfTheGenericTools |
| `tools.Deliver` | TestToolsDeliverReportsWhatBecameOfTheImages |
| `tools.ImageEntries` | TestToolsDeliverReportsWhatBecameOfTheImages, TestToolsReadEventOfAnImageAndOfText |
| `tools.ImageEntry` | TestToolsReadEventOfAnImageAndOfText |
| `tools.ImageEntry.Meta` | TestToolsDeliverReportsWhatBecameOfTheImages, TestToolsReadEventOfAnImageAndOfText |
| `tools.InputBudgetBytes` | TestToolsMeasureAgainstTheInputBudget |
| `tools.Measure` | TestToolsMeasureAgainstTheInputBudget |
| `tools.Measurement` | TestToolsMeasureAgainstTheInputBudget |
| `tools.Measurement.Fits` | TestToolsMeasureAgainstTheInputBudget |
| `tools.Measurement.InputBudgetBytes` | TestToolsMeasureAgainstTheInputBudget |
| `tools.Measurement.RenderedBytes` | TestToolsMeasureAgainstTheInputBudget |
| `tools.Output` | TestLoopHandledCallIsAnsweredNotRun, TestLoopRunEngineOrderAndHookArguments, TestLoopRunStopsTheProcessGroupsOfTheRound, TestLoopRunStopsWithTheErrorUnchanged, TestLoopRunTerminalNamesAreTheRounds, TestToolsOutputIsTextOrTypedParts, TestToolsOutputItemAnswersACall, TestToolsReadEventOfAnImageAndOfText, TestToolsRunsTheBuildersCalls |
| `tools.Output.Text` | TestLoopHandledCallIsAnsweredNotRun, TestLoopRunEngineOrderAndHookArguments, TestLoopRunStopsTheProcessGroupsOfTheRound, TestLoopRunStopsWithTheErrorUnchanged, TestLoopRunTerminalNamesAreTheRounds, TestToolsOutputIsTextOrTypedParts, TestToolsReadEventOfAnImageAndOfText, TestToolsRunsTheBuildersCalls |
| `tools.Output.Typed` | TestToolsOutputIsTextOrTypedParts, TestToolsReadEventOfAnImageAndOfText |
| `tools.Output.Value` | TestToolsOutputIsTextOrTypedParts, TestToolsOutputItemAnswersACall, TestToolsReadEventOfAnImageAndOfText |
| `tools.OutputItem` | TestToolsOutputItemAnswersACall, TestToolsReadEventOfAnImageAndOfText |
| `tools.OutputReserveTokens` | TestToolsMeasureAgainstTheInputBudget |
| `tools.ReadEvent` | TestToolsReadEventOfAnImageAndOfText |
| `tools.ReadFileTool` | TestToolsRunsTheBuildersCalls |
| `tools.Replay` | TestToolsReplayKeepsWhatTheProviderAcceptsBack |
| `tools.Tools` | TestLoopRunEngineOrderAndHookArguments, TestLoopRunReturnsTheInterruptOfAShellCall, TestLoopRunStopsTheProcessGroupsOfTheRound, TestToolsDeliverReportsWhatBecameOfTheImages, TestToolsReadEventOfAnImageAndOfText, TestToolsRunsTheBuildersCalls |
| `tools.Tools.Enabled` | TestLoopRunReturnsTheInterruptOfAShellCall, TestLoopRunStopsTheProcessGroupsOfTheRound, TestToolsDeliverReportsWhatBecameOfTheImages, TestToolsReadEventOfAnImageAndOfText, TestToolsRunsTheBuildersCalls |
| `tools.Tools.Excluded` | TestToolsReadEventOfAnImageAndOfText, TestToolsRunsTheBuildersCalls |
| `tools.Tools.Protected` | TestToolsRunsTheBuildersCalls |
| `tools.Tools.Repo` | TestToolsReadEventOfAnImageAndOfText, TestToolsRunsTheBuildersCalls |
| `tools.Tools.Root` | TestLoopRunReturnsTheInterruptOfAShellCall, TestLoopRunStopsTheProcessGroupsOfTheRound, TestToolsDeliverReportsWhatBecameOfTheImages, TestToolsReadEventOfAnImageAndOfText, TestToolsRunsTheBuildersCalls |
| `tools.Tools.Session` | TestLoopRunReturnsTheInterruptOfAShellCall, TestLoopRunStopsTheProcessGroupsOfTheRound, TestToolsDeliverReportsWhatBecameOfTheImages, TestToolsReadEventOfAnImageAndOfText, TestToolsRunsTheBuildersCalls |

### kogen-review

| Use | Tests |
| --- | --- |
| `review.Candidate` | TestInputFieldsReachTheReview |
| `review.Candidate.Builder` | TestCandidateFieldsAreWhatTheReviewReads, TestExternalReviewIsNeverTheBuildersSession |
| `review.Candidate.Excluded` | TestCandidateExcludedPathsAreOutsideTheCandidateTree, TestCandidateFieldsAreWhatTheReviewReads, TestExternalReviewApprovesInItsOwnSession |
| `review.Candidate.Protected` | TestCandidateFieldsAreWhatTheReviewReads, TestExternalReviewApprovesInItsOwnSession, TestMaterialSetIsMadeByTheReviewFromItsInput |
| `review.Candidate.Repo` | TestCandidateFieldsAreWhatTheReviewReads |
| `review.Candidate.Root` | TestCandidateFieldsAreWhatTheReviewReads |
| `review.Candidate.Start` | TestCandidateFieldsAreWhatTheReviewReads |
| `review.Candidate.Tree` | TestCandidateFieldsAreWhatTheReviewReads |
| `review.Checkpoint` | TestEnvMethodSetPinnedThroughTheInterface, TestIndependentResumeJudgesTheSavedVerdict, TestIndependentResumeOfABlockedReviewReopensIt, TestInputFieldsReachTheReview, TestResumeOfAReviewContinuesItsSavedConversation, TestReviewerInterfacePinned |
| `review.Checkpoint.Base` | TestIndependentResumeJudgesTheSavedVerdict, TestIndependentResumeOfABlockedReviewReopensIt |
| `review.Checkpoint.Pass` | TestIndependentResumeJudgesTheSavedVerdict, TestIndependentResumeOfABlockedReviewReopensIt, TestRepairKeepsTheEarlierVerdictsAsAttempts, TestResumeOfAReviewContinuesItsSavedConversation, TestReviewerInterfacePinned |
| `review.Checkpoint.Session` | TestIndependentResumeJudgesTheSavedVerdict, TestIndependentResumeOfABlockedReviewReopensIt, TestIndependentReviewChangesIsTheReviewBlockedStop, TestRepairKeepsTheEarlierVerdictsAsAttempts, TestResumeOfAReviewContinuesItsSavedConversation, TestReviewerInterfacePinned |
| `review.Checkpoint.Stage` | TestIndependentResumeJudgesTheSavedVerdict, TestIndependentResumeOfABlockedReviewReopensIt, TestIndependentReviewChangesIsTheReviewBlockedStop, TestResumeOfAReviewContinuesItsSavedConversation, TestReviewerInterfacePinned |
| `review.Commands` | TestMaterialSetIsMadeByTheReviewFromItsInput |
| `review.Commands.Acceptance` | TestMaterialSetIsMadeByTheReviewFromItsInput |
| `review.Commands.Checks` | TestMaterialSetIsMadeByTheReviewFromItsInput |
| `review.Commands.Fix` | TestMaterialSetIsMadeByTheReviewFromItsInput |
| `review.DecodeSession` | TestDecodeSessionReadsWhatACheckpointKept, TestIndependentReviewApprovesTheGreenPass, TestIndependentReviewChangesIsTheReviewBlockedStop, TestRepairKeepsTheEarlierVerdictsAsAttempts |
| `review.Done` | TestEnvMethodSetPinnedThroughTheInterface, TestToolsOfAReviewAreTheCallersNativeTools |
| `review.Done.Call` | TestToolsOfAReviewAreTheCallersNativeTools |
| `review.Done.Result` | TestToolsOfAReviewAreTheCallersNativeTools |
| `review.Env` | TestEnvMethodSetPinnedThroughTheInterface, TestReviewerInterfacePinned |
| `review.Evidence` | TestInputFieldsReachTheReview |
| `review.Evidence.BaselineFailing` | TestIndependentResumeOfABlockedReviewReopensIt |
| `review.Evidence.Commands` | TestMaterialSetIsMadeByTheReviewFromItsInput |
| `review.Evidence.Diagnostic` | TestExternalReviewApprovesInItsOwnSession, TestIntentFieldsMakeTheReviewersAuthority |
| `review.Evidence.Pass` | TestCandidateFieldsAreWhatTheReviewReads, TestInputFieldsReachTheReview, TestMaterialSetIsMadeByTheReviewFromItsInput |
| `review.File` | TestFileNamesTheReviewRecord, TestIndependentReviewApprovesTheGreenPass, TestInputFieldsReachTheReview, TestIntentFieldsMakeTheReviewersAuthority, TestRepairKeepsTheEarlierVerdictsAsAttempts |
| `review.Independent` | TestCandidateExcludedPathsAreOutsideTheCandidateTree, TestCandidateFieldsAreWhatTheReviewReads, TestEnvErrorsStopTheReviewUnchanged, TestEnvUnreadPartsBlockAnApproval, TestIndependentApprovalOfAnUnverifiedTreeIsVerifiedFirst, TestIndependentRejectedVerdictsEndTheReviewWithoutAVerdict, TestIndependentResumeJudgesTheSavedVerdict, TestIndependentResumeOfABlockedReviewReopensIt, TestIndependentReviewApprovesTheGreenPass, TestIndependentReviewChangesIsTheReviewBlockedStop, TestIndependentReviewWithoutAVerdictIsHandedToTheBuild, TestInputFieldsReachTheReview, TestIntentFieldsMakeTheReviewersAuthority, TestIntentSourceProvenanceIsStatedAndGuarded, TestMaterialSetIsMadeByTheReviewFromItsInput, TestRepairKeepsTheEarlierVerdictsAsAttempts, TestResumeOfAReviewContinuesItsSavedConversation, TestReviewerInterfacePinned, TestToolsOfAReviewAreTheCallersNativeTools |
| `review.Input` | TestReviewerInterfacePinned |
| `review.Input.Candidate` | TestCandidateExcludedPathsAreOutsideTheCandidateTree, TestCandidateFieldsAreWhatTheReviewReads, TestExternalReviewApprovesInItsOwnSession, TestExternalReviewIsNeverTheBuildersSession, TestInputFieldsReachTheReview, TestMaterialSetIsMadeByTheReviewFromItsInput |
| `review.Input.Checkpoint` | TestIndependentResumeJudgesTheSavedVerdict, TestIndependentResumeOfABlockedReviewReopensIt, TestInputFieldsReachTheReview, TestResumeOfAReviewContinuesItsSavedConversation, TestReviewerInterfacePinned |
| `review.Input.Evidence` | TestCandidateFieldsAreWhatTheReviewReads, TestExternalReviewApprovesInItsOwnSession, TestIndependentResumeOfABlockedReviewReopensIt, TestInputFieldsReachTheReview, TestIntentFieldsMakeTheReviewersAuthority, TestMaterialSetIsMadeByTheReviewFromItsInput |
| `review.Input.Executor` | TestExternalReviewApprovesInItsOwnSession, TestInputFieldsReachTheReview |
| `review.Input.Intent` | TestExternalReviewApprovesInItsOwnSession, TestInputFieldsReachTheReview, TestIntentFieldsMakeTheReviewersAuthority, TestIntentSourceProvenanceIsStatedAndGuarded |
| `review.Input.Last` | TestIndependentResumeOfABlockedReviewReopensIt, TestInputFieldsReachTheReview |
| `review.Input.Number` | TestIndependentResumeOfABlockedReviewReopensIt, TestInputFieldsReachTheReview |
| `review.Input.Run` | TestInputFieldsReachTheReview |
| `review.Input.Tools` | TestExternalReviewApprovesInItsOwnSession, TestExternalReviewWithoutAVerdictIsHandedToTheBuild, TestInputFieldsReachTheReview, TestToolsOfAReviewAreTheCallersNativeTools |
| `review.Intent` | TestInputFieldsReachTheReview, TestIntentFieldsMakeTheReviewersAuthority |
| `review.Intent.Approval` | TestExternalReviewApprovesInItsOwnSession, TestIntentFieldsMakeTheReviewersAuthority |
| `review.Intent.Plan` | TestExternalReviewApprovesInItsOwnSession, TestIntentFieldsMakeTheReviewersAuthority |
| `review.Intent.Role` | TestIntentFieldsMakeTheReviewersAuthority |
| `review.Intent.Slug` | TestIntentFieldsMakeTheReviewersAuthority |
| `review.Intent.Source` | TestIntentFieldsMakeTheReviewersAuthority, TestIntentSourceProvenanceIsStatedAndGuarded |
| `review.Intent.Text` | TestExternalReviewApprovesInItsOwnSession, TestIntentFieldsMakeTheReviewersAuthority |
| `review.Invalid` | TestIndependentReviewApprovesTheGreenPass |
| `review.Invalid.Base` | TestExternalReviewWithoutAVerdictIsHandedToTheBuild, TestIndependentResumeOfABlockedReviewReopensIt, TestIndependentReviewWithoutAVerdictIsHandedToTheBuild |
| `review.Invalid.ForcedModel` | TestIndependentRejectedVerdictsEndTheReviewWithoutAVerdict, TestIndependentReviewWithoutAVerdictIsHandedToTheBuild |
| `review.Invalid.ForcedRoute` | TestIndependentRejectedVerdictsEndTheReviewWithoutAVerdict, TestIndependentReviewWithoutAVerdictIsHandedToTheBuild |
| `review.Invalid.NoToolReminder` | TestIndependentRejectedVerdictsEndTheReviewWithoutAVerdict, TestIndependentReviewWithoutAVerdictIsHandedToTheBuild |
| `review.Invalid.Pass` | TestExternalReviewWithoutAVerdictIsHandedToTheBuild, TestIndependentResumeOfABlockedReviewReopensIt, TestIndependentReviewWithoutAVerdictIsHandedToTheBuild |
| `review.Invalid.Reason` | TestEnvUnreadPartsBlockAnApproval, TestExternalReviewWithoutAVerdictIsHandedToTheBuild, TestIndependentRejectedVerdictsEndTheReviewWithoutAVerdict, TestIndependentReviewWithoutAVerdictIsHandedToTheBuild |
| `review.Invalid.RejectedVerdict` | TestExternalReviewWithoutAVerdictIsHandedToTheBuild, TestIndependentRejectedVerdictsEndTheReviewWithoutAVerdict, TestIndependentReviewWithoutAVerdictIsHandedToTheBuild |
| `review.Invalid.Reopened` | TestIndependentResumeOfABlockedReviewReopensIt, TestIndependentReviewWithoutAVerdictIsHandedToTheBuild |
| `review.Invalid.Repaired` | TestIndependentResumeOfABlockedReviewReopensIt, TestIndependentReviewWithoutAVerdictIsHandedToTheBuild |
| `review.Invalid.Review` | TestExternalReviewWithoutAVerdictIsHandedToTheBuild, TestIndependentResumeOfABlockedReviewReopensIt, TestIndependentReviewWithoutAVerdictIsHandedToTheBuild |
| `review.InvalidRecord` | TestIndependentReviewWithoutAVerdictIsHandedToTheBuild, TestInvalidRecordIsTheRecordOfAReviewWithoutAVerdict |
| `review.MaterialSet` | TestEnvMethodSetPinnedThroughTheInterface |
| `review.MaterialSet.Authority` | TestIndependentReviewApprovesTheGreenPass, TestIntentFieldsMakeTheReviewersAuthority, TestIntentSourceProvenanceIsStatedAndGuarded, TestMaterialSetIsMadeByTheReviewFromItsInput |
| `review.MaterialSet.Checks` | TestMaterialSetIsMadeByTheReviewFromItsInput |
| `review.MaterialSet.Files` | TestMaterialSetIsMadeByTheReviewFromItsInput |
| `review.MaterialSet.Source` | TestIndependentReviewApprovesTheGreenPass, TestMaterialSetIsMadeByTheReviewFromItsInput |
| `review.NewExternal` | TestExternalReviewApprovesInItsOwnSession, TestExternalReviewChangesIsTheReviewBlockedStop, TestExternalReviewIsNeverTheBuildersSession, TestExternalReviewWithoutAVerdictIsHandedToTheBuild, TestNewExternalLimitsTheRoundsOfASession, TestReviewerInterfacePinned |
| `review.Request` | TestEnvMethodSetPinnedThroughTheInterface |
| `review.Request.ForceTool` | TestIndependentReviewApprovesTheGreenPass, TestIndependentReviewWithoutAVerdictIsHandedToTheBuild |
| `review.Request.Input` | TestCandidateFieldsAreWhatTheReviewReads, TestEnvUnreadPartsBlockAnApproval, TestIndependentApprovalOfAnUnverifiedTreeIsVerifiedFirst, TestIndependentRejectedVerdictsEndTheReviewWithoutAVerdict, TestIndependentResumeOfABlockedReviewReopensIt, TestIndependentReviewApprovesTheGreenPass, TestResumeOfAReviewContinuesItsSavedConversation, TestToolsOfAReviewAreTheCallersNativeTools |
| `review.Request.Instructions` | TestIndependentReviewApprovesTheGreenPass |
| `review.Request.Prefill` | TestIndependentReviewApprovesTheGreenPass |
| `review.Request.Retain` | TestIndependentReviewApprovesTheGreenPass |
| `review.Request.Role` | TestIndependentReviewApprovesTheGreenPass, TestIntentFieldsMakeTheReviewersAuthority, TestResumeOfAReviewContinuesItsSavedConversation |
| `review.Request.Tools` | TestIndependentReviewApprovesTheGreenPass, TestResumeOfAReviewContinuesItsSavedConversation |
| `review.Result` | TestReviewerInterfacePinned |
| `review.Result.Invalid` | TestEnvUnreadPartsBlockAnApproval, TestExternalReviewWithoutAVerdictIsHandedToTheBuild, TestIndependentRejectedVerdictsEndTheReviewWithoutAVerdict, TestIndependentResumeOfABlockedReviewReopensIt, TestIndependentReviewApprovesTheGreenPass, TestIndependentReviewWithoutAVerdictIsHandedToTheBuild |
| `review.Result.Pass` | TestExternalReviewApprovesInItsOwnSession, TestIndependentApprovalOfAnUnverifiedTreeIsVerifiedFirst, TestIndependentResumeJudgesTheSavedVerdict, TestIndependentResumeOfABlockedReviewReopensIt, TestIndependentReviewApprovesTheGreenPass, TestIndependentReviewChangesIsTheReviewBlockedStop, TestInputFieldsReachTheReview |
| `review.Result.Record` | TestCandidateExcludedPathsAreOutsideTheCandidateTree, TestCandidateFieldsAreWhatTheReviewReads, TestExternalReviewApprovesInItsOwnSession, TestExternalReviewChangesIsTheReviewBlockedStop, TestExternalReviewIsNeverTheBuildersSession, TestIndependentApprovalOfAnUnverifiedTreeIsVerifiedFirst, TestIndependentResumeJudgesTheSavedVerdict, TestIndependentResumeOfABlockedReviewReopensIt, TestIndependentReviewApprovesTheGreenPass, TestIndependentReviewChangesIsTheReviewBlockedStop, TestInputFieldsReachTheReview, TestResumeOfAReviewContinuesItsSavedConversation, TestReviewerInterfacePinned |
| `review.Reviewer` | TestIndependentReviewApprovesTheGreenPass, TestReviewerInterfacePinned |
| `review.Reviewer.Resume` | TestReviewerInterfacePinned |
| `review.Reviewer.Review` | TestIndependentReviewApprovesTheGreenPass, TestReviewerInterfacePinned |
| `review.Run` | TestInputFieldsReachTheReview |
| `review.Run.Dir` | TestInputFieldsReachTheReview |
| `review.Run.ID` | TestInputFieldsReachTheReview |
| `review.Session` | TestDecodeSessionReadsWhatACheckpointKept, TestIndependentResumeJudgesTheSavedVerdict, TestIndependentResumeOfABlockedReviewReopensIt, TestRepairKeepsTheEarlierVerdictsAsAttempts, TestResumeOfAReviewContinuesItsSavedConversation, TestReviewerInterfacePinned, TestSessionEncodeIsTheCheckpointBytes |
| `review.Session.Attempts` | TestIndependentReviewChangesIsTheReviewBlockedStop, TestRepairKeepsTheEarlierVerdictsAsAttempts, TestSessionEncodeIsTheCheckpointBytes, TestSessionFieldTypes |
| `review.Session.Encode` | TestDecodeSessionReadsWhatACheckpointKept, TestIndependentResumeJudgesTheSavedVerdict, TestIndependentResumeOfABlockedReviewReopensIt, TestResumeOfAReviewContinuesItsSavedConversation, TestReviewerInterfacePinned, TestSessionEncodeIsTheCheckpointBytes |
| `review.Session.Input` | TestIndependentResumeOfABlockedReviewReopensIt, TestIndependentReviewApprovesTheGreenPass, TestRepairKeepsTheEarlierVerdictsAsAttempts, TestResumeOfAReviewContinuesItsSavedConversation, TestSessionEncodeIsTheCheckpointBytes, TestSessionFieldTypes |
| `review.Session.Role` | TestIndependentResumeJudgesTheSavedVerdict, TestIndependentResumeOfABlockedReviewReopensIt, TestIndependentReviewApprovesTheGreenPass, TestRepairKeepsTheEarlierVerdictsAsAttempts, TestResumeOfAReviewContinuesItsSavedConversation, TestReviewerInterfacePinned, TestSessionEncodeIsTheCheckpointBytes, TestSessionFieldTypes |
| `review.Session.Tools` | TestResumeOfAReviewContinuesItsSavedConversation, TestSessionFieldTypes |
| `review.Session.Tree` | TestIndependentResumeJudgesTheSavedVerdict, TestIndependentResumeOfABlockedReviewReopensIt, TestIndependentReviewChangesIsTheReviewBlockedStop, TestReviewerInterfacePinned, TestSessionFieldTypes |
| `review.Session.Usage` | TestIndependentResumeJudgesTheSavedVerdict, TestIndependentResumeOfABlockedReviewReopensIt, TestReviewerInterfacePinned, TestSessionEncodeIsTheCheckpointBytes, TestSessionFieldTypes |
| `review.Session.Verdict` | TestDecodeSessionReadsWhatACheckpointKept, TestIndependentResumeJudgesTheSavedVerdict, TestIndependentResumeOfABlockedReviewReopensIt, TestIndependentReviewChangesIsTheReviewBlockedStop, TestReviewerInterfacePinned, TestSessionEncodeIsTheCheckpointBytes, TestSessionFieldTypes |
| `review.Submitted` | TestSessionFieldTypes |
| `review.Submitted.Findings` | TestRepairKeepsTheEarlierVerdictsAsAttempts, TestSessionFieldTypes |
| `review.Submitted.Pass` | TestRepairKeepsTheEarlierVerdictsAsAttempts, TestSessionFieldTypes |
| `review.Submitted.Tree` | TestRepairKeepsTheEarlierVerdictsAsAttempts, TestSessionFieldTypes |
| `review.Submitted.Verdict` | TestRepairKeepsTheEarlierVerdictsAsAttempts, TestSessionFieldTypes |
| `review.Tools` | TestInputFieldsReachTheReview |
| `review.Tools.Home` | TestExternalReviewApprovesInItsOwnSession, TestToolsOfAReviewAreTheCallersNativeTools |
| `review.Tools.Native` | TestToolsOfAReviewAreTheCallersNativeTools |
| `review.Tools.Record` | TestToolsOfAReviewAreTheCallersNativeTools |
| `review.Tools.Round` | TestExternalReviewApprovesInItsOwnSession, TestExternalReviewWithoutAVerdictIsHandedToTheBuild, TestToolsOfAReviewAreTheCallersNativeTools |
| `review.Verdict` | TestIndependentResumeJudgesTheSavedVerdict, TestIndependentResumeOfABlockedReviewReopensIt, TestReviewerInterfacePinned, TestSessionEncodeIsTheCheckpointBytes, TestSessionFieldTypes |
| `review.Verdict.Findings` | TestDecodeSessionReadsWhatACheckpointKept, TestIndependentResumeOfABlockedReviewReopensIt, TestSessionEncodeIsTheCheckpointBytes, TestSessionFieldTypes |
| `review.Verdict.Verdict` | TestDecodeSessionReadsWhatACheckpointKept, TestIndependentResumeJudgesTheSavedVerdict, TestIndependentResumeOfABlockedReviewReopensIt, TestIndependentReviewChangesIsTheReviewBlockedStop, TestReviewerInterfacePinned, TestSessionEncodeIsTheCheckpointBytes, TestSessionFieldTypes |

### kogen-verification

| Use | Tests |
| --- | --- |
| `verify.Acceptance.Output` | TestVerifyFailedAcceptance, TestVerifyGreenPass |
| `verify.Acceptance.Problems` | TestPassComplete, TestPassFeedbackSections, TestVerifyFailedAcceptance, TestVerifyFailedCheck, TestVerifyGreenPass |
| `verify.Acceptance.Scenarios` | TestPassChecksLine, TestPassComplete, TestPassValid, TestVerifyFailedAcceptance, TestVerifyGreenPass |
| `verify.Cap` | TestCap |
| `verify.CommandResult` | TestCommandResultJSON, TestEnvFailure, TestPassChecksLine, TestPassFeedbackSections, TestPassJSON, TestVerifyFailedSetup, TestVerifyGreenPass |
| `verify.CommandResult.Command` | TestCommandResultJSON, TestPassComplete, TestPassFeedbackSections, TestPassJSON, TestVerifyFailedSetup, TestVerifyGreenPass, TestVerifyStageGate |
| `verify.CommandResult.Name` | TestCommandResultJSON, TestPassChecksLine, TestPassComplete, TestPassFeedbackSections, TestPassJSON, TestVerifyFailedCheck, TestVerifyFailedSetup, TestVerifyGreenPass, TestVerifyStageGate |
| `verify.CommandResult.Output` | TestCommandResultJSON, TestEnvFailure, TestPassFeedbackSections, TestPassJSON, TestVerifyExcludedIntents, TestVerifyFailedAcceptance, TestVerifyFailedCheck, TestVerifyFailedSetup, TestVerifyGreenPass |
| `verify.CommandResult.Status` | TestCommandResultJSON, TestEnvFailure, TestPassComplete, TestPassFeedbackSections, TestPassJSON, TestVerifyFailedCheck, TestVerifyFailedSetup, TestVerifyGreenPass, TestVerifyStageGate |
| `verify.Diagnostic` | TestDiagnostic |
| `verify.EnvFailure` | TestEnvFailure |
| `verify.Fix` | TestFix |
| `verify.Input` | TestFix, TestVerifyInputFields |
| `verify.Input.Config` | TestVerifyInputFields |
| `verify.Input.Excluded` | TestVerifyExcludedIntents, TestVerifyInputFields |
| `verify.Input.Intact` | TestFix, TestVerifyGreenPass, TestVerifyInputFields, TestVerifyIntactRefusal |
| `verify.Input.Intent` | TestVerifyInputFields |
| `verify.Input.Log` | TestFix, TestVerifyGreenPass, TestVerifyInputFields |
| `verify.Input.MaintainedModels` | TestVerifyInputFields, TestVerifyModelAndMaintainedModels |
| `verify.Input.N` | TestVerifyInputFields |
| `verify.Input.Repo` | TestVerifyInputFields, TestVerifyRepoDefault |
| `verify.Input.Root` | TestVerifyInputFields |
| `verify.Input.RunDir` | TestVerifyGreenPass, TestVerifyInputFields |
| `verify.Input.Stage` | TestVerifyInputFields, TestVerifyStageGate |
| `verify.Input.Start` | TestVerifyInputFields |
| `verify.Input.Tools` | TestFix, TestVerifyGreenPass, TestVerifyInputFields, TestVerifyModelAndMaintainedModels |
| `verify.MaintainedModelResult.Path` | TestPassFeedbackSections, TestPassJSON, TestVerifyModelAndMaintainedModels |
| `verify.MaintainedModelResult.Result` | TestPassFeedbackSections, TestPassJSON, TestVerifyModelAndMaintainedModels |
| `verify.ModelResult.Problems` | TestPassComplete, TestPassFeedbackSections, TestPassJSON, TestVerifyGreenPass, TestVerifyModelAndMaintainedModels |
| `verify.ModelResult.State` | TestPassChecksLine, TestPassComplete, TestPassFeedbackSections, TestPassJSON, TestPassValid, TestVerifyGreenPass, TestVerifyModelAndMaintainedModels, TestVerifyStageGate |
| `verify.NotRunNotice` | TestNotRunNotice, TestVerifyFailedCheck |
| `verify.Pass` | TestPassChecksLine, TestPassComplete, TestPassFeedbackSections, TestPassJSON, TestPassValid, TestVerifyInputFields |
| `verify.Pass.Acceptance` | TestPassChecksLine, TestPassComplete, TestPassFeedbackSections, TestPassJSON, TestPassValid, TestVerifyFailedAcceptance, TestVerifyFailedCheck, TestVerifyFailedSetup, TestVerifyGreenPass, TestVerifyStageGate |
| `verify.Pass.Checks` | TestPassChecksLine, TestPassComplete, TestPassFeedbackSections, TestPassJSON, TestVerifyExcludedIntents, TestVerifyFailedAcceptance, TestVerifyFailedCheck, TestVerifyGreenPass, TestVerifyIntactRefusal, TestVerifyStageGate |
| `verify.Pass.ChecksLine` | TestPassChecksLine, TestVerifyFailedAcceptance, TestVerifyGreenPass, TestVerifyModelAndMaintainedModels |
| `verify.Pass.Clean` | TestPassComplete, TestPassJSON, TestPassValid, TestVerifyFailedCheck, TestVerifyGreenPass, TestVerifyMovedFiles |
| `verify.Pass.Complete` | TestPassComplete, TestVerifyFailedCheck, TestVerifyGreenPass, TestVerifyModelAndMaintainedModels, TestVerifyStageGate |
| `verify.Pass.Failing` | TestPassComplete, TestPassJSON, TestVerifyFailedAcceptance, TestVerifyFailedCheck, TestVerifyFailedSetup, TestVerifyGreenPass, TestVerifyModelAndMaintainedModels, TestVerifyMovedFiles, TestVerifyStageGate |
| `verify.Pass.Feedback` | TestPassFeedbackSections, TestVerifyFailedAcceptance, TestVerifyFailedCheck, TestVerifyFailedSetup, TestVerifyModelAndMaintainedModels, TestVerifyMovedFiles |
| `verify.Pass.Green` | TestPassComplete, TestPassJSON, TestVerifyExcludedIntents, TestVerifyFailedAcceptance, TestVerifyFailedCheck, TestVerifyFailedSetup, TestVerifyGreenPass, TestVerifyModelAndMaintainedModels, TestVerifyMovedFiles, TestVerifyRepoDefault, TestVerifyStageGate |
| `verify.Pass.Legacy` | TestPassComplete, TestPassJSON, TestVerifyGreenPass |
| `verify.Pass.MaintainedModels` | TestPassFeedbackSections, TestPassJSON, TestVerifyGreenPass, TestVerifyModelAndMaintainedModels |
| `verify.Pass.Model` | TestPassChecksLine, TestPassComplete, TestPassFeedbackSections, TestPassJSON, TestPassValid, TestVerifyGreenPass, TestVerifyModelAndMaintainedModels, TestVerifyStageGate |
| `verify.Pass.Moved` | TestPassComplete, TestPassFeedbackSections, TestPassJSON, TestVerifyFailedCheck, TestVerifyGreenPass, TestVerifyMovedFiles |
| `verify.Pass.N` | TestPassFeedbackSections, TestPassJSON, TestPassValid, TestVerifyGreenPass, TestVerifyIntactRefusal |
| `verify.Pass.NotRun` | TestPassComplete, TestPassFeedbackSections, TestPassJSON, TestVerifyFailedAcceptance, TestVerifyFailedCheck, TestVerifyFailedSetup, TestVerifyGreenPass, TestVerifyModelAndMaintainedModels, TestVerifyMovedFiles, TestVerifyStageGate |
| `verify.Pass.Setup` | TestPassComplete, TestPassFeedbackSections, TestPassJSON, TestVerifyFailedAcceptance, TestVerifyFailedSetup, TestVerifyGreenPass, TestVerifyStageGate |
| `verify.Pass.Tree` | TestPassJSON, TestPassValid, TestVerifyExcludedIntents, TestVerifyGreenPass, TestVerifyIntactRefusal, TestVerifyMovedFiles, TestVerifyRepoDefault |
| `verify.Pass.Valid` | TestPassValid, TestVerifyGreenPass |
| `verify.Readiness` | TestReadinessSetup |
| `verify.Readiness.Config` | TestReadinessSetup |
| `verify.Readiness.Log` | TestReadinessSetup |
| `verify.Readiness.Repo` | TestReadinessSetup |
| `verify.Readiness.Root` | TestReadinessSetup |
| `verify.Readiness.Setup` | TestReadinessSetup |
| `verify.Readiness.Tools` | TestReadinessSetup |
| `verify.RequireTools` | TestRequireTools |
| `verify.RunWatched` | TestRunWatched |
| `verify.ScenarioResult.Detail` | TestVerifyFailedAcceptance, TestVerifyGreenPass |
| `verify.ScenarioResult.Line` | TestPassComplete, TestVerifyFailedAcceptance, TestVerifyGreenPass |
| `verify.ScenarioResult.Name` | TestPassComplete, TestVerifyFailedAcceptance, TestVerifyGreenPass |
| `verify.ScenarioResult.Status` | TestPassComplete, TestVerifyFailedAcceptance, TestVerifyGreenPass |
| `verify.Stage` | TestVerifyInputFields, TestVerifyStageGate |
| `verify.Stage.Checks` | TestVerifyInputFields, TestVerifyStageGate |
| `verify.Stage.ID` | TestVerifyInputFields, TestVerifyStageGate |
| `verify.Tools` | TestFix, TestReadinessSetup, TestRequireTools, TestToolsRequire, TestVerifyGreenPass, TestVerifyInputFields, TestVerifyModelAndMaintainedModels |
| `verify.Tools.Cucumber` | TestFix, TestToolsRequire |
| `verify.Tools.Node` | TestFix, TestReadinessSetup, TestRequireTools, TestToolsRequire, TestVerifyGreenPass, TestVerifyModelAndMaintainedModels |
| `verify.Tools.Quint` | TestFix, TestToolsRequire, TestVerifyGreenPass, TestVerifyModelAndMaintainedModels |
| `verify.Tools.Require` | TestToolsRequire |
| `verify.Verify` | TestVerifyExcludedIntents, TestVerifyFailedAcceptance, TestVerifyFailedCheck, TestVerifyFailedSetup, TestVerifyGreenPass, TestVerifyInputFields, TestVerifyIntactRefusal, TestVerifyModelAndMaintainedModels, TestVerifyMovedFiles, TestVerifyRepoDefault, TestVerifyStageGate |
