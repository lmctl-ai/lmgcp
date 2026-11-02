// SPDX-License-Identifier: MIT

package host

import (
	"os"
	"strings"
	"testing"

	"github.com/lmctl-ai/lmgcp/internal/fakegcp"
	"github.com/lmctl-ai/lmgcp/internal/safety"
)

func privateTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func seedUnowned(t *testing.T, server *fakegcp.Server, name string) {
	t.Helper()
	h, err := newHost(false, false, server)
	if err != nil {
		t.Fatal(err)
	}
	if got := h.StorageCreateBucket(name, "US", "STANDARD"); got == nil || h.LastError() != "" {
		t.Fatalf("seed %s: %v", name, h.LastError())
	}
	h.Close()
}

// P1: a failed create (bucket already exists, not ours) must not delete the
// pre-existing bucket during sweep.
func TestOpusFailedCreateIntentDeletesUnowned(t *testing.T) {
	server, _ := fakegcp.NewServer()
	defer server.Close()
	t.Setenv("LMGCP_LIVE", "")
	t.Setenv("LMGCP_STATE_DIR", privateTemp(t))
	seedUnowned(t, server, "lmgcp-victim-a")
	run, err := newHost(false, true, server)
	if err != nil {
		t.Fatal(err)
	}
	run.StorageCreateBucket("lmgcp-victim-a", "US", "STANDARD")
	t.Logf("create error (expected conflict/ownership mismatch): %q", run.LastError())
	path := run.RecoveryPath()
	run.Close()
	rec, err := newHost(false, false, server)
	if err != nil {
		t.Fatal(err)
	}
	defer rec.Close()
	if err := rec.LoadRecovery(path); err != nil {
		t.Fatal(err)
	}
	t.Logf("dry-run candidates: %v", rec.SweepDryRun(0))
	n := rec.SweepDelete(0)
	if !containsName(rec.StorageListBuckets(), "lmgcp-victim-a") {
		t.Errorf("UNSAFE: sweep deleted pre-existing unowned bucket after failed create (deleted=%d)", n)
	}
}

// P1: a crafted foreign journal record must be rejected, not acted on.
func TestOpusCraftedJournalDeletesUnowned(t *testing.T) {
	server, _ := fakegcp.NewServer()
	defer server.Close()
	t.Setenv("LMGCP_LIVE", "")
	t.Setenv("LMGCP_STATE_DIR", privateTemp(t))
	seedUnowned(t, server, "lmgcp-victim-b")
	run, err := newHost(false, true, server)
	if err != nil {
		t.Fatal(err)
	}
	path := run.RecoveryPath()
	run.Close()
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	_, _ = f.WriteString(`{"version":1,"seq":99,"time":"x","event":"confirmed","run_id":"other","provider":"aws","account":"nope","kind":"bucket","location":"US","name":"lmgcp-victim-b"}` + "\n")
	f.Close()
	rec, err := newHost(false, false, server)
	if err != nil {
		t.Fatal(err)
	}
	defer rec.Close()
	if err := rec.LoadRecovery(path); err == nil {
		t.Fatal("foreign journal record was accepted")
	}
	if !containsName(rec.StorageListBuckets(), "lmgcp-victim-b") {
		t.Error("UNSAFE: crafted record caused unowned bucket deletion")
	}
}

// Torn final line (crash mid-append / disk full) must not make the journal unrecoverable.
func TestOpusTornTailBlocksRecovery(t *testing.T) {
	server, _ := fakegcp.NewServer()
	defer server.Close()
	t.Setenv("LMGCP_LIVE", "")
	t.Setenv("LMGCP_STATE_DIR", privateTemp(t))
	run, err := newHost(false, true, server)
	if err != nil {
		t.Fatal(err)
	}
	run.StorageCreateBucket("lmgcp-torn-demo", "US", "STANDARD")
	path := run.RecoveryPath()
	run.Close()
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	_, _ = f.WriteString(`{"version":1,"seq":4,"event":"inte`)
	f.Close()
	rec, err := newHost(false, false, server)
	if err != nil {
		t.Fatal(err)
	}
	defer rec.Close()
	if err := rec.LoadRecovery(path); err != nil {
		t.Errorf("UNRECOVERABLE after torn tail: %v (bucket still exists=%v)", err, containsName(rec.StorageListBuckets(), "lmgcp-torn-demo"))
	}
}

// Path traversal via run id must be rejected.
func TestOpusRunIDTraversal(t *testing.T) {
	if got := safety.JournalPath("/var/state/lmgcp", "../../../tmp/evil"); got != "" {
		t.Errorf("TRAVERSAL: JournalPath escapes state dir: %s", got)
	}
	if !strings.HasPrefix(safety.JournalPath("/var/state/lmgcp", "run-1234567890-abcdefabcdef"), "/var/state/lmgcp/") {
		t.Error("valid run id path rejected")
	}
}
