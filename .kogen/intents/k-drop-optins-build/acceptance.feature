Feature: kogen-build stops reading the opt-in options K-DROP-OPTINS removes from the coordinator
  K-DROP-OPTINS removes models.<role>.max_rounds (with DefaultExternalRounds), shape_workers,
  shape_audit, shape_decision_check and review_material from the coordinator's project
  configuration. kogen-build must build and pass against the coordinator it pins today, where the
  options still exist, and against the coordinator without them, using today's default values as
  constants. The coordinator without them is synthesized from the pinned one in a temporary folder;
  the behaviour scenarios run against the pinned coordinator, where a project can still set the options.

  Scenario: kogen-build builds, vets and passes its tests against the coordinator without the opt-in options
    Given the coordinator kogen-build pins, synthesized without max_rounds, shape_workers, shape_audit, shape_decision_check and review_material
    Then go build, go vet and go test of every kogen-build package pass against it

  Scenario Outline: A role's max_rounds no longer sets its round cap: the executor's own cap does, and 6 for a review role without one
    Then a Build whose claude-code <role> role sets max_rounds 1 under an executor cap of <cap> rounds for that role stops executor_round_limit after <rounds> <role> rounds

    Examples:
      | role   | cap | rounds |
      | build  | 3   | 3      |
      | review | 6   | 6      |
      | review | 0   | 6      |

  Scenario Outline: Resume and recover ignore the removed options when they compare the saved and current configuration
    Then resume compares a saved configuration that differs only in "<field>" as the <result> configuration

    Examples:
      | field                          | result  |
      | models.build.max_rounds        | same    |
      | review_material mode on_demand | same    |
      | review_material mode empty     | same    |
      | shape_workers                  | same    |
      | shape_audit                    | same    |
      | shape_decision_check           | same    |
      | checks                         | changed |
      | models.build.model             | changed |
      | protect                        | changed |

  Scenario: A project that still sets review_material mode on_demand is reviewed with the parts material
    Then a Build of a project whose project.yaml sets review_material mode on_demand lands after a review with parts material and the plain reviewer prompt

  Scenario: The on_demand review material mode is deleted from the Build
    Then no non-test Go file under build names the on_demand review material mode
