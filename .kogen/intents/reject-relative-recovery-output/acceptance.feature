Feature: Recovery refuses a relative archive output path
  This feature runs Recover against a Build that stopped on a builder-reported blocker, once with a relative output path and once with an absolute one outside the checkout.

  Scenario: A relative output path is refused before anything is written
    Given reject-relative-recovery-output: the kogen-build checkout
    When reject-relative-recovery-output: the driver test "TestAcceptanceRelativeRecoveryOutputRefused" runs in the build package
    Then reject-relative-recovery-output: the refusal names the path and nothing was written

  Scenario: An absolute output path outside the checkout still recovers
    Given reject-relative-recovery-output: the kogen-build checkout
    When reject-relative-recovery-output: the driver test "TestAcceptanceAbsoluteRecoveryOutputStillRecovers" runs in the build package
    Then reject-relative-recovery-output: recovery completes and the archive holds the candidate
