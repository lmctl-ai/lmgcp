#!/usr/bin/env bash
# Verifies linkage, version, notices, schema parity, memory budget, safety rules, unit tests, and all examples.
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$HERE"

# Always ensure live mode is disabled during automated tests
unset LMGCP_LIVE

BUILD_DIR="${BUILD_DIR:-bin}"
mkdir -p "$BUILD_DIR"
REPO_FOR_COMPILER="$PWD"
if command -v cygpath >/dev/null 2>&1; then
  # Paths embedded in lmscript source are opened by Node, outside MSYS path
  # translation. Use native Windows form for compiler-visible paths.
  REPO_FOR_COMPILER="$(cygpath -m "$PWD")"
fi
ext=""
case "$(uname -s)" in MINGW*|MSYS*|CYGWIN*) ext=".exe" ;; esac
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "$BUILD_DIR/lmgcp$ext" ./cmd/lmgcp
HOST="$BUILD_DIR/lmgcp$ext"

pass=0
ok() { pass=$((pass+1)); echo "ok $pass - $1"; }
skip() { pass=$((pass+1)); echo "ok $pass # SKIP $1"; }
fail() { echo "FAIL: $*" >&2; exit 1; }

[ -x "$HOST" ] || fail "$HOST not executable"
ok "binary exists and is executable ($HOST)"
python3 tests/check-host-flags.py schema/lmgcp.dsl.json "$HOST" || fail "hard flag cutover"
python3 scripts/test-hostkit-patch.py || fail "cloud integrity portability controls"
python3 scripts/check-hostkit-patch.py || fail "cloud dependency integrity"
go test github.com/lmctl-ai/lmcloud/hostkit github.com/lmctl-ai/lmcloud/hostkit/cloudhost github.com/lmctl-ai/lmcloud/hostkit/fakeadapter github.com/lmctl-ai/lmcloud/hostkit/httpguard github.com/lmctl-ai/lmcloud/hostkit/invoke || fail "cloud compatibility tests"
ok "cloud compatibility integrity and unit tests"

# 1. Platform linkage check (Rule 14: static or dynamic glibc allowlist only on Linux)
uname_s="$(uname -s)"
if [ "$uname_s" = "Linux" ]; then
  if command -v readelf >/dev/null 2>&1; then
    needed="$(readelf -d "$HOST" 2>/dev/null | grep '(NEEDED)' || true)"
    if [ -n "$needed" ]; then
      bad="$(echo "$needed" | grep -vE '\[(libc\.so\.6|libm\.so\.6|libpthread\.so\.0|libdl\.so\.2|librt\.so\.1|ld-linux[^]]*)\]' || true)"
      [ -z "$bad" ] || fail "disallowed NEEDED libraries on Linux: $bad"
    fi
  elif command -v ldd >/dev/null 2>&1; then
    ldd_out="$(ldd "$HOST" 2>&1 || true)"
    case "$ldd_out" in
      *"not a dynamic executable"*|*"statically linked"*) ;;
      *)
        bad="$(echo "$ldd_out" | awk '/=>/ {print $1}' | grep -vE '^(libc\.so\.6|libm\.so\.6|libpthread\.so\.0|libdl\.so\.2|librt\.so\.1|ld-linux.*|linux-vdso\.so\..*)$' || true)"
        [ -z "$bad" ] || fail "disallowed dynamic libs on Linux: $bad"
        ;;
    esac
  fi
  ok "Linux linkage check passed (Rule 14 allowlist / static Go binary)"
elif [ "$uname_s" = "Darwin" ]; then
  if command -v otool >/dev/null 2>&1; then
    bad="$(otool -L "$HOST" | tail -n +2 | awk '{print $1}' | grep -vE '^(/usr/lib/|/System/Library/)' || true)"
    [ -z "$bad" ] || fail "disallowed macOS dynamic libraries: $bad"
  fi
  ok "macOS linkage check passed"
