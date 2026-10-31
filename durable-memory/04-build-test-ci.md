# 04 — Build, Test, Release & Versions

## Local Build & Verification Gate

By user decision, **local build and `./test.sh` are the primary verification gate** (there is no blocking CI gate for documentation/memory updates, and `.github/workflows/ci.yml` ignores `durable-memory/**` and `**.md`).

```bash
# Stamp version (or pass explicit X.Y.Z), build binary, and regenerate embedded schema if signatures changed:
./scripts/stamp-version.sh 0.1.6
CGO_ENABLED=0 go build -o bin/lmgcp ./cmd/lmgcp
python3 scripts/gen-schema.py ./bin/lmgcp schema/lmgcp.dsl.json
CGO_ENABLED=0 go build -o bin/lmgcp ./cmd/lmgcp

# Run the full 57-check offline verification suite:
./test.sh
```

> **Go Module Note**: Do not add unused blank imports of `cloud.google.com/go/*` subpackages and run `go mod tidy`; `go.mod` and `go.sum` pin the exact required Google Cloud Go SDK packages (`cloud.google.com/go/auth`, `cloud.google.com/go/compute/metadata`, `google.golang.org/api`).

## What `./test.sh` Verifies (57 Checks)

1. Executable existence and Linux Rule 14 linkage (`CGO_ENABLED=0` static Go binary / glibc allowlist, zero GPL/LGPL libraries).
2. CLI contracts: `--version`, `--help`, `--notices`, `--lmcode-schema`, `--lmcode-schema --json`, `--lmcode-signatures`, `--lmcode-signatures --without-std`, and `--sweep`.
3. Schema & signature parity:
   - `lmscript --lmcode-signatures --without-std schema/lmgcp.dsl.json` matches `./bin/lmgcp --lmcode-signatures --without-std` (298 host signatures).
   - `lmscript --lmcode-signatures schema/lmgcp.dsl.json` matches `./bin/lmgcp --lmcode-signatures` (370 total signatures).
   - `./bin/lmgcp --lmcode-schema --json` is byte-identical to `schema/lmgcp.dsl.json`.
4. Error & memory budget exit codes:
   - Invalid CLI flags exit `2`.
   - Invalid `LMGCP_MAX_MEMORY_MB` values (`""`, `0`, `-5`, `abc`) exit `2` and name `LMGCP_MAX_MEMORY_MB` on `stderr`.
   - Corrupt `.dslb` bytecode exits `3` with `LOAD_BAD_BYTECODE`.
   - Exceeding `LMGCP_MAX_MEMORY_MB` exits `3` with `LMGCP_MAX_MEMORY_MB` on `stderr` and `RUN_HOST` JSON on `stdout`.
5. Go unit tests: `go test ./internal/safety ./internal/host`.
6. All 40 `.lms` examples in `examples/*.lms` execute cleanly against the in-process fake GCP server, print `ok ...` and `estimated cost: $0.00xx` (or `$0.00`), write `build/evidence/live-cost-<example>.json`, and produce valid PNGs (`89504e47` magic bytes) in `docs/images/`.
7. All 40 `lmcloud` OpenTofu equivalents in `compare/<example>/` (`main.tf` + `<example>.lms`), `compare/RESULTS.md`, and `./compare/run.sh 04_gcs_bucket_lifecycle_ephemeral` pass.

## 5-Platform Release Workflow (`.github/workflows/ci.yml`)

- Runs on code pushes to `main`, `v*` tags, and `workflow_dispatch` using fake endpoints only (no OIDC, no cloud credentials).
- Matrix builds native binaries across 5 targets: `linux-x64` (`ubuntu-22.04`), `linux-arm64` (`ubuntu-22.04-arm`), `macos-arm64` (`macos-15`), `macos-x64` (`macos-15-intel`), and `windows-x64` (`windows-2022`). While cross-repo rules permit Windows to be a stub early when native C/C++ dependencies require it, `lmgcp-windows-x64.exe` is a full functional `CGO_ENABLED=0` Go binary that passes the entire `./test.sh` suite on Windows.
- The `release` job verifies `dist/lmgcp-linux-x64 --version` against the release tag, generates `SHA256SUMS`, and publishes all 6 assets (`5` platform binaries + `SHA256SUMS`) to GitHub Releases.

## Versions Released

- **`v0.1.1`**: Initial `lmgcp` host, 16-domain fake GCP server, safety guardrails, and first 26 examples.
- **`v0.1.2`**: Added `RunLedger` price table, per-example cost cap enforcement (`$0.05` default / `$0.25` hard ceiling), `estimated cost: $0.00xx` output, and README cost notes.
- **`v0.1.4`**: Expanded gallery to 40 `.lms` cloud stories and updated `schema/lmgcp.dsl.json` to omit built-in `lmcode_std_v1` definitions for `lmscript >= 0.2.420`.
- **`v0.1.6` (Latest)**: Added all 40 `lmcloud` (OpenTofu) equivalents under `compare/`, `compare/run.sh` (portable to macOS Bash 3.2 and BSD userland), `compare/RESULTS.md`, and KMS ledger cleanup in `CleanupLedger()`.

## Host flag cutover

Only the lmcode host option family is accepted; retired spellings are refused before initializing fake or live transports. The full gate checks exact schema bytes, signatures, new loader dispatch and retired-option refusal in a scratch HOME. SDK pin 65f0ef2 supports source-compiler v6; compiler selection checks emitted capability. DSLB and .dslb names are unchanged. Cross-tool comparison uses explicit caller PATH rather than a shared checkout.
