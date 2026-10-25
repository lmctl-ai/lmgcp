// SPDX-License-Identifier: MIT

package host

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lmctl-ai/lmcode-go"
)

func TestHostSignatureParity(t *testing.T) {
	h, err := NewHost()
	if err != nil {
		t.Fatalf("NewHost failed: %v", err)
	}
	defer h.Close()

	reg := lmcode.NewRegistry()
	if err := Register(reg, h); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	lmscriptBin := os.Getenv("LMSCRIPT_BIN")
	if lmscriptBin == "" {
		lmscriptBin, err = exec.LookPath("lmscript")
		if err != nil {
			t.Skip("lmscript not on PATH, skipping signature parity check")
		}
	}

	schemaPath := filepath.Join("..", "..", "schema", "lmgcp.dsl.json")
	out, err := exec.Command(lmscriptBin, "--dsl-signatures", schemaPath).CombinedOutput()
	if err != nil {
		t.Fatalf("lmscript --dsl-signatures failed: %v\n%s", err, string(out))
	}

	wantLines := strings.Split(strings.TrimSpace(string(out)), "\n")
	gotLines := reg.Signatures()
	if len(gotLines) != len(wantLines) {
		t.Fatalf("signature count mismatch: got %d, want %d", len(gotLines), len(wantLines))
	}
	for i := range wantLines {
		if gotLines[i] != wantLines[i] {
			t.Errorf("signature[%d] = %q, want %q", i, gotLines[i], wantLines[i])
		}
	}
}

func TestHostEndToEndFakeServicesAndSafety(t *testing.T) {
	h, err := NewHost()
	if err != nil {
		t.Fatalf("NewHost failed: %v", err)
	}
	defer h.Close()

	if h.IsLive() || h.AuthMode() != "fake-emulator" {
		t.Fatalf("expected fake-emulator mode by default, got mode=%s live=%v", h.AuthMode(), h.IsLive())
	}

	// 1. Verify baseline read-only inventory & compliance
	inv := h.InventorySummary()
	if inv["compute_instances"] != "7" || inv["storage_buckets"] != "5" {
		t.Fatalf("unexpected baseline inventory: %+v", inv)
	}
	comp := h.AuditLabelCompliance([]string{"env", "owner", "cost_center", "data_class"})
	if comp["compute_compliant"] != "5" || comp["compute_total"] != "7" {
		t.Fatalf("unexpected label compliance result: %+v", comp)
	}

	// 2. Verify safety blocks deleting or modifying baseline resources
	if h.ComputeDeleteInstance("us-central1-a", "prod-api-gw-01") {
		t.Fatal("safety violation: allowed deleting baseline instance")
	}
	if h.StorageDeleteBucket("prod-customer-invoices-us") {
		t.Fatal("safety violation: allowed deleting baseline bucket")
	}
	if h.ComputeCreateInstance("us-central1-a", "bad-prefix-vm", "e2-micro", "debian-12").StatusVal != "ERROR" {
		t.Fatal("safety violation: allowed creating instance without lmgcp prefix")
	}
	if h.ComputeCreateInstance("us-central1-a", "lmgcp-gpu-vm", "a2-highgpu-1g", "debian-12").StatusVal != "ERROR" {
		t.Fatal("safety violation: allowed creating expensive GPU instance")
	}

	// 3. Create ephemeral resources across services, verify ledger, and clean up
	vm := h.ComputeCreateInstance("us-central1-a", "lmgcp-worker-01", "e2-micro", "debian-12")
	if vm.StatusVal != "RUNNING" || !h.LedgerOwns("compute_instance", "lmgcp-worker-01") {
		t.Fatalf("failed to create ephemeral VM: %+v (err=%s)", vm, h.LastError())
	}
	if !h.ComputeStopInstance("us-central1-a", "lmgcp-worker-01") {
		t.Fatalf("failed to stop ephemeral VM: %s", h.LastError())
	}
	if h.ComputeGetInstance("us-central1-a", "lmgcp-worker-01").StatusVal != "TERMINATED" {
		t.Fatal("expected VM status TERMINATED after stop")
	}

	bkt := h.StorageCreateBucket("lmgcp-test-bkt", "US-CENTRAL1", "STANDARD")
	if bkt.NameVal != "lmgcp-test-bkt" {
		t.Fatalf("failed to create bucket: %s", h.LastError())
	}
	if !h.StorageSetBucketLifecycle("lmgcp-test-bkt", 90, 30) {
		t.Fatalf("failed to set bucket lifecycle: %s", h.LastError())
	}
	if !h.StoragePutObject("lmgcp-test-bkt", "reports/q1.csv", "metric,val\nlatency,42\n", "text/csv") {
		t.Fatalf("failed to put object: %s", h.LastError())
	}
	if gotObj := h.StorageReadObject("lmgcp-test-bkt", "reports/q1.csv"); !strings.Contains(gotObj, "latency,42") {
		t.Fatalf("unexpected object content: %q", gotObj)
	}

	top := h.PubsubCreateTopic("lmgcp-events")
	sub := h.PubsubCreateSubscription("lmgcp-events-sub", top.NameVal, 30, "")
	msgID := h.PubsubPublish("lmgcp-events", "order_id=1001 token=ya29.secretToken999", map[string]string{"env": "test"})
	if msgID == "" {
		t.Fatalf("failed to publish pubsub message: %s", h.LastError())
	}
	pulled := h.PubsubPull(sub.NameVal, 5)
	if len(pulled) != 1 || strings.Contains(pulled[0], "secretToken999") || !strings.Contains(pulled[0], "ya29.[REDACTED]") {
		t.Fatalf("expected pulled message with redacted token, got: %+v", pulled)
	}

	svc := h.RunDeployService("us-central1", "lmgcp-api", "us-central1-docker.pkg.dev/demo/app:v1", 1000, 512, 5)
	if svc.NameVal != "lmgcp-api" || !h.RunSetTrafficSplit("us-central1", "lmgcp-api", 80, 20) {
		t.Fatalf("cloud run deploy/traffic split failed: %s", h.LastError())
	}

	// Sweeper test
	h.SeedDemoAge("bucket", "lmgcp-test-bkt", 12)
	dry := h.SweepDryRun(6)
	if len(dry) != 1 || !strings.Contains(dry[0], "lmgcp-test-bkt") {
		t.Fatalf("expected sweep dry-run to find aged bucket, got: %+v", dry)
	}

	// Cleanup all ledger resources
	deleted := h.CleanupLedger()
	if deleted < 5 || h.LedgerCount() != 0 {
		t.Fatalf("expected CleanupLedger to delete >=5 resources and leave 0 in ledger, deleted=%d remaining=%d", deleted, h.LedgerCount())
	}
}
