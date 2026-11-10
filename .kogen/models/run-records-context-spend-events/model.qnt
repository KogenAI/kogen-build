module run_records {
  var blobs: Set[int]
  var refs: List[int]
  var calls: Set[int]
  var responses: Set[int]
  val AllReferencesResolve = refs.indices().forall(i => blobs.contains(refs[i]))
  val CallsHaveResponses = calls.subseteq(responses)
  action init = all {
    blobs' = Set(), refs' = [], calls' = Set(), responses' = Set()
  }
  action record = {
    nondet h = Set(1, 2, 3).oneOf()
    all { blobs' = blobs.union(Set(h)), refs' = refs.append(h), calls' = calls, responses' = responses }
  }
  action response = {
    nondet c = Set(1, 2, 3).oneOf()
    all { responses' = responses.union(Set(c)), calls' = calls, blobs' = blobs, refs' = refs }
  }
  action execute = {
    nondet c = Set(1, 2, 3).oneOf()
    all { responses.contains(c), calls' = calls.union(Set(c)), responses' = responses, blobs' = blobs, refs' = refs }
  }
  action step = any { record, response, execute }
}
