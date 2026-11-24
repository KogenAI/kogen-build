Feature: Record configured model role sources in run facts
  This feature publishes real run facts through startFacts and update, reads the files,
  and reads and inherits an older facts record that carries no model source metadata.

  Background:
    Given build-record-model-sources: the facts publication fixture is available

  Scenario: Distinguish the team build lineup from a local review lineup
    When build-record-model-sources: I publish facts with this input:
      """
      {"LocalRole":"review"}
      """
    Then build-record-model-sources: the published and readable facts satisfy:
      """
      {"sources":{"shape":".kogen/models.yaml","build":".kogen/models.yaml","review":".kogen/local/models.yaml"}}
      """

  Scenario: Distinguish a local build lineup from the team review lineup
    When build-record-model-sources: I publish facts with this input:
      """
      {"LocalRole":"build"}
      """
    Then build-record-model-sources: the published and readable facts satisfy:
      """
      {"sources":{"shape":".kogen/models.yaml","build":".kogen/local/models.yaml","review":".kogen/models.yaml"}}
      """

  Scenario: Accept missing source metadata and retain old facts values
    When build-record-model-sources: I publish facts with this input:
      """
      {"Legacy":{"checkout_id":"old-checkout","slug":"old-slug","run_id":"previous","phase":"reviewing","state":"review_blocked","verification_pass":null,"verification":[{"n":3,"green":true,"tree":"old-tree","failing":[],"not_run":[],"path":"old/acceptance-3.ndjson"}],"reviews":[{"n":2,"verdict":"changes","findings":[{"severity":"note","path":"app.txt","line":1,"message":"looks fine"}],"path":"old/review-2.json"}]}}
      """
    Then build-record-model-sources: the published and readable facts satisfy:
      """
      {"missing":true,"legacy":{"checkout_id":"old-checkout","slug":"old-slug","run_id":"previous","phase":"reviewing","state":"review_blocked","verification_pass":null,"verification":[{"n":3,"green":true,"tree":"old-tree","failing":[],"not_run":[],"path":"old/acceptance-3.ndjson"}],"reviews":[{"n":2,"verdict":"changes","findings":[{"severity":"note","path":"app.txt","line":1,"message":"looks fine"}],"path":"old/review-2.json"}]}}
      """
