#!/usr/bin/env bash
# Phase A shared-schema smoke: build, schema/signature parity, trivial real+dry.
set -euo pipefail
cd "$(dirname "$0")"
LMSCRIPT_BIN="${LMSCRIPT_BIN:-/home/ubuntu/lmctl-mono/lmscript/bin/lmscript}"
mkdir -p bin
CGO_ENABLED=0 go build -trimpath -o bin/lmgcp ./cmd/lmgcp
./scripts/sync-lmcloud-schema.sh --verify
cmp -s <(./bin/lmgcp cloud --lmcode-schema --json) schema/lmcloud.dsl.json
diff -u <("$LMSCRIPT_BIN" --lmcode-signatures schema/lmcloud.dsl.json) <(./bin/lmgcp cloud --lmcode-signatures)
diff -u <("$LMSCRIPT_BIN" --lmcode-signatures schema/lmcloud.dsl.json --without-std) <(./bin/lmgcp cloud --lmcode-signatures --without-std)
"$LMSCRIPT_BIN" -e '
s = load_schema(schema="schema/lmcloud.dsl.json");
p = _lmcode_ s;
  let prov = cloud.provider();
  lmcode_return_put("prov", prov);
end _lmcode_
r = run_lmcode(p, {command:["./bin/lmgcp","cloud","--lmcode","$DSLFILE"]});
assert(r.ok && r.result.prov == "gcp", "trivial shared-schema host failed");
'
go vet ./...
echo "PHASE A PASS"
