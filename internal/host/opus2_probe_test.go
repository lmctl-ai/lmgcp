package host

import (
	"os"
	"strings"
	"testing"

	"github.com/lmctl-ai/lmgcp/internal/fakegcp"
	"github.com/lmctl-ai/lmgcp/internal/safety"
)

func o2seed(t *testing.T, srv *fakegcp.Server, name string) {
	h, err := newHost(false, false, srv)
	if err != nil {
		t.Fatal(err)
	}
	if h.StorageCreateBucket(name, "US", "STANDARD"); h.LastError() != "" {
		t.Fatalf("seed: %s", h.LastError())
	}
	h.Close()
}

func o2env(t *testing.T) *fakegcp.Server {
	srv, err := fakegcp.NewServer()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Close)
	t.Setenv("LMGCP_LIVE", "")
	d := t.TempDir()
	os.Chmod(d, 0o700)
	t.Setenv("LMGCP_STATE_DIR", d)
	return srv
}

func o2lines(t *testing.T, path string) string {
	b, _ := os.ReadFile(path)
	var ev []string
	for _, l := range strings.Split(string(b), "\n") {
		if i := strings.Index(l, `"event":"`); i >= 0 {
			ev = append(ev, strings.SplitN(l[i+9:], `"`, 2)[0])
		}
	}
	return strings.Join(ev, ",")
}

func TestO2FailedCreate(t *testing.T) {
	srv := o2env(t)
	o2seed(t, srv, "lmgcp-o2-victim-a")
	run, err := newHost(false, true, srv)
	if err != nil {
		t.Fatal(err)
	}
	run.StorageCreateBucket("lmgcp-o2-victim-a", "US", "STANDARD")
	t.Logf("create err=%q", run.LastError())
	p := run.RecoveryPath()
	run.Close()
	t.Logf("journal events: %s", o2lines(t, p))
	rec, _ := newHost(false, false, srv)
	defer rec.Close()
	if err := rec.LoadRecovery(p); err != nil {
		t.Fatal(err)
	}
	d, a, r := rec.SweepDeleteDetailed(0)
	t.Logf("sweep deleted=%d absent=%d retained=%d", d, a, r)
	if !containsName(rec.StorageListBuckets(), "lmgcp-o2-victim-a") {
		t.Error("STILL EXPLOITABLE: unowned bucket deleted")
	}
}

func TestO2CraftedBucketMatchingBinding(t *testing.T) {
	srv := o2env(t)
	o2seed(t, srv, "lmgcp-o2-victim-b")
	run, _ := newHost(false, true, srv)
	p := run.RecoveryPath()
	run.Close()
	b, _ := os.ReadFile(p)
	meta := strings.SplitN(string(b), "\n", 2)[0]
	crafted := strings.Replace(meta, `"event":"meta"`, `"event":"confirmed","kind":"bucket","location":"US","name":"lmgcp-o2-victim-b"`, 1)
	crafted = strings.Replace(crafted, `"seq":1`, `"seq":50`, 1)
	f, _ := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString(crafted + "\n")
	f.Close()
	rec, _ := newHost(false, false, srv)
	defer rec.Close()
	if err := rec.LoadRecovery(p); err != nil {
		t.Fatalf("load: %v", err)
	}
	d, a, r := rec.SweepDeleteDetailed(0)
	t.Logf("bucket crafted: deleted=%d absent=%d retained=%d", d, a, r)
	if !containsName(rec.StorageListBuckets(), "lmgcp-o2-victim-b") {
		t.Error("STILL EXPLOITABLE: crafted bucket line deleted unowned bucket")
	}
}

