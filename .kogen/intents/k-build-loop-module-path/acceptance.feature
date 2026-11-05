Feature: k-build-loop-module-path: Require every Kogen component at its current main

  Scenario: k-build-loop-module-path: go.mod requires every Kogen component at its current main and no bare kogen-loop
    When k-build-loop-module-path: I read the module requirements of go.mod
    Then k-build-loop-module-path: go.mod has no replace directive
    And k-build-loop-module-path: go.mod does not require the module "kogen-loop"
    And k-build-loop-module-path: go.mod requires exactly these modules, none indirect:
      | path                                  | version                            |
      | github.com/KogenAI/kogen-coordinator  | v0.0.0-20261105001101-4116df40cbf1 |
      | github.com/KogenAI/kogen-executor     | v0.0.0-20261104160936-8a4fecd1ce0a |
      | github.com/KogenAI/kogen-loop         | v0.0.0-20261104223449-7e80986bcd04 |
      | github.com/KogenAI/kogen-review       | v0.0.0-20261104231132-7f6ef7d1c0a3 |
      | github.com/KogenAI/kogen-verification | v0.0.0-20261104204404-74798e7aa09e |

  Scenario: k-build-loop-module-path: No Go file, package or build-list module uses the bare kogen-loop path
    When k-build-loop-module-path: I grep every Go file for an import path starting with "kogen-loop/"
    Then k-build-loop-module-path: grep finds nothing
    When k-build-loop-module-path: I list every package kogen-build and its tests build from
    Then k-build-loop-module-path: no listed package is "kogen-loop" or below it
    And k-build-loop-module-path: the listed packages include "github.com/KogenAI/kogen-loop/sys"
    When k-build-loop-module-path: I run go mod tidy -diff
    Then k-build-loop-module-path: go mod tidy reports no difference
    When k-build-loop-module-path: I list the modules of the build list
    Then k-build-loop-module-path: the build list is exactly, in order:
      | path                                  | version                            |
      | github.com/KogenAI/kogen-build        |                                    |
      | github.com/KogenAI/kogen-coordinator  | v0.0.0-20261105001101-4116df40cbf1 |
      | github.com/KogenAI/kogen-executor     | v0.0.0-20261104160936-8a4fecd1ce0a |
      | github.com/KogenAI/kogen-loop         | v0.0.0-20261104223449-7e80986bcd04 |
      | github.com/KogenAI/kogen-review       | v0.0.0-20261104231132-7f6ef7d1c0a3 |
      | github.com/KogenAI/kogen-verification | v0.0.0-20261104204404-74798e7aa09e |
