// SPDX-License-Identifier: MIT

package safety

import (
	"strings"
	"testing"
)

func TestRedactSecrets(t *testing.T) {
	samples := []struct {
		in      string
		forbid  string
		expect  string
	}{
		{
			in:     "Bearer ya29.a0AfH6SMBx9SecretTokenValue12345",
			forbid: "a0AfH6SMBx9SecretTokenValue12345",
			expect: "ya29.[REDACTED]",
		},
		{
			in:     "key=AIzaSyDa1234567890abcdefABCDEF12345",
			forbid: "SyDa1234567890abcdefABCDEF12345",
			expect: "AIza[REDACTED]",
		},
		{
			in:     "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQC\n-----END PRIVATE KEY-----",
			forbid: "MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQC",
			expect: "[REDACTED_PRIVATE_KEY]",
		},
		{
			in:     "postgres://admin:SuperSecretPass99@10.10.16.10:5432/orders",
			forbid: "SuperSecretPass99",
			expect: "postgres://[REDACTED]@10.10.16.10:5432/orders",
		},
		{
			in:     `{"type":"service_account","private_key":"-----BEGIN RSA PRIVATE KEY-----abc"}`,
			forbid: "BEGIN RSA PRIVATE KEY",
			expect: `"private_key":"[REDACTED]"`,
		},
	}

	for _, tc := range samples {
		got := RedactSecrets(tc.in)
		if strings.Contains(got, tc.forbid) {
			t.Fatalf("RedactSecrets leaked %q in %q", tc.forbid, got)
		}
		if !strings.Contains(got, tc.expect) {
			t.Fatalf("RedactSecrets(%q) = %q, want substring %q", tc.in, got, tc.expect)
		}
	}
}

func TestMaskProjectID(t *testing.T) {
	got := MaskProjectID("acme-production-987654")
	if strings.Contains(got, "acme-production") {
		t.Fatalf("MaskProjectID leaked raw project prefix: %s", got)
	}
	if got != "gcp-proj-****7654" {
		t.Fatalf("MaskProjectID = %q, want %q", got, "gcp-proj-****7654")
	}
}

func TestPrefixAndShapeGuardrails(t *testing.T) {
	if err := ValidateResourcePrefix("bucket", "prod-important-bucket"); err == nil {
		t.Fatal("expected non-lmgcp bucket name to be rejected")
	}
	if err := ValidateResourcePrefix("bucket", "lmgcp-ephemeral-bucket"); err != nil {
		t.Fatalf("expected lmgcp- prefix to be accepted: %v", err)
	}
	if err := ValidateResourcePrefix("bq_dataset", "lmgcp_ephemeral_ds"); err != nil {
		t.Fatalf("expected lmgcp_ prefix to be accepted: %v", err)
	}
	if err := ValidateMachineType("a2-highgpu-1g"); err == nil {
		t.Fatal("expected GPU machine type to be rejected")
	}
	if err := ValidateMachineType("e2-micro"); err != nil {
		t.Fatalf("expected e2-micro to be allowed: %v", err)
	}
	if err := ValidateSqlTier("db-n1-highmem-64"); err == nil {
		t.Fatal("expected large SQL tier to be rejected")
	}
	if err := ValidateSqlTier("db-f1-micro"); err != nil {
		t.Fatalf("expected db-f1-micro to be allowed: %v", err)
	}
}

func TestRunLedgerOwnership(t *testing.T) {
	ledger := NewRunLedger("test-run-01")
	ledger.Record("bucket", "US-CENTRAL1", "lmgcp-bkt-1", 1700000000)

	if !ledger.Owns("bucket", "lmgcp-bkt-1") {
		t.Fatal("expected ledger to own recorded bucket")
	}
	if err := ledger.ValidateModifyOrDelete("bucket", "lmgcp-bkt-1"); err != nil {
		t.Fatalf("expected recorded bucket delete validation to pass: %v", err)
	}
	if err := ledger.ValidateModifyOrDelete("bucket", "lmgcp-other-run-bkt"); err == nil {
		t.Fatal("expected unrecorded bucket with prefix to be rejected by ledger check")
	}
	if err := ledger.ValidateModifyOrDelete("bucket", "prod-customer-invoices-us"); err == nil {
		t.Fatal("expected baseline bucket without prefix to be rejected")
	}
	ledger.Remove("bucket", "US-CENTRAL1", "lmgcp-bkt-1")
	if ledger.Owns("bucket", "lmgcp-bkt-1") {
		t.Fatal("expected removed bucket to no longer be owned")
	}
}
