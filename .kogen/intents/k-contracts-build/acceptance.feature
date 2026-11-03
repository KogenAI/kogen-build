Feature: Producer-owned consumer contracts
  The kogen 9bf6073 consumer's exported calls and three response-budget cases
  belong in the producer's ordinary offline test gate, not in a consumer checkout.

  Scenario: Run the producer-owned synthetic consumer contract
    Then the producer consumer-contract package passes at "./contract"
