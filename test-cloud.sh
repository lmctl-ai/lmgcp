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
# Dry-run spy: mutators must not touch the provider. The endpoint is loopback
# but closed, so any real mutator call would fail the dry invocation.
DRY_DIR="$(mktemp -d)"; cat > "$DRY_DIR/dry.lms" <<LMS
s = load_schema(schema="$PWD/schema/lmcloud.dsl.json");
p = _lmcode_ s;
  cloud.set_endpoint("all", "http://127.0.0.1:1");
  cloud.bucket_create("lmcloud-dry-bucket");
  cloud.object_put("lmcloud-dry-bucket", "k", "v");
  cloud.queue_create("lmcloud-dry-queue");
  cloud.queue_send("lmcloud-dry-queue", "m");
  cloud.table_create("lmclouddrytable");
  cloud.item_put("lmclouddrytable", cloud.item_new("k").set_string("a", "v"));
  if (cloud.last_error().code() == ErrorCode.OK) { assert_close(1.0, 1.0, 0.0); } else { assert_close(0.0, 1.0, 0.0); }
end _lmcode_
r = run_lmcode(p, {command:["$PWD/bin/lmgcp","cloud","--lmcode","\$DSLFILE"]});
LMS
"$LMSCRIPT_BIN" --dry-run "$DRY_DIR/dry.lms" >/dev/null || { rm -rf "$DRY_DIR"; echo "FAIL dry-run mutator spy"; exit 1; }
rm -rf "$DRY_DIR"
echo "dry-run mutator spy PASS"
SIM_SH="${SIM_SH:-${LMCLOUD_DIR:-/home/ubuntu/cloud/lmcloud}/scripts/sim.sh}"
LMCLOUD_DIR="${LMCLOUD_DIR:-/home/ubuntu/cloud/lmcloud}"
if [[ -x "$LMCLOUD_DIR/tests/run-conformance.sh" ]]; then
  export LMCLOUD_SIM_DIR="${LMCLOUD_SIM_DIR:-/home/ubuntu/cloud/lmcloud-sims/.cache/sims}"
  if [[ -n "${GCPSIM_JVM_JAR:-}" ]]; then
    PORT="$(python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1]);s.close()')"
    ENDPOINT="http://127.0.0.1:$PORT"
    java -jar "$GCPSIM_JVM_JAR" serve --port "$PORT" >/tmp/lmgcp-test-cloud-gcpsim.log 2>&1 &
    SIM_PID=$!
    cleanup() { kill "$SIM_PID" 2>/dev/null || true; wait "$SIM_PID" 2>/dev/null || true; }
  elif [[ -x "$SIM_SH" ]]; then
    cleanup() { "$SIM_SH" stop gcp --name lmgcp-cloud-test >/dev/null 2>&1 || true; }
    trap cleanup EXIT
    ENDPOINT="$("$SIM_SH" start gcp --name lmgcp-cloud-test)"
    SIM_PID=""
  else
    echo "SKIP lmcloud conformance (no gcpsim/JVM and sim.sh not found)"
    exit 0
  fi
  for _ in $(seq 1 80); do curl -sf -o /dev/null "$ENDPOINT/health" && break; sleep 0.25; done
  trap cleanup EXIT
  "$LMCLOUD_DIR/tests/run-conformance.sh" "$PWD/scripts/lmgcp-cloud-host.sh" "$ENDPOINT" object-storage
  "$LMCLOUD_DIR/tests/run-conformance.sh" "$PWD/scripts/lmgcp-cloud-host.sh" "$ENDPOINT" queue
  "$LMCLOUD_DIR/tests/run-conformance.sh" "$PWD/scripts/lmgcp-cloud-host.sh" "$ENDPOINT" cost-perf
  "$LMCLOUD_DIR/tests/run-conformance.sh" "$PWD/scripts/lmgcp-cloud-host.sh" "$ENDPOINT" tables
  if [[ -n "${GCPSIM_JVM_JAR:-}" ]]; then
    AUTH_CFG="$(mktemp)"
    cat > "$AUTH_CFG" <<'JSON'
{"tokens":{"denied-token":"serviceAccount:denied@example.test"},"Statement":[{"Effect":"Deny","ActionGlob":"*","ResourceGlob":"*","Principal":"serviceAccount:denied@example.test"}]}
JSON
    EPORT="$(python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1]);s.close()')"
    java -jar "$GCPSIM_JVM_JAR" serve --auth enforce --auth-config "$AUTH_CFG" --port "$EPORT" >/tmp/lmgcp-test-cloud-enforce.log 2>&1 &
    EPID=$!
    for _ in $(seq 1 80); do curl -sf -o /dev/null "http://127.0.0.1:$EPORT/health" && break; sleep 0.25; done
    LMCLOUD_ENFORCE_DENIED_GCP_TOKEN=denied-token "$LMSCRIPT_BIN" "$LMCLOUD_DIR/tests/conformance/object-storage/12-auth-enforced-denial.lms" "$PWD/scripts/lmgcp-cloud-host.sh" "http://127.0.0.1:$EPORT" "enf-gcp-blob"
    LMCLOUD_ENFORCE_DENIED_GCP_TOKEN=denied-token "$LMSCRIPT_BIN" "$LMCLOUD_DIR/tests/conformance/queue/13-queue-auth-enforced-denial.lms" "$PWD/scripts/lmgcp-cloud-host.sh" "http://127.0.0.1:$EPORT" "enf-gcp-queue"
    LMCLOUD_ENFORCE_DENIED_GCP_TOKEN=denied-token "$LMSCRIPT_BIN" "$LMCLOUD_DIR/tests/conformance/tables/14-tables-auth-enforced-denial.lms" "$PWD/scripts/lmgcp-cloud-host.sh" "http://127.0.0.1:$EPORT" "enfgcptbl"
    kill "$EPID" 2>/dev/null || true; wait "$EPID" 2>/dev/null || true
    rm -f "$AUTH_CFG"
    echo "Auth enforce scenarios 12, 13 & 14 PASS"
  fi
  cleanup; trap - EXIT
else
  echo "SKIP lmcloud conformance (lmcloud repo not found)"
fi
echo "PHASE A/B/C PASS"
