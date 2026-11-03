module unmoved {
  var streak: int
  var notice: bool
  var eligible: bool
  action init = all { streak' = 0, notice' = false, eligible' = false }
  action step = nondet red = oneOf(Set(false, true)) { nondet same = oneOf(Set(false, true)) { nondet edited = oneOf(Set(false, true)) {
    all {
      streak' = if (not(red)) 0 else if (same) streak + 1 else 1,
      eligible' = (red and same and streak > 0 and edited),
      notice' = (red and same and streak > 0 and edited)
    }
  } } }
  val NoticeOnlyForEditedRepeat = notice implies eligible
  val Nonnegative = streak >= 0
}
