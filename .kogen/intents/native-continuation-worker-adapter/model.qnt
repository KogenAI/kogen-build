module native_continuation_worker_adapter {
  var prepared: bool
  var sameEvidence: bool
  var freshFence: bool
  var held: bool
  var executions: int
  var authorizedExecution: bool

  action init = all {
    prepared' = false, sameEvidence' = true, freshFence' = false,
    held' = false, executions' = 0, authorizedExecution' = true
  }
  action prepare = all {
    prepared' = true, sameEvidence' = sameEvidence, freshFence' = false,
    held' = held, executions' = executions, authorizedExecution' = authorizedExecution
  }
  action drift = all {
    sameEvidence' = false, freshFence' = false,
    prepared' = prepared, held' = held, executions' = executions,
    authorizedExecution' = authorizedExecution
  }
  action hold = all {
    held' = true, freshFence' = false, sameEvidence' = sameEvidence,
    prepared' = prepared, executions' = executions,
    authorizedExecution' = authorizedExecution
  }
  action claim = all {
    freshFence' = true, sameEvidence' = sameEvidence,
    prepared' = prepared, held' = held, executions' = executions,
    authorizedExecution' = authorizedExecution
  }
  action execute = all {
    prepared, sameEvidence, freshFence, not(held), executions == 0,
    executions' = executions + 1,
    authorizedExecution' = (prepared and sameEvidence and freshFence and not(held)),
    freshFence' = false, prepared' = prepared, sameEvidence' = sameEvidence, held' = held
  }
  action refuse = all {
    prepared' = prepared, sameEvidence' = sameEvidence, freshFence' = freshFence,
    held' = held, executions' = executions, authorizedExecution' = authorizedExecution
  }
  action step = any { prepare, drift, hold, claim, execute, refuse }
  val Safe = executions <= 1 and authorizedExecution
}
