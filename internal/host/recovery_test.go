// SPDX-License-Identifier: MIT

package host

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/api/googleapi"

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
	stateDir := t.TempDir()
	if err := os.Chmod(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LMGCP_STATE_DIR", stateDir)

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
		t.Fatalf("sweep deleted %d, want 2 in the explicit fake harness", got)
	}
	if got := h.ledger.Count(); got != 0 {
		t.Fatalf("ledger count = %d, want 0", got)
	}
}

func TestAbsentErrorRequiresTyped404Reason(t *testing.T) {
	h := &Host{}
	h.fail(&googleapi.Error{Code: 404, Errors: []googleapi.ErrorItem{{Reason: "notFound"}}})
	if !h.absentError() {
		t.Fatal("typed 404 notFound was not classified absent")
	}
	h.fail(&googleapi.Error{Code: 404, Errors: []googleapi.ErrorItem{{Reason: "forbidden"}}})
	if h.absentError() {
		t.Fatal("404 with non-notFound reason was classified absent")
	}
	h.fail(fmt.Errorf("404 not found: lmgcp-404-demo"))
	if h.absentError() {
		t.Fatal("message text was used for absence classification")
	}
}

func TestDefiniteCreateFailureClassification(t *testing.T) {
	for _, code := range []int{400, 401, 403, 409, 412} {
		if !definiteCreateFailure(&googleapi.Error{Code: code}) {
			t.Fatalf("code %d should be definite", code)
		}
	}
	for _, code := range []int{408, 429, 499, 500, 503} {
		if definiteCreateFailure(&googleapi.Error{Code: code}) {
			t.Fatalf("code %d should remain pending", code)
		}
	}
	if definiteCreateFailure(nil) {
		t.Fatal("nil error classified definite")
	}
}

func TestFake404CarriesNotFoundReason(t *testing.T) {
	h, err := NewHost(false)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	_ = h.StorageGetBucket("lmgcp-does-not-exist")
	if !h.absentError() {
		t.Fatalf("fake 404 did not classify absent: %q", h.LastError())
	}
}

func TestPendingIntentRecoveryAndLabeledSibling(t *testing.T) {
	server, err := fakegcp.NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	t.Setenv("LMGCP_LIVE", "")
	state := t.TempDir()
	if err := os.Chmod(state, 0o700); err != nil {
		t.Fatal(err)
	}
	runID := "run-1234567890-abcdefabcdef"

	owned, err := newHost(false, false, server)
	if err != nil {
		t.Fatal(err)
	}
	if !owned.SetRunID(runID) {
		t.Fatal(owned.LastError())
	}
	if got := owned.StorageCreateBucket("lmgcp-pending-owned", "US", "STANDARD"); got == nil || owned.LastError() != "" {
		t.Fatalf("owned bucket: %v", owned.LastError())
	}
	owned.Close()

	unowned, err := newHost(false, false, server)
	if err != nil {
		t.Fatal(err)
	}
	if got := unowned.StorageCreateBucket("lmgcp-pending-unowned", "US", "STANDARD"); got == nil || unowned.LastError() != "" {
		t.Fatalf("unowned bucket: %v", unowned.LastError())
	}
	unowned.Close()

	path := filepath.Join(state, "journal-"+runID+".jsonl")
	meta := safety.JournalMeta{RunID: runID, Provider: "gcp", Account: "lmgcp-sandbox-8f2a", Tenant: "lmgcp-sandbox-8f2a", Scope: "us-central1", Endpoint: "in-process-fake"}
	j, err := safety.OpenJournal(path, meta, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := j.Intent("bucket", "US", "lmgcp-pending-owned", 0); err != nil {
		t.Fatal(err)
	}
	if err := j.Intent("bucket", "US", "lmgcp-pending-unowned", 0); err != nil {
		t.Fatal(err)
	}
	j.Close()

	rec, err := newHost(false, false, server)
	if err != nil {
		t.Fatal(err)
	}
	defer rec.Close()
	if err := rec.LoadRecovery(path); err != nil {
		t.Fatal(err)
	}
	deleted, _, retained := rec.SweepDeleteDetailed(0)
	if deleted != 1 || retained != 1 {
		t.Fatalf("pending sweep deleted=%d retained=%d, want 1/1", deleted, retained)
	}
	names := rec.StorageListBuckets()
	if containsName(names, "lmgcp-pending-owned") {
		t.Fatal("owned pending-intent bucket survived")
	}
	if !containsName(names, "lmgcp-pending-unowned") {
		t.Fatal("labeled unowned sibling was deleted")
	}
}

func TestFakeOwnershipBypassUnreachableWithoutFake(t *testing.T) {
	h := &Host{}
	owned, absent, reason := h.verifyOwnership(safety.LedgerEntry{Kind: "pubsub_topic", Name: "lmgcp-topic"})
	if owned || absent || reason == "" {
		t.Fatalf("non-fake ownership check accepted an unverified kind: owned=%v absent=%v reason=%q", owned, absent, reason)
	}
}
