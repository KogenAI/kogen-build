package build

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/KogenAI/kogen-verification/verify"
)

// The baseline reuse shadow only observes. For each check the baseline runs, it asks whether
// an earlier green result of the same check (same command, start tree and check environment, by
// Verification's reuse key) could have been reused, still runs the check exactly as before and
// logs one reuse_shadow event. It keeps the receipts that make a later Build's answer possible.
// No check is skipped and nothing here changes a verdict, an order, an exit code or another
// event; a failure of the shadow is logged once as reuse_shadow_unavailable and nothing more.

const (
	reuseReceiptFile  = "kogen-reuse-shadow"
	reuseReceiptBound = 2000
	reuseSourceBase   = "baseline"
	reuseSourcePass   = "pass"
)

// newReuseSnapshot is the Verification snapshot of a tree; tests count its calls through it.
var newReuseSnapshot = verify.NewReuseSnapshot

// reuseReceipt is one line of the receipt file: a check's run under its reuse key.
type reuseReceipt struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	Status int    `json:"status"`
	WallMS int64  `json:"wall_ms"`
	Source string `json:"source"`
	Log    string `json:"log"`
}

// reuseShadowState is what the shadow keeps for one run log: whether it already reported itself
// unavailable (at most once per run log), and how far the pass events of the run log are read.
type reuseShadowState struct {
	mu          sync.Mutex
	unavailable bool
	offset      int64
	pending     map[string]string // check name -> reuse_sha256 of its command event, until its result
}

var reuseShadowStates sync.Map // *Run -> *reuseShadowState

// reuseShadowState is the state of the Build's run log, nil without a run log.
func (b *Attempt) reuseShadowState() *reuseShadowState {
	if b.Run == nil || b.Run.LogPath == "" {
		return nil
	}
	st, _ := reuseShadowStates.LoadOrStore(b.Run, &reuseShadowState{pending: map[string]string{}})
	return st.(*reuseShadowState)
}

// reuseShadowUnavailable records that the shadow cannot work, once per run log.
func (b *Attempt) reuseShadowUnavailable(reason string) {
	st := b.reuseShadowState()
	if st == nil {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.unavailable {
		return
	}
	st.unavailable = true
	if strings.TrimSpace(reason) == "" {
		reason = "unknown failure"
	}
	b.Run.Event(map[string]any{"event": "reuse_shadow_unavailable", "reason": reason})
}

// reuseShadowOff reports whether the shadow cannot work in this run log: it then does nothing.
func (b *Attempt) reuseShadowOff() bool {
	st := b.reuseShadowState()
	if st == nil {
		return true
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.unavailable
}

// reuseReceiptPath is the receipt file, directly under the repository's git common directory
// (next to kogen-baselines).
func reuseReceiptPath(root string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--git-common-dir")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git common directory: %w", err)
	}
	common := strings.TrimSpace(string(out))
	if !filepath.IsAbs(common) {
		common = filepath.Join(root, common)
	}
	common, err = filepath.EvalSymlinks(common)
	if err != nil {
		return "", fmt.Errorf("git common directory: %w", err)
	}
	return filepath.Join(common, reuseReceiptFile), nil
}

// readReuseReceipts reads the receipt file; a missing file holds no receipt, an unreadable or
// corrupt one is an error.
func readReuseReceipts(path string) ([]reuseReceipt, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("receipt file: %w", err)
	}
	var out []reuseReceipt
	lines := bytes.Split(raw, []byte("\n"))
	for n, line := range lines {
		if n == len(lines)-1 && len(line) == 0 {
			continue
		}
		var fields struct {
			Key    *string `json:"key"`
			Name   *string `json:"name"`
			Status *int    `json:"status"`
			WallMS *int64  `json:"wall_ms"`
			Source *string `json:"source"`
			Log    *string `json:"log"`
		}
		if err := json.Unmarshal(line, &fields); err != nil || fields.Key == nil || fields.Name == nil || fields.Status == nil || fields.WallMS == nil || fields.Source == nil || fields.Log == nil || *fields.Key == "" || *fields.Name == "" || *fields.Source == "" || *fields.Log == "" {
			return nil, fmt.Errorf("receipt file %s is corrupt at line %d", path, n+1)
		}
		out = append(out, reuseReceipt{Key: *fields.Key, Name: *fields.Name, Status: *fields.Status, WallMS: *fields.WallMS, Source: *fields.Source, Log: *fields.Log})
	}
	return out, nil
}

// appendReuseReceipts appends receipts to the file and keeps its newest reuseReceiptBound lines.
func appendReuseReceipts(path string, receipts ...reuseReceipt) error {
	var add []byte
	for _, r := range receipts {
		line, err := json.Marshal(r)
		if err != nil {
			return err
		}
		add = append(add, append(line, '\n')...)
	}
	old, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("receipt file: %w", err)
	}
	if bytes.Count(old, []byte("\n"))+len(receipts) <= reuseReceiptBound {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
		if err != nil {
			return fmt.Errorf("receipt file: %w", err)
		}
		_, err = f.Write(add)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return fmt.Errorf("receipt file: %w", err)
		}
		return nil
	}
	all := bytes.Split(bytes.TrimSuffix(append(old, add...), []byte("\n")), []byte("\n"))
	all = all[len(all)-reuseReceiptBound:]
	f, err := os.CreateTemp(filepath.Dir(path), ".reuse-shadow-")
	if err != nil {
		return fmt.Errorf("receipt file: %w", err)
	}
	defer os.Remove(f.Name())
	_, err = f.Write(append(bytes.Join(all, []byte("\n")), '\n'))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(f.Name(), path)
	}
	if err != nil {
		return fmt.Errorf("receipt file: %w", err)
	}
	return nil
}

