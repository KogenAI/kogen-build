package review

import "flag"

// update is the test-only `-update` flag of `go test ./... -update` (README, Golden files): every
// package of the module declares it, so that a test binary accepts the flag when it is given to
// all of them. This package has no golden file; the flag changes nothing here. It is read by
// tests only; the kogen program and this library have no such flag, setting or variable.
var update = flag.Bool("update", false, "rewrite the golden files of the tests from the current output instead of comparing with them")
