Feature: Safe reconstruction of the held Problem20 consumer
  This feature drives the standalone kogen-build binary through its reconstruct and reconstruction modes
  against a fixture shaped like the Problem20 evidence: queue item bb13338f-045e-4853-8cc4-90ec80dc8b4f
  version 18, accepted request revision 3, two failed prior Build receipts with their candidate trees and
  two refused prior attempts. Receipt files, spend and the checkout are local stand-ins; only identifiers,
  revisions and hashes are the real Problem20 values.

  Background:
    Given safe-reconstruct-unpublished-native-draft: the standalone kogen-build binary is built
    And safe-reconstruct-unpublished-native-draft: a Problem20 checkout with retained receipts and fully proved evidence

  Scenario: An eligible recovery preserves old evidence, fences the old generation and creates exactly one fresh attempt
    When safe-reconstruct-unpublished-native-draft: I run the reconstruction
    Then safe-reconstruct-unpublished-native-draft: the run exits 0 and reports a created reconstruction bound to the original request and baseline
    And safe-reconstruct-unpublished-native-draft: the archive holds byte-identical copies of the retained receipts and the evidence file
    And safe-reconstruct-unpublished-native-draft: the retained receipts and the evidence file are byte-identical to before the run

  Scenario: A replay after a proved recovery creates no second attempt, wake or archive
    When safe-reconstruct-unpublished-native-draft: I run the reconstruction
    And safe-reconstruct-unpublished-native-draft: I remember the reported attempt and wake
    And safe-reconstruct-unpublished-native-draft: I run the reconstruction again with a different archive destination
    Then safe-reconstruct-unpublished-native-draft: the run reports the same attempt and wake as before, with exactly one of each
    And safe-reconstruct-unpublished-native-draft: no second archive was created and the first archive is reported
    And safe-reconstruct-unpublished-native-draft: a readback of the queue item still reports the first created attempt and wake and nothing more

  Scenario Outline: An interruption after <phase> is resumed without a second attempt or wake and without erasing evidence
    When safe-reconstruct-unpublished-native-draft: I run the reconstruction stopping after "<phase>"
    Then safe-reconstruct-unpublished-native-draft: the run exits 1 and reports an interrupted reconstruction without attempt or wake
    And safe-reconstruct-unpublished-native-draft: a readback of the queue item reports the interrupted reconstruction without attempt or wake
    When safe-reconstruct-unpublished-native-draft: I record the archive contents
    And safe-reconstruct-unpublished-native-draft: I run the reconstruction
    Then safe-reconstruct-unpublished-native-draft: the run exits 0 and reports a created reconstruction bound to the original request and baseline
    And safe-reconstruct-unpublished-native-draft: the archive was already preserved and is unchanged after the resumed run
    And safe-reconstruct-unpublished-native-draft: the archive holds byte-identical copies of the retained receipts and the evidence file
    And safe-reconstruct-unpublished-native-draft: the retained receipts and the evidence file are byte-identical to before the run
    When safe-reconstruct-unpublished-native-draft: I remember the reported attempt and wake
    And safe-reconstruct-unpublished-native-draft: I run the reconstruction
    Then safe-reconstruct-unpublished-native-draft: the run reports the same attempt and wake as before, with exactly one of each

    Examples:
      | phase     |
      | preserved |
      | fenced    |

  Scenario Outline: A missing or contradicting prerequisite stays held and creates nothing: <defect>
    Given safe-reconstruct-unpublished-native-draft: the evidence has this defect: "<defect>"
    When safe-reconstruct-unpublished-native-draft: I run the reconstruction
    Then safe-reconstruct-unpublished-native-draft: the run exits 1 and reports a held reconstruction that names its owner and a next action
    And safe-reconstruct-unpublished-native-draft: no archive was created
    And safe-reconstruct-unpublished-native-draft: the retained receipts and the evidence file are byte-identical to before the run
    And safe-reconstruct-unpublished-native-draft: a readback of the queue item shows no attempt, no wake and no reconstruction

    Examples:
      | defect                     |
      | effects unknown            |
      | effects missing            |
      | publication observed       |
      | external effect uncertain  |
      | active owner               |
      | owner proof missing        |
      | custody not revoked        |
      | custody proof missing      |
      | request revision changed   |
      | request hash changed       |
      | baseline moved             |
      | receipt file missing       |
      | receipt copy changed       |

  Scenario: A duplicate before an eligible recovery stays held both times and creates no attempt
    Given safe-reconstruct-unpublished-native-draft: the evidence has this defect: "effects unknown"
    When safe-reconstruct-unpublished-native-draft: I run the reconstruction
    And safe-reconstruct-unpublished-native-draft: I run the reconstruction
    Then safe-reconstruct-unpublished-native-draft: the run exits 1 and reports a held reconstruction that names its owner and a next action
    And safe-reconstruct-unpublished-native-draft: a readback of the queue item shows no attempt, no wake and no reconstruction

  Scenario: A second reconciliation with different evidence after a recovery stays held and keeps the first attempt
    When safe-reconstruct-unpublished-native-draft: I run the reconstruction
    And safe-reconstruct-unpublished-native-draft: I remember the reported attempt and wake
    And safe-reconstruct-unpublished-native-draft: the evidence lists one more prior attempt
    And safe-reconstruct-unpublished-native-draft: I run the reconstruction again with a different archive destination
    Then safe-reconstruct-unpublished-native-draft: the run exits 1 and reports a held duplicate reconciliation
    And safe-reconstruct-unpublished-native-draft: a readback of the queue item still reports the first created attempt and wake and nothing more

  Scenario Outline: A different consumer is never reconstructed: <defect>
    Given safe-reconstruct-unpublished-native-draft: the evidence has this defect: "<defect>"
    When safe-reconstruct-unpublished-native-draft: I run the reconstruction
    Then safe-reconstruct-unpublished-native-draft: the run exits 1 and reports a held reconstruction without attempt or wake
    And safe-reconstruct-unpublished-native-draft: no archive was created
    And safe-reconstruct-unpublished-native-draft: a readback of the queue item shows no attempt, no wake and no reconstruction

    Examples:
      | defect               |
      | other queue item     |
      | other obligation     |

  Scenario Outline: Claimed landing, deployment, acceptance or reused old proof is refused: <defect>
    Given safe-reconstruct-unpublished-native-draft: the evidence has this defect: "<defect>"
    When safe-reconstruct-unpublished-native-draft: I run the reconstruction
    Then safe-reconstruct-unpublished-native-draft: the run exits 1 and reports a held reconstruction that names its owner and a next action
    And safe-reconstruct-unpublished-native-draft: no archive was created
    And safe-reconstruct-unpublished-native-draft: a readback of the queue item shows no attempt, no wake and no reconstruction

    Examples:
      | defect               |
      | claimed landing      |
      | claimed deployment   |
      | claimed acceptance   |
      | reused build result  |
      | reused review result |
      | reused check result  |

  Scenario: The readback after recovery names every identifier and keeps landing, deployment and acceptance separate
    When safe-reconstruct-unpublished-native-draft: I run the reconstruction
    And safe-reconstruct-unpublished-native-draft: I remember the reported attempt and wake
    And safe-reconstruct-unpublished-native-draft: I read back the reconstruction of the queue item
    Then safe-reconstruct-unpublished-native-draft: the readback exits 0 and equals the report of the creating run
    And safe-reconstruct-unpublished-native-draft: the readback reports the three outcomes landing, deployment and acceptance separately as not observed while the queue item stays held

  Scenario: A readback never authorizes a recovery
    When safe-reconstruct-unpublished-native-draft: I read back the reconstruction of the queue item
    Then safe-reconstruct-unpublished-native-draft: a readback of the queue item shows no attempt, no wake and no reconstruction
