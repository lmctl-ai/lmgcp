#!/usr/bin/env bash
# Select by emitted bytecode capability, not CLI version or schema support alone.
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK="$(mktemp -d "${TMPDIR:-/tmp}/lmgcp-compiler-XXXXXX")"
trap 'rm -rf "$WORK"' EXIT
candidate="${LMSCRIPT_BIN:-$(command -v lmscript || true)}"
printf '%s\n' '{"dsl.schema":2,"name":"compiler_probe","types":{},"functions":{},"context":{}}' > "$WORK/probe.schema.json"
# A v6 compiler carries the v3 std catalog; template_render is the stable
# marker that also proves implicit std is bundled (no external --std file).
if [ -n "$candidate" ] &&
   "$candidate" --lmcode-signatures "$WORK/probe.schema.json" 2>/dev/null | grep -q '^fn template_render(string, Json) -> string$' &&
   "$candidate" --lmcode-schema "$WORK/probe.schema.json" --json >/dev/null 2>&1
then
  printf '%s\n' "$candidate"
else
  if [ -n "${LMGCP_DEBUG:-}" ]; then
    echo "--- compiler probe diagnostics (candidate=$candidate)" >&2
    [ -n "$candidate" ] && "$candidate" --lmcode-signatures "$WORK/probe.schema.json" 2>&1 | grep 'template_render' | head -3 >&2
  fi
  echo "error: no implicit-std lmcode v6 compiler found; install @lmctl-ai/lmctl@0.2.420 so lmscript is on PATH, or set LMSCRIPT_BIN" >&2
  exit 2
fi
