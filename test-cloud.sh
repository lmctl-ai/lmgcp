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
SIM_SH="${SIM_SH:-/home/ubuntu/cloud/lmcloud-sims/scripts/sim.sh}"
LMCLOUD_DIR="${LMCLOUD_DIR:-/home/ubuntu/cloud/lmcloud}"
if [[ -x "$SIM_SH" && -x "$LMCLOUD_DIR/tests/run-conformance.sh" ]]; then
  export LMCLOUD_SIM_DIR="${LMCLOUD_SIM_DIR:-/home/ubuntu/cloud/lmcloud-sims/.cache/sims}"
  cleanup() { "$SIM_SH" stop gcp --name lmgcp-cloud-test >/dev/null 2>&1 || true; }
  trap cleanup EXIT
  ENDPOINT="$("$SIM_SH" start gcp --name lmgcp-cloud-test)"
  "$LMCLOUD_DIR/tests/run-conformance.sh" "$PWD/scripts/lmgcp-cloud-host.sh" "$ENDPOINT" object-storage
  echo "object-storage conformance PASS; queue conformance is WIP (gcpsim REST modifyAckDeadline gap)"
  cleanup; trap - EXIT
else
  echo "SKIP lmcloud conformance (sim or lmcloud repo not found)"
fi
echo "PHASE A/B PASS"
