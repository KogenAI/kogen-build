package core

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
	"sync"
	"time"

	"github.com/KogenAI/kogen-coordinator/durable"
)

// Run is a Build's run record (LOOP-10): `.kogen/local/runs/<id>/log.jsonl` with requests and
// usage, tool-call byte counts and hashes, verification passes, reviews and the outcome;
// `result.json`; `review-<n>.json`; the checkpoint. Credentials never appear in it.
type Run struct {
	ID      string `json:"id"`
	Dir     string `json:"dir"`
	LogPath string `json:"log"`

	mu     sync.Mutex
	logErr error
}

// ISOTime is the timestamp format of run records: UTC with microseconds.
func ISOTime(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000000Z") }

// StartRun creates a new run folder (0700) with a fresh 8-hex id under root.
func StartRun(root string) (*Run, error) {
	runs := filepath.Join(root, ".kogen/local/runs")
	if err := os.MkdirAll(runs, 0o777); err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 100; attempt++ {
		var raw [4]byte
		if _, err := rand.Read(raw[:]); err != nil {
			return nil, err
		}
		id := hex.EncodeToString(raw[:])
		dir := filepath.Join(runs, id)
		err := os.Mkdir(dir, 0o777)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if err := os.Chmod(dir, 0o700); err != nil {
			return nil, err
		}
		return &Run{ID: id, Dir: dir, LogPath: filepath.Join(dir, "log.jsonl")}, nil
	}
	return nil, errors.New("no free run id")
}

// EncodeJSON is the JSON of v without HTML escaping and without the final newline.
func EncodeJSON(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(b.Bytes(), []byte("\n")), nil
}

// Event appends one event to the run log, stamped with the time (`at`). A write failure is
// kept and reported by the next checkpoint.
func (r *Run) Event(event map[string]any) {
	if r == nil {
		return
	}
	line := make(map[string]any, len(event)+1)
	for k, v := range event {
		line[k] = v
	}
	line["at"] = ISOTime(time.Now())
	b, err := EncodeJSON(line)
	r.mu.Lock()
	defer r.mu.Unlock()
	if err == nil {
		var f *os.File
		if f, err = os.OpenFile(r.LogPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o666); err == nil {
			_, err = f.Write(append(b, '\n'))
			if cerr := f.Close(); err == nil {
				err = cerr
			}
		}
	}
	if err != nil && r.logErr == nil {
		r.logErr = fmt.Errorf("run log %s: %w", r.LogPath, err)
	}
}

// Log implements verify.Logger.
func (r *Run) Log(event map[string]any) { r.Event(event) }

// LogErr is the first run log write failure not yet taken, if any.
func (r *Run) LogErr() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.logErr
}

// TakeLogErr returns and clears the first run log write failure.
func (r *Run) TakeLogErr() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	err := r.logErr
	r.logErr = nil
	return err
}

// Write writes `name` in the run folder as JSON and a newline.
func (r *Run) Write(name string, data any) error {
	b, err := EncodeJSON(data)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(r.Dir, name), append(b, '\n'), 0o666)
}

// PublishCheckpoint publishes a complete checkpoint by same-directory atomic rename: a
// private (0600) temporary file, synced, then renamed over `checkpoint.json`.
func (r *Run) PublishCheckpoint(doc any) error {
	b, err := EncodeJSON(doc)
	if err != nil {
		return err
	}
	path := filepath.Join(r.Dir, "checkpoint.json")
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	werr := func() error {
		if err := f.Chmod(0o600); err != nil {
			return err
		}
		if _, err := f.Write(append(b, '\n')); err != nil {
			return err
		}
		return durable.Sync(f)
	}()
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		os.Remove(tmp)
		return werr
	}
	return os.Rename(tmp, path)
}
