Feature: Explain edits that leave red failures unmoved
  Scenario: Three unrelated edit rounds with a shared cause
    Then the unmoved red Build fixture "shared" satisfies its feedback contract
      """
      {"failures":12,"changedFile":"unrelated.txt","cause":"observed output was never produced","firstNoticePass":2,"ordinaryFeedback":"unchanged and in order","stopAfterToolFreeRounds":2}
      """
  Scenario: Twelve failures persist through nine unrelated edit rounds
    Then the unmoved red Build fixture "history" satisfies its feedback contract
      """
      {"failures":12,"changedFile":"unrelated.txt","cause":"observed output was never produced","firstNoticePass":2,"ordinaryFeedback":"unchanged and in order","stopAfterToolFreeRounds":2}
      """
  Scenario: One scenario starts passing in the second round
    Then the unmoved red Build fixture "partial" satisfies its feedback contract
      """
      {"failures":12,"changedFile":"unrelated.txt","cause":"observed output was never produced","firstNoticePass":2,"ordinaryFeedback":"unchanged and in order","stopAfterToolFreeRounds":2}
      """
  Scenario: Different failures do not get a common cause
    Then the unmoved red Build fixture "different" satisfies its feedback contract
      """
      {"failures":12,"changedFile":"unrelated.txt","cause":"observed output was never produced","firstNoticePass":2,"ordinaryFeedback":"unchanged and in order","stopAfterToolFreeRounds":2}
      """
  Scenario: First red and passing Build compatibility
    Then the unmoved red Build fixture "compatibility" satisfies its feedback contract
      """
      {"failures":12,"changedFile":"unrelated.txt","cause":"observed output was never produced","firstNoticePass":2,"ordinaryFeedback":"unchanged and in order","stopAfterToolFreeRounds":2}
      """
