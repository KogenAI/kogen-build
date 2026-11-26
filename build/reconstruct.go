package build

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/KogenAI/kogen-coordinator/durable"
	"github.com/KogenAI/kogen-coordinator/git"
	"github.com/KogenAI/kogen-loop/sys"
)

// Safe reconstruction of one held consumer: Stack Menu Problem20. After the caller's evidence
// proves that nothing was published, that no native owner, process or custody remains, that the
// accepted request and baseline are unchanged and that the old generation is revoked, the route
// preserves the old evidence in a new archive, durably fences the old generation and records
// exactly one fresh attempt and one wake for it. It never runs Shape, Build, checks, Review,
// landing or deployment, never touches the queue item and never deletes or replaces evidence.

const (
	problem20Item       = "bb13338f-045e-4853-8cc4-90ec80dc8b4f"
	problem20Version    = 18
	problem20Obligation = "Problem20"
	problem20Document   = "queue-request-bb13338f-045e-4853-8cc4-90ec80dc8b4f"
	problem20Revision   = 3
	problem20RequestSHA = "ee87cf09b602ba3eec64112ecf8e85bbf1d539864308a2ab542ac467a9202cb4"

	reconstructionsDir = ".kogen/local/reconstructions"

	reconAbsent      = "absent"
	reconHeld        = "held"
	reconInterrupted = "interrupted"
	reconCreated     = "created"

	phasePreserved = "preserved"
	phaseFenced    = "fenced"
	phaseCreated   = "created"

	pending     = "pending"
	notObserved = "not_observed"

	unnamedOwner = "unnamed (the evidence names no owner)"
)

// ReconstructInput is one `reconstruct` run.
type ReconstructInput struct {
	Project string
	// Evidence is the caller's evidence file.
	Evidence string
	// Archive is the new absolute directory for the preserved evidence.
	Archive string
	// StopAfter ("preserved" or "fenced") ends the run right after that phase is durable.
	StopAfter string
}

// ReconstructionProblem is the structured error of a held, interrupted or absent reconstruction.
type ReconstructionProblem struct {
	Category string
	Kind     string
	Message  string
}

type ReconQueueItem struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
	State   string `json:"state"`
}

type ReconRequest struct {
	Document string `json:"document"`
	Revision int    `json:"revision"`
	SHA256   string `json:"sha256"`
}

type ReconBaseline struct {
	Head string `json:"head"`
}

type ReconReceipt struct {
	Build         string `json:"build"`
	ReceiptSHA256 string `json:"receipt_sha256"`
	CandidateTree string `json:"candidate_tree"`
	Path          string `json:"path"`
	CopySHA256    string `json:"copy_sha256"`
}

type ReconPrior struct {
	Receipts []ReconReceipt  `json:"receipts"`
	Attempts []string        `json:"attempts"`
	Spend    json.RawMessage `json:"spend"`
}

type ReconGeneration struct {
	Old string `json:"old"`
	New string `json:"new"`
}

type ReconAttempt struct {
	ID              string `json:"id"`
	Generation      string `json:"generation"`
	RequestRevision int    `json:"request_revision"`
	RequestSHA256   string `json:"request_sha256"`
	BaselineHead    string `json:"baseline_head"`
}

type ReconWake struct {
	ID      string `json:"id"`
	Attempt string `json:"attempt"`
}

type ReconGates struct {
	Admission string `json:"admission"`
	Shape     string `json:"shape"`
	Build     string `json:"build"`
	Checks    string `json:"checks"`
	Review    string `json:"review"`
}

// Reconstruction is the `reconstruction` object of the standalone envelope. An absent
// reconstruction carries only State, Attempts and Wakes.
type Reconstruction struct {
	State      string           `json:"state"`
	QueueItem  *ReconQueueItem  `json:"queue_item,omitempty"`
	Obligation string           `json:"obligation,omitempty"`
	Request    *ReconRequest    `json:"request,omitempty"`
	Baseline   *ReconBaseline   `json:"baseline,omitempty"`
	Archive    string           `json:"archive,omitempty"`
	Prior      *ReconPrior      `json:"prior,omitempty"`
	Generation *ReconGeneration `json:"generation,omitempty"`
	Attempts   []ReconAttempt   `json:"attempts"`
	Wakes      []ReconWake      `json:"wakes"`
	Gates      *ReconGates      `json:"gates,omitempty"`
	Landing    string           `json:"landing,omitempty"`
	Deployment string           `json:"deployment,omitempty"`
	Acceptance string           `json:"acceptance,omitempty"`
	Owner      string           `json:"owner,omitempty"`
	NextAction string           `json:"next_action,omitempty"`
	Reason     string           `json:"reason,omitempty"`
}