// baselineShadow is the shadow of one baseline: the receipt file as the baseline began, and the
// one Verification snapshot of the baseline's start tree.
type baselineShadow struct {
	b        *Attempt
	path     string
	receipts []reuseReceipt
	snapshot verify.ReuseSnapshot
}

// shadowProbe is the question asked of the shadow before a check runs.
type shadowProbe struct {
	key string
	hit *reuseReceipt
}

// reuseShadowBegin starts the shadow of a baseline: it reads the receipt file and takes the
// snapshot once, for every check of the baseline. tree is the start tree when the caller knows
// it. Nil when the shadow is unavailable; the other methods then do nothing.
func (b *Attempt) reuseShadowBegin(tree string) *baselineShadow {
	if b.reuseShadowOff() {
		return nil
	}
	path, err := reuseReceiptPath(b.Root)
	if err != nil {
		b.reuseShadowUnavailable(err.Error())
		return nil
	}
	receipts, err := readReuseReceipts(path)
	if err != nil {
		b.reuseShadowUnavailable(err.Error())
		return nil
	}
	if tree == "" {
		if tree, err = snapshotTree(b.checkRepo(), b.Start, b.Excluded); err != nil {
			b.reuseShadowUnavailable("start tree: " + err.Error())
			return nil
		}
	}
	return &baselineShadow{b: b, path: path, receipts: receipts, snapshot: newReuseSnapshot(b.s, b.deps.Tools, tree, b.Root)}
}

// probe is the reuse key of a command and the newest green receipt under it, if any.
func (sh *baselineShadow) probe(command string) shadowProbe {
	if sh == nil {
		return shadowProbe{}
	}
	p := shadowProbe{key: sh.snapshot.Key(command)}
	for i := len(sh.receipts) - 1; i >= 0; i-- {
		if r := sh.receipts[i]; r.Key == p.key && r.Status == 0 {
			p.hit = &sh.receipts[i]
			break
		}
	}
	return p
}

// observe logs the reuse_shadow event of a check that ran and keeps its receipt.
func (sh *baselineShadow) observe(p shadowProbe, name string, status int, wallMS int64) {
	if sh == nil || p.key == "" || sh.b.reuseShadowOff() {
		return
	}
	b := sh.b
	wouldReuse, receipt := p.hit != nil, ""
	if p.hit != nil {
		receipt = p.hit.Log
	}
	agrees := !wouldReuse || status == 0
	b.Run.Event(map[string]any{"event": "reuse_shadow", "name": name, "reuse_sha256": p.key, "would_reuse": wouldReuse,
		"receipt": receipt, "actual_status": status, "actual_wall_ms": wallMS, "agrees": agrees})
	if !agrees {
		b.s.Progress(fmt.Sprintf("kogen: reuse shadow: check %s would have been reused from %s, but it exited %d", name, receipt, status))
	}
	r := reuseReceipt{Key: p.key, Name: name, Status: status, WallMS: wallMS, Source: reuseSourceBase, Log: b.Run.LogPath}
	if err := appendReuseReceipts(sh.path, r); err != nil {
		b.reuseShadowUnavailable(err.Error())
		return
	}
	sh.receipts = append(sh.receipts, r)
}

// passReceipts reads the run log from where it was last read and returns a receipt for every
// configured check that ran in a whole-Intent verification pass since, with the reuse_sha256
// Verification put on that check's command event. A check the pass did not run (the ledger
// answered it) has no command event and no receipt; the baseline's commands carry no key.
func (b *Attempt) passReceipts(st *reuseShadowState) []reuseReceipt {
	checks := map[string]bool{}
	for _, c := range b.Config.Checks {
		checks[c.Name] = true
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	f, err := os.Open(b.Run.LogPath)
	if err != nil {
		return nil
	}
	defer f.Close()
	if _, err := f.Seek(st.offset, io.SeekStart); err != nil {
		return nil
	}
	raw, err := io.ReadAll(f)
	if err != nil {
		return nil
	}
	end := bytes.LastIndexByte(raw, '\n')
	if end < 0 {
		return nil
	}
	raw = raw[:end+1]
	var receipts []reuseReceipt
	for _, line := range bytes.Split(raw, []byte("\n")) {
		var e struct {
			Event        string  `json:"event"`
			Name         string  `json:"name"`
			ReuseSHA256  string  `json:"reuse_sha256"`
			BeforeChange bool    `json:"before_change"`
			Status       int     `json:"status"`
			WallMS       float64 `json:"wall_ms"`
		}
		if len(line) == 0 || json.Unmarshal(line, &e) != nil || !checks[e.Name] {
			continue
		}
		switch e.Event {
		case "command":
			if e.ReuseSHA256 != "" && !e.BeforeChange {
				st.pending[e.Name] = e.ReuseSHA256
			}
		case "command_result":
			if key, ok := st.pending[e.Name]; ok {
				delete(st.pending, e.Name)
				receipts = append(receipts, reuseReceipt{Key: key, Name: e.Name, Status: e.Status, WallMS: int64(e.WallMS), Source: reuseSourcePass, Log: b.Run.LogPath})
			}
		}
	}
	st.offset += int64(len(raw))
	return receipts
}

// recordPassReceipts appends the receipts of the whole-Intent verification passes that ran
// since it last looked, so that the next Build starting on the landed tree finds them.
func (b *Attempt) recordPassReceipts() {
	st := b.reuseShadowState()
	if st == nil || len(b.Config.Checks) == 0 || b.reuseShadowOff() {
		return
	}
	receipts := b.passReceipts(st)
	if len(receipts) == 0 {
		return
	}
	path, err := reuseReceiptPath(b.Root)
	if err == nil {
		err = appendReuseReceipts(path, receipts...)
	}
	if err != nil {
		b.reuseShadowUnavailable(err.Error())
	}
}
