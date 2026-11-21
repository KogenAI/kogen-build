Feature: Classify a stalled external Build as no progress
  This feature runs real Build rounds with a scripted external executor, local Git and scripted verification passes.

  Scenario: Same-tree clean red streak on the final path
    When build-classifies-stalled-external-pass: the scripted build runs with:
      """
      {"Staged":false,"Same":true,"Sequence":[1,1,1,1,1],"Rounds":3,"Passes":3,"Outcome":"no_progress","Exit":1,"Message":["candidate/no_progress","acceptance","Failure 1 (line 2)"],"Lacks":["executor_round_limit"],"Stage":"","Failing":"","StallEvents":1,"RepeatEvents":0}
      """
    Then build-classifies-stalled-external-pass: the build satisfies the expected result

  Scenario: Same-tree clean red streak on the staged path
    When build-classifies-stalled-external-pass: the scripted build runs with:
      """
      {"Staged":true,"Same":true,"Sequence":[1,1,1,1,1],"Rounds":3,"Passes":-1,"Outcome":"no_progress","Exit":1,"Message":["candidate/no_progress","stage-foundation-1"],"Lacks":["executor_round_limit"],"Stage":"foundation","Failing":"stage-foundation-1","StallEvents":1,"RepeatEvents":0}
      """
    Then build-classifies-stalled-external-pass: the build satisfies the expected result

  Scenario: A decreasing failing set then green finishes unimpeded
    When build-classifies-stalled-external-pass: the scripted build runs with:
      """
      {"Staged":false,"Same":false,"Sequence":[5,4,3,2,1,0],"Rounds":6,"Passes":6,"Outcome":"landed","Exit":0,"Message":[],"Lacks":["no_progress","executor_round_limit"],"Stage":"","Failing":"","StallEvents":0,"RepeatEvents":0}
      """
    Then build-classifies-stalled-external-pass: the build satisfies the expected result

  Scenario: An identical failure repeated under churning edits still stops
    When build-classifies-stalled-external-pass: the scripted build runs with:
      """
      {"Staged":false,"Same":false,"Sequence":[1,1,1,1,1],"Rounds":4,"Passes":4,"Outcome":"no_progress","Exit":1,"Message":["candidate/no_progress","acceptance","Failure 1 (line 2)"],"Lacks":["executor_round_limit"],"Stage":"","Failing":"","StallEvents":0,"RepeatEvents":1}
      """
    Then build-classifies-stalled-external-pass: the build satisfies the expected result
