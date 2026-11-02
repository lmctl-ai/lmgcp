// SPDX-License-Identifier: MIT

package safety

import (
	"path/filepath"
	"testing"
)

func TestJournalCrashBetweenIntentAndConfirm(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "journal.jsonl")
	meta := JournalMeta{RunID: "run-test-crash", Provider: "gcp", Account: "acct", Tenant: "acct", Scope: "us-central1", Endpoint: "in-process-fake"}
	j, err := OpenJournal(path, meta)
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