// reconstructionRecord is the durable record, keyed by queue item and evidence digest.
type reconstructionRecord struct {
	Phase          string         `json:"phase"`
	EvidenceSHA256 string         `json:"evidence_sha256"`
	Report         Reconstruction `json:"report"`
}

type reconEvidence struct {
	QueueItem *struct {
		ID      string `json:"id"`
		Version int    `json:"version"`
	} `json:"queue_item"`
	Obligation     string        `json:"obligation"`
	Owner          string        `json:"owner"`
	Request        *ReconRequest `json:"request"`
	CurrentRequest *struct {
		Revision int    `json:"revision"`
		SHA256   string `json:"sha256"`
	} `json:"current_request"`
	Baseline *ReconBaseline `json:"baseline"`
	Prior    *struct {
		Attempts *[]string `json:"attempts"`
		Builds   *[]struct {
			ID            string `json:"id"`
			ReceiptSHA256 string `json:"receipt_sha256"`
			CandidateTree string `json:"candidate_tree"`
			Path          string `json:"path"`
			CopySHA256    string `json:"copy_sha256"`
		} `json:"builds"`
		Spend json.RawMessage `json:"spend"`
	} `json:"prior"`
	Effects *struct {
		Publication string `json:"publication"`
		External    string `json:"external"`
	} `json:"effects"`
	Owners *[]struct {
		Name string `json:"name"`
	} `json:"owners"`
	Custody *struct {
		Generation string `json:"generation"`
		Revoked    *bool  `json:"revoked"`
	} `json:"custody"`
	Claims map[string]any `json:"claims"`
}

// hold is a refusal that names its reason, the actual owner and the next action.
type hold struct{ reason, owner, next string }

func (e *reconEvidence) owner() string {
	if e.Owner != "" {
		return e.Owner
	}
	return unnamedOwner
}

func (e *reconEvidence) holdOf(reason, next string) *hold {
	return &hold{reason, e.owner(), next}
}

func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return x != ""
	case float64:
		return x != 0
	case []any:
		return len(x) > 0
	case map[string]any:
		return len(x) > 0
	}
	return true
}

func gatesPending() *ReconGates {
	return &ReconGates{pending, pending, pending, pending, pending}
}

// prior echoes the evidence's identity values verbatim, resolving receipt paths against dir.
func (e *reconEvidence) prior(dir string) *ReconPrior {
	p := &ReconPrior{Receipts: []ReconReceipt{}, Attempts: []string{}}
	if e.Prior == nil {
		return p
	}
	if e.Prior.Builds != nil {
		for _, b := range *e.Prior.Builds {
			p.Receipts = append(p.Receipts, ReconReceipt{b.ID, b.ReceiptSHA256, b.CandidateTree, resolveFrom(dir, b.Path), b.CopySHA256})
		}
	}
	if e.Prior.Attempts != nil {
		p.Attempts = append(p.Attempts, *e.Prior.Attempts...)
	}
	if len(e.Prior.Spend) > 0 {
		p.Spend = slices.Clone(e.Prior.Spend)
	}
	return p
}

func resolveFrom(dir, p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(dir, p)
}

// report is the reconstruction object of the held consumer, before any phase is durable.
func (e *reconEvidence) report(dir, state string) Reconstruction {
	r := Reconstruction{
		State: state, Attempts: []ReconAttempt{}, Wakes: []ReconWake{},
		QueueItem: &ReconQueueItem{State: reconHeld}, Obligation: e.Obligation,
		Request: &ReconRequest{}, Baseline: &ReconBaseline{},
		Prior: e.prior(dir), Gates: gatesPending(),
		Landing: notObserved, Deployment: notObserved, Acceptance: notObserved,
		Owner: e.owner(),
	}
	if e.QueueItem != nil {
		r.QueueItem.ID, r.QueueItem.Version = e.QueueItem.ID, e.QueueItem.Version
	}
	if e.Request != nil {
		*r.Request = *e.Request
	}
	if e.Baseline != nil {
		*r.Baseline = *e.Baseline
	}
	return r
}

