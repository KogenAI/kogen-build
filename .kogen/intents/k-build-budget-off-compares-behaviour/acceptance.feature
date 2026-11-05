Feature: The budget-off test compares what a Build does
  TestBuildRequestBudgetOff (build/build_request_budget_test.go) runs a budget-omitted Build and
  then a budget-off Build of the same scripted fixture. It compares only what a user or caller of
  Build observes: exit code, stop code and outcome; the Git facts (commits, parent, tree, message
  title, trailer, porcelain status); the builder and review request counts; the documented run-log
  event names in order; and the phase and reason each Build ended in. Neither Build's log.jsonl,
  checkpoint.json or result.json may carry a name that mentions the request budget.

  The harness runs the test from a test binary compiled in a scratch copy of this checkout. With an
  injection, it changes the second Build of the test process, the budget-off one (with "noise", both
  Builds get different noise).

  Background:
    Given k-build-budget-off-compares-behaviour: the test binary of package build is compiled with the injection harness in a scratch copy of this checkout

  Scenario: Two Builds of the unchanged fixture compare equal, and the test stays fast
    When k-build-budget-off-compares-behaviour: TestBuildRequestBudgetOff runs once without injection
    Then k-build-budget-off-compares-behaviour: TestBuildRequestBudgetOff passed within 40 seconds

  Scenario: Builds that differ only in timing values, host load figures, step durations and a toolchain probe's status pass
    When k-build-budget-off-compares-behaviour: TestBuildRequestBudgetOff runs once with injection "noise"
    Then k-build-budget-off-compares-behaviour: TestBuildRequestBudgetOff passed

  Scenario Outline: A request-budget name in the budget-off Build's <file> fails the test, naming the field and the file
    When k-build-budget-off-compares-behaviour: TestBuildRequestBudgetOff runs once with injection "<injection>"
    Then k-build-budget-off-compares-behaviour: TestBuildRequestBudgetOff failed, reporting '<report>'

    Examples:
      | injection            | file            | report                                                                                 |
      | budget-in-checkpoint | checkpoint.json | request-budget name "build_request_budget" in checkpoint.json of the budget-off Build |
      | budget-in-result     | result.json     | request-budget name "request_budget_left" in result.json of the budget-off Build      |
      | budget-in-log        | log.jsonl       | request-budget name "build_request_budget" in log.jsonl of the budget-off Build       |

  Scenario Outline: A budget-off Build that behaves differently fails the test, naming the observation (<injection>)
    When k-build-budget-off-compares-behaviour: TestBuildRequestBudgetOff runs once with injection "<injection>"
    Then k-build-budget-off-compares-behaviour: TestBuildRequestBudgetOff failed, reporting '<report>'

    Examples:
      | injection             | report                                                                              |
      | extra-builder-request | budget-off Build differs in builder requests: budget omitted 8, budget off 9        |
      | provider-refusal      | budget-off Build differs in stop code: budget omitted "", budget off "provider/refused" |
      | second-commit         | budget-off Build differs in commits: budget omitted 1, budget off 2                 |

  Scenario: No masking list or log-rewriting helper remains in package build's tests
    Then k-build-budget-off-compares-behaviour: package build's budget test file still defines TestBuildRequestBudgetOff
    And k-build-budget-off-compares-behaviour: no test file of package build contains any of:
      | volatileKeys                                  |
      | volatileKey(                                  |
      | observeBuild                                  |
      | budgetObservation                             |
      | <volatile>                                    |
      | TestVolatileKey                               |
      | TestTheGoldenMaskCatchesTimingAndHostLoadOnly |

  Scenario: No product code, timeout or limit changes
    # The SHA-256 values are those of the tree this Intent was shaped on (3dcdb86).
    Then k-build-budget-off-compares-behaviour: every Go file, go.mod and go.sum except build/build_request_budget_test.go has the SHA-256 of the shaped tree, and no other Go file exists:
      | 8e32458fa83a0754c12e6dfd978c51520202afd7d2f3d2b6be2bbe8e72871486 | build/baseline.go                                   |
      | 30291a24583d86f33582611b1aa13d67518e7d747f511aa121c5b832551452c7 | build/baseline_cache.go                             |
      | 24d8d1f1f6d1a6c62f8e40e399c7e9674ea9df42794d7bf6cf0745d3f6785358 | build/baseline_cache_test.go                        |
      | 816d0d8b59e9647eace3c6d40e44a3bfec94905e1b8a61cd4d149beb10317e9f | build/baseline_wall_test.go                         |
      | 5acdafdb071ddd6f8e65d69040ef7720f637e50686eaabfc6b042bf697ba527f | build/blocker_grounding.go                          |
      | e5251c427afd5553a2afac0d5f41fc77de86e9629fab9691b7ce2982b7a793ea | build/blocker_test.go                               |
      | 9624199392547ca787d7aa00675fb1d1ce7802f41f23cd96f896d805f5f447c0 | build/brief_caps_test.go                            |
      | b0e03abe243d30d13d0ff9ace64a28d7aebf3f7283123b77cadb021e565ba57d | build/build.go                                      |
      | d6530510dacfabc13db184456c5107e31d0a3fb2072a9c27d63d1f9d5ea15875 | build/build_test.go                                 |
      | ca19e3ab243c2fb923333dc2c49115913432ad81af36d2621f4b228ad4ee360e | build/builder.go                                    |
      | 28e05d3c9360ac47f4660f1186234e47416774bee947ee6dc4fe14c1856d5b6e | build/cache_id_test.go                              |
      | 4b1a31dd4bb8c69287401f507be40395756173c96f44144bedc0c695d70d744c | build/checkreuse.go                                 |
      | 528c4d4240aa75a03ab3051e7e73bb5d98bece182a75043fb054d12ae2da0ed5 | build/checkreuse_test.go                            |
      | f10d79bb31a48d345c523f8f694eefe7793bfc69a3f3540596e0952f506b1902 | build/commit.go                                     |
      | 2ba1f0cd8eb7a838de0a54d7bff1f2e5eda7e909a72b69c84250df93b71afba1 | build/commit_failed_resume_test.go                  |
      | e5886a9a6aa149b61d07db4bd898c20da6ab8034af328b132d617dfef5a84af6 | build/commit_test.go                                |
      | a0ccdc693886dbd9faf97254612d1a90142945757bb6bb5ce31550bf6fed4d40 | build/context.go                                    |
      | 200a2433a855cc7f27a9689d8d7bc4e5560ffcbf21b93da22d7b8a2c3cf839e0 | build/context_external_test.go                      |
      | 99e51ccfa6cc5c19871ce519a9bf4eb49d997f7dafaebf2b30aead8c00abafbf | build/context_test.go                               |
      | 261ed77ef872cfa270bbcac4b753aa51ecb368dea593cbe475f9b4855bd07194 | build/core_names.go                                 |
      | a2d8fdce27d2c105316539d73c2a354b8aeb85d63ca4fd9912d686cf14c64394 | build/core_run.go                                   |
      | 979df85968c7700ecfc51a3dc7c220540ba561e5a602515a9f505112d7c0a3c5 | build/core_stages.go                                |
      | f593b0db10bf564c4964ccec65761635e8b703ce610bd8e0f181675d91a0c4d4 | build/core_state.go                                 |
      | 68e4a2253d514f06a6a47c3d4e3c64c0803754cbee17616def5162276caa33ac | build/env_failure_park_test.go                      |
      | 4c8ee41460a495ab0cac2165b7bb022f26c27f849fc4f90adc90786e86ba978d | build/envfailure.go                                 |
      | 9111ee1d01844be418e1de6f32acbdcd5035bc76603b7931f62adb66b6b80104 | build/executor_external.go                          |
      | 399370cdf6bcc36d9591083163297a0cf8bafb056a7c23095f091ccb7061567a | build/executor_native.go                            |
      | 8c92ae6b9f1517de33fd2f25082b948ad2c6ba5ddac5ba5bfd14cddc471b8086 | build/executor_native_test.go                       |
      | 849cbe1c70bd8b6f9ac185059559b46d3a5856616ec8f2cc812c3891c026935c | build/executor_test.go                              |
      | b37d2253e547abd0b6c6aea0f9e53d7a556e5e6afb618cc0487df51b722551cc | build/facts.go                                      |
      | 74302f5ec6a5035a37b4174fa0d15f07b8fce146d0b404fdf81e7320b19e2f0c | build/facts_test.go                                 |
      | 93e3b23a2fa1d600e93305372bc0527f22fea526a6cc9618b2064b352d11f8b6 | build/failing_check_test.go                         |
      | b2ddf8108e435f14a0cdfdef334b67b0f015d3b87bc45008458542392b7115ad | build/golden_test.go                                |
      | 89975adf84723bd61dc3042210c66b803c2474c62e142bc3a702e1e23e072b8b | build/golden_update_test.go                         |
      | 37975bf1bdb269b0b533e11e963dd76ea2c62501ac78c0bdc1a95be7caaf3ef6 | build/helper_report_test.go                         |
      | 4ac594b50d9f41b9f8b37f76faae7175bb3256144d8350cbc56613e49100dd87 | build/helper_test.go                                |
      | b4fd5807f47348fdf13c47136d2ede7c8c98af4ba1549fda21af0954b2923da5 | build/hooks.go                                      |
      | 7fe617432bc364458067a2d9df82b317ff4d9de502da198c5341db0b78299866 | build/hooks_test.go                                 |
      | 78ddcc952c321ad48e4f63bfe54a6ec0bc0f05c3cd2ca842dd237a51c7e980e0 | build/host.go                                       |
      | 1e837c1b290f131204a12ae8f2a7ec8e32f1d4d3ded5890ca905c897480736d2 | build/k15_anthropic_validator_test.go               |
      | 424196703c19d88c2fdc1a4611d3c96977f0e6ce3f918d57ebab4a3112a877a5 | build/k18_binding_test.go                           |
      | 9ee12af0a86b9df03a15346959c7bbc3c643a62313b9699c61a63fcf6d6fb73c | build/k19_no_progress_test.go                       |
      | 044c6d0698f070ce4d2148c692afeb25744997e5c83751484c73afbce223121d | build/k_review_invalid_resume_test.go               |
      | 1fb405e6a586ace80626e9bc1e6c44e442ae1a93c7e28b1fe13aa5b8d677aac4 | build/k_review_verdict_strand_test.go               |
      | 659c6f90e745ceec7976592b112efba2130a70b3aa638f1fcecff1e70cb5d6fb | build/k_reviewretry2_test.go                        |
      | 165350fda64e007a3a30d25c767bea37b50694376a319d35b318b69a318f1ad3 | build/k_reviewretry_cli_test.go                     |
      | 7fbb3588284ab39935370518f4d577ac5e09ae2b72ce715c3a2f857151f2c1cc | build/k_reviewretry_decrel_test.go                  |
      | 6e98872c35ba107c33e0f18089f10a3162e11e52a1853f665997337b87a9dfa6 | build/k_reviewretry_safety_test.go                  |
      | 201a6a9a5d0b1acd8ab5bd55d2f69ba44c7dd97103a5b0767b8006c1b1302eec | build/k_reviewretry_test.go                         |
      | 9e155b7d02840394535f07c85583598a754a928987d447c42965bb339ac36ac2 | build/k_reviewretry_valid_test.go                   |
      | 74757c79bbb41c65ee236cfc70fb56c2e8dce8676ba68a6002fb42702c2a7d5f | build/k_revod_gate_parts_test.go                    |
      | d6674232ce2316debea1258ef098cc08acdb6f701faef28ab3035ba456c1fbe4 | build/k_revod_gate_test.go                          |
      | d4fe2ad7ff446ae3a3943a59b465f90a8daac7b67e72ae7a7c9d78b1fe870bda | build/krevodb_test.go                               |
      | 8158a41222f16500eaad4cb7b17cd74b27b0eacd35d547309d9947faf77b0e80 | build/legacy.go                                     |
      | d11d844fb7b2e2ec346147832168a4849785514fde29cd3a6475e63a73c65387 | build/legacy76_test.go                              |
      | 42715e8bdc1377012a961f8dd4fde2bc338f22c9f2cacea889992fd14234de55 | build/legacy_test.go                                |
      | 8def3103c092b9701b8c93d603786db992229cd3b61fdffae5008691a40349e6 | build/lineup_layout_test.go                         |
      | 63d60727464da78050bc5493ff94fde4017eed1a39f431fc618781c7690e3349 | build/lineup_test.go                                |
      | 4d684e5d7367a5525ed1f11d38907ada13b99f4fa4171f11cf9dd0cb013e44f2 | build/loop_knobs_test.go                            |
      | 376c1c911aa43425e022243c9dd9b93b51723ac59ed853f78a0d8796812c4e51 | build/loop_ownership_test.go                        |
      | 7ad96c0c1f497bece6ef1850d57741b64b9aa154e320b4ebd165ecc955829504 | build/main_test.go                                  |
      | 0cc0c3c3e1400481118513c99e74d6e636a215ebe9c1be286267e15aee1135f1 | build/maintained.go                                 |
      | 2682e793c9dadd52c747d5065e92387ab309afed87ac3e85f425f14158075bd1 | build/maintained_test.go                            |
      | fbc875dd5dc5a55f25cf7538e9167ca1b9a5d1d5d5baf9cded478256367aca2d | build/material.go                                   |
      | f963608f74142600f632b129552838c49c5f2058539605d7ceed6ebc19114211 | build/material_test.go                              |
      | 8bc814b27c318481223288ccd8f4de2b2a7a81f70eaff63d6d436f3799a92b6c | build/named_feedback_test.go                        |
      | 7892c89c0b704adcee7ac4c163b3110f273448b5df7db7bc17598ce5305381fb | build/native_default_test.go                        |
      | dd5d748aee3a45682803005d199e66430651af84b9cc9391808244a3e0914272 | build/noprogress.go                                 |
      | 143fa705676e3904f6d3099103a3e3c084d906f34b0afd696c0fec252fff275d | build/noprogress_test.go                            |
      | ffae1ab3d93017bf3115afddab7d864dfd65544721e20ab56b2cb82e8d68fca0 | build/policy.go                                     |
      | 34f5c5d48c207a589626aa6f5cb929860bc7c35c23ebb0e025bb24c0f6def405 | build/policy_test.go                                |
      | c2801134a746760e4cc06ae1acdbefce91b5f78d9433f07dcf5a3a061bb1fa66 | build/prompt_text.go                                |
      | 59245fd7d88ae84b896119061f4995d35f4f9e4b219b590dc8216a35a4f6b389 | build/read_ledger_test.go                           |
      | ab701b2fa300b0483e8bbf775a79315aaa56726383a3db28e3bb63a41abc2994 | build/readledger.go                                 |
      | ac525784d61602c26acf2062cf9528564d3517123c3eb84326ab2a5ff775b267 | build/records.go                                    |
      | 67d758683dad0fa1f6fc24df2b1a1b1ebc752c51623d9ddda7fa9289f26eddaf | build/records_test.go                               |
      | ddf3bf6ddfb5bc833040007ab9c0028dc4f2686768d43e7ff46f8157a809d2f6 | build/recover.go                                    |
      | e32b28f096a2e69c09d1b0417e758dd7616637f0138e67ff58813b3737f96e53 | build/recover_test.go                               |
      | 4e0e47e5c73b6fcd6c26518e92ab3a10a6c63c6b85228be8737d10aa72c12365 | build/response_budget_test.go                       |
      | ac041e7fe6706365438f60f20e14dd2566a4e11c390526ad9477538238311327 | build/resume.go                                     |
      | 5c3108df42595614650274c5400063289ec6fe85d142fea21527962a60a9e99f | build/resume_index_test.go                          |
      | 39d53df33888c40d9969762efbedf48b48fc337ada6371537907c6007de8ce33 | build/resume_test.go                                |
      | 3ae1c35a981e06041a13166b2b93c0211762538df5533371a7eaea403856f801 | build/review_batch_test.go                          |
      | f7a5c95d3f12f7466e9dd4a030f6ff00eafe81e8c48299d49717d54b293809a9 | build/review_budget_test.go                         |
      | 01527b7a175044e659e31703ca1d26186979564114d041671e38a922941e3cce | build/review_delivery_test.go                       |
      | f1a3b8769fec6f273fef529d593c8ce2d096139f9a6fc69c53deb79290fe3ff5 | build/review_lineup_test.go                         |
      | df57c8ee981185b01087aaa67b9640cee0e13a1de7ab7c3097bb81770b27b622 | build/review_parts_build_test.go                    |
      | edce22a08d57f262e252ea1a525554ca0799016d3afd57174ebd1f43bebe2a98 | build/review_parts_source_test.go                   |
      | 2e1afdf8cffcdd6db4d229c14815f0d9fb8708f5136b57671f4b887da55d0316 | build/review_prefill_test.go                        |
      | 270a41d238bb4bbc1cb990a7fd1ee60c64135f3dd95af606b048818989367878 | build/review_resume_external_test.go                |
      | 23b0535f0e0d9b8c1a0f1521c7b1d74fb2abcd0423485686c364016a63cc1f08 | build/review_retry.go                               |
      | da34154efae3062504208abaccb3524402f0a6858bbc468e4d587051f8b89ad5 | build/review_test.go                                |
      | 46a3f7f76c39aca551be39120ecd3d980805fd534e7412ef8011039335dbe4eb | build/route.go                                      |
      | 6938617863057287aaadd0cae12ddf2d07fe610e0af74e7210243053fd755ee5 | build/run.go                                        |
      | f88e62b737aae40e2a9464c9e682be624767876422c2e61073398807fd3a65e2 | build/same_config_test.go                           |
      | 9ac33f05ff34404fd81a96c1a9cad7685b85447d86978b9768fc90f29c37d262 | build/silent_shell_test.go                          |
      | 8b2dc756f1b49ce85620dbe7e1e9272cabd57fd9358386f24ccf179287515e7b | build/stages.go                                     |
      | df6fddc1644ff6b51ecce9a22e6755e36c228c3f1ed7778acf85e9c6301c989a | build/stages_plan_test.go                           |
      | 3177eb48b5f6c55a96e5591ec1c3978b248fff062275bb22a09beeff1d0fe1c3 | build/stages_test.go                                |
      | 4a5e5c5e5f23e46be982cbd5a340f99ef2aa92cbfee009a6414b485c09b45d13 | build/standalone_test.go                            |
      | b4f61c071f9a5fd57518cf957ac29bac390f0047689396303abfcfebd4a6ba72 | build/state.go                                      |
      | 372c4fb0ad8f95cd864c1a99d20aea5d7c1efeda69953ff8a5e50f16c79dcb42 | build/state_golden_test.go                          |
      | 4e017b1b092c22bbfbc877b7d4747f2f078818b98d620d7da2ca6598c0bd5243 | build/stop.go                                       |
      | 05ef7895dcd847bc18543bb3badd24fb86ecd4c1eed87254c690c4ec46a880ed | build/unit_test.go                                  |
      | c13905e1eff7b6010c1964396d7c8e0cdac28164f648765aba8fc8d0496eeae0 | build/unmoved.go                                    |
      | 563f049ace9413cbbb0d5eea1aa39b0979fbe4a95cfbf6218ab278d290f23d2a | build/unmoved_test.go                               |
      | 4917a1311f2f9f1c35e754e20ef675a7cbc94facf38fe8e7986cec4df196bb62 | build/usage_limit_resume_test.go                    |
      | ee27dad91c9eeb08dfef7cb216b3a5350d1d94e09ed72062165fd055014fb110 | build/util.go                                       |
      | de027df6e23523203b4538e8ec777629837183ff9c7cd4a45da4a158f9cafe42 | cmd/kogen-build/main.go                             |
      | d5eca02d5767c01e2cfbc71c09d135f4ef49acea0b749d3a9d00dabb809e7fdc | consumes/kogen-coordinator/durable_test.go          |
      | 0619f2642a52fa38f790c55022efcbe04903b9acba8577ac6d5583d99d12fe9f | consumes/kogen-coordinator/git_commit_test.go       |
      | a19800935af1a41660521de1cb40fb885eb15bcb7ea6e851c1f7bd361ed5e5d9 | consumes/kogen-coordinator/git_repo_test.go         |
      | 7adb372832324ad4de5d46c394baef4c0a130065892771df34350a97b2e8f001 | consumes/kogen-coordinator/helpers_test.go          |
      | 3b544d4f1ce89ea1f04abda6e1f112872b37bb04cef5d4bd0e23cf2c286f028b | consumes/kogen-coordinator/main_test.go             |
      | 11d25136b9ca2e1536ce586c68a390c82018625b1456f76b572c3b65fd6c28af | consumes/kogen-coordinator/project_approval_test.go |
      | 88d846cf7bbefc131c04ab7035cdfcc813a35098de44a5b9c86fd5c73a744e6c | consumes/kogen-coordinator/project_config_test.go   |
      | b3a30c3b4b71c216945ed18519021fae60df734a1574fa3bad96b4421c510e21 | consumes/kogen-coordinator/project_facts_test.go    |
      | 944ae1b89349014df86ed2142f72fa55b5c8fa173d8b47343585cc22acaf5c3d | consumes/kogen-coordinator/project_fixture_test.go  |
      | a270bca8832a9653747604ade66dd4eaa3adac5e91c4f53a212aa7382fb43d3c | consumes/kogen-coordinator/project_intent_test.go   |
      | e30af1b39dd95d27ff69170c6706bc6e7bd7bae1713ac95938cd0a6baa74fd11 | consumes/kogen-coordinator/route_test.go            |
      | b5509b19f8fe07e75cc71201c1aeb70362862fca11b69ee972f7d80744ec7f7e | consumes/kogen-executor/agent_test.go               |
      | e7bbb4dd1793c496bfbb517221a7e75e1a7999e2d86ac9325354a0517769ad69 | consumes/kogen-executor/core_test.go                |
      | 750462baa4e622fb012f1fd3970fe869ab494dee7207562a394cbe15df4a8469 | consumes/kogen-executor/executor_test.go            |
      | 527714c39dedbe1df1664bc6e812592a755c07550f95c370665df2ee60c7b858 | consumes/kogen-executor/main_test.go                |
      | 16f517bda73c6cefc47956233f0a55ec20efe3d7e6f91a5c608cf619083d396f | consumes/kogen-executor/thinking_test.go            |
      | a3f9e8de89c645facd409a8806e23db5386fb6b74677cf68ece44679a8eaa03f | consumes/kogen-loop/loop_test.go                    |
      | 8db8491a98b9a0ca590c1052427cb740c1375a79ac2b3044a7d2e9f458df34ba | consumes/kogen-loop/main_test.go                    |
      | 34666583d369cb64334c8b6fdeebc9776efac898e56ae966b93996d15f194a95 | consumes/kogen-loop/provider_test.go                |
      | 82a5aa56984d3a2b3a3d7ffc297a21477c72021300d4bfbf2b733bcad8f0e0cc | consumes/kogen-loop/sys_test.go                     |
      | e3d969b7f57aae23c3e4478a6b91cdcf07912d0ab0c2d8a74cc4b7a9b612166a | consumes/kogen-loop/tools_test.go                   |
      | b2e8f6fa2e55dbe196cbc020e588381606019ee8d58c1753d341026f7a085e5e | consumes/kogen-review/api_test.go                   |
      | bcb5606ad44800e13fb19857f1159e372779039a94b5068b055bb8817665547c | consumes/kogen-review/env_test.go                   |
      | 56984dd983e3930c0ae87395669e49cb5657cf307a3705be2046e0db11206fa2 | consumes/kogen-review/external_test.go              |
      | b6df651159233c3aac038d87d3e8dce2b9ed855f3dcf83b2db2cd9146248e05d | consumes/kogen-review/helper_test.go                |
      | 722c06e2c093dadf039a500164ce3828d1755cc480c4dd3aff77b6416b44227d | consumes/kogen-review/main_test.go                  |
      | 37f99193deef9923b815c6342e93d8ca0987139999c8d59ee90d03172aa8e51a | consumes/kogen-review/material_test.go              |
      | fec19e9412f4b938d9e0d44db6cada5251947192f8d9818111f7aab74d3d5e62 | consumes/kogen-review/native_test.go                |
      | b04466735ee1391a8ff7fc8d2b8e75b182947719a8f84a3333ef817004bf8160 | consumes/kogen-review/session_test.go               |
      | 0613e7c762dad3c2e92429055b4ee358cd49a499eccbea974d177f8a8b35ce96 | consumes/kogen-verification/helper_test.go          |
      | dbf64ab0d611b7ef1db379c6d9103ecfcc328a6ebb584248e84556ac548069a9 | consumes/kogen-verification/main_test.go            |
      | 1b3aa456cd2b87e0edfbc898e3b0cb2584e87b177d58cbe878e1c24e48e407ac | consumes/kogen-verification/pass_test.go            |
      | e1a7c75e957c848d4eec4883df15ba7b45bff239022e06de222f9ed5ae678054 | consumes/kogen-verification/text_test.go            |
      | 236b3b5d646d266d22713ceecfbb0efb8edfd66a9e5997253b2d32f277665347 | consumes/kogen-verification/tools_test.go           |
      | 40a1f9c9bf7156f777da2273f8ad4979d6fd70491cd619f825f019689fed1e61 | consumes/kogen-verification/verify_test.go          |
      | 277cfd7d7f310ddf5ccb99f2bda75c5af543656ae0111099ec16c0ee685a1fb5 | contract/consumer_test.go                           |
      | 93393d79e3fe33492e1486290fe4cb144b42dac6b81058301ef9b0372f177795 | contract/fixture_test.go                            |
      | eb578e2b6605bcd5d3ec7cefe1a5b16a790a3c99f372832c4c23b958a5cadca7 | contract/report_test.go                             |
      | 17be3349ce3c08fa42ae35d6d65fcfd3c0b72bbaf6efcb196c3e7481c940d93a | contract/response_budget_test.go                    |
      | bfcfa998e3171f3fcb64eda9257d1f1950e38b356152df6ef3602f492b363934 | go.mod                                              |
      | 44690a0e668ede21a20209c6a69c27de2b4771bff8e1e0dbdf0a47faccbe8ace | go.sum                                              |
    And k-build-budget-off-compares-behaviour: these definitions in build/build_request_budget_test.go have the SHA-256 they have in the shaped tree:
      | redTry                             | 586d700e83f01bc0190b5d61eab67684a9e658c10e89f4d93bd72d30c327389b |
      | redContent                         | 1b34420316f85808d712a4068c5d6ff65e7aee1f2ecd24fb276a7f923a6b3ea4 |
      | budgetOf                           | ea136c71c2057e8f24fb1ca530d702c9abde08b8560143953b6af7959c442f23 |
      | budgetEvents                       | a363ab270f6369f2f561bc7dab5b5e5f13430c0b58090e88c8b1c6eca706111d |
      | TestBuildRequestBudgetAcrossPasses | d2e2eca7405f7139712cb955256571d90f009fe2320d54da12ab1145543386b9 |
      | TestBuildRequestBudgetResume       | 3ef7e391e52312a1dcf8a17ce3ee138566ffe478889f2e0655b3913ace40c5cd |
      | TestBuildRequestBudgetReview       | fbe0b3fb19f3e11d4a8b8bb61630197a02ea8caf78456ea4d051fbb16897cd43 |
