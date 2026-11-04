Feature: k-build-accept-canonical-loop-path: Accept the canonical kogen-loop module path

  Scenario: k-build-accept-canonical-loop-path: go.mod requires kogen-loop only by its canonical path
    When k-build-accept-canonical-loop-path: I read the module requirements of go.mod
    Then k-build-accept-canonical-loop-path: go.mod requires exactly these modules, none indirect:
      | path                                  | version                            |
      | github.com/KogenAI/kogen-coordinator  | v0.0.0-20261104083402-f4bd4aebd8c2 |
      | github.com/KogenAI/kogen-executor     | v0.0.0-20261104084012-b2ffb64382e8 |
      | github.com/KogenAI/kogen-loop         | v0.0.0-20261104063532-dea3eb375dba |
      | github.com/KogenAI/kogen-review       | v0.0.0-20261104090930-ba84b86318eb |
      | github.com/KogenAI/kogen-verification | v0.0.0-20261104143230-7ecd05abea32 |
    And k-build-accept-canonical-loop-path: go.mod has no replace directive

  Scenario: k-build-accept-canonical-loop-path: No package or Go file uses the bare kogen-loop path
    When k-build-accept-canonical-loop-path: I list every package kogen-build and its tests build from
    Then k-build-accept-canonical-loop-path: no listed package is "kogen-loop" or below it
    And k-build-accept-canonical-loop-path: the listed packages include "github.com/KogenAI/kogen-loop/sys"
    And k-build-accept-canonical-loop-path: no Go file of the checkout imports "kogen-loop" or a package below it

  Scenario: k-build-accept-canonical-loop-path: kogen-build compiles with kogen-verification 7ecd05a
    When k-build-accept-canonical-loop-path: I build every package with "github.com/KogenAI/kogen-verification" required at "v0.0.0-20261104143230-7ecd05abea32"
    Then k-build-accept-canonical-loop-path: the build succeeds without a "cannot use" diagnostic

  Scenario: k-build-accept-canonical-loop-path: The consumes inventory names the canonical kogen-loop module
    When k-build-accept-canonical-loop-path: I read "consumes/README.md"
    Then k-build-accept-canonical-loop-path: its "`consumes/kogen-loop/`" row names the producer module "`github.com/KogenAI/kogen-loop`"