func (e *reconEvidence) heldReport(dir string, h *hold) Reconstruction {
	r := e.report(dir, reconHeld)
	r.Owner, r.NextAction, r.Reason = h.owner, h.next, h.reason
	return r
}

func recordPath(root, item, digest string) string {
	return filepath.Join(root, reconstructionsDir, item+"-"+digest+".json")
}

// safeItem reports whether an item id can name a record file.
func safeItem(item string) bool {
	return item != "" && item != "." && item != ".." && !strings.ContainsAny(item, "/\\\x00") && !strings.HasPrefix(item, "-")
}

// recordsFor lists the durable records of a queue item, with their evidence digests.
func recordsFor(root, item string) (map[string]*reconstructionRecord, error) {
	out := map[string]*reconstructionRecord{}
	if !safeItem(item) {
		return out, nil
	}
	dir := filepath.Join(root, reconstructionsDir)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		name := e.Name()
		digest, ok := strings.CutPrefix(name, item+"-")
		if !ok || !strings.HasSuffix(digest, ".json") {
			continue
		}
		digest = strings.TrimSuffix(digest, ".json")
		if len(digest) != 64 {
			continue
		}
		raw, err := readBounded(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		var rec reconstructionRecord
		if err := json.Unmarshal(raw, &rec); err != nil || rec.EvidenceSHA256 != digest || rec.Report.QueueItem == nil || rec.Report.QueueItem.ID != item {
			return nil, fmt.Errorf("%s is not a valid reconstruction record", filepath.Join(dir, name))
		}
		out[digest] = &rec
	}
	return out, nil
}

// publishRecord atomically and durably replaces the record of this item and evidence digest.
func publishRecord(root string, rec *reconstructionRecord) error {
	dir := filepath.Join(root, reconstructionsDir)
	if err := mkdirPrivate(dir); err != nil {
		return err
	}
	b, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	path := recordPath(root, rec.Report.QueueItem.ID, rec.EvidenceSHA256)
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	werr := f.Chmod(0o600)
	if werr == nil {
		_, werr = f.Write(append(b, '\n'))
	}
	if werr == nil {
		werr = durable.Sync(f)
	}
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		os.Remove(tmp)
		return werr
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	if err := syncDir(dir); err != nil {
		return err
	}
	return syncDir(filepath.Dir(dir))
}

// ReadReconstruction is the read-only readback of a queue item's reconstruction. It creates,
// fences and archives nothing. The exit status is 0 for a created reconstruction and 1 otherwise.
func ReadReconstruction(project, item string, env []string) (Reconstruction, int, *ReconstructionProblem) {
	absent := Reconstruction{State: reconAbsent, Attempts: []ReconAttempt{}, Wakes: []ReconWake{}}
	problem := func(kind, msg string) (Reconstruction, int, *ReconstructionProblem) {
		return absent, 1, &ReconstructionProblem{"reconstruction", kind, msg}
	}
	repo, err := git.OpenReadOnly(project, env)
	if err != nil {
		return problem("unreadable", continuationDetail(err))
	}
	recs, err := recordsFor(repo.Dir, item)
	if err != nil {
		return problem("unreadable", err.Error())
	}
	if len(recs) == 0 {
		return problem(reconAbsent, "no reconstruction is recorded for queue item "+item)
	}
	digests := make([]string, 0, len(recs))
	for d := range recs {
		digests = append(digests, d)
	}
	slices.Sort(digests)
	rec := recs[digests[0]]
	if rec.Report.State == reconCreated {
		return rec.Report, 0, nil
	}
	return rec.Report, 1, &ReconstructionProblem{"reconstruction", reconInterrupted, rec.Report.NextAction}
}

