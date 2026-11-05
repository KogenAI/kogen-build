Feature: The baseline identity tests pass under any umask
  TestBaselinePerBaseIdentity and TestBaselinePerBaseCustody (build/baseline_cache_test.go) compare
  the baseline identity of a repository's main checkout with that of a `git worktree add` checkout
  of the same commit. baselineIdentity hashes every file's mode, so the fixture's checkout must have
  the modes git gives its own checkout under the process umask. Both tests pass under umask 0002,
  022 and 077, still fail when baselineIdentity loses a detection they check, and baselineIdentity
  itself is unchanged.

  Scenario Outline: Both tests pass under umask <umask>
    When k-build-baseline-umask: go test runs TestBaselinePerBaseIdentity and TestBaselinePerBaseCustody in package build under umask <umask>
    Then k-build-baseline-umask: "TestBaselinePerBaseIdentity and TestBaselinePerBaseCustody" passed

    Examples:
      | umask |
      | 0002  |
      | 022   |
      | 077   |

  Scenario: Under umask 0002 the identity test still detects a permission change
    When k-build-baseline-umask: go test runs TestBaselinePerBaseIdentity and TestBaselinePerBaseCustody in package build under umask 0002 with baselineIdentity changed so that it "ignores permission bits"
    Then k-build-baseline-umask: "TestBaselinePerBaseIdentity" failed

  Scenario: Under umask 0002 the identity test still detects a content change
    When k-build-baseline-umask: go test runs TestBaselinePerBaseIdentity and TestBaselinePerBaseCustody in package build under umask 0002 with baselineIdentity changed so that it "ignores file contents"
    Then k-build-baseline-umask: "TestBaselinePerBaseIdentity" failed

  Scenario: Under umask 0002 both tests still require one identity for two checkouts of the same repository
    When k-build-baseline-umask: go test runs TestBaselinePerBaseIdentity and TestBaselinePerBaseCustody in package build under umask 0002 with baselineIdentity changed so that it "depends on the checkout path"
    Then k-build-baseline-umask: "TestBaselinePerBaseIdentity and TestBaselinePerBaseCustody" failed

  Scenario: baselineIdentity's product code is unchanged
    Then k-build-baseline-umask: the file "build/baseline_cache.go" has sha256 30291a24583d86f33582611b1aa13d67518e7d747f511aa121c5b832551452c7