fi

# 2. Version check
want_v="$(head -n1 VERSION | tr -d '[:space:]')"
want="lmgcp $want_v"
got="$("$HOST" --version | tr -d '\r')"
[ "$got" = "$want" ] || fail "--version was '$got', want '$want'"
ok "--version prints '$want'"

# 3. Help and notices check
help_out="$("$HOST" --help | tr -d '\r')"
grep -Fq "LMGCP_MAX_MEMORY_MB" <<<"$help_out" || fail "--help missing LMGCP_MAX_MEMORY_MB"
grep -Fq "glibc >= 2.35" <<<"$help_out" || fail "--help missing glibc >= 2.35 note"
ok "--help documents memory budget, safety, and glibc baseline"

notices_out="$("$HOST" --notices | tr -d '\r')"
grep -Fq "Apache License 2.0" <<<"$notices_out" || fail "--notices missing Apache License 2.0"
grep -Fq "lmcode-go" <<<"$notices_out" || fail "--notices missing lmcode-go"
ok "--notices prints embedded third-party notices"

# 4. Schema & signature parity
sdk_lock_prefix="$(awk '{print substr($2,1,12)}' sdk.lock | tr -d '\r')"
go_mod_prefix="$(awk '/github\.com\/lmctl-ai\/lmcode-go/ {sub(/.*-/, "", $2); print $2}' go.mod | tr -d '\r')"
[ -n "$sdk_lock_prefix" ] && [ "$sdk_lock_prefix" = "$go_mod_prefix" ] || fail "sdk.lock ($sdk_lock_prefix) does not match go.mod lmcode-go ($go_mod_prefix)"
LMSCRIPT="$(scripts/select-compiler.sh)"
diff -u <("$LMSCRIPT" --lmcode-signatures schema/lmgcp.dsl.json | tr -d '\r') <("$HOST" --lmcode-signatures | tr -d '\r') >/dev/null || fail "schema/host signature drift"
ok "host signatures match schema/lmgcp.dsl.json (and sdk.lock matches go.mod)"
diff -u <("$LMSCRIPT" --lmcode-signatures schema/lmgcp.dsl.json --without-std | tr -d '\r') <("$HOST" --lmcode-signatures --without-std | tr -d '\r') >/dev/null || fail "host-only signature drift"
ok "host-only compiler/registry signature parity"

cmp <("$HOST" --lmcode-schema --json) schema/lmgcp.dsl.json >/dev/null || fail "embedded schema differs from schema/lmgcp.dsl.json"
ok "embedded schema is byte-identical"

schema_summary="$("$HOST" --lmcode-schema | tr -d '\r')"
grep -Fq "context gcp: Gcp" <<<"$schema_summary" || fail "schema summary missing context gcp: Gcp"
grep -Fq "ComputeInstance:" <<<"$schema_summary" || fail "schema summary missing ComputeInstance"
grep -Fq "StorageBucket:" <<<"$schema_summary" || fail "schema summary missing StorageBucket"
ok "--lmcode-schema summary formats context and types"

# 5. Sweeper CLI check
sweep_out="$("$HOST" --sweep --older-than-hours 1 | tr -d '\r')"
grep -Fq "sweep dry-run" <<<"$sweep_out" || fail "--sweep failed: $sweep_out"
ok "--sweep dry-run CLI works"

# 6. Invalid usage & invalid memory budget env return exit 2
set +e
invalid_out="$("$HOST" --invalid-flag 2>/dev/null)"
code=$?
set -e
[ "$code" -eq 2 ] || fail "--invalid-flag exited $code, want 2"
grep -Fq "LMCODE_RETURN_BEGIN" <<<"$invalid_out" || fail "--invalid-flag did not emit a failure frame"
ok "invalid CLI flag returns exit 2 with a failure frame"

