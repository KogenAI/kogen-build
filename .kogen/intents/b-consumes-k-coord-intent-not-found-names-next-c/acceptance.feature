Feature: b-consumes-k-coord-intent-not-found-names-next-c: kogen-build follows the missing-Intent refusal that names the slug and the next commands

  kogen-coordinator's k-coord-intent-not-found-names-next-command makes project.RequireIntentDir
  refuse a slug with no Intent folder as "intent/not_found: Intent does not exist: <slug> (looked
  in .kogen/intents/<slug>)" followed by a line naming the next commands and, when other Intents
  exist, a "did you mean" line. kogen-build pins that producer's main and its two tests that pin
  the refusal expect the new text only; production code and everything else stay as they are.

  Scenario: The producer's main is pinned and nothing else changes except the two tests
    Then b-consumes-k-coord-intent-not-found-names-next-c: the file "go.mod" has the SHA-256 "36465db4e55c9304d84a7c75c47409251d34c7b160ad50d45ac0d7bc88a3ca23"
    And b-consumes-k-coord-intent-not-found-names-next-c: the file "go.sum" has the SHA-256 "93b0d6ae6ad4d8d69b2d8b284f4876253fdfde717b19aff53936c2411a964590"
    And b-consumes-k-coord-intent-not-found-names-next-c: the build of the checkout uses kogen-coordinator "v0.0.0-20261107115359-b1f2988eb753" without a replacement
    And b-consumes-k-coord-intent-not-found-names-next-c: the files under "build,cmd,consumes,contract,docs" other than these have the digest "e8d9546a6e8bccfa12cd3550e6bb462024bdda94ef3251a332f71b97efead909":
      | file                                                   |
      | build/build_test.go                                    |
      | consumes/kogen-coordinator/project_intent_test.go      |
    And b-consumes-k-coord-intent-not-found-names-next-c: the file "README.md" has the SHA-256 "309962d582a8d52eb9a38d6dcc5aeeb2d6beaaa96d361d2347ec716111827ca9"

  Scenario: The two tests that pin the refusal pass against the pinned producer
    When b-consumes-k-coord-intent-not-found-names-next-c: I run the Go test "TestRequireIntentDir" of "./consumes/kogen-coordinator"
    Then b-consumes-k-coord-intent-not-found-names-next-c: that run passed and no test was skipped
    When b-consumes-k-coord-intent-not-found-names-next-c: I run the Go test "TestStartRefusals" of "./build"
    Then b-consumes-k-coord-intent-not-found-names-next-c: that run passed and no test was skipped

  Scenario: No test accepts the old text and both tests pin the new one
    Then b-consumes-k-coord-intent-not-found-names-next-c: no Go test file under "build,cmd,consumes,contract" states the refusal without the slug
    And b-consumes-k-coord-intent-not-found-names-next-c: the Go file "build/build_test.go" contains "looked in .kogen/intents/"
    And b-consumes-k-coord-intent-not-found-names-next-c: the Go file "build/build_test.go" contains "<request-file> creates one"
    And b-consumes-k-coord-intent-not-found-names-next-c: the Go file "consumes/kogen-coordinator/project_intent_test.go" contains "looked in .kogen/intents/other"
    And b-consumes-k-coord-intent-not-found-names-next-c: the Go file "consumes/kogen-coordinator/project_intent_test.go" contains "<request-file> creates one"
    And b-consumes-k-coord-intent-not-found-names-next-c: the Go file "consumes/kogen-coordinator/project_intent_test.go" contains "did you mean: "
