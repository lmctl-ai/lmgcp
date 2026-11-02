// SPDX-License-Identifier: MIT

package host

import (
	"testing"

	"github.com/lmctl-ai/lmgcp/internal/fakegcp"
	"github.com/lmctl-ai/lmgcp/internal/safety"
)

// TestAbortRecoverySweep reproduces the SolReviewer orphan probe without real
// GCP: create a bucket against a persistent loopback fake, abandon the host
// without cleanup, reopen the journal in a new host, and reclaim it.
func TestAbortRecoverySweep(t *testing.T) {
	server, err := fakegcp.NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	t.Setenv("LMGCP_LIVE", "")
	t.Setenv("LMGCP_STATE_DIR", t.TempDir())

	first, err := newHost(false, true, server)
	if err != nil {
		t.Fatal(err)
	}
	runID := first.RunID()
	path := first.RecoveryPath()
	if path == "" {
		t.Fatal("recovery journal path empty")
	}
	if got := first.StorageCreateBucket("lmgcp-recovery-demo", "US", "STANDARD"); got == nil || got.NameVal != "lmgcp-recovery-demo" {
		t.Fatalf("create bucket: %#v", got)
	}
	if first.LastError() != "" {
		t.Fatalf("create bucket failed: %s", first.LastError())
	}
	// Simulate the hard abort: no CleanupLedger, no in-memory ownership.
	first.Close()

	// A same-prefix resource created outside the journal must survive recovery.
	unowned, err := newHost(false, false, server)
	if err != nil {
		t.Fatal(err)
	}
	if got := unowned.StorageCreateBucket("lmgcp-unrelated-demo", "US", "STANDARD"); got == nil || unowned.LastError() != "" {
		t.Fatalf("create unrelated bucket: %v", unowned.LastError())
	}
	unowned.Close()

	recovery, err := newHost(false, false, server)
	if err != nil {
		t.Fatal(err)
	}
	defer recovery.Close()
	if err := recovery.LoadRecovery(path); err != nil {
		t.Fatal(err)
	}
	if recovery.RunID() != runID {
		t.Fatalf("run id = %q, want %q", recovery.RunID(), runID)
	}
	if got := recovery.SweepDelete(0); got != 1 {
		t.Fatalf("first sweep deleted %d, want 1 (last_error=%q)", got, recovery.LastError())
	}
	names := recovery.StorageListBuckets()
	if containsName(names, "lmgcp-recovery-demo") {
		t.Fatalf("owned bucket survived recovery: %v", names)
	}
	if !containsName(names, "lmgcp-unrelated-demo") {
		t.Fatalf("unowned same-prefix bucket was deleted: %v", names)
	}
	if got := recovery.SweepDelete(0); got != 0 {
		t.Fatalf("second sweep deleted %d, want 0", got)
	}
	loaded, err := safety.LoadJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	sawDeleted := false
	for _, record := range loaded.Records {
		if record.Event == safety.EventDeleted && record.Name == "lmgcp-recovery-demo" {
			sawDeleted = true
		}
	}
	if !sawDeleted {
		t.Fatal("journal lacks deletion tombstone")
	}
}

func containsName(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}

func TestSweepDeletesBQTable(t *testing.T) {
	t.Setenv("LMGCP_LIVE", "")
	t.Setenv("LMGCP_STATE_DIR", "")
	h, err := NewHost(false)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if got := h.BQCreateDataset("lmgcp_sweep_ds", "US"); got == nil || h.LastError() != "" {
		t.Fatalf("create dataset: %v", h.LastError())
	}
	if got := h.BQCreateTable("lmgcp_sweep_ds", "lmgcp_sweep_tbl", "name:string", ""); got == nil || h.LastError() != "" {
		t.Fatalf("create table: %v", h.LastError())
	}
	if got := h.SweepDelete(0); got != 2 {
		t.Fatalf("sweep deleted %d, want 2 (last_error=%q)", got, h.LastError())
	}
	if got := h.ledger.Count(); got != 0 {
		t.Fatalf("ledger count = %d, want 0", got)
	}
}
