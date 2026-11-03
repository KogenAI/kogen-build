// No-edit rounds of a Build: a round that leaves the candidate tree byte-identical to the
// tree of the last red pass is not verified again; the first such round gets the no-edit
// notice, the next one stops the Build as no_progress (no edits). Edits reset the notice.
module noedit {
  var tree: int
  var redTree: int
  var runs: int
  var noticed: bool
  var stopped: bool
  var blocked: bool
  var rounds: int
  var noEditRounds: int
  var editRounds: int

  action init = all {
    tree' = 0, redTree' = -1, runs' = 0, noticed' = false, stopped' = false, blocked' = false,
    rounds' = 0, noEditRounds' = 0, editRounds' = 0,
  }

  // A round with edits: a new tree, verified once and red.
  action edit = all {
    not(stopped),
    tree' = tree + 1, redTree' = tree + 1, runs' = 1, noticed' = false,
    stopped' = false, blocked' = false,
    rounds' = rounds + 1, noEditRounds' = noEditRounds, editRounds' = editRounds + 1,
  }

  // A round without edits: verified only when no red pass checked this tree yet; otherwise
  // the notice, or the stop after a notice.
  action noEdit = all {
    not(stopped),
    tree' = tree, redTree' = tree,
    runs' = if (redTree == tree) runs else runs + 1,
    noticed' = (redTree == tree),
    stopped' = (redTree == tree and noticed),
    blocked' = false,
    rounds' = rounds + 1, noEditRounds' = noEditRounds + 1, editRounds' = editRounds,
  }

  // A valid protected-file blocker report stops the Build as build_blocked.
  action report = all {
    not(stopped),
    tree' = tree, redTree' = redTree, runs' = runs, noticed' = noticed, stopped' = true, blocked' = true,
    rounds' = rounds + 1, noEditRounds' = noEditRounds, editRounds' = editRounds,
  }

  action stay = all {
    stopped,
    tree' = tree, redTree' = redTree, runs' = runs, noticed' = noticed, stopped' = stopped, blocked' = blocked,
    rounds' = rounds, noEditRounds' = noEditRounds, editRounds' = editRounds,
  }

  action step = any { edit, noEdit, report, stay }

  val AcceptanceOncePerTree = runs <= 1
  val NoEditStopFollowsNotice = (stopped and not(blocked)) implies noticed
  val RoundsCountedApart = noEditRounds + editRounds <= rounds
}
