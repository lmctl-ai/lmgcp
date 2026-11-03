// SPDX-License-Identifier: MIT

package safety

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSyncDirIsPortable(t *testing.T) {
	if err := syncDir(t.TempDir()); err != nil {
		t.Fatalf("sync state directory: %v", err)
	}
}

func TestJournalCrashBetweenIntentAndConfirm(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "journal.jsonl")
	meta := JournalMeta{RunID: "run-1234567890-abcdefabcdef", Provider: "gcp", Account: "acct", Tenant: "acct", Scope: "us-central1", Endpoint: "in-process-fake"}
	j, err := OpenJournal(path, meta, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := j.Intent("bucket", "US", "lmgcp-crash-bucket", 0); err != nil {
		t.Fatal(err)
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Meta.RunID != meta.RunID {
		t.Fatalf("run id = %q", loaded.Meta.RunID)
	}
	if len(loaded.Records) != 2 { // meta + intent
		t.Fatalf("records = %d", len(loaded.Records))
	}
	if loaded.Records[1].Event != EventIntent || loaded.Records[1].Name != "lmgcp-crash-bucket" {
		t.Fatalf("intent not durable: %#v", loaded.Records[1])
	}
	var ledger RunLedger
	ledger.ReplaceFromJournal(loaded.Records)
	snap := ledger.Snapshot()
	if len(snap) != 1 || snap[0].State != "pending" {
		t.Fatalf("pending recovery = %#v", snap)
	}
}

func TestNewRunLedgerUniqueID(t *testing.T) {
	a, b := NewRunLedger(""), NewRunLedger("")
	if a.RunID() == b.RunID() || a.RunID() == "run-20261024" {
		t.Fatalf("non-unique run ids: %q %q", a.RunID(), b.RunID())
	}
}

func TestJournalLockAndTornTail(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "journal.jsonl")
	meta := JournalMeta{RunID: "run-1234567890-abcdefabcdef", Provider: "gcp", Account: "acct", Tenant: "acct", Scope: "us-central1", Endpoint: "in-process-fake"}
	first, err := OpenJournal(path, meta, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := OpenJournal(path, meta, true); err == nil {
		t.Fatal("second locked open succeeded")
	}
	if err := first.Intent("bucket", "US", "lmgcp-torn", 0); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"version":1,"seq":9,"event":"inte`); err != nil {
		t.Fatal(err)
	}
	f.Close()
	loaded, err := LoadJournal(path)
	if err != nil {
		t.Fatalf("torn final line was fatal: %v", err)
	}
	if len(loaded.Warnings) == 0 {
		t.Fatal("torn final line produced no warning")
	}
	if len(loaded.Records) != 2 { // meta + intent
		t.Fatalf("records = %d", len(loaded.Records))
	}
	// A second open must succeed after the first lock is released.
	second, err := OpenJournal(path, meta, true)
	if err != nil {
		t.Fatal(err)
	}
	second.Close()
}

func TestJournalRejectsCraftedRecordAndTraversal(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "journal.jsonl")
	meta := JournalMeta{RunID: "run-1234567890-abcdefabcdef", Provider: "gcp", Account: "acct", Tenant: "acct", Scope: "us-central1", Endpoint: "in-process-fake"}
	j, err := OpenJournal(path, meta, false)
	if err != nil {
		t.Fatal(err)
	}
	j.Close()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"version":1,"seq":99,"time":"x","event":"confirmed","run_id":"other","provider":"aws","account":"nope","kind":"bucket","location":"US","name":"lmgcp-victim"}` + "\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err := LoadJournal(path); err == nil {
		t.Fatal("crafted foreign record was accepted")
	}
	if got := JournalPath("/var/state/lmgcp", "../../../tmp/evil"); got != "" {
		t.Fatalf("traversal path = %q, want empty", got)
	}
}
