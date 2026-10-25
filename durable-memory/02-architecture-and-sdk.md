# 02 — Architecture & Google Cloud Go SDK

## Repository Layout

- `cmd/lmgcp/main.go`: CLI entry point (`--version`, `--help`, `--notices`, `--dsl-schema [--json]`, `--dsl-signatures`, `--dsl [--dry-run]`, `--sweep [--older-than-hours N] [--delete]`), embeds `VERSION` and `THIRD_PARTY_NOTICES`.
- `internal/membudget/membudget.go`: `LMGCP_MAX_MEMORY_MB` parser (default 4096 MiB), explicit allocation charge tracker (`alloc_scratch_mb`), and 2ms runtime heap watchdog (`exit 137` on breach).
- `internal/safety/safety.go` & `safety_test.go`: Secret redaction, project ID masking, prefix/shape/tier guardrails, and `RunLedger` ownership tracking.
- `internal/fakegcp/seed.go` & `server.go`: Deterministic in-process HTTP REST server emulating 16 Google Cloud REST APIs pre-seeded with 43 realistic resources across `prod-vpc`, `staging-vpc`, and `analytics-vpc`.
- `internal/host/types.go`, `collections.go`, `host.go`, `register.go`, `host_test.go`: `Host` implementation wrapping official Google Cloud Go SDK clients and registering 365 DSL signatures on `lmcode.Registry`.
- `schema/lmgcp.dsl.json` & `schema/schema.go`: Embedded JSON schema (generated via `scripts/gen-schema.py` to match `./bin/lmgcp --dsl-signatures` 100%).