for bad_mb in "0" "-10" "notanumber" ""; do
  set +e
  err_out="$(LMGCP_MAX_MEMORY_MB="$bad_mb" "$HOST" --version 2>&1)"
  code=$?
  set -e
  [ "$code" -eq 2 ] || fail "LMGCP_MAX_MEMORY_MB='$bad_mb' exited $code, want 2"
  grep -Fq "LMGCP_MAX_MEMORY_MB" <<<"$err_out" || fail "stderr did not name LMGCP_MAX_MEMORY_MB: $err_out"
done
ok "invalid LMGCP_MAX_MEMORY_MB values exit 2 with variable named on stderr"

# 7. Corrupt bytecode returns exit 3 with LOAD_BAD_BYTECODE
mkdir -p build
bad_code="$(mktemp build/bad_code.XXXXXX)"
trap 'rm -f "$bad_code"' EXIT
echo "not valid dslb bytecode" > "$bad_code"
set +e
bad_out="$("$HOST" --lmcode "$bad_code" 2>&1)"
bad_rc=$?
set -e
[ "$bad_rc" -eq 3 ] || fail "corrupt bytecode exited $bad_rc, want 3"
grep -Fq "LMCODE_RETURN_BEGIN" <<<"$bad_out" || fail "bad bytecode missing envelope: $bad_out"
grep -Fq '"code":"LOAD_BAD_BYTECODE"' <<<"$bad_out" || fail "bad bytecode missing LOAD_BAD_BYTECODE: $bad_out"
ok "corrupt bytecode rejected with exit 3, envelope, and LOAD_BAD_BYTECODE"

# 8. Memory budget enforcement test (runaway allocation returns a RUN_HOST envelope)
export PATH="$PWD/$BUILD_DIR:$PATH"
mem_dir="$(mktemp -d build/mem.XXXXXX)"
cat > "$mem_dir/oom.lms" <<LMS
s = load_schema(cli="lmgcp");
p = _lmcode_ s;
  let ok = gcp.alloc_scratch_mb(32);
end _lmcode_
set_env("LMGCP_MAX_MEMORY_MB", "8");
r = run_lmcode(p);
assert(!r.ok && r.code == "RUN_HOST" && contains(r.message, "LMGCP_MAX_MEMORY_MB"), "memory budget failure envelope");
LMS
set +e
"$LMSCRIPT" "$mem_dir/oom.lms" 2>"$mem_dir/oom.stderr"
oom_rc=$?
set -e
oom_stderr="$(cat "$mem_dir/oom.stderr")"
rm -rf "$mem_dir"
[ "$oom_rc" -eq 0 ] || fail "memory budget test exited $oom_rc, want 0 (envelope assertion)"
grep -Fq "LMGCP_MAX_MEMORY_MB" <<<"$oom_stderr" || fail "memory budget stderr missing LMGCP_MAX_MEMORY_MB: $oom_stderr"
ok "memory budget limit enforced with RUN_HOST envelope and stderr message"

# 8b. Dry invocation: real reads, dummy mutators, zero ledger writes, asserts run
dry_dir="$(mktemp -d build/dry.XXXXXX)"
cat > "$dry_dir/dry.lms" <<LMS
s = load_schema(schema="$REPO_FOR_COMPILER/schema/lmgcp.dsl.json");
p = _lmcode_ s;
  let before = gcp.ledger_count();
  let dummy = gcp.compute_create_instance("us-central1-a", "lmgcp-dry-vm", "e2-micro", "debian-12");
  let after = gcp.ledger_count();
  assert_close(float(before), float(after), 0.0);
  let inv = gcp.inventory_summary();
  assert_close(float(str_to_int(inv.get("compute_instances"))), 7.0, 0.0);
  assert_close(float(str_len(gcp.set_region("us-east1").region())), 8.0, 0.0);
  assert_close(1.0, 1.0, 0.0);
