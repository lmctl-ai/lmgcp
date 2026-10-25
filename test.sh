#!/usr/bin/env bash
# Verifies linkage, version, notices, schema parity, memory budget, safety rules, unit tests, and all examples.
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$HERE"

# Always ensure live mode is disabled during automated tests
unset LMGCP_LIVE

BUILD_DIR="${BUILD_DIR:-bin}"
HOST="$BUILD_DIR/lmgcp"
if [ ! -x "$HOST" ] && [ -x "$HOST.exe" ]; then
  HOST="$HOST.exe"
fi
if [ ! -x "$HOST" ]; then
  mkdir -p "$BUILD_DIR"
  ext=""
  case "$(uname -s)" in MINGW*|MSYS*|CYGWIN*) ext=".exe" ;; esac
  CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "$BUILD_DIR/lmgcp$ext" ./cmd/lmgcp
  HOST="$BUILD_DIR/lmgcp$ext"
fi

pass=0
ok() { pass=$((pass+1)); echo "ok $pass - $1"; }
fail() { echo "FAIL: $*" >&2; exit 1; }

[ -x "$HOST" ] || fail "$HOST not executable"
ok "binary exists and is executable ($HOST)"

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
echo "$help_out" | grep -q "LMGCP_MAX_MEMORY_MB" || fail "--help missing LMGCP_MAX_MEMORY_MB"
echo "$help_out" | grep -q "glibc >= 2.35" || fail "--help missing glibc >= 2.35 note"
ok "--help documents memory budget, safety, and glibc baseline"

notices_out="$("$HOST" --notices | tr -d '\r')"
echo "$notices_out" | grep -q "Apache License 2.0" || fail "--notices missing Apache License 2.0"
echo "$notices_out" | grep -q "lmcode-go" || fail "--notices missing lmcode-go"
ok "--notices prints embedded third-party notices"

# 4. Schema & signature parity
LMSCRIPT="$(scripts/select-compiler.sh)"
diff -u <("$LMSCRIPT" --dsl-signatures schema/lmgcp.dsl.json | tr -d '\r') <("$HOST" --dsl-signatures | tr -d '\r') >/dev/null || fail "schema/host signature drift"
ok "host signatures match schema/lmgcp.dsl.json"
diff -u <("$LMSCRIPT" --dsl-signatures schema/lmgcp.dsl.json --without-std | tr -d '\r') <("$HOST" --dsl-signatures --without-std | tr -d '\r') >/dev/null || fail "host-only signature drift"
ok "host-only compiler/registry signature parity"

cmp <("$HOST" --dsl-schema --json) schema/lmgcp.dsl.json >/dev/null || fail "embedded schema differs from schema/lmgcp.dsl.json"
ok "embedded schema is byte-identical"

schema_summary="$("$HOST" --dsl-schema | tr -d '\r')"
echo "$schema_summary" | grep -q "context gcp: Gcp" || fail "schema summary missing context gcp: Gcp"
echo "$schema_summary" | grep -q "ComputeInstance:" || fail "schema summary missing ComputeInstance"
echo "$schema_summary" | grep -q "StorageBucket:" || fail "schema summary missing StorageBucket"
ok "--dsl-schema summary formats context and types"

# 5. Sweeper CLI check
sweep_out="$("$HOST" --sweep --older-than-hours 1 | tr -d '\r')"
echo "$sweep_out" | grep -q "sweep dry-run" || fail "--sweep failed: $sweep_out"
ok "--sweep dry-run CLI works"

# 6. Invalid usage & invalid memory budget env return exit 2
set +e
"$HOST" --invalid-flag >/dev/null 2>&1
code=$?
set -e
[ "$code" -eq 2 ] || fail "--invalid-flag exited $code, want 2"
ok "invalid CLI flag returns exit 2"

for bad_mb in "0" "-10" "notanumber" ""; do
  set +e
  err_out="$(LMGCP_MAX_MEMORY_MB="$bad_mb" "$HOST" --version 2>&1)"
  code=$?
  set -e
  [ "$code" -eq 2 ] || fail "LMGCP_MAX_MEMORY_MB='$bad_mb' exited $code, want 2"
  echo "$err_out" | grep -q "LMGCP_MAX_MEMORY_MB" || fail "stderr did not name LMGCP_MAX_MEMORY_MB: $err_out"
done
ok "invalid LMGCP_MAX_MEMORY_MB values exit 2 with variable named on stderr"

