Feature: The no-edit notice and stop quote the first output line of the failing check
  This feature checks that the notice and the candidate/no_progress stop of a Build whose round made no edits quote the first nonblank output line of the check named as the first failure.

  Scenario: The notice and the stop quote the first nonblank line of the first failing check
    Given build-no-edit-stop-quotes-output: the kogen-build checkout
    When build-no-edit-stop-quotes-output: the Build test "TestNoEditQuoteAcc1Notice" runs in the build package
    Then build-no-edit-stop-quotes-output: the notice quotes the first nonblank output line right after the sentence naming the first failure

  Scenario: The stop quotes the first output line right after the first failure line
    Given build-no-edit-stop-quotes-output: the kogen-build checkout
    When build-no-edit-stop-quotes-output: the Build test "TestNoEditQuoteAcc2Stop" runs in the build package
    Then build-no-edit-stop-quotes-output: the stop has the output line right after the first failure line and keeps its other lines

  Scenario: A long first output line is cut to 200 characters
    Given build-no-edit-stop-quotes-output: the kogen-build checkout
    When build-no-edit-stop-quotes-output: the Build test "TestNoEditQuoteAcc3Cut" runs in the build package
    Then build-no-edit-stop-quotes-output: a 300 character first output line is cut to 200 characters in the notice and in the stop

  Scenario: An acceptance scenario as first failure adds nothing
    Given build-no-edit-stop-quotes-output: the kogen-build checkout
    When build-no-edit-stop-quotes-output: the Build test "TestNoEditQuoteAcc4AcceptanceScenarioFirst" runs in the build package
    Then build-no-edit-stop-quotes-output: the notice and the stop are unchanged when the first failure is an acceptance scenario

  Scenario: A failing check without output adds nothing
    Given build-no-edit-stop-quotes-output: the kogen-build checkout
    When build-no-edit-stop-quotes-output: the Build test "TestNoEditQuoteAcc5NoOutput" runs in the build package
    Then build-no-edit-stop-quotes-output: the notice and the stop are unchanged when the failing check has no nonblank output
