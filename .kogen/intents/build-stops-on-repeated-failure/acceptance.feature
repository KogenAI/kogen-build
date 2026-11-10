Feature: Stop repeated failure despite edits
  This feature exercises real Build rounds with scripted native and external builders, local Git and verification reports.

  Scenario: Native edits cannot hide stagnation
    When build-stops-on-repeated-failure: the scripted build runs with:
      """
      {"Mode":"native","Sequence":[1,1,1,1],"Passes":4,"Outcome":"no_progress","Exit":1,"Message":["candidate/no_progress","acceptance","Failure 1 (line 2)"],"Event":true}
      """
    Then build-stops-on-repeated-failure: the build satisfies the expected result

  Scenario: External edits cannot hide stagnation
    When build-stops-on-repeated-failure: the scripted build runs with:
      """
      {"Mode":"external","Sequence":[1,1,1,1],"Passes":4,"Outcome":"no_progress","Exit":1,"Message":["candidate/no_progress","acceptance","Failure 1 (line 2)"],"Event":true}
      """
    Then build-stops-on-repeated-failure: the build satisfies the expected result

  Scenario: Real progress finishes green
    When build-stops-on-repeated-failure: the scripted build runs with:
      """
      {"Mode":"native","Sequence":[7,6,5,3,2,2,0],"Passes":7,"Outcome":"landed","Exit":0,"Message":[],"Event":false}
      """
    Then build-stops-on-repeated-failure: the build satisfies the expected result

  Scenario: Reset after a different failure
    When build-stops-on-repeated-failure: the scripted build runs with:
      """
      {"Mode":"native","Sequence":[1,1,1,2,1,1,1,0],"Passes":8,"Outcome":"landed","Exit":0,"Message":[],"Event":false}
      """
    Then build-stops-on-repeated-failure: the build satisfies the expected result