# 7. Corrupt bytecode returns exit 3 with LOAD_BAD_BYTECODE
mkdir -p build
bad_code="$(mktemp build/bad_code.XXXXXX)"
trap 'rm -f "$bad_code"' EXIT
echo "not valid dslb bytecode" > "$bad_code"
set +e
bad_out="$("$HOST" --dsl "$bad_code" 2>&1)"
bad_rc=$?
set -e
[ "$bad_rc" -eq 3 ] || fail "corrupt bytecode exited $bad_rc, want 3"
echo "$bad_out" | grep -q "LOAD_BAD_BYTECODE" || fail "bad bytecode missing LOAD_BAD_BYTECODE: $bad_out"
ok "corrupt bytecode rejected with exit 3 and LOAD_BAD_BYTECODE"

# 8. Memory budget enforcement test (runaway allocation fails cleanly with exit 3)
export PATH="$PWD/$BUILD_DIR:$PATH"
mem_dir="$(mktemp -d build/mem.XXXXXX)"
cat > "$mem_dir/oom.lms" <<LMS
p = _dsl_ cli="lmgcp";
  let ok = gcp.alloc_scratch_mb(32);
end _dsl_
savedsl(p, "$mem_dir/oom.dslb");
LMS
"$LMSCRIPT" "$mem_dir/oom.lms" >/dev/null 2>&1
set +e
oom_stdout="$(LMGCP_MAX_MEMORY_MB=8 "$HOST" --dsl "$mem_dir/oom.dslb" 2>"$mem_dir/oom.stderr")"
oom_rc=$?
set -e
oom_stderr="$(cat "$mem_dir/oom.stderr")"
rm -rf "$mem_dir"
[ "$oom_rc" -eq 3 ] || fail "memory budget test exited $oom_rc, want 3"
echo "$oom_stderr" | grep -q "LMGCP_MAX_MEMORY_MB" || fail "memory budget stderr missing LMGCP_MAX_MEMORY_MB: $oom_stderr"
echo "$oom_stdout" | grep -q '"code": "RUN_HOST"' || fail "memory budget stdout missing RUN_HOST: $oom_stdout"
ok "memory budget limit enforced with exit 3, stderr message, and JSON RUN_HOST"

# 9. Go unit tests (safety & fake GCP SDK integration)
go test ./internal/safety ./internal/host >/dev/null || fail "go test failed"
ok "Go unit tests pass (internal/safety, internal/host)"

# 10. Verify all examples compile and run
shopt -s nullglob
example_files=(examples/*.lms)
[ "${#example_files[@]}" -ge 40 ] || fail "expected at least 40 examples, found ${#example_files[@]}"
for f in "${example_files[@]}"; do
  base="$(basename "$f" .lms)"
  out="$("$LMSCRIPT" "$f" 2>&1)" || { echo "$out" >&2; fail "$f failed"; }
  echo "$out" | grep -q "^ok " || { echo "$out" >&2; fail "$f printed no ok line"; }
  cost_line="$(echo "$out" | grep -E '^estimated cost: \$0\.(00|[0-9]{4})' || true)"
  [ -n "$cost_line" ] || { echo "$out" >&2; fail "$f missing 'estimated cost: \$0.00xx' line"; }
  [ -s "build/evidence/live-cost-${base}.json" ] || fail "$f did not write build/evidence/live-cost-${base}.json"
  ok "$f ($cost_line)"
done

# 11. Verify gallery images in docs/images/ exist and have valid PNG magic bytes
img_files=(docs/images/*.png)
if [ "${#img_files[@]}" -gt 0 ]; then
  for img in "${img_files[@]}"; do
    [ -s "$img" ] || fail "empty image $img"
    magic="$(head -c 4 "$img" | od -An -tx1 | tr -d ' \n')"
    [ "$magic" = "89504e47" ] || fail "$img is not a valid PNG (magic=$magic)"
  done
  ok "verified ${#img_files[@]} PNG images in docs/images/"
fi

# 12. Verify all 40 lmcloud (OpenTofu) equivalents under compare/ and compare/RESULTS.md
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
./compare/run.sh 04_gcs_bucket_lifecycle_ephemeral >/dev/null || fail "compare/run.sh 04_gcs_bucket_lifecycle_ephemeral failed"
ok "verified ${#compare_dirs[@]} lmcloud equivalents under compare/, compare/RESULTS.md, and compare/run.sh"

echo "all $pass checks passed"
