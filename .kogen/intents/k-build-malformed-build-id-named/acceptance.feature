Feature: resume and recover name an argument that is not a Build id

  A Build id is 8 lowercase hexadecimal characters. Resume and recover refuse an argument of another
  shape, or a well-formed id with no recorded Build, with their own reason and the command that lists
  Build attempts. A recorded Build whose checkpoint cannot be used keeps an unknown-checkpoint refusal
  that names the failed fact.

  Scenario Outline: Resume says "<id>" is not a Build id and names kogen intent list
    Given k-build-malformed-build-id-named: a committed checkout
    When k-build-malformed-build-id-named: I run the standalone resume with id "<id>"
    Then k-build-malformed-build-id-named: no Build run was created and no file in the checkout changed
    And k-build-malformed-build-id-named: the command exits 1 with category "candidate", kind "resume_unsupported" and message:
      """
      "<id>" is not a Build id; a Build id is 8 lowercase hexadecimal characters, such as 0a1b2c3d; kogen intent list lists Build attempts
      """

    Examples:
      | id        |
      | build-1   |
      | DEADBEEF  |
      | 0a1b2c3   |
      | 0a1b2c3d4 |

  Scenario Outline: Recover says "<id>" is not a Build id and names kogen intent list
    Given k-build-malformed-build-id-named: a committed checkout
    When k-build-malformed-build-id-named: I run recover with id "<id>" into a new output directory
    Then k-build-malformed-build-id-named: no Build run was created and no file in the checkout changed
    And k-build-malformed-build-id-named: the new output directory was not created
    And k-build-malformed-build-id-named: the command exits 1 with category "candidate", kind "recovery_unsupported" and message:
      """
      "<id>" is not a Build id; a Build id is 8 lowercase hexadecimal characters, such as 0a1b2c3d; kogen intent list lists Build attempts
      """

    Examples:
      | id        |
      | build-1   |
      | 0a1b2c3d4 |

  Scenario: Resume of a well-formed id with no recorded Build says so and names kogen intent list
    Given k-build-malformed-build-id-named: a committed checkout
    When k-build-malformed-build-id-named: I run the standalone resume with id "0a1b2c3d"
    Then k-build-malformed-build-id-named: no Build run was created and no file in the checkout changed
    And k-build-malformed-build-id-named: the command exits 1 with category "candidate", kind "resume_unsupported" and message:
      """
      Build 0a1b2c3d: no Build with this id in this checkout; kogen intent list lists Build attempts
      """

  Scenario: Recover of a well-formed id with no recorded Build says so and names kogen intent list
    Given k-build-malformed-build-id-named: a committed checkout
    When k-build-malformed-build-id-named: I run recover with id "0a1b2c3d" into a new output directory
    Then k-build-malformed-build-id-named: no Build run was created and no file in the checkout changed
    And k-build-malformed-build-id-named: the new output directory was not created
    And k-build-malformed-build-id-named: the command exits 1 with category "candidate", kind "recovery_unsupported" and message:
      """
      Build 0a1b2c3d: no Build with this id in this checkout; kogen intent list lists Build attempts
      """

  Scenario: Resume of a recorded Build with a damaged checkpoint names the failed fact
    Given k-build-malformed-build-id-named: a committed checkout with a recorded Build "0a1b2c3d" whose checkpoint.json holds "{broken"
    When k-build-malformed-build-id-named: I run the standalone resume with id "0a1b2c3d"
    Then k-build-malformed-build-id-named: no Build run was created and no file in the checkout changed
    And k-build-malformed-build-id-named: the command exits 1 with category "candidate", kind "resume_refused" and message:
      """
      Build 0a1b2c3d: unknown checkpoint: malformed required checkpoint fields; inspect the checkpoint and checkout, correct drift or obtain fresh approval for a new Build
      """

  Scenario: Recover of a recorded Build with a damaged checkpoint names the failed fact
    Given k-build-malformed-build-id-named: a committed checkout with a recorded Build "0a1b2c3d" whose checkpoint.json holds "{broken"
    When k-build-malformed-build-id-named: I run recover with id "0a1b2c3d" into a new output directory
    Then k-build-malformed-build-id-named: no Build run was created and no file in the checkout changed
    And k-build-malformed-build-id-named: the new output directory was not created
    And k-build-malformed-build-id-named: the command exits 1 with category "candidate", kind "recovery_refused" and message:
      """
      unknown checkpoint: malformed required checkpoint fields; inspect the run/receipt and checkout; correct the named fact or use ordinary Refine and fresh exact approval
      """

  Scenario: Resume of a recorded Build without a checkpoint names the failed fact
    Given k-build-malformed-build-id-named: a committed checkout with a recorded Build "0a1b2c3d" that has no checkpoint.json
    When k-build-malformed-build-id-named: I run the standalone resume with id "0a1b2c3d"
    Then k-build-malformed-build-id-named: no Build run was created and no file in the checkout changed
    And k-build-malformed-build-id-named: the command exits 1 with category "candidate", kind "resume_unsupported" and message:
      """
      Build 0a1b2c3d: unknown checkpoint: absent or legacy checkpoint; inspect the checkpoint and checkout, correct drift or obtain fresh approval for a new Build
      """

  Scenario: Recover of a recorded Build without a checkpoint names the failed fact
    Given k-build-malformed-build-id-named: a committed checkout with a recorded Build "0a1b2c3d" that has no checkpoint.json
    When k-build-malformed-build-id-named: I run recover with id "0a1b2c3d" into a new output directory
    Then k-build-malformed-build-id-named: no Build run was created and no file in the checkout changed
    And k-build-malformed-build-id-named: the new output directory was not created
    And k-build-malformed-build-id-named: the command exits 1 with category "candidate", kind "recovery_unsupported" and message:
      """
      unknown checkpoint: absent checkpoint; inspect the run/receipt and checkout; correct the named fact or use ordinary Refine and fresh exact approval
      """

  Scenario: Valid resume and recover behave as before
    Then k-build-malformed-build-id-named: the existing Go tests "TestAnInterruptedBuildResumesAndLandsOnceInTheSameConversation" and "TestRecoveryArchivesRawFilteredBinaryModeSymlinkAndDeletedPathsWithoutIndexOrIgnoredChanges" pass in package "./build"
