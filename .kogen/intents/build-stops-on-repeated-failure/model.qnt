module repeated_failure {
 var fingerprint: int
 var count: int
 var stopped: bool
 var eligible: bool
 var same: bool
 var previous: int
 action init = all { fingerprint' = 0, count' = 0, stopped' = false, eligible' = false, same' = false, previous' = 0 }
 action step = {
  nondet fp = oneOf(Set(1, 2, 3))
  nondet cleanRed = oneOf(Set(true, false))
  val updated = if (cleanRed) { if (fp == fingerprint) count + 1 else 1 } else 0
  all {
   fingerprint' = if (cleanRed) fp else 0,
   count' = updated,
   stopped' = updated >= 4,
   eligible' = cleanRed,
   same' = fp == fingerprint,
   previous' = count
  }
 }
 val Boundary = stopped == (count >= 4)
 val Reset = (not(eligible) implies count == 0) and ((eligible and not(same)) implies count == 1)
 val Consecutive = (eligible and same) implies count == previous + 1
}
