module maintained {
 var landed: Set[int]
 var current: Set[int]
 var checked: Set[int]
 var green: bool
 action init = all { landed' = Set(), current' = Set(), checked' = Set(), green' = false }
 action land = { nondet id = oneOf(1.to(3)); all { landed' = landed.union(Set(id)), current' = current.union(Set(id)), checked' = Set(), green' = false } }
 action verify = all { landed' = landed, current' = current, checked' = current, green' = true }
 action red = all { landed' = landed, current' = current, checked' = current, green' = false }
 action step = any { land, verify, red }
 val HistoryKept = landed == current
 val Enforced = green implies checked == current
}
