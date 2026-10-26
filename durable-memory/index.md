# `lmgcp` Durable Memory Index

Current architecture, safety/cost model, schema, examples, `lmcloud` (OpenTofu) comparison suite, live-test procedure, and open items for `lmctl-ai/lmgcp` (`v0.1.6`).

## Document Map

- [01-purpose-and-safety.md](01-purpose-and-safety.md) — Purpose, fake-by-default transport (`LMGCP_LIVE=1` opt-in), no OIDC/credentials in CI, secret redaction, project ID masking, `lmgcp_` / `lmgcp-` prefix + `lmgcp_test` label, shape allowlists, `RunLedger` LIFO teardown, `--sweep` orphan sweeper, and `us-central1` cost price table (`$0.05` default cap / `$0.25` hard ceiling).
- [02-architecture-and-sdk.md](02-architecture-and-sdk.md) — Repository layout, `lmcode-go` (`sdk.lock`) + Google Cloud Go SDK pins (`go.mod`), in-process `fakegcp` server (43 seeded resources), and `LMGCP_MAX_MEMORY_MB` memory budget contract (exit `2` on invalid env, exit `3` with `RUN_HOST` JSON + `CleanupLedger()` on breach).
- [03-schema-and-services.md](03-schema-and-services.md) — Curated 16-domain GCP service set, 20 host types in `schema/lmgcp.dsl.json`, 298 host-only signatures (`--without-std`), 370 total signatures, and implicit `lmcode_std_v1` support on `lmscript >= 0.2.420`.
- [04-build-test-ci.md](04-build-test-ci.md) — Local build and `./test.sh` (57 checks) as the verification gate (no blocking CI gate by user decision), schema generation (`scripts/gen-schema.py`), 5-platform release workflow (`SHA256SUMS`), and released versions (`v0.1.1` .. `v0.1.6`).
- [05-examples-and-visuals.md](05-examples-and-visuals.md) — Catalog of all 40 `.lms` cloud stories (14 read-only `$0.00` audits + 26 mutating ephemeral stories totaling `$0.0218` and 88 resources), `lmctl autorun` execution, and README cost labels.
- [06-lmcloud-equivalents.md](06-lmcloud-equivalents.md) — All 40 `lmcloud` (OpenTofu `v1.13.2`) equivalents in [`compare/`](../compare/), [`compare/run.sh`](../compare/run.sh), [`compare/RESULTS.md`](../compare/RESULTS.md) parity summary (`Same` vs `Differs & Why`), and known `lmcloud` findings/limitations.
- [07-live-testing-and-open-items.md](07-live-testing-and-open-items.md) — Live GCP testing procedure ([`docs/LIVE_TESTING.md`](../docs/LIVE_TESTING.md)) and genuinely open items (live GCP testing `LMGCP_LIVE=1` pending user test).