// Reconstruct runs the reconstruction of the Problem20 consumer. It returns the report, the
// exit status (0 created, 1 held or interrupted) and the structured error of a non-created
// reconstruction.
func Reconstruct(s *sys.Session, in ReconstructInput) (Reconstruction, int, *ReconstructionProblem) {
	evPath, err := filepath.Abs(in.Evidence)
	if err != nil {
		evPath = in.Evidence
	}
	dir := filepath.Dir(evPath)
	heldProblem := func(r Reconstruction) (Reconstruction, int, *ReconstructionProblem) {
		s.Progress("kogen: reconstruction held: " + r.Reason + "; next: " + r.NextAction)
		return r, 1, &ReconstructionProblem{"reconstruction", reconHeld, r.Reason}
	}
	raw, err := readBounded(evPath)
	if err != nil {
		e := &reconEvidence{}
		return heldProblem(e.heldReport(dir, e.holdOf("the evidence file cannot be read: "+err.Error(), "supply a readable evidence file with --evidence")))
	}
	var ev reconEvidence
	if err := json.Unmarshal(raw, &ev); err != nil {
		return heldProblem(ev.heldReport(dir, ev.holdOf("the evidence file is not the expected JSON object: "+err.Error(), "correct the evidence file and rerun")))
	}
	digest := sha(raw)
	repo, err := git.OpenReadOnly(in.Project, s.Env)
	if err != nil {
		return heldProblem(ev.heldReport(dir, ev.holdOf("the checkout cannot be opened: "+continuationDetail(err), "name the Problem20 checkout with --project")))
	}
	root := repo.Dir

	if h := ev.identityHold(); h != nil {
		return heldProblem(ev.heldReport(dir, h))
	}
	recs, err := recordsFor(root, problem20Item)
	if err != nil {
		return heldProblem(ev.heldReport(dir, ev.holdOf("the recorded reconstruction is unreadable: "+err.Error(), "repair or inspect "+reconstructionsDir+" without deleting evidence")))
	}
	for d := range recs {
		if d != digest {
			return heldProblem(ev.heldReport(dir, ev.holdOf("a different reconstruction is already recorded for this queue item (duplicate reconciliation, evidence "+d+")", "read it back with the reconstruction mode; do not reconcile again with different evidence")))
		}
	}
	if rec := recs[digest]; rec != nil && rec.Phase == phaseCreated {
		s.Progress("kogen: reconstruction already created; reporting it unchanged")
		return rec.Report, 0, nil
	}

	locked := false
	prove := func() (*hold, map[string][]byte) {
		return ev.eligibility(root, dir, repo.Head(), !locked, recs[digest])
	}
	if h, _ := prove(); h != nil {
		return heldProblem(ev.heldReport(dir, h))
	}
	if recs[digest] == nil {
		if h := ev.archiveHold(in.Archive, digest); h != nil {
			return heldProblem(ev.heldReport(dir, h))
		}
	}

	lock, err := git.AcquireBuild(root, s)
	if err != nil {
		return heldProblem(ev.heldReport(dir, ev.holdOf("the ordinary native Build lock is held: "+continuationDetail(err), "wait for the running Build to finish, then rerun")))
	}
	defer lock.Release()
	locked = true
	// Under the lock another run may have progressed; prove everything again.
	recs, err = recordsFor(root, problem20Item)
	if err != nil {
		return heldProblem(ev.heldReport(dir, ev.holdOf("the recorded reconstruction is unreadable: "+err.Error(), "repair or inspect "+reconstructionsDir+" without deleting evidence")))
	}
	for d := range recs {
		if d != digest {
			return heldProblem(ev.heldReport(dir, ev.holdOf("a different reconstruction is already recorded for this queue item (duplicate reconciliation, evidence "+d+")", "read it back with the reconstruction mode; do not reconcile again with different evidence")))
		}
	}
	rec := recs[digest]
	if rec != nil && rec.Phase == phaseCreated {
		return rec.Report, 0, nil
	}
	h, receipts := prove()
	if h != nil {
		return heldProblem(ev.heldReport(dir, h))
	}

	fail := func(what string, err error) (Reconstruction, int, *ReconstructionProblem) {
		msg := what + ": " + err.Error()
		if rec != nil {
			r := rec.Report
			return r, 1, &ReconstructionProblem{"reconstruction", reconInterrupted, msg}
		}
		return heldProblem(ev.heldReport(dir, ev.holdOf(msg, "correct the cause and rerun with the same evidence")))
	}

	stopped := func(phase string) bool { return in.StopAfter == phase }
	interrupted := func() (Reconstruction, int, *ReconstructionProblem) {
		s.Progress("kogen: reconstruction interrupted after the " + rec.Phase + " phase")
		return rec.Report, 1, &ReconstructionProblem{"reconstruction", reconInterrupted, "stopped after the " + rec.Phase + " phase; " + rec.Report.NextAction}
	}

	// Phase 1: preserve.
	if rec == nil {
		archive := in.Archive
		s.Progress("kogen: preserving prior evidence in " + archive)
		if err := ev.preserve(root, dir, raw, digest, archive, receipts); err != nil {
			return fail("preserving the prior evidence failed", err)
		}
		r := ev.report(dir, reconInterrupted)
		r.Archive = archive
		r.NextAction = "rerun reconstruct with the same evidence file to fence the old generation and create the fresh attempt"
		rec = &reconstructionRecord{Phase: phasePreserved, EvidenceSHA256: digest, Report: r}
		if err := publishRecord(root, rec); err != nil {
			rec = nil
			return fail("recording the preserved phase failed", err)
		}
	}
	if stopped(phasePreserved) && rec.Phase == phasePreserved {
		return interrupted()
	}
	// Phase 2: fence the old generation and allocate the new one.
	if rec.Phase == phasePreserved {
		s.Progress("kogen: fencing the old generation " + ev.Custody.Generation)
		next := *rec
		next.Phase = phaseFenced
		next.Report.Generation = &ReconGeneration{Old: ev.Custody.Generation, New: freshID("gen-", 16, ev.usedIdentifiers())}
		next.Report.NextAction = "rerun reconstruct with the same evidence file to create the fresh attempt and wake"
		if err := publishRecord(root, &next); err != nil {
			return fail("recording the fenced phase failed", err)
		}
		rec = &next
	}
	if stopped(phaseFenced) && rec.Phase == phaseFenced {
		return interrupted()
	}
	// Phase 3: create exactly one attempt and one wake.
	s.Progress("kogen: creating the fresh attempt bound to the original request and baseline")
	next := *rec
	used := append(ev.usedIdentifiers(), next.Report.Generation.New)
	attempt := ReconAttempt{
		ID: freshID("", 16, used), Generation: next.Report.Generation.New,
		RequestRevision: problem20Revision, RequestSHA256: problem20RequestSHA, BaselineHead: ev.Baseline.Head,
	}
	wake := ReconWake{ID: freshID("", 16, append(used, attempt.ID)), Attempt: attempt.ID}
	next.Phase = phaseCreated
	next.Report.State = reconCreated
	next.Report.Attempts = []ReconAttempt{attempt}
	next.Report.Wakes = []ReconWake{wake}
	next.Report.NextAction = "run the ordinary Shape, Build, checks and Review for attempt " + attempt.ID + "; Problem20 stays held until the ordinary pipeline proves its outcome"
	if err := publishRecord(root, &next); err != nil {
		return fail("recording the created attempt failed", err)
	}
	return next.Report, 0, nil
}

