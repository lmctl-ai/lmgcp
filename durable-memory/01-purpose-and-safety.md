# 01 — Purpose, Safety & Cost Model

`lmgcp` is the Google Cloud Platform scripting host for `lmscript` (`lmctl`), built on `github.com/lmctl-ai/lmcode-go` and the official Google Cloud Go SDK (`cloud.google.com/go/auth`, `cloud.google.com/go/compute/metadata`, and `google.golang.org/api`, Apache-2.0). It ships as a single native executable (`CGO_ENABLED=0`, MIT license, zero GPL/LGPL dependencies).

## Binding Safety & Cost Model (`internal/safety/`)

1. **Fake-by-Default Transport (`LMGCP_LIVE=1` Opt-In)**:
   - Unless `LMGCP_LIVE=1` is explicitly exported in the environment, `host.NewHost()` starts an in-process HTTP REST server (`internal/fakegcp`) on `127.0.0.1:0` and binds all Google Cloud Go SDK clients to it with `option.WithoutAuthentication()`.
   - Ambient `gcloud` / ADC credentials on the local machine are never read unless `LMGCP_LIVE=1`.
2. **No OIDC and No Cloud Credentials in CI**:
   - By user decision, public GitHub Actions CI runs strictly against the in-process fake server. No OIDC provider or GCP credentials exist in CI workflows.
3. **Secret Redaction (`safety.RedactSecrets`)**:
   - `AccessSecretVersion` is intentionally excluded from the host API (Secret Manager calls inspect metadata only).
   - Every error string, log entry, Pub/Sub message payload, and Firestore document field passes through `safety.RedactSecrets()`, masking OAuth bearer tokens (`ya29.[REDACTED]`), Google API keys (`AIza[REDACTED]`), PEM private keys (`PRIVATE KEY [REDACTED]`), JWTs, and password parameters.
4. **Project ID Masking (`safety.MaskProjectID`)**:
   - `gcp.project()` and `gcp.masked_project()` always return masked identifiers (`gcp-proj-****8f2a`).
5. **Mandatory `lmgcp_` / `lmgcp-` Prefix & `lmgcp_test` Label (`ValidateResourcePrefix`, `EnrichLabels`)**:
   - Every resource creation call validates that the target resource name (or Firestore collection name) begins with `lmgcp_` or `lmgcp-`.
   - Created resources are automatically labeled with `lmgcp_test=<run-id>` and `lmgcp_created_at=<epoch>`.
6. **Compute & SQL Shape Allowlists (`ValidateMachineType`, `ValidateSqlTier`)**:
   - Compute Engine VMs are restricted to `e2-micro`, `e2-small`, `e2-medium`, `f1-micro`, and `g1-small`.
   - Cloud SQL tiers are restricted to `db-f1-micro`, `db-g1-small`, and `db-custom-1-3840`.
   - GPU (`a2-*`, `g2-*`) and large instance shapes are rejected pre-flight.
7. **`RunLedger` Ownership & LIFO Teardown (`ValidateModifyOrDelete`, `CleanupLedger`)**:
   - Modifying or deleting any resource requires both the `lmgcp_` / `lmgcp-` prefix and membership in the current run's `RunLedger`. Pre-existing project resources cannot be modified or deleted.
   - `gcp.cleanup_ledger()` deletes all ledger-recorded resources in reverse creation order (LIFO). Teardown also runs automatically on runtime error, watchdog exit, or `Host.Close()` when `autoCleanup` is active.
   - IAM examples (`10` and `32`) are strictly read-only (`iam_list_service_accounts`, `iam_get_service_account`, `iam_list_bindings`); the host exposes no IAM mutation methods.
8. **Orphan Sweeper (`--sweep`, `SweepDryRun`, `SweepDelete`)**:
   - `./bin/lmgcp --sweep [--older-than-hours N] [--delete]` (and DSL methods `gcp.sweep_dry_run(hours)` / `gcp.sweep_delete(hours)`) lists (dry-run default) or deletes `lmgcp_` / `lmgcp-` ledger resources at least `N` hours old.
9. **Cost Ledger, Price Table & Per-Example Caps (`LookupPrice`, `ChargeResource`, `FormatCostReport`)**:
   - `internal/safety/safety.go` embeds a `us-central1` on-demand price table covering all 22 mutating resource kinds. Hosts and tests never call Cloud Billing or Cost Explorer APIs.
   - `ChargeResource` prices each resource before creation and refuses to create the next resource if cumulative cost would exceed the example's cap (`DefaultExampleCapUSD = $0.05`, configurable via `gcp.set_cost_cap_usd(cap)` up to `HardCostCeilingUSD = $0.25`).
   - Every example writes `build/evidence/live-cost-<example>.json` (`gcp.write_cost_evidence(...)`) and prints `estimated cost: $0.00xx (...)` (or `estimated cost: $0.00 (read-only; ...)` for read-only examples).
