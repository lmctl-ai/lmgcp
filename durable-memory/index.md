# lmgcp Durable Memory Index

- [01-purpose-and-safety.md](01-purpose-and-safety.md) — Mission, 7 binding Cloud Safety Guardrails, secret redaction, project ID masking, prefix/cost checks, and `RunLedger` ownership.
- [02-architecture-and-sdk.md](02-architecture-and-sdk.md) — Go 1.26 + `lmcode-go` host architecture, Google Cloud Go SDK (`google.golang.org/api` + `cloud.google.com/go/auth`), and in-process `fakegcp` REST emulator.
- [03-schema-and-services.md](03-schema-and-services.md) — 16 GCP APIs, 19 DSL object types, and 365 DSL signatures in `schema/lmgcp.dsl.json`.
- [04-build-test-ci.md](04-build-test-ci.md) — Build commands, `scripts/gen-schema.py`, `test.sh`, memory budget (`LMGCP_MAX_MEMORY_MB`), 5-platform GitHub Actions CI, and live GCP opt-in workflow.
- [05-examples-and-visuals.md](05-examples-and-visuals.md) — Catalog of all 26 self-executing `.lms` cloud stories and their `docs/images/*.png` visuals.
