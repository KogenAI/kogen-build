Feature: Diagnostic validation refusals name the invalid input field
  This feature starts a Build with invalid diagnostic data in a scratch checkout through a compiled driver that calls the Build entry point and reports the refusal and the checkout state.

  Scenario: Blank diagnostic text names the text field
    Given diagnostic-refusal-names-invalid-field: the diagnostic driver is compiled
    When diagnostic-refusal-names-invalid-field: a Build is started in a scratch checkout with the diagnostic case "blank text"
    Then diagnostic-refusal-names-invalid-field: the Build is refused as intent/context_unavailable with exit code 2
    Then diagnostic-refusal-names-invalid-field: no run or checkout change was made
    Then diagnostic-refusal-names-invalid-field: the refusal detail names only the "text" field

  Scenario: Invalid UTF-8 diagnostic text names the text field
    Given diagnostic-refusal-names-invalid-field: the diagnostic driver is compiled
    When diagnostic-refusal-names-invalid-field: a Build is started in a scratch checkout with the diagnostic case "invalid UTF-8 text"
    Then diagnostic-refusal-names-invalid-field: the Build is refused as intent/context_unavailable with exit code 2
    Then diagnostic-refusal-names-invalid-field: no run or checkout change was made
    Then diagnostic-refusal-names-invalid-field: the refusal detail names only the "text" field

  Scenario: A relative source path names the source path field
    Given diagnostic-refusal-names-invalid-field: the diagnostic driver is compiled
    When diagnostic-refusal-names-invalid-field: a Build is started in a scratch checkout with the diagnostic case "relative source path"
    Then diagnostic-refusal-names-invalid-field: the Build is refused as intent/context_unavailable with exit code 2
    Then diagnostic-refusal-names-invalid-field: no run or checkout change was made
    Then diagnostic-refusal-names-invalid-field: the refusal detail names only the "source path" field

  Scenario: An empty source path names the source path field
    Given diagnostic-refusal-names-invalid-field: the diagnostic driver is compiled
    When diagnostic-refusal-names-invalid-field: a Build is started in a scratch checkout with the diagnostic case "empty source path"
    Then diagnostic-refusal-names-invalid-field: the Build is refused as intent/context_unavailable with exit code 2
    Then diagnostic-refusal-names-invalid-field: no run or checkout change was made
    Then diagnostic-refusal-names-invalid-field: the refusal detail names only the "source path" field

  Scenario: An invalid UTF-8 source path names the source path field
    Given diagnostic-refusal-names-invalid-field: the diagnostic driver is compiled
    When diagnostic-refusal-names-invalid-field: a Build is started in a scratch checkout with the diagnostic case "invalid UTF-8 source path"
    Then diagnostic-refusal-names-invalid-field: the Build is refused as intent/context_unavailable with exit code 2
    Then diagnostic-refusal-names-invalid-field: no run or checkout change was made
    Then diagnostic-refusal-names-invalid-field: the refusal detail names only the "source path" field

  Scenario: A mismatched digest names the SHA-256 field
    Given diagnostic-refusal-names-invalid-field: the diagnostic driver is compiled
    When diagnostic-refusal-names-invalid-field: a Build is started in a scratch checkout with the diagnostic case "mismatched digest"
    Then diagnostic-refusal-names-invalid-field: the Build is refused as intent/context_unavailable with exit code 2
    Then diagnostic-refusal-names-invalid-field: no run or checkout change was made
    Then diagnostic-refusal-names-invalid-field: the refusal detail names only the "SHA-256" field

  Scenario: An upper-case digest names the SHA-256 field
    Given diagnostic-refusal-names-invalid-field: the diagnostic driver is compiled
    When diagnostic-refusal-names-invalid-field: a Build is started in a scratch checkout with the diagnostic case "upper-case digest"
    Then diagnostic-refusal-names-invalid-field: the Build is refused as intent/context_unavailable with exit code 2
    Then diagnostic-refusal-names-invalid-field: no run or checkout change was made
    Then diagnostic-refusal-names-invalid-field: the refusal detail names only the "SHA-256" field
