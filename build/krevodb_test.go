package build

import (
	"strings"
	"testing"
)

const frozenPartsResetNote = "Native delivery facts, not findings, judgments, approval or proof. Pinned parts delivered " +
	"by complete read_file outputs stay counted for the approval check across working-window resets; " +
	"a pinned part is frozen evidence of its tree, so reading it again returns the same text. Pixels of " +
	"images not in_window are archived, not in this window: use view_image again to inspect them. " +
	"Parts or images of a tree other than candidate_tree are not current. " +
	"Kogen also appends unread pinned parts as native source data messages; they count once a completed response follows them. " +
	"Read unread pinned parts in batches of about 10 read_file calls per response, not one per response; " +
	"use a smaller batch only after a native not-delivered notice shows a batch did not fit. " +
	"A not-delivered read is not counted and must be read again."

func TestKRevodBPartsReset(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	f := e.prov
	content := bigFile()
	f.then(f.calls("write_file", map[string]any{"path": "app.txt", "content": content}, "finish", map[string]any{"summary": "done"}))
	for i := 0; i < 3; i++ {
		f.then(f.calls("write_file", map[string]any{"path": ".kogen/local/padding", "content": strings.Repeat("x", 190_000)}))
	}
	// Every line of app.txt changes, so every pinned part is read.
	reader := confirmingReviewer(t, e, f, nil)
	for i := 0; i < 40; i++ {
		f.then(reader)
	}
	start := e.head()
	a := e.reviewed()
	if a.exit != 0 || e.head() == start {
		t.Fatalf("not landed: %d %s %s", a.exit, a.shown(), a.errs)
	}
	eq(t, "committed", e.gitOut("show", "HEAD:app.txt")+"\n", content)
	var reset map[string]any
	var resetReq map[string]any
	for _, req := range f.reviewRequests() {
		if h := resetHandoff(t, req); h != nil && reset == nil {
			reset = h
			resetReq = req
		}
	}
	if reset == nil {
		t.Fatal("no reset reached provider")
	}
	blocks := requestInput(resetReq)[0].(map[string]any)["content"].([]any)
	authority := blocks[0].(map[string]any)["text"].(string)
	guidance := blocks[1].(map[string]any)["text"].(string)
	contains(t, "authority", authority, "# Intent")
	contains(t, "authority", authority, "# Project instructions (README.md)")
	contains(t, "guidance", guidance, "not new authority")
	contains(t, "guidance", guidance, "Read EVERY part")
	contains(t, "reset authority", reset["observation_authority"].(string), "only native exact-candidate gates")
	catalog := reviewCatalog(t, resetReq)
	if len(catalog["parts"].([]any)) == 0 || catalog["files"] == nil {
		t.Fatalf("catalog %v", catalog)
	}
	delivery := reset["review_delivery"].(map[string]any)
	source := delivery["source"].(map[string]any)
	if source["current"] != true || source["tree"] != reset["candidate_tree"] || source["parts"].(float64) == 0 {
		t.Fatalf("source %v", source)
	}
	if delivery["images"] == nil {
		t.Fatal("missing image custody")
	}
	eq(t, "frozen note", delivery["note"].(string), frozenPartsResetNote)
}