end _lmcode_
r = run_lmcode(p, {command:["$HOST","--lmcode","\$DSLFILE"]});
LMS
set +e
dry_out="$("$LMSCRIPT" --dry-run "$dry_dir/dry.lms" 2>&1)"
dry_rc=$?
set -e
rm -rf "$dry_dir"
[ "$dry_rc" -eq 0 ] || fail "dry invocation exited $dry_rc: $dry_out"
grep -Fq '"dry":true' <<<"$dry_out" || fail "dry invocation missing dry envelope: $dry_out"
grep -Fq '"ok":true' <<<"$dry_out" || fail "dry invocation missing ok envelope: $dry_out"
ok "dry invocation executes reads/asserts with zero ledger writes"

# 8c. Repeated --lmcode: lmscript builder default fragment and selector pairs
builder_dir="$(mktemp -d build/builder.XXXXXX)"
cat > "$builder_dir/builder.lms" <<LMS
s = load_schema(schema="$REPO_FOR_COMPILER/schema/lmgcp.dsl.json");
p1 = _lmcode_ s;
  let a = 1;
end _lmcode_
p2 = _lmcode_ s;
  let b = 2;
end _lmcode_
b = lmcode_build("$HOST");
b.add(p1);
b.add(p2);
r = run_lmcode(b);
assert(r.ok, "default builder failed");
assert(r.parts == 2, "default builder parts");
b2 = lmcode_build("$HOST");
b2.add(p1);
b2.add(p2);
r2 = run_lmcode(b2);
assert(r2.ok, "repeated --lmcode builder failed");
assert(r2.parts == 2, "repeated --lmcode builder parts");
LMS
builder_out="$("$LMSCRIPT" "$builder_dir/builder.lms" 2>&1)" || { rm -rf "$builder_dir"; fail "builder invocation failed: $builder_out"; }
rm -rf "$builder_dir"
ok "single and builder invocations use only repeated --lmcode FILE selectors"

# 8d. Dry memory-limit watchdog keeps dry metadata and a valid envelope
watchdog_dir="$(mktemp -d build/watchdog.XXXXXX)"
cat > "$watchdog_dir/watchdog.lms" <<LMS
s = load_schema(schema="$REPO_FOR_COMPILER/schema/lmgcp.dsl.json");
p = _lmcode_ s;
  let x = "x";
  while (str_len(x) < 1000000000) { x = x + x; }
end _lmcode_
r = run_lmcode(p, {command:["$HOST","--lmcode","\$DSLFILE"]});
LMS
set +e
watchdog_out="$(LMGCP_MAX_MEMORY_MB=8 "$LMSCRIPT" --dry-run "$watchdog_dir/watchdog.lms" 2>&1)"
watchdog_rc=$?
set -e
rm -rf "$watchdog_dir"
[ "$watchdog_rc" -eq 0 ] || fail "dry watchdog invocation exited $watchdog_rc: $watchdog_out"
grep -Fq '"dry":true' <<<"$watchdog_out" || fail "dry watchdog frame lost dry:true: $watchdog_out"
grep -Fq '"ok":false' <<<"$watchdog_out" || fail "dry watchdog frame missing failure: $watchdog_out"
ok "dry memory-limit watchdog emits a lmscript-accepted dry frame"

# 8e. One-slot builder abort retains part:0 and dry:true
builder_wd_dir="$(mktemp -d build/builder-wd.XXXXXX)"
cat > "$builder_wd_dir/wd.lms" <<LMS
s = load_schema(schema="$REPO_FOR_COMPILER/schema/lmgcp.dsl.json");
p = _lmcode_ s;
  let x = "x";
  while (str_len(x) < 1000000000) { x = x + x; }
