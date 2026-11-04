package consumes

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/KogenAI/kogen-coordinator/durable"
)

// build/context.go, build/core_run.go, build/material.go, build/recover.go and build/stages.go
// flush each durable file through durable.Sync and take its error as the write's error: an open
// file flushes to nil, a file that cannot be flushed (closed) is an error. The suite never calls
// SkipSyncInTests, so Sync is the real flush here.
func TestDurableSyncFlushesAndReportsFailure(t *testing.T) {
	var _ func(*os.File) error = durable.Sync

	f, err := os.Create(filepath.Join(t.TempDir(), "durable"))
	check(t, err)
	if _, err := f.WriteString("record\n"); err != nil {
		t.Fatal(err)
	}
	if err := durable.Sync(f); err != nil {
		t.Errorf("durable.Sync of an open file = %v, want nil", err)
	}
	check(t, f.Close())
	if err := durable.Sync(f); err == nil {
		t.Errorf("durable.Sync of a closed file = nil, want the flush error")
	}
}