func TestO2CraftedForeignBinding(t *testing.T) {
	srv := o2env(t)
	run, _ := newHost(false, true, srv)
	p := run.RecoveryPath()
	run.Close()
	f, _ := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString(`{"version":1,"seq":99,"time":"x","event":"confirmed","run_id":"other","provider":"aws","account":"nope","kind":"bucket","location":"US","name":"lmgcp-o2-x"}` + "\n")
	f.Close()
	rec, _ := newHost(false, false, srv)
	defer rec.Close()
	err := rec.LoadRecovery(p)
	t.Logf("foreign binding load err=%v", err)
	if err == nil {
		t.Error("foreign-binding record accepted")
	}
}

// Non-bucket kind in in-process fake: ownership bypass.
func TestO2CraftedTopicFake(t *testing.T) {
	srv := o2env(t)
	h0, _ := newHost(false, false, srv)
	h0.PubsubCreateTopic("lmgcp-o2-topic")
	if h0.LastError() != "" {
		t.Fatal(h0.LastError())
	}
	h0.Close()
	run, _ := newHost(false, true, srv)
	p := run.RecoveryPath()
	run.Close()
	b, _ := os.ReadFile(p)
	meta := strings.SplitN(string(b), "\n", 2)[0]
	crafted := strings.Replace(strings.Replace(meta, `"event":"meta"`, `"event":"confirmed","kind":"pubsub_topic","name":"lmgcp-o2-topic"`, 1), `"seq":1`, `"seq":51`, 1)
	f, _ := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString(crafted + "\n")
	f.Close()
	rec, _ := newHost(false, false, srv)
	defer rec.Close()
	if err := rec.LoadRecovery(p); err != nil {
		t.Fatal(err)
	}
	d, a, r := rec.SweepDeleteDetailed(0)
	t.Logf("fake topic crafted: deleted=%d absent=%d retained=%d", d, a, r)
}

// NEW: second create of an owned name fails 409 -> FailIntent drops the owned record.
func TestO2DoubleCreateLosesOwnership(t *testing.T) {
	srv := o2env(t)
	run, _ := newHost(false, true, srv)
	run.StorageCreateBucket("lmgcp-o2-mine", "US", "STANDARD")
	if run.LastError() != "" {
		t.Fatal(run.LastError())
	}
	run.StorageCreateBucket("lmgcp-o2-mine", "US", "STANDARD")
	t.Logf("second create err=%q owns=%v", run.LastError(), run.ledger.Owns("bucket", "lmgcp-o2-mine"))
	p := run.RecoveryPath()
	run.Close()
	t.Logf("events: %s", o2lines(t, p))
	rec, _ := newHost(false, false, srv)
	defer rec.Close()
	if err := rec.LoadRecovery(p); err != nil {
		t.Fatal(err)
	}
	d, a, r := rec.SweepDeleteDetailed(0)
	t.Logf("sweep deleted=%d absent=%d retained=%d", d, a, r)
	if containsName(rec.StorageListBuckets(), "lmgcp-o2-mine") {
		t.Error("LEAK: owned bucket forgotten after a failed duplicate create")
	}
}

func TestO2Lock(t *testing.T) {
	srv := o2env(t)
	run, _ := newHost(false, true, srv)
	p := run.RecoveryPath()
	run.StorageCreateBucket("lmgcp-o2-active", "US", "STANDARD")
	second, err := safety.OpenJournal(p, safety.JournalMeta{}, true)
	t.Logf("second writer open (zero meta) err=%v", err)
	if second != nil {
		second.Close()
	}
	rec, _ := newHost(false, false, srv)
	defer rec.Close()
	err = rec.LoadRecovery(p)
	t.Logf("sweep during active run err=%v", err)
	if err == nil {
		t.Error("sweep allowed while run holds lock")
	}
	run.Close()
}

func TestO2SetRunID(t *testing.T) {
	_ = o2env(t)
	run, _ := newHost(false, true, nil)
	defer run.Close()
	ok := run.SetRunID("evil")
	t.Logf("set_run_id after attach ok=%v err=%q runid=%s", ok, run.LastError(), run.RunID())
	if ok {
		t.Error("set_run_id accepted after attach")
	}
}
