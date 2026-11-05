// k-review-reopen-after-external-repair: the resume of a Build whose independent review ended with a
// blocking verdict, after the candidate was changed outside the Build. Trees are numbered; every
// external repair makes a fresh one.
module reopen {
  // The checkpoint's format version: 4 (current), 3 (previous) or 2 (one this Kogen cannot read).
  var version: int
  // The candidate tree in the checkout.
  var tree: int
  // A custody-relevant change was made with the repair: a protected (baseline) file, the staged
  // index, the approval, HEAD or the configuration.
  var custodyDrift: bool
  // "blocked" (the checkpoint of a blocking verdict), "reviewing" (the reopened review after its
  // fresh pass, waiting for the same reviewer), "refused" or "landed".
  var stage: str
  // The tree of the fresh pass of the resume that reopened the review (-1: none).
  var verified: int
  // The tree a reviewer approval recorded before the resume judged (-1: none).
  var stale: int
  // The committed tree (-1: nothing committed).
  var committed: int

  action init = {
    nondet v = Set(2, 3, 4).oneOf()
    nondet s = Set(-1, 0).oneOf()
    all {
      version' = v,
      tree' = 0,
      custodyDrift' = false,
      stage' = "blocked",
      verified' = -1,
      stale' = s,
      committed' = -1,
    }
  }

  // An external repair of the candidate while the Build is stopped.
  action repair = all {
    stage == "blocked",
    tree' = tree + 1,
    version' = version, custodyDrift' = custodyDrift, stage' = stage, verified' = verified, stale' = stale, committed' = committed,
  }

  // A repair that also changes custody.
  action driftCustody = all {
    stage == "blocked",
    tree' = tree + 1,
    custodyDrift' = true,
    version' = version, stage' = stage, verified' = verified, stale' = stale, committed' = committed,
  }

  // The explicit resume: custody drift and an unreadable version refuse before anything runs;
  // otherwise the review is reopened with a fresh pass of the checkout as it is now, whatever the
  // checkpoint's candidate was. Nothing saved stands in for that pass.
  action resume = all {
    stage == "blocked",
    tree' = tree, version' = version, custodyDrift' = custodyDrift, stale' = stale, committed' = committed,
    if (custodyDrift or not(Set(3, 4).contains(version))) all {
      stage' = "refused",
      verified' = verified,
    } else all {
      stage' = "reviewing",
      verified' = tree,
    },
  }

  // The same reviewer maintains its finding: the review is blocked again, in a new version 4
  // checkpoint; the pass of this resume does not carry over to the next one.
  action maintain = all {
    stage == "reviewing",
    stage' = "blocked",
    verified' = -1,
    version' = 4,
    stale' = stale,
    tree' = tree, custodyDrift' = custodyDrift, committed' = committed,
  }

  // The same reviewer approves the freshly verified tree; it is committed.
  action approve = all {
    stage == "reviewing",
    tree == verified,
    stage' = "landed",
    committed' = verified,
    tree' = tree, version' = version, custodyDrift' = custodyDrift, verified' = verified, stale' = stale,
  }

  action done = all {
    stage == "landed" or stage == "refused",
    tree' = tree, version' = version, custodyDrift' = custodyDrift, stage' = stage, verified' = verified, stale' = stale, committed' = committed,
  }

  action step = any { repair, driftCustody, resume, maintain, approve, done }

  // A commit is of the tree the reopened review's fresh pass verified, never of a tree only a
  // stale approval judged.
  val CommittedWasVerified = committed == -1 or (committed == verified and committed == tree)
  // The reopened review always works from a fresh pass of the current (repaired) checkout.
  val ReopenedVerifiesCurrentTree = stage != "reviewing" or verified == tree
  // A resume is refused only for custody drift or a version this Kogen cannot read, and a refusal
  // commits nothing.
  val RefusalOnlyForCustodyOrVersion = stage != "refused" or ((custodyDrift or not(Set(3, 4).contains(version))) and committed == -1)
  // Custody drift never reopens a review and never commits.
  val CustodyDriftNeverReopens = not(custodyDrift) or (stage != "reviewing" and committed == -1)
}