// usedIdentifiers are every prior id and hash a fresh identifier must differ from.
func (e *reconEvidence) usedIdentifiers() []string {
	used := []string{problem20Item, problem20RequestSHA, e.Baseline.Head, e.Custody.Generation}
	used = append(used, *e.Prior.Attempts...)
	for _, b := range *e.Prior.Builds {
		used = append(used, b.ID, b.ReceiptSHA256, b.CandidateTree, b.CopySHA256)
	}
	return used
}

func freshID(prefix string, n int, used []string) string {
	for {
		b := make([]byte, n)
		if _, err := rand.Read(b); err != nil {
			panic("reconstruction: no randomness: " + err.Error())
		}
		id := prefix + hex.EncodeToString(b)
		if !slices.Contains(used, id) {
			return id
		}
	}
}

// identityHold refuses everything but the Problem20 consumer.
func (e *reconEvidence) identityHold() *hold {
	next := "this route reconstructs only Problem20 (queue item " + problem20Item + ", request " + problem20Document + " revision 3); use the ordinary pipeline for anything else"
	if e.QueueItem == nil || e.QueueItem.ID != problem20Item {
		return e.holdOf("the queue item is not Problem20 "+problem20Item, next)
	}
	if e.QueueItem.Version != problem20Version {
		return e.holdOf(fmt.Sprintf("the queue item version is not %d", problem20Version), next)
	}
	if e.Obligation != problem20Obligation {
		return e.holdOf("the obligation is not "+problem20Obligation, next)
	}
	if e.Request == nil || *e.Request != (ReconRequest{problem20Document, problem20Revision, problem20RequestSHA}) {
		return e.holdOf("the original accepted request is not "+problem20Document+" revision 3 with its recorded SHA-256", next)
	}
	return nil
}

