Feature: Attribute native Build context spend
  This feature covers KOGEN-058 in kogen-build, not review instrumentation.
  Local scripted providers supply all responses; no live account is used.

  Scenario: Request and tool custody
    Given run-records-context-spend-events: the local scripted Build fixture is available
    Then run-records-context-spend-events: request and tool blobs reconstruct the exchange
      """json
      {"requests":2,"calls":4,"read_result":"1\tbaseline"}
      """

  Scenario: Response linkage
    Given run-records-context-spend-events: the local scripted Build fixture is available
    Then run-records-context-spend-events: response and call identities link exactly
      """json
      {"response_ids":["rr-response-1","rr-response-2"]}
      """

  Scenario: Reset and refusal attribution
    Given run-records-context-spend-events: the local scripted Build fixture is available
    Then run-records-context-spend-events: reset and refusal retain measured attribution
      """json
      {"ceiling_bytes":524288,"output_reserve_tokens":131072}
      """
