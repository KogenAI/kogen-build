Feature: Consume only the Claude Code 2.1.318 contract

  Scenario: Adopt the producer and preserve both first prompt paths
    Then b-consumes-e-claude-code-2-1-318: the pinned producer exports version "2.1.318"
    And b-consumes-e-claude-code-2-1-318: the first prompt test passes for:
      | TestContextExternalFirstPrompt/context_false |
      | TestContextExternalFirstPrompt/context_true  |

  Scenario: Replace the fake runtime version contract
    Then b-consumes-e-claude-code-2-1-318: the fake runtime advertises only "2.1.318"