// eligibility proves every prerequisite and returns the retained receipt bytes. Nothing is
// written. The Build lock is only inspected when checkLock is set (it is ours once taken). A
// reconstruction that already fenced the old generation is not re-refused for what it fenced.
func (e *reconEvidence) eligibility(root, dir, head string, checkLock bool, rec *reconstructionRecord) (*hold, map[string][]byte) {
	if h := e.identityHold(); h != nil {
		return h, nil
	}
	if e.Effects == nil || e.Effects.Publication != "none" || e.Effects.External != "none" {
		return e.holdOf("publication and external side effects are not proved absent (effects must be present and both \"none\")",
			"prove with evidence that no publication and no external side effect occurred, then rerun"), nil
	}
	if e.Owners == nil {
		return e.holdOf("the absence of a current native owner, process or custody is not proved (owners is absent)",
			"list the live owners, or supply an empty owners list as proof that none remains"), nil
	}
	if len(*e.Owners) > 0 {
		o := (*e.Owners)[0].Name
		if o == "" {
			o = "unnamed live owner"
		}
		return &hold{"a current native owner, process or custody remains: " + o, o,
			"wait for " + o + " to finish or release the obligation, then supply evidence that no owner remains"}, nil
	}
	if e.Custody == nil || e.Custody.Revoked == nil || !*e.Custody.Revoked || e.Custody.Generation == "" {
		return e.holdOf("the old generation's single-writer custody is not proved revoked and fenced",
			"revoke the old generation's custody, then supply custody with its generation and revoked=true"), nil
	}
	if e.CurrentRequest == nil || e.CurrentRequest.Revision != e.Request.Revision || e.CurrentRequest.SHA256 != e.Request.SHA256 {
		return e.holdOf("the request the queue holds now differs from the original accepted request revision or SHA-256",
			"restore the accepted request revision 3, or refine and approve a new Intent; this route binds only the original request"), nil
	}
	if e.Baseline == nil || e.Baseline.Head == "" || e.Baseline.Head != head {
		return e.holdOf("the checkout's HEAD is not the original baseline commit",
			"restore the checkout to the original baseline commit "+e.baselineHead()+" or supply evidence for the checkout's baseline"), nil
	}
	if e.Prior == nil || e.Prior.Attempts == nil || e.Prior.Builds == nil || len(e.Prior.Spend) == 0 || string(bytes.TrimSpace(e.Prior.Spend)) == "null" {
		return e.holdOf("the prior attempts, builds or spend are absent from the evidence",
			"supply every prior attempt id, failed Build receipt and the spend"), nil
	}
	receipts := map[string][]byte{}
	for i, b := range *e.Prior.Builds {
		if b.ID == "" || b.Path == "" || b.CopySHA256 == "" {
			return e.holdOf(fmt.Sprintf("prior build %d lacks its id, retained receipt path or copy SHA-256", i+1),
				"supply each prior build's id, receipt path and the SHA-256 of that file"), nil
		}
		path := resolveFrom(dir, b.Path)
		data, err := readBounded(path)
		if err != nil {
			return e.holdOf("the retained receipt of build "+b.ID+" is absent or unreadable: "+err.Error(),
				"restore the retained receipt file "+path+" byte-identically; it is never regenerated"), nil
		}
		if sha(data) != b.CopySHA256 {
			return e.holdOf("the retained receipt of build "+b.ID+" does not hash to its recorded copy SHA-256",
				"restore the retained receipt file "+path+" to the recorded bytes; it is never replaced"), nil
		}
		receipts[path] = data
	}
	for k, v := range e.Claims {
		if truthy(v) {
			return e.holdOf("the evidence claims "+k+", which this route refuses: it observes no landing, deployment or acceptance and accepts no old Build, check or Review result as satisfying a gate",
				"remove the claim; the fresh attempt must pass the ordinary pipeline's gates itself"), nil
		}
	}
	if checkLock {
		if held, err := git.NewLock(git.BuildLockPath(root)).Held(); held || err != nil {
			return e.holdOf("the ordinary native Build lock of the checkout is held by another process",
				"wait for that Build to finish, then rerun"), nil
		}
	}
	if rec != nil {
		if _, err := os.Stat(filepath.Join(rec.Report.Archive, "manifest.json")); err != nil {
			return e.holdOf("the archive preserved by this reconstruction is missing: "+rec.Report.Archive,
				"restore the preserved archive "+rec.Report.Archive+"; it is never recreated over a different one"), nil
		}
	}
	return nil, receipts
}

