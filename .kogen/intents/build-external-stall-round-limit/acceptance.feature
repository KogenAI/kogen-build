Feature: Report a stalled external Build as the executor round limit
  This feature runs real Build rounds with a scripted external executor (or the scripted native provider), local Git and scripted verification passes.

  Scenario: Same-tree clean red streak on the final path
    When build-external-stall-round-limit: the scripted build runs with:
      """
      {"Staged":false,"Native":false,"MaxRounds":0,"Same":true,"Sequence":[1,1,1,1,1],"Rounds":3,"Passes":3,"Outcome":"executor_round_limit","Exit":1,"Message":["candidate/executor_round_limit","failing: acceptance","Failure 1 (line 2)"],"Lacks":["candidate/no_progress"],"Stage":"","Failing":"","StallEvents":1,"RepeatEvents":0}
      """
    Then build-external-stall-round-limit: the build satisfies the expected result

  Scenario: Same-tree clean red streak on the staged path
    When build-external-stall-round-limit: the scripted build runs with:
      """
      {"Staged":true,"Native":false,"MaxRounds":0,"Same":true,"Sequence":[1,1,1,1,1],"Rounds":3,"Passes":-1,"Outcome":"executor_round_limit","Exit":1,"Message":["candidate/executor_round_limit","failing: stage-foundation-1"],"Lacks":["candidate/no_progress"],"Stage":"foundation","Failing":"stage-foundation-1","StallEvents":1,"RepeatEvents":0}
      """
    Then build-external-stall-round-limit: the build satisfies the expected result

  Scenario: Round bound with changing failures
    When build-external-stall-round-limit: the scripted build runs with:
      """
      {"Staged":false,"Native":false,"MaxRounds":3,"Same":false,"Sequence":[3,2,1],"Rounds":3,"Passes":3,"Outcome":"executor_round_limit","Exit":1,"Message":["candidate/executor_round_limit"],"Lacks":["candidate/no_progress"],"Stage":"","Failing":"","StallEvents":0,"RepeatEvents":0}
      """
    Then build-external-stall-round-limit: the build satisfies the expected result

  Scenario: Native identical failures still stop as no progress
    When build-external-stall-round-limit: the scripted build runs with:
      """
      {"Staged":false,"Native":true,"MaxRounds":0,"Same":false,"Sequence":[1,1,1,1,1],"Rounds":-1,"Passes":4,"Outcome":"no_progress","Exit":1,"Message":["candidate/no_progress","four consecutive verification passes have identical failures"],"Lacks":["executor_round_limit"],"Stage":"","Failing":"","StallEvents":0,"RepeatEvents":1}
      """
    Then build-external-stall-round-limit: the build satisfies the expected result

  Scenario: A decreasing failing set then green finishes unimpeded
    When build-external-stall-round-limit: the scripted build runs with:
      """
      {"Staged":false,"Native":false,"MaxRounds":0,"Same":false,"Sequence":[5,4,3,2,1,0],"Rounds":6,"Passes":6,"Outcome":"landed","Exit":0,"Message":[],"Lacks":["candidate/no_progress","executor_round_limit"],"Stage":"","Failing":"","StallEvents":0,"RepeatEvents":0}
      """
    Then build-external-stall-round-limit: the build satisfies the expected result