end _lmcode_
b = lmcode_build("$HOST");
b.add(p);
r = run_lmcode(b);
LMS
set +e
builder_wd_out="$(LMGCP_MAX_MEMORY_MB=8 "$LMSCRIPT" --dry-run "$builder_wd_dir/wd.lms" 2>&1)"
builder_wd_rc=$?
set -e
rm -rf "$builder_wd_dir"
[ "$builder_wd_rc" -eq 0 ] || fail "one-slot builder abort exited $builder_wd_rc: $builder_wd_out"
grep -Fq '"part":0' <<<"$builder_wd_out" || fail "one-slot builder abort lost part:0: $builder_wd_out"
grep -Fq '"dry":true' <<<"$builder_wd_out" || fail "one-slot builder abort lost dry:true: $builder_wd_out"
ok "one-slot builder abort retains part:0 and dry:true"

# 8f. Durable abort journal and cross-process --sweep --ledger recovery
abort_dir="$(mktemp -d build/abort.XXXXXX)"
abort_state="$(mktemp -d build/abort-state.XXXXXX)"
cat > "$abort_dir/abort.lms" <<LMS
s = load_schema(schema="$REPO_FOR_COMPILER/schema/lmgcp.dsl.json");
p = _lmcode_ s;
  let x = "x";
  while (str_len(x) < 1000000000) { x = x + x; }
end _lmcode_
r = run_lmcode(p, {command:["$HOST","--lmcode","\$DSLFILE"]});
print(json_stringify(r));
LMS
abort_out="$(LMGCP_STATE_DIR="$abort_state" LMGCP_MAX_MEMORY_MB=8 "$LMSCRIPT" "$abort_dir/abort.lms" 2>&1)" || true
rm -rf "$abort_dir"
grep -Fq '"code":"RUN_HOST"' <<<"$abort_out" || fail "abort frame missing RUN_HOST: $abort_out"
grep -Fq 'recovery run=' <<<"$abort_out" || fail "abort frame missing recovery run id: $abort_out"
abort_ledger="$(grep -o 'ledger=[^ )]*' <<<"$abort_out" | tail -n 1 | cut -d= -f2-)"
[ -s "$abort_ledger" ] || fail "abort journal missing or empty: $abort_ledger"
"$HOST" --sweep --ledger "$abort_ledger" --older-than-hours 0 --delete > "$abort_state/sweep1.out" 2>&1 || fail "first recovery sweep failed"
grep -Fq "recovery: run=" "$abort_state/sweep1.out" || fail "recovery sweep did not load journal"
grep -Fq "deleted 0 resource(s)" "$abort_state/sweep1.out" || fail "empty journal sweep was not idempotent"
"$HOST" --sweep --ledger "$abort_ledger" --older-than-hours 0 --delete > "$abort_state/sweep2.out" 2>&1 || fail "second recovery sweep failed"
grep -Fq "deleted 0 resource(s)" "$abort_state/sweep2.out" || fail "second recovery sweep was not idempotent"
rm -rf "$abort_state"
ok "durable abort journal loads in a separate --sweep process"

# 8g. Real kill -9 recovery against gcpsim when the optional binary exists
sim_dir="${LMCLOUD_SIM_DIR:-/home/ubuntu/cloud/lmcloud-sims/.cache/sims}"
if [ -x "$sim_dir/gcpsim" ]; then
  sim_out="$(LMSCRIPT_BIN="$LMSCRIPT" ./scripts/recovery-gcpsim.sh 2>&1)" || { echo "$sim_out" >&2; fail "gcpsim kill-9 recovery failed"; }
  if grep -q '^SKIP ' <<<"$sim_out"; then
    skip "gcpsim not available"
  else
    grep -Fq "PASS gcpsim kill-9 recovery" <<<"$sim_out" || { echo "$sim_out" >&2; fail "gcpsim recovery did not report PASS"; }
    ok "gcpsim kill-9 recovery reclaims owned bucket and keeps unowned sibling"
  fi
else
  skip "gcpsim not available"
fi

# 9. Go unit tests (safety & fake GCP SDK integration)
go test ./internal/safety ./internal/host >/dev/null || fail "go test failed"
ok "Go unit tests pass (internal/safety, internal/host)"

