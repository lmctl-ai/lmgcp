#!/usr/bin/env bash
# Render gallery scenes only when the draw/plot hosts speak the current lmcode
# envelope protocol. Until then test.sh prints a SKIP line and the committed
# docs/images/*.png remain the last renderings.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
LMSCRIPT="${LMSCRIPT_BIN:-lmscript}"

probe_host() {
  local host="$1"
  command -v "$host" >/dev/null 2>&1 || return 1
  "$host" --lmcode-schema --json >/dev/null 2>&1 || return 1
  local dir out
  dir="$(mktemp -d "${TMPDIR:-/tmp}/lmgcp-gallery-XXXXXX")"
  out="$dir/out"
  cat > "$dir/probe.lms" <<LMS
s = load_schema(cli="$host");
p = _lmcode_ s;
  let x = 1;
end _lmcode_
r = run_lmcode(p);
LMS
  if ! "$LMSCRIPT" --dry-run "$dir/probe.lms" >"$out" 2>&1; then
    rm -rf "$dir"
    return 1
  fi
  if ! grep -q '"dry":true' "$out" || ! grep -q '"ok":true' "$out"; then
    rm -rf "$dir"
    return 1
  fi
  rm -rf "$dir"
}

if ! probe_host lmdraw || ! probe_host lmplot; then
  echo "SKIP lmdraw not envelope-capable"
  exit 0
fi

shopt -s nullglob
scenes=(gallery/*.draw.lms)
[ "${#scenes[@]}" -gt 0 ] || { echo "SKIP no gallery scenes"; exit 0; }
for scene in "${scenes[@]}"; do
  base="$(basename "$scene" .draw.lms)"
  input="build/evidence/live-cost-${base}.json"
  "$LMSCRIPT" "$scene" "$input"
done
echo "rendered ${#scenes[@]} gallery scenes"
