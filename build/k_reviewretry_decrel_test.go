//go:build !race

package build

import (
	"net/http"
	"os"
	"strings"
	"testing"
)

// The S-DECREL case pins 232 source parts, and the reverse-order case in a fresh review 113 more, the shape of the historical failure, and drives a
// real review across several working windows: it is the heaviest fixture of the Build tests
// (hundreds of private files and 500 KB checkpoints per request). It runs in the ordinary test
// run and in the acceptance bridge, and is left out of the race detector's run, whose cost it
// would add to the package's whole time budget although the scripted flow has a single
// conversation and no concurrency of its own.
func init() {
	heavyAllowanceCases = append(heavyAllowanceCases, func(t *testing.T, kit allowanceKit) {
		// An approval given while parts are unread is rejected (the rejected-verdict allowance), a
		// prose-only turn then is sent back (the other allowance), and the review stays one review
		// in one conversation until an approval comes after every part was delivered.
		t.Run("S-DECREL: unread rejections, prose, then the forced verdict in the same conversation", func(t *testing.T) {
			t.Parallel()
			e, f := kit.build(t, pinned232())
			var carried int
			f.then(func(w http.ResponseWriter, r *http.Request) {
				// Kogen's own custody at this approval: the parts its prefill carried to the
				// requests so far. A read is not needed: the fixture delivers by prefill alone.
				carried = len(prefilledNumbers(e.events(e.onlyRun()), 1))
				submit(f, "approve")(w, r)
			})
			f.then(repeatHandler(deliverThen(e, f, prose("I have reviewed everything."),
				submit(f, "approve", finding("note", "fine"))), 40)...) // the last one is the reminder request: forced
			kit.check(t, e, f, pinned232(), 232)
			if unread := 232 - carried; unread <= 0 || unread >= 232 {
				t.Fatalf("unread %d at the rejected approval: delivery was catalogued, not real", unread)
			}
		})
	})

	heavyAllowanceCases = append(heavyAllowanceCases, func(t *testing.T, kit allowanceKit) {
		// The reverse order runs as the fresh review 2 of a Build whose review 1 ended on two
		// prose-only turns, so it also shows that a fresh native review keeps its own delivery rule:
		// review 1's native delivery of its own parts counts for nothing in review 2.
		t.Run("reverse order in a fresh review: prose, then a rejected verdict, then a valid one", func(t *testing.T) {
			t.Parallel()
			e, f := kit.build(t, pinned113())
			var carried int
			f.then(prose("a"), prose("b"), // review 1: two prose-only turns end it
				prose("Looks fine to me."), // review 2
				func(w http.ResponseWriter, r *http.Request) { // forced; still rejected: forcing is not delivery
					carried = len(prefilledNumbers(e.events(e.onlyRun()), 2))
					submit(f, "approve")(w, r)
				})
			f.then(repeatHandler(deliverThen(e, f, submit(f, "approve", finding("note", "fine"))), 40)...)
			a := e.reviewed()
			if a.exit != 0 {
				t.Fatalf("exit %d\n%s", a.exit, a.shown())
			}
			id := e.onlyRun()
			eq(t, "review 1 parts", len(pinnedPartFiles(e.root, 1)), 113)
			second := pinnedPartFiles(e.root, 2)
			eq(t, "review 2 parts", len(second), 113)
			for _, p := range second {
				if !strings.Contains(p, "/review-source/2-") {
					t.Fatalf("review 2 part %s is not its own", p)
				}
			}
			// Review 2 is a conversation of its own.
			keys := map[string]bool{}
			for _, req := range f.reviewRequests() {
				if cacheKey(req) == "" {
					t.Fatal("a review request without a cache key")
				}
				keys[cacheKey(req)] = true
			}
			eq(t, "conversations", len(keys), 2)
			events := e.events(id)
			res := e.result(id)
			eq(t, "reviews", res["reviews"], any(float64(2)))
			if _, err := os.Stat(e.runPath(id, "review-1.json")); !os.IsNotExist(err) {
				t.Fatal("an invalid review has a review record")
			}
			var first map[string]any
			for _, ev := range eventsNamed(events, "review") {
				if ev["review"] == float64(1) {
					first = ev
				}
			}
			if first == nil {
				t.Fatal("no verdict event for review 1")
			}
			eq(t, "review 1", first["reason"], any("no_tool_reminder_exhausted"))
			eq(t, "review 2", e.jsonFile(e.runPath(id, "review-2.json"))["verdict"], any("approve"))
			// The forced approval was rejected for the parts review 2 had not been given: only the
			// prefill of its own requests before that moment counts.
			if unread := 113 - carried; unread <= 0 || unread >= 113 {
				t.Fatalf("unread %d at the forced approval: delivery was catalogued, not real", unread)
			}
			// Review 2's only accepted verdict came after all of its parts were delivered.
			if material := materialEventsOf(events, 1); len(material) != 0 {
				t.Fatalf("review 1 accepted a verdict: %v", material)
			}
			material := materialEventsOf(events, 2)
			if len(material) == 0 {
				t.Fatal("review 2 accepted no verdict")
			}
			for _, ev := range material {
				eq(t, "parts read", ev["parts_read"], any(float64(113)))
			}
		})
	})
}