# 10. Verify all examples compile and run
shopt -s nullglob
example_files=(examples/[0-9][0-9]_*.lms)
[ "${#example_files[@]}" -ge 40 ] || fail "expected at least 40 examples, found ${#example_files[@]}"
for f in "${example_files[@]}"; do
  base="$(basename "$f" .lms)"
  out="$("$LMSCRIPT" "$f" 2>&1)" || { echo "$out" >&2; fail "$f failed"; }
  grep -q '^ok ' <<<"$out" || { echo "$out" >&2; fail "$f printed no ok line"; }
  cost_line="$(echo "$out" | grep -E '^estimated cost: \$0\.(00|[0-9]{4})' || true)"
  [ -n "$cost_line" ] || { echo "$out" >&2; fail "$f missing 'estimated cost: \$0.00xx' line"; }
  [ -s "build/evidence/live-cost-${base}.json" ] || fail "$f did not write build/evidence/live-cost-${base}.json"
  ok "$f ($cost_line)"
done

# The account inventory example is outside the 40 numbered comparison/cost labs.
out="$("$LMSCRIPT" examples/find_lmgcp_resources.lms 2>&1)" || { echo "$out" >&2; fail "prefix inventory example failed"; }
grep -q '^ok ' <<<"$out" || fail "prefix inventory example printed no ok line"
ok "account inventory example"

# 11. Gallery scenes are separate, capability-gated, and skipped until lmdraw/lmplot advance
gallery_out="$(LMSCRIPT_BIN="$LMSCRIPT" ./scripts/render-gallery.sh 2>&1)" || { echo "$gallery_out" >&2; fail "gallery render probe failed"; }
if grep -q '^SKIP ' <<<"$gallery_out"; then
  skip "lmdraw not envelope-capable"
else
  ok "gallery scenes rendered"
fi

# 12. Verify gallery images in docs/images/ exist and have valid PNG magic bytes
img_files=(docs/images/*.png)
if [ "${#img_files[@]}" -gt 0 ]; then
  for img in "${img_files[@]}"; do
    [ -s "$img" ] || fail "empty image $img"
    magic="$(head -c 4 "$img" | od -An -tx1 | tr -d ' \n')"
    [ "$magic" = "89504e47" ] || fail "$img is not a valid PNG (magic=$magic)"
  done
  ok "verified ${#img_files[@]} PNG images in docs/images/"
fi

# 13. Verify all 40 lmcloud (OpenTofu) equivalents under compare/ and compare/RESULTS.md
[ -x "compare/run.sh" ] || fail "compare/run.sh missing or not executable"
[ -s "compare/RESULTS.md" ] || fail "compare/RESULTS.md missing or empty"
compare_dirs=(compare/[0-9][0-9]_*)
[ "${#compare_dirs[@]}" -eq "${#example_files[@]}" ] || fail "expected ${#example_files[@]} compare/* dirs, found ${#compare_dirs[@]}"
for d in "${compare_dirs[@]}"; do
  ex="$(basename "$d")"
  [ -s "$d/main.tf" ] || fail "missing $d/main.tf"
  [ -s "$d/${ex}.lms" ] || fail "missing $d/${ex}.lms"
  grep -q 'source  = "hashicorp/google"' "$d/main.tf" || fail "$d/main.tf missing hashicorp/google provider"
done
compare_out="$(./compare/run.sh 04_gcs_bucket_lifecycle_ephemeral 2>&1)" || { echo "$compare_out" >&2; fail "compare/run.sh 04_gcs_bucket_lifecycle_ephemeral failed"; }
if grep -q '^SKIP compare:' <<<"$compare_out"; then
  skip "lmcloud OpenTofu execution unavailable; current lmcode schema discovery is required"
else
  ok "verified ${#compare_dirs[@]} lmcloud equivalents under compare/, compare/RESULTS.md, and compare/run.sh"
fi

echo "all $pass checks passed"
