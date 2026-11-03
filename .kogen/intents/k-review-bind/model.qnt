// The review approval binding of k-review-bind: an approval commits only a tree whose bytes
// reached the reviewer. Trees are numbered; every write makes a fresh tree.
module reviewbind {
  // The candidate tree in the checkout.
  var current: int
  // The tree whose bytes the reviewer holds: the reviewed source, then its own native edits
  // and the trees whose diff Kogen delivered.
  var seen: int
  // Every tree whose bytes reached the reviewer.
  var delivered: Set[int]
  // The next fresh tree number.
  var fresh: int
  // The committed tree, -1 while nothing is committed.
  var committed: int
  // The reviewer is an external executor, whose edits Kogen cannot attribute.
  var external: bool

  action init = {
    nondet ext = Set(true, false).oneOf()
    all {
      current' = 0,
      seen' = 0,
      delivered' = Set(0),
      fresh' = 1,
      committed' = -1,
      external' = ext,
    }
  }

  // Any writer other than the reviewer's own Kogen tool calls (an editor, a watcher, or any
  // edit inside an external reviewer's round).
  action outsideWrite = all {
    committed == -1,
    current' = fresh,
    fresh' = fresh + 1,
    seen' = seen,
    delivered' = delivered,
    committed' = committed,
    external' = external,
  }

  // A native reviewer's write_file or shell call: seen follows it when the tree before the call
  // was the seen tree.
  action reviewerEdit = all {
    committed == -1,
    not(external),
    current' = fresh,
    fresh' = fresh + 1,
    seen' = if (current == seen) fresh else seen,
    delivered' = if (current == seen) delivered.union(Set(fresh)) else delivered,
    committed' = committed,
    external' = external,
  }

  // An approval of the current tree: it commits when the tree is the seen one; otherwise Kogen
  // delivers the diff from seen to the current tree and a fresh verdict is required.
  action approve = all {
    committed == -1,
    current' = current,
    fresh' = fresh,
    external' = external,
    if (current == seen) all {
      committed' = current,
      seen' = seen,
      delivered' = delivered,
    } else all {
      committed' = committed,
      seen' = current,
      delivered' = delivered.union(Set(current)),
    },
  }

  action done = all {
    committed != -1,
    current' = current,
    seen' = seen,
    delivered' = delivered,
    fresh' = fresh,
    committed' = committed,
    external' = external,
  }

  action step = any { outsideWrite, reviewerEdit, approve, done }

  // Every committed tree was delivered to the reviewer.
  val CommittedWasDelivered = committed == -1 or delivered.contains(committed)
  // A commit is of the tree the reviewer held when it approved.
  val CommittedIsSeen = committed == -1 or committed == seen
  // The seen tree is always one whose bytes were delivered.
  val SeenWasDelivered = delivered.contains(seen)
}
