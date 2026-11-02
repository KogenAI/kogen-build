Feature: The no-defect review filter applies only to on_demand review material and judges the finding's own claim
  A blocking review finding that asserts no product defect (such as inspection not completed)
  is invalid feedback only when the project's review_material mode is on_demand, and there a
  no-defect phrase that the finding only quotes does not count. Each scenario runs one real
  Build of a fixture project whose builder changes app.txt and whose reviewer submits a
  changes verdict with the one blocking finding below.

  Scenario: On demand, a concrete defect that quotes 'no defects found' stays blocking
    Given the project review_material mode is "on_demand"
    When the reviewer submits a changes verdict whose only finding is this blocker at app.txt line 1:
      """
      The scanner prints 'no defects found' for broken input: app.txt no longer reports the parse error of an unterminated record.
      """
    Then the finding stands: the Build stops review_blocked with that changes verdict and the branch unchanged

  Scenario: On demand, a concrete defect that quotes "no defects found" in double quotes stays blocking
    Given the project review_material mode is "on_demand"
    When the reviewer submits a changes verdict whose only finding is this blocker at app.txt line 1:
      """
      The scanner prints "no defects found" for broken input: app.txt no longer reports the parse error of an unterminated record.
      """
    Then the finding stands: the Build stops review_blocked with that changes verdict and the branch unchanged

  Scenario: On demand, an inspection-not-completed blocker is still invalid feedback
    Given the project review_material mode is "on_demand"
    When the reviewer submits a changes verdict whose only finding is this blocker at app.txt line 1:
      """
      The required inspection of the complete pinned candidate source was not completed... no product defect is asserted by this finding.
      """
    Then the finding is refused as invalid feedback: the Build stops review_invalid, never review_blocked, and the branch is unchanged

  Scenario: On demand, an unquoted no-defect claim is still invalid feedback
    Given the project review_material mode is "on_demand"
    When the reviewer submits a changes verdict whose only finding is this blocker at app.txt line 1:
      """
      No defect was found, but the inspection is not complete.
      """
    Then the finding is refused as invalid feedback: the Build stops review_invalid, never review_blocked, and the branch is unchanged

  Scenario: In parts mode, an inspection-not-completed blocker is a blocker as before the on_demand gate
    Given the project review_material mode is "parts"
    When the reviewer submits a changes verdict whose only finding is this blocker at app.txt line 1:
      """
      The required inspection of the complete pinned candidate source was not completed... no product defect is asserted by this finding.
      """
    Then the finding stands: the Build stops review_blocked with that changes verdict and the branch unchanged

  Scenario: In parts mode, a finding that quotes 'no defects found' is a blocker as before the on_demand gate
    Given the project review_material mode is "parts"
    When the reviewer submits a changes verdict whose only finding is this blocker at app.txt line 1:
      """
      The scanner prints 'no defects found' for broken input: app.txt no longer reports the parse error of an unterminated record.
      """
    Then the finding stands: the Build stops review_blocked with that changes verdict and the branch unchanged
