Feature: kogen-build states exactly what it relies on from each Kogen dependency
  kogen-build keeps contract/ (what it provides) and adds one consumes/<producer>/ suite per Kogen
  dependency: kogen-coordinator, kogen-executor, kogen-loop, kogen-review and kogen-verification.
  Each suite is a Go package of test files written only against the producer's exported API.
  Every use of the producer in kogen-build's own non-test code (build and cmd/kogen-build) is
  named by a test of the suite and has a row of the consumes/README.md inventory naming a test
  whose body uses it; the inventory lists nothing else. The suites run in go test ./..., isolated
  from the caller's Git, HOME, network proxies and module proxy, at the versions go.mod pins now.

  Scenario: Each Kogen dependency's consumes suite covers exactly the uses of it in kogen-build's own code
    When k-consumes-build: the consumes observer judges this checkout against these suites:
      | suite              | module                                 |
      | kogen-coordinator  | github.com/KogenAI/kogen-coordinator   |
      | kogen-executor     | github.com/KogenAI/kogen-executor      |
      | kogen-loop         | kogen-loop                             |
      | kogen-review       | github.com/KogenAI/kogen-review        |
      | kogen-verification | github.com/KogenAI/kogen-verification  |
    Then k-consumes-build: the observer finds no problem
    And k-consumes-build: these paths are as committed at HEAD:
      | build    |
      | cmd      |
      | contract |
      | go.mod   |
      | go.sum   |
    And k-consumes-build: the only main package outside "features" is "github.com/KogenAI/kogen-build/cmd/kogen-build"

  Scenario: The consumes suites are part of the normal test command and pass at the pinned versions
    When k-consumes-build: "go test -count=1 -trimpath ./consumes/..." runs in this checkout
    Then k-consumes-build: go test passed and ran tests in each of these packages:
      | github.com/KogenAI/kogen-build/consumes/kogen-coordinator  |
      | github.com/KogenAI/kogen-build/consumes/kogen-executor     |
      | github.com/KogenAI/kogen-build/consumes/kogen-loop         |
      | github.com/KogenAI/kogen-build/consumes/kogen-review       |
      | github.com/KogenAI/kogen-build/consumes/kogen-verification |
    And k-consumes-build: "go list ./..." lists each of these packages:
      | github.com/KogenAI/kogen-build/consumes/kogen-coordinator  |
      | github.com/KogenAI/kogen-build/consumes/kogen-executor     |
      | github.com/KogenAI/kogen-build/consumes/kogen-loop         |
      | github.com/KogenAI/kogen-build/consumes/kogen-review       |
      | github.com/KogenAI/kogen-build/consumes/kogen-verification |
      | github.com/KogenAI/kogen-build/contract                    |

  Scenario: The consumes suites pass isolated from the caller's Git, HOME, network and module proxy, under the race detector in shuffled order
    When k-consumes-build: "go test -race -count=1 -shuffle=on ./consumes/..." runs in this checkout with a bogus GIT_DIR, a fresh HOME that signs Git commits, refusing HTTP proxies and no module proxy
    Then k-consumes-build: go test passed and ran tests in each of these packages:
      | github.com/KogenAI/kogen-build/consumes/kogen-coordinator  |
      | github.com/KogenAI/kogen-build/consumes/kogen-executor     |
      | github.com/KogenAI/kogen-build/consumes/kogen-loop         |
      | github.com/KogenAI/kogen-build/consumes/kogen-review       |
      | github.com/KogenAI/kogen-build/consumes/kogen-verification |