func (e *reconEvidence) baselineHead() string {
	if e.Baseline == nil {
		return "(none named)"
	}
	return e.Baseline.Head
}

// archiveHold refuses an archive destination that is not new (and not this reconstruction's own).
func (e *reconEvidence) archiveHold(archive, digest string) *hold {
	if !filepath.IsAbs(archive) {
		return e.holdOf("the archive destination is not an absolute path: "+archive, "name a new absolute directory with --archive")
	}
	st, err := os.Lstat(archive)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err == nil && st.IsDir() {
		if raw, err := readBounded(filepath.Join(archive, "manifest.json")); err == nil {
			var m struct {
				Evidence string `json:"evidence_sha256"`
			}
			if json.Unmarshal(raw, &m) == nil && m.Evidence == digest {
				return nil
			}
		}
	}
	return e.holdOf("the archive destination exists and is not this reconstruction's own archive: "+archive, "name a new directory with --archive")
}

// preserve writes the complete archive beside its destination and publishes it with one rename,
// so the archive either does not exist or is complete and is never modified afterwards.
func (e *reconEvidence) preserve(root, dir string, raw []byte, digest, archive string, receipts map[string][]byte) error {
	if st, err := os.Lstat(archive); err == nil && st.IsDir() {
		return nil // this reconstruction's own, already complete archive
	}
	parent := filepath.Dir(archive)
	if err := mkdirPrivate(parent); err != nil {
		return err
	}
	stage := archive + "." + digest[:16] + ".partial"
	if st, err := os.Lstat(stage); err == nil && st.IsDir() {
		if err := os.RemoveAll(stage); err != nil { // our own unpublished staging, never evidence
			return err
		}
	}
	if err := mkdirPrivate(stage); err != nil {
		return err
	}
	if err := writeNew(stage, "evidence.json", raw); err != nil {
		return err
	}
	type file struct {
		Name   string `json:"name"`
		Source string `json:"source"`
		SHA256 string `json:"sha256"`
	}
	files := []file{{"evidence.json", "", digest}}
	for i, r := range e.prior(dir).Receipts {
		name := fmt.Sprintf("receipts/%02d-%s", i+1, filepath.Base(r.Path))
		if err := writeNew(stage, name, receipts[r.Path]); err != nil {
			return err
		}
		files = append(files, file{name, r.Path, r.CopySHA256})
	}
	base := e.report(dir, reconInterrupted)
	manifest, err := json.MarshalIndent(struct {
		EvidenceSHA256 string          `json:"evidence_sha256"`
		QueueItem      *ReconQueueItem `json:"queue_item"`
		Obligation     string          `json:"obligation"`
		Request        *ReconRequest   `json:"request"`
		Baseline       *ReconBaseline  `json:"baseline"`
		Prior          *ReconPrior     `json:"prior"`
		Files          []file          `json:"files"`
	}{digest, base.QueueItem, base.Obligation, base.Request, base.Baseline, base.Prior, files}, "", "  ")
	if err != nil {
		return err
	}
	if err := writeNew(stage, "manifest.json", append(manifest, '\n')); err != nil {
		return err
	}
	if err := syncDirectories(stage); err != nil {
		return err
	}
	if _, err := os.Lstat(archive); err == nil {
		return fmt.Errorf("%s appeared while preserving", archive)
	}
	if err := os.Rename(stage, archive); err != nil {
		return err
	}
	return syncDir(parent)
}
