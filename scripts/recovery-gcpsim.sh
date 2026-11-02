#!/usr/bin/env bash
# Real cross-process recovery acceptance against the optional gcpsim binary.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
SIM_DIR="${LMCLOUD_SIM_DIR:-/home/ubuntu/cloud/lmcloud-sims/.cache/sims}"
SIM="$SIM_DIR/gcpsim"
if [[ ! -x "$SIM" ]]; then
  echo "SKIP gcpsim not available"
  exit 0
fi
LMSCRIPT="${LMSCRIPT_BIN:-lmscript}"
PORT="$(python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1]);s.close()')"
STATE="$(mktemp -d "${TMPDIR:-/tmp}/lmgcp-recovery-state.XXXXXX")"
DATA="$(mktemp -d "${TMPDIR:-/tmp}/lmgcp-recovery-data.XXXXXX")"
chmod 700 "$STATE"
REAL_HOST="$ROOT/bin/lmgcp"
HOST="$STATE/host.sh"
cat > "$HOST" <<'WRAPPER'
#!/usr/bin/env bash
printf '%s\n' "$$" > "$LMGCP_HOST_PID_FILE"
exec "$LMGCP_REAL_HOST" "$@"
WRAPPER
chmod +x "$HOST"
LOG="$STATE/gcpsim.log"
cleanup() {
  [[ -n "${LM_PID:-}" ]] && kill -9 "$LM_PID" 2>/dev/null || true
  [[ -n "${SIM_PID:-}" ]] && kill "$SIM_PID" 2>/dev/null || true
  [[ -n "${SIM_PID:-}" ]] && wait "$SIM_PID" 2>/dev/null || true
  sleep 0.2
  rm -rf "$STATE" "$DATA" 2>/dev/null || true
}
trap cleanup EXIT
"$SIM" serve --port "$PORT" --data-dir "$DATA" > "$LOG" 2>&1 &
SIM_PID=$!
for _ in $(seq 1 100); do
  if curl -sf -o /dev/null "http://127.0.0.1:$PORT/storage/v1/b?project=test"; then break; fi
  sleep 0.1
done
BASE="http://127.0.0.1:$PORT"
curl -sf -o /dev/null -X POST "$BASE/storage/v1/b?project=test" -H 'Content-Type: application/json' -d '{"name":"lmgcp-unowned-sibling"}'
cat > "$STATE/run.lms" <<LMS
s = load_schema(schema="$ROOT/schema/lmgcp.dsl.json");
p = _lmcode_ s;
  let b = gcp.storage_create_bucket("lmgcp-kill9-demo", "US", "STANDARD");
  let o = gcp.storage_put_object("lmgcp-kill9-demo", "payload.txt", "kill9", "text/plain");
  let n = 0;
  while (n < 1000000000) { n = n + 1; }
end _lmcode_
r = run_lmcode(p, {command:["$HOST","--lmcode","\$DSLFILE"]});
LMS
LMGCP_HOST_PID_FILE="$STATE/host.pid" LMGCP_REAL_HOST="$REAL_HOST" LMGCP_ENDPOINT="$BASE" LMGCP_STATE_DIR="$STATE" "$LMSCRIPT" "$STATE/run.lms" > "$STATE/run.out" 2>&1 &
LM_PID=$!
JOURNAL=""
for _ in $(seq 1 200); do
  JOURNAL="$(find "$STATE" -name 'journal-run-*.jsonl' -type f | head -n 1)"
  if [[ -n "$JOURNAL" ]] && grep -Fq '"event":"confirmed"' "$JOURNAL" && grep -Fq '"kind":"bucket"' "$JOURNAL"; then break; fi
  sleep 0.05
done
[[ -n "$JOURNAL" ]] || { echo "FAIL gcpsim recovery: journal not created"; exit 1; }
for _ in $(seq 1 100); do [[ -s "$STATE/host.pid" ]] && break; sleep 0.05; done
HOST_PID="$(cat "$STATE/host.pid" 2>/dev/null || true)"
[[ -n "$HOST_PID" ]] || { echo "FAIL gcpsim recovery: lmgcp child pid not found"; exit 1; }
kill -9 "$HOST_PID" 2>/dev/null || true
wait "$LM_PID" 2>/dev/null || true
RUN_ID="$(basename "$JOURNAL" .jsonl)"; RUN_ID="${RUN_ID#journal-}"
LMGCP_ENDPOINT="$BASE" LMGCP_STATE_DIR="$STATE" "$REAL_HOST" --sweep --run-id "$RUN_ID" --older-than-hours 0 --delete > "$STATE/sweep1.out" 2>&1
grep -Fq "deleted 1 resource(s)" "$STATE/sweep1.out" || { cat "$STATE/sweep1.out"; echo "FAIL gcpsim recovery: first sweep did not delete"; exit 1; }
STATUS="$(curl -s -o /dev/null -w '%{http_code}' "$BASE/storage/v1/b/lmgcp-kill9-demo")"
[[ "$STATUS" == "404" ]] || { echo "FAIL gcpsim recovery: bucket remains ($STATUS)"; exit 1; }
STATUS="$(curl -s -o /dev/null -w '%{http_code}' "$BASE/storage/v1/b/lmgcp-unowned-sibling")"
[[ "$STATUS" == "200" ]] || { echo "FAIL gcpsim recovery: unowned sibling deleted ($STATUS)"; exit 1; }
LMGCP_ENDPOINT="$BASE" LMGCP_STATE_DIR="$STATE" "$REAL_HOST" --sweep --run-id "$RUN_ID" --older-than-hours 0 --delete > "$STATE/sweep2.out" 2>&1
grep -Fq "deleted 0 resource(s)" "$STATE/sweep2.out" || { cat "$STATE/sweep2.out"; echo "FAIL gcpsim recovery: second sweep not idempotent"; exit 1; }
echo "PASS gcpsim kill-9 recovery"
