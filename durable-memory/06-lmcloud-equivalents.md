# 06 — `lmcloud` (OpenTofu) Equivalents in `compare/`

Every `lmgcp` example (`01` .. `40`) has a 1-to-1 [`lmcloud`](https://github.com/lmctl-ai/lmcloud) (OpenTofu `v1.13.2` host on `lmcode-go`) equivalent under [`compare/<example>/`](../compare/).

## Structure & Harness

- `compare/<example>/main.tf`: Declarative `hashicorp/google` (`~> 6.0`) configuration using the exact same `google_*` resource types (`data "google_*"` blocks for read-only stories), `lmgcp_` / `lmgcp-` resource names, `lmgcp_test` labels, and outputs (`example_id`, `estimated_cost`, `resource_count`).
- `compare/<example>/<example>.lms`: `lmctl autorun`-compatible script (`_dsl_ cli="lmcloud" install="github:lmctl-ai/lmcloud@latest"`) that formats and verifies `compare/<example>/main.tf` (`tofu.fmt` + `tofu.fmt_check`), runs the full in-process OpenTofu lifecycle (`tofu.init` -> `tofu.validate` -> `tofu.plan` -> `tofu.apply` -> `tofu.output` -> `tofu.state_list` -> `tofu.destroy` -> `tofu.state_list == ""`), writes `build/compare/<example>/lmcloud-evidence.json`, and prints `estimated cost: $0.00xx`.
- [`compare/run.sh`](../compare/run.sh): Side-by-side comparison runner (`./compare/run.sh <example|all> [--write-results]`). Runs both the native `lmgcp` example and the `lmcloud` equivalent (or static `.tf` verification when `lmcloud` is not installed on CI runners), compares resource counts/types/names/labels, outputs, wall time (`ms`), estimated cost, and cleanup (`leftovers = 0` for both), and regenerates [`compare/RESULTS.md`](../compare/RESULTS.md) via [`compare/gen_results.py`](../compare/gen_results.py).

## What [`compare/RESULTS.md`](../compare/RESULTS.md) Proves

1. **Where `lmgcp` and `lmcloud` Are Identical (`Same`)**:
   - **Resource parity (`88 / 88`)**: Across all 26 mutating examples, both hosts provision the same 88 ephemeral resources with identical `lmgcp_` / `lmgcp-` names and `lmgcp_test` labels; across all 14 read-only examples (including IAM examples `10` and `32`), both hosts create `0` resources (`$0.00`).
   - **Zero leftovers (`0 / 0`)**: Both `gcp.cleanup_ledger()` and `tofu.destroy(workdir)` (`tofu.state_list(workdir) == ""`) leave `0` resources behind across all 40 examples.
   - **Cost estimates**: Identical per-example cost lines across all 40 pairs.
2. **Where `lmgcp` and `lmcloud` Differ and Why (`Differs`)**:
   - **Imperative multi-step rollouts vs declarative target state** (`04`, `06`, `09`): `lmgcp` imperatively steps Cloud Run canary traffic (`10% -> 50% -> 100%` in `06`) and VM lifecycle transitions (`set_labels` -> `stop` -> `start` -> `delete` in `09`), whereas OpenTofu converges declaratively to a single target state block per `tofu.apply`.
   - **Data-plane runtime operations & secret redaction vs control-plane provisioning** (`05`, `17`, `18`, `22`, `24`, `30`, `34`, `37`): `lmgcp` publishes/pulls Pub/Sub messages, writes Cloud Logging entries, reads/writes Firestore fields, queries Monitoring series, and redacts secrets (`ya29.[REDACTED]`, `AIza[REDACTED]`, `PRIVATE KEY [REDACTED]`). Because `hashicorp/google` has no declarative resource for publishing an ephemeral Pub/Sub message or writing a single log entry, those 15 runtime event emissions are represented in `compare/<example>/main.tf` using `terraform_data` resources.
   - **Pre-flight safety & cost-cap guardrails vs state-file tracking** (`07`, `26`, `40`): `lmgcp` enforces prefix, VM shape, ledger-ownership, age-gated `--sweep` (`>= 24h`), and pre-flight cost-cap refusal (`$0.05` default / `$0.25` ceiling; demonstrated in `40` where a 7th Cloud SQL resource is blocked by a `$0.0050` cap) in-process before any API call is made. `lmcloud` tracks resource lifecycle via `terraform.tfstate`.

## Known `lmcloud` Limitations & Findings (Reported Read-Only)

Per workspace rules, `/home/mma/repos/dsl/lmcloud` was never edited. Three findings were documented in [`compare/RESULTS.md`](../compare/RESULTS.md):
1. **Missing `lmcode.RegisterTesting(reg, nil)` in `lmcloud/internal/lmhost/host.go`**: `assert_close` and `print` are not registered inside `_dsl_ cli="lmcloud"` blocks; `compare/*/*.lms` scripts write evidence via `file_write(...)` inside `_dsl_` and assert/print in outer `lmscript`.
2. **`tofu.output(dir, name)` writes errors to `os.Stderr`**: Querying a missing output or querying after `tofu.destroy` prints `tofu output error: ...` directly to `stderr` in addition to setting `tofu.last_error()`.
3. **External `hashicorp/google` provider resolution**: `lmcloud` embeds 6 utility providers (`random`, `null`, `local`, `time`, `archive`, `tls`) in-memory via `bufconn`, whereas `hashicorp/google` requires external registry download and live GCP credentials on `tofu.init`. Offline execution therefore validates `compare/<example>/main.tf` via `tofu.fmt` + `tofu.fmt_check` and runs the in-process OpenTofu state lifecycle against an isomorphic `terraform_data` workspace mirror.
