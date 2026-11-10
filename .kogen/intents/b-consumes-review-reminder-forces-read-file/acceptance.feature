Feature: Consume both forced review reminder contracts
  This feature preserves the verdict-strand test while accepting the producer's unread-part reminder.

  Scenario: Recognize both reminder contracts
    # The callable selector receives auto, forced submit_review, forced read_file;
    # it must return indexes [1,2] and the unchanged corresponding bodies.
    Then b-consumes-review-reminder-forces-read-file: both forced choices are recognized

  Scenario: Preserve the real Build review regression
    Then b-consumes-review-reminder-forces-read-file: the reset regression passes
