Feature: Pin kogen-build to the signed Coordinator revision
  This delivery updates the Coordinator dependency only: go.mod and necessary go.sum changes.
  The version comes from the GitHub-verified signed Coordinator commit b427b145576a5bcb096b3b203483471e0d69eb8f.
  This is the dependency prerequisite for held kogen-build/native-supersede-revised-scope item
  5f1ba719-1736-4f5a-9da6-9466e4afeac7 request revision 3. It neither implements supersession
  nor alters retained Builds, their custody, or queue states.
  The local scenario checks the pin; the configured setup and checks and the normal native approval,
  independent Review, signed commit, and GitHub verified landing are separate delivery gates.
  On failure retain evidence and keep the consumer held. The consumer owner separately confirms
  the pin on landed main before fresh Shape.

  Scenario: Resolve the exact Coordinator pin from the consumer module
    When pin-coordinator-strict-build-lock: I run go list -m for "github.com/KogenAI/kogen-coordinator" in the consumer module
    Then pin-coordinator-strict-build-lock: the resolved module line is "github.com/KogenAI/kogen-coordinator v0.0.0-20261124131633-b427b145576a"
