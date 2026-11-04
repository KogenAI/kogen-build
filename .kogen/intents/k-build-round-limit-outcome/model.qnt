// Builder rounds after a red pass, native or external. A round either changes the candidate
// tree, rewrites it unchanged with a file-write call, or leaves it unchanged without one. Only
// the last kind is a round without edits: the first after a red pass of this tree gets the
// no-edit notice instead of a verification, the next stops the Build as no_progress. Every other
// round is verified; for an external builder the third consecutive identical red pass stops the
// Build as executor_round_limit, and a native builder keeps repairing.
module roundlimit {
  var external: bool
  // a remembered red pass checked the current tree
  var redKnown: bool
  // the no-edit notice was given for the remembered red pass
  var noticed: bool
  // consecutive verified red passes of the same tree and failing set
  var streak: int
  var verifiedLast: bool
  var wroteLast: bool
  var outcome: str
  var rounds: int

  action init = {
    nondet ext = oneOf(Set(true, false))
    all {
      external' = ext, redKnown' = false, noticed' = false, streak' = 0,
      verifiedLast' = false, wroteLast' = false, outcome' = "running", rounds' = 0,
    }
  }

  // A round that changed the tree (by a write call or otherwise): a new red pass.
  action changed = {
    nondet wrote = oneOf(Set(true, false))
    all {
      outcome == "running",
      external' = external, redKnown' = true, noticed' = false, streak' = 1,
      verifiedLast' = true, wroteLast' = wrote, outcome' = "running", rounds' = rounds + 1,
    }
  }

  // A round with a file-write call that left the tree as it was: verified, the same red again.
  action rewrite = all {
    outcome == "running",
    val s = if (redKnown) streak + 1 else 1
    all {
      external' = external, redKnown' = true, noticed' = false, streak' = s,
      verifiedLast' = true, wroteLast' = true,
      outcome' = if (external and s >= 3) "executor_round_limit" else "running",
      rounds' = rounds + 1,
    }
  }

  // A round without a file-write call that left the tree as it was.
  action quiet = all {
    outcome == "running",
    external' = external, wroteLast' = false, rounds' = rounds + 1,
    if (not(redKnown)) all {
      redKnown' = true, noticed' = false, streak' = 1, verifiedLast' = true, outcome' = "running",
    } else if (not(noticed)) all {
      redKnown' = true, noticed' = true, streak' = streak, verifiedLast' = false, outcome' = "running",
    } else all {
      redKnown' = true, noticed' = true, streak' = streak, verifiedLast' = false, outcome' = "no_progress",
    },
  }

  action stay = all {
    outcome != "running",
    external' = external, redKnown' = redKnown, noticed' = noticed, streak' = streak,
    verifiedLast' = verifiedLast, wroteLast' = wroteLast, outcome' = outcome, rounds' = rounds,
  }

  action step = any { changed, rewrite, quiet, stay }

  val NoProgressFollowsNotice = outcome == "no_progress" implies (noticed and not(wroteLast) and not(verifiedLast))
  val WriteRoundsAreVerified = (rounds > 0 and wroteLast) implies verifiedLast
  val RoundLimitIsExternalThirdIdenticalRed = outcome == "executor_round_limit" implies (external and streak == 3 and wroteLast)
  val NativeKeepsRepairing = not(external) implies outcome != "executor_round_limit"
}
