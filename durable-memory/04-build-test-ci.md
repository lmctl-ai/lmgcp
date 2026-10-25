# 04 — Build, Test & CI

## Building & Regenerating Schema

```bash
./scripts/stamp-version.sh
go build -o bin/lmgcp ./cmd/lmgcp
python3 scripts/gen-schema.py ./bin/lmgcp schema/lmgcp.dsl.json
go build -o bin/lmgcp ./cmd/lmgcp
```

Note: Do not run `go mod tidy` with unused blank imports of `cloud.google.com/go/*` subpackages, as `go mod tidy` walks hundreds of transitive test dependencies; `go.mod` and `go.sum` pin the exact required Google Cloud Go SDK modules (`cloud.google.com/go/auth v0.23.0`, `cloud.google.com/go/compute/metadata v0.9.0`, `google.golang.org/api v0.293.0`).

## Running the Test Suite (`./test.sh`)

`./test.sh` verifies:
1. CLI flags (`--version`, `--help`, `--dsl-schema`, `--dsl-schema --json`, `--dsl-signatures`, `--notices`, `--sweep`)
2. Byte-for-byte schema parity (`cmp <(./bin/lmgcp --dsl-schema --json) schema/lmgcp.dsl.json`) and zero-diff signature parity (`diff -u <(lmscript --dsl-signatures schema/lmgcp.dsl.json) <(./bin/lmgcp --dsl-signatures)`)
3. Go unit & integration tests (`go test ./internal/safety ./internal/host`)
4. Memory budget enforcement (`LMGCP_MAX_MEMORY_MB` invalid values and over-budget exit 137)
5. All 26 `.lms` examples in `examples/*.lms` and non-empty PNG outputs in `images/` and `docs/images/`.
