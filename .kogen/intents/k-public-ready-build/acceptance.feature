Feature: Public repository documentation
  Scenario: README serves a public developer
    Then the public README contains these literals:
      | literal |
      | kogen-build |
      | Kogen |
      | go build -o kogen-build ./cmd/kogen-build |
      | npm ci --prefix testdata/tools/cucumber |
      | go test |
      | https://github.com/KogenAI/kogen |
    And the README contains the inquiry address "contact@kogen.dev" exactly 1 time
    And the public README does not match "GOPRIVATE|insteadOf|private.{0,40}(module|mirror|workspace)|token.{0,20}(setup|install)"

  Scenario: No policy or collaboration-status copy
    Then these public policy files are absent:
      | path |
      | SECURITY.md |
      | CONTRIBUTING.md |
      | BRANDING.md |
    And the public README does not match "not accept.{0,30}(contribution|pull request)|contributions? (are|is) (closed|not accepted)|issues (are )?disabled|pull requests (are )?disabled|maintenance promise"

  Scenario: Public documentation has no machine-specific paths
    Then tracked documentation has no real home paths or workspace references

  Scenario: History findings are available without rewriting history
    Then "docs/public-history-audit.md" contains these report fields:
      | literal |
      | History |
      | commit |
      | file |
      | category |
