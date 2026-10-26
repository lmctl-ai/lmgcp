# Live GCP Testing Procedure (`lmgcp`)

**Status**: **Pending user test.** No live Google Cloud project has been accessed during development or CI. Unless `LMGCP_LIVE=1` is explicitly exported in the environment, `lmgcp` binds all Google Cloud Go SDK clients to its in-process loopback REST server (`internal/fakegcp`) with `option.WithoutAuthentication()`.

Public GitHub Actions CI never uses GCP credentials or OIDC. Perform live testing locally on your own machine against a dedicated non-production sandbox GCP project.

---

## 1. Prerequisites & Binary Verification

Download the latest release binary for your platform (or build locally with `go build -o bin/lmgcp ./cmd/lmgcp`):

```bash
mkdir -p lmgcp-live-check && cd lmgcp-live-check
gh release download v0.1.6 --repo lmctl-ai/lmgcp --pattern lmgcp-linux-x64 --pattern SHA256SUMS
sha256sum --ignore-missing --check SHA256SUMS
chmod +x lmgcp-linux-x64
./lmgcp-linux-x64 --version
```

Ensure `lmscript` (`@lmctl-ai/lmctl >= 0.2.420`) and `lmdraw` are available on `PATH`.

---

## 2. Configure Application Default Credentials (ADC) & Sandbox Environment

Select a dedicated non-production GCP project where your account holds permissions for the test resources:

```bash
# Verify Application Default Credentials are active (do not print the token)
gcloud auth application-default print-access-token >/dev/null

export LMGCP_PROJECT="your-nonprod-sandbox-project-id"
export LMGCP_REGION="us-central1"
export LMGCP_ZONE="us-central1-a"
export LMGCP_LIVE=1
```

Safety invariants active during `LMGCP_LIVE=1`:
- `gcp.project()` and `gcp.masked_project()` always mask the project ID (`gcp-proj-****xxxx`).
- All outputs, logs, Pub/Sub messages, and Firestore document reads pass through `safety.RedactSecrets()` (`ya29.[REDACTED]`, `AIza[REDACTED]`, `PRIVATE KEY [REDACTED]`).
- `AccessSecretVersion` is absent from the host; Secret Manager calls inspect metadata only.
- IAM examples (`10_iam_least_privilege_review.lms` and `32_iam_binding_blast_radius_audit.lms`) are strictly read-only.

---

## 3. Phase 1 — Read-Only Smoke Tests (`$0.00`)

Run the cost-free read-only inventory and posture examples first to verify ADC authentication and API connectivity without creating or mutating any resources:

```bash
lmscript examples/01_project_inventory_overview.lms
lmscript examples/02_label_compliance_audit.lms
lmscript examples/03_vpc_firewall_security_review.lms
lmscript examples/08_vpc_resource_topology_graph.lms
lmscript examples/38_cost_free_cis_posture_scorecard.lms
```

Each script prints `estimated cost: $0.00 (read-only; resources=0, minutes_alive=0, requests=0)` and writes `build/evidence/live-cost-<example>.json`.

---

## 4. Phase 2 — Guarded Ephemeral Write & Teardown Tests

Run individual mutating examples to exercise creation, inspection, and automatic `RunLedger` LIFO teardown (`gcp.cleanup_ledger()`):

```bash
# Cloud Storage bucket + 2 objects ($0.0003 estimated)
lmscript examples/04_gcs_bucket_lifecycle_ephemeral.lms

# Pub/Sub topic + subscription + dead-letter queue ($0.0006 estimated)
lmscript examples/05_pubsub_dead_letter_pipeline.lms

# Secret Manager metadata-only lifecycle ($0.0002 estimated)
lmscript examples/13_secret_manager_metadata_audit.lms

# Cost cap refusal gate ($0.0018 estimated; blocks 7th resource over $0.0050 cap)
lmscript examples/40_cost_cap_ledger_live_readiness.lms
```

Every created resource:
1. Must start with `lmgcp_` or `lmgcp-` (`safety.ValidateResourcePrefix`),
2. Carries labels `lmgcp_test=<run-id>` and `lmgcp_created_at=<epoch>`,
3. Is charged against the `RunLedger` price table before creation (default cap `$0.05`, hard ceiling `$0.25`),
4. Is deleted in reverse order by `gcp.cleanup_ledger()` before the script exits (and on any runtime error).

---

## 5. Phase 3 — Side-by-Side `lmgcp` vs `lmcloud` (OpenTofu) Comparison

To compare native `lmgcp` execution against `lmcloud` (`hashicorp/google` provider) on the same project:

```bash
./compare/run.sh 04_gcs_bucket_lifecycle_ephemeral
```

---

## 6. Post-Run Orphan Sweeper Verification

Always run the sweeper after live testing to confirm zero leftover `lmgcp_` / `lmgcp-` resources remain:

```bash
# Dry-run (lists any leftover lmgcp_* resources with age >= 0 hours)
./bin/lmgcp --sweep --older-than-hours 0

# Delete any leftover ledger-owned test resources if found
./bin/lmgcp --sweep --older-than-hours 0 --delete

# Return shell to default fake-only mode
unset LMGCP_LIVE LMGCP_PROJECT
```
