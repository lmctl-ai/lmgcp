# 07 — Live Testing Procedure & Open Items

## Live-Test Procedure ([`docs/LIVE_TESTING.md`](../docs/LIVE_TESTING.md))

Full step-by-step instructions for running `lmgcp` against a real non-production GCP sandbox project are documented in [`docs/LIVE_TESTING.md`](../docs/LIVE_TESTING.md) and [`README.md`](../README.md):

1. **Authenticate & opt in**: Verify Application Default Credentials (`gcloud auth application-default print-access-token >/dev/null`) and export `LMGCP_PROJECT`, `LMGCP_REGION=us-central1`, `LMGCP_ZONE=us-central1-a`, and `LMGCP_LIVE=1`.
2. **Phase 1 — Read-only smoke tests (`$0.00`)**: Run read-only examples (`01`, `02`, `03`, `08`, `38`) first to confirm ADC connectivity and masked project output with zero mutations.
3. **Phase 2 — Guarded ephemeral write & teardown tests**: Run individual mutating examples (`04`, `05`, `13`, `40`) to verify `lmgcp_` / `lmgcp-` prefix creation, `lmgcp_test` labeling, `RunLedger` cost-cap enforcement, and LIFO `gcp.cleanup_ledger()` teardown.
4. **Phase 3 — Side-by-side `lmcloud` comparison**: Run `./compare/run.sh <example>` to compare native `lmgcp` execution against `lmcloud` (`hashicorp/google` provider) on the same project.
5. **Post-run sweeper check**: Run `./bin/lmgcp --sweep --older-than-hours 0` (and `--delete` if needed) to verify `0` leftover `lmgcp_*` resources, then `unset LMGCP_LIVE LMGCP_PROJECT`.

## Genuinely Open Items

1. **Live GCP Execution Pending the User (`LMGCP_LIVE=1`)**:
   - Ambient `gcloud` credentials on this machine belong to a real account and were never touched during development or CI. All 40 examples and 40 `compare/` equivalents have been verified offline against the in-process fake server and OpenTofu lifecycle, and are marked **READY TO TEST / pending user test**.
2. **Live Cost Calibration vs Price Table**:
   - All per-example cost figures in `README.md`, `build/evidence/live-cost-*.json`, and `compare/RESULTS.md` are currently labeled `"estimated from price table, not yet measured live"`. Once the user executes a live run on a sandbox GCP project, the recorded ledger evidence can be compared against actual billing export / Cost Explorer data ~24h later.
3. **Live `hashicorp/google` Provider Run in `compare/run.sh`**:
   - Offline comparison verifies `compare/<example>/main.tf` formatting/structure and runs the in-process OpenTofu lifecycle via `terraform_data`. Running `tofu.init` / `tofu.apply` / `tofu.destroy` directly against live GCP with the external `hashicorp/google` provider is pending user approval of live testing.
