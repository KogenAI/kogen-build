Feature: k-build-note-line-integer: Preserve integer note lines
  Scenario Outline: k-build-note-line-integer: Review approval displays an integer line
    When k-build-note-line-integer: I run the review hook with executor "<executor>"
    Then k-build-note-line-integer: the hook passes with integer note output "app.txt:1: looks fine" and not "app.txt:1.0: looks fine"

    Examples:
      | executor |
      | pinned |
      | 391e0b6 |
