// K-BASELINEONCE: within one Build, a configured check's passed result is recorded under its key
// (the check, the tree and the environment) and reused at a later whole-Intent pass on the same key;
// the checks the previous pass failed run first; a pass is green and a Build lands only when every
// configured check passed on the pass's own tree and environment in this Build.
module baselineonce {
  type Check = str
  type Key = { check: Check, tree: int, env: int }

  pure val CHECKS: Set[Check] = Set("first", "second")
  pure val TREES: Set[int] = Set(1, 2)
  pure val ENVS: Set[int] = Set(1, 2)

  // The current tree and environment of the checkout.
  var tree: int
  var env: int
  // The within-Build ledger: keys of check executions that passed and were recorded.
  var ledger: Set[Key]
  // Ground truth: every key on which a check actually executed and passed in this Build.
  var passedOn: Set[Key]
  // The configured checks the last whole-Intent pass failed (the targeted checks of the next pass).
  var lastFailing: Set[Check]
  // The last whole-Intent pass: its key's tree and environment, and whether it was green.
  var passTree: int
  var passEnv: int
  var passGreen: bool
  var passes: int
  var landed: bool
  var landedTree: int
  var landedEnv: int

  def key(c: Check, t: int, e: int): Key = { check: c, tree: t, env: e }

  // The baseline: every configured check runs on the unchanged tree; the passed ones are recorded.
  action init = all {
    nondet t = TREES.oneOf()
    nondet e = ENVS.oneOf()
    nondet outcome = CHECKS.setOfMaps(Set(true, false)).oneOf()
    val ok = CHECKS.filter(c => outcome.get(c)).map(c => key(c, t, e))
    all {
      tree' = t, env' = e,
      ledger' = ok, passedOn' = ok,
      lastFailing' = Set(),
      passTree' = 0, passEnv' = 0, passGreen' = false, passes' = 0,
      landed' = false, landedTree' = 0, landedEnv' = 0,
    }
  }

  action unchangedRest = all {
    ledger' = ledger, passedOn' = passedOn, lastFailing' = lastFailing,
    passTree' = passTree, passEnv' = passEnv, passGreen' = passGreen, passes' = passes,
    landed' = landed, landedTree' = landedTree, landedEnv' = landedEnv,
  }

  // A builder or reviewer edit, or nothing (the same tree).
  action edit = all { not(landed), nondet t = TREES.oneOf() all { tree' = t, env' = env, unchangedRest } }

  // A toolchain or environment change between check points (or across a resume).
  action retool = all { not(landed), nondet e = ENVS.oneOf() all { env' = e, tree' = tree, unchangedRest } }

  // A whole-Intent pass: reused checks do not run; the targeted (last failing) ones run first, and
  // when one of them fails nothing else runs; every execution that passes is recorded.
  action verifyPass = all {
    not(landed),
    passes < 4,
    nondet outcome = CHECKS.setOfMaps(Set(true, false)).oneOf()
    val reused = CHECKS.filter(c => ledger.contains(key(c, tree, env)))
    val targeted = lastFailing.exclude(reused)
    val targetedFailed = targeted.exists(c => not(outcome.get(c)))
    val executed = if (targetedFailed) targeted else CHECKS.exclude(reused)
    val passedNow = executed.filter(c => outcome.get(c))
    val failedNow = executed.filter(c => not(outcome.get(c)))
    val recorded = passedNow.map(c => key(c, tree, env))
    all {
      ledger' = ledger.union(recorded),
      passedOn' = passedOn.union(recorded),
      lastFailing' = failedNow,
      passTree' = tree, passEnv' = env,
      passGreen' = CHECKS.forall(c => reused.contains(c) or passedNow.contains(c)),
      passes' = passes + 1,
      tree' = tree, env' = env,
      landed' = false, landedTree' = 0, landedEnv' = 0,
    }
  }

  // The commit of a green pass whose tree and environment are still current.
  action land = all {
    not(landed), passGreen, passTree == tree, passEnv == env,
    landed' = true, landedTree' = tree, landedEnv' = env,
    tree' = tree, env' = env,
    ledger' = ledger, passedOn' = passedOn, lastFailing' = lastFailing,
    passTree' = passTree, passEnv' = passEnv, passGreen' = passGreen, passes' = passes,
  }

  action step = any { edit, retool, verifyPass, land }

  // A reused result is always one this Build recorded from an execution that passed on that same key.
  val ReuseOnlyPassed = ledger.subseteq(passedOn)
  // A green pass has every configured check passed on its own tree and environment in this Build.
  val GreenOnFullSuite = passGreen implies CHECKS.forall(c => passedOn.contains(key(c, passTree, passEnv)))
  // A Build lands only on a tree and environment every configured check passed on.
  val LandsOnFullSuite = landed implies CHECKS.forall(c => passedOn.contains(key(c, landedTree, landedEnv)))
}
