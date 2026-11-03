Feature: Golden tests pin the bytes of persisted Build state

  Background:
    Given the state goldens of this checkout are:
      | build/testdata/state/fresh/checkpoint.json          |
      | build/testdata/state/attempts/checkpoint.json       |
      | build/testdata/state/review-approve/checkpoint.json |
      | build/testdata/state/review-approve/review-1.json   |
      | build/testdata/state/review-changes/checkpoint.json |
      | build/testdata/state/review-changes/review-1.json   |
      | build/testdata/state/resumed/checkpoint.json        |

  Scenario: The goldens hold the persisted bytes of a fresh run, a run with several attempts, reviewed runs and a resumed run
    Then each state golden is one line of JSON that ends with a newline
    And each checkpoint golden has the top-level keys "version, identity, candidate, index, approval, state, recovery_raw"
    And the state of each checkpoint golden starts with the keys:
      | root                |
      | slug                |
      | config              |
      | intent_text         |
      | start               |
      | baseline            |
      | baseline_digests    |
      | excluded            |
      | run                 |
      | git_index           |
      | start_tree          |
      | phase               |
      | t0                  |
      | usage               |
      | round               |
      | pass                |
      | review              |
      | repairs             |
      | last_pass           |
      | last_review         |
      | blocker             |
      | continuation        |
      | approval_bytes      |
      | parent              |
      | root_run            |
      | model_active_ms     |
      | unknown_requests    |
      | attempt_usage       |
      | working_context     |
      | source_provenance   |
      | prompt_cache_states |
    And each review record golden has the keys "attempts, effort, findings, model, tree, usage, verdict"
    And the state goldens hold these values:
      | golden                                              | pointer                                     | value      |
      | build/testdata/state/fresh/checkpoint.json          | /version                                    | 4          |
      | build/testdata/state/fresh/checkpoint.json          | /recovery_raw                               | null       |
      | build/testdata/state/fresh/checkpoint.json          | /state/phase                                | "building" |
      | build/testdata/state/fresh/checkpoint.json          | /state/round                                | 1          |
      | build/testdata/state/fresh/checkpoint.json          | /state/pass                                 | 0          |
      | build/testdata/state/fresh/checkpoint.json          | /state/review                               | 0          |
      | build/testdata/state/fresh/checkpoint.json          | /state/last_pass                            | null       |
      | build/testdata/state/fresh/checkpoint.json          | /state/last_review                          | null       |
      | build/testdata/state/fresh/checkpoint.json          | /state/parent                               | null       |
      | build/testdata/state/fresh/checkpoint.json          | /state/root_run                             | null       |
      | build/testdata/state/fresh/checkpoint.json          | /state/continuation/stage                   | "builder"  |
      | build/testdata/state/attempts/checkpoint.json       | /version                                    | 4          |
      | build/testdata/state/attempts/checkpoint.json       | /state/phase                                | "building" |
      | build/testdata/state/attempts/checkpoint.json       | /state/round                                | 3          |
      | build/testdata/state/attempts/checkpoint.json       | /state/pass                                 | 2          |
      | build/testdata/state/attempts/checkpoint.json       | /state/last_pass/n                          | 2          |
      | build/testdata/state/attempts/checkpoint.json       | /state/last_pass/green                      | false      |
      | build/testdata/state/attempts/checkpoint.json       | /state/unknown_requests                     | 1          |
      | build/testdata/state/attempts/checkpoint.json       | /state/usage/reasoning                      | null       |
      | build/testdata/state/attempts/checkpoint.json       | /state/parent                               | null       |
      | build/testdata/state/attempts/checkpoint.json       | /state/root_run                             | null       |
      | build/testdata/state/attempts/checkpoint.json       | /state/continuation/stage                   | "builder"  |
      | build/testdata/state/review-approve/checkpoint.json | /version                                    | 4          |
      | build/testdata/state/review-approve/checkpoint.json | /state/review                               | 1          |
      | build/testdata/state/review-approve/checkpoint.json | /state/last_pass/green                      | true       |
      | build/testdata/state/review-approve/checkpoint.json | /state/last_review/n                        | 1          |
      | build/testdata/state/review-approve/checkpoint.json | /state/last_review/verdict                  | "approve"  |
      | build/testdata/state/review-approve/checkpoint.json | /state/continuation/stage                   | "review_verify" |
      | build/testdata/state/review-approve/checkpoint.json | /state/continuation/review/verdict/verdict  | "approve"  |
      | build/testdata/state/review-approve/checkpoint.json | /state/continuation/review/attempts/0/verdict | "approve" |
      | build/testdata/state/review-changes/checkpoint.json | /version                                    | 4          |
      | build/testdata/state/review-changes/checkpoint.json | /state/review                               | 1          |
      | build/testdata/state/review-changes/checkpoint.json | /state/last_pass/green                      | true       |
      | build/testdata/state/review-changes/checkpoint.json | /state/last_review/n                        | 1          |
      | build/testdata/state/review-changes/checkpoint.json | /state/last_review/verdict                  | "changes"  |
      | build/testdata/state/review-changes/checkpoint.json | /state/last_review/findings/0/severity      | "blocking" |
      | build/testdata/state/review-changes/checkpoint.json | /state/continuation/stage                   | "review_verify" |
      | build/testdata/state/review-changes/checkpoint.json | /state/continuation/review/verdict/verdict  | "changes"  |
      | build/testdata/state/review-changes/checkpoint.json | /state/continuation/review/attempts/0/verdict | "changes" |
      | build/testdata/state/review-approve/review-1.json   | /verdict                                    | "approve"  |
      | build/testdata/state/review-approve/review-1.json   | /attempts/0/verdict                         | "approve"  |
      | build/testdata/state/review-changes/review-1.json   | /verdict                                    | "changes"  |
      | build/testdata/state/review-changes/review-1.json   | /findings/0/severity                        | "blocking" |
      | build/testdata/state/review-changes/review-1.json   | /attempts/0/verdict                         | "changes"  |
      | build/testdata/state/resumed/checkpoint.json        | /state/attempt_usage                        | {"input":0,"cached":0,"output":0,"reasoning":0} |
    And in each review checkpoint golden the last pass, the last review, the reviewer session and its only attempt name the same tree
    And each review record golden has one attempt, and its tree and the attempt's tree are the same 40-hex Git tree id
    And "build/testdata/state/resumed/checkpoint.json" is "build/testdata/state/attempts/checkpoint.json" resumed: the same envelope, and the same state except "run, parent, root_run, t0, attempt_usage"
    And the parent and the root run of "build/testdata/state/resumed/checkpoint.json" are the run id of "build/testdata/state/attempts/checkpoint.json", and its own run id differs

  Scenario: The tests compare the goldens byte for byte, and only -update rewrites them
    Given a scratch copy of this checkout for the state goldens
    And in the copy one space is inserted after the first "{" of each state golden
    When the new tests of the build package run in the copy with "go test ./build -count=1 -v"
    Then the run fails, its output names each state golden, and no state golden in the copy changed
    When the new tests of the build package run in the copy with "go test ./build -count=1 -v -update"
    Then the run exits 0, no test fails, and every state golden in the copy has the checkout's bytes again

  Scenario: A change to the checkpoint format of the Build state fails the checkpoint goldens
    Given a scratch copy of this checkout for the state goldens
    And in the copy's "build/state.go" 'json:"round"' is replaced by 'json:"rounds"'
    When the new tests of the build package run in the copy with "go test ./build -count=1 -v"
    Then the run fails and its output names:
      | build/testdata/state/fresh/checkpoint.json          |
      | build/testdata/state/attempts/checkpoint.json       |
      | build/testdata/state/review-approve/checkpoint.json |
      | build/testdata/state/review-changes/checkpoint.json |
      | build/testdata/state/resumed/checkpoint.json        |

  Scenario: A change to the review code's persisted format fails the review goldens
    Given a scratch copy of this checkout for the state goldens
    And in the copy's "build/review.go" '"effort": r.sess.Role.Effort' is replaced by '"reasoning_effort": r.sess.Role.Effort'
    And in the copy's "build/review.go" 'json:"attempts"' is replaced by 'json:"verdicts"'
    When the new tests of the build package run in the copy with "go test ./build -count=1 -v"
    Then the run fails and its output names:
      | build/testdata/state/review-approve/checkpoint.json |
      | build/testdata/state/review-approve/review-1.json   |
      | build/testdata/state/review-changes/checkpoint.json |
      | build/testdata/state/review-changes/review-1.json   |

  Scenario: No production code changes
    Then no Go file outside tests, go.mod or go.sum differs from HEAD in this checkout
