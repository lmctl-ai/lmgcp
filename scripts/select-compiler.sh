#!/usr/bin/env bash
# Select by emitted bytecode capability, not CLI version or schema support alone.
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK="$(mktemp -d "${TMPDIR:-/tmp}/lmgcp-compiler-XXXXXX")"
trap 'rm -rf "$WORK"' EXIT
WORK_FOR_COMPILER="$WORK"
if command -v cygpath >/dev/null 2>&1; then
  # Node runs outside MSYS path translation, so give it the native Windows
  # spelling of the temporary probe directory and its files.
  WORK_FOR_COMPILER="$(cygpath -m "$WORK")"
fi
candidate="${LMSCRIPT_BIN:-$(command -v lmscript || true)}"
printf '%s\n' '{"dsl.schema":2,"name":"compiler_probe","types":{},"functions":{},"context":{}}' > "$WORK/probe.schema.json"
signatures=""
if [ -n "$candidate" ]; then
  signatures="$("$candidate" --lmcode-signatures "$WORK_FOR_COMPILER/probe.schema.json" 2>/dev/null || true)"
fi
cat > "$WORK/check.lms" <<LMS
s = load_schema(schema="$WORK_FOR_COMPILER/probe.schema.json");
p = _lmcode_ s;
  let x = 1;
end _lmcode_
r = run_lmcode(p, {command:["probe","--lmcode","\$DSLFILE"]});
LMS

# A v6 compiler carries the v3 std catalog; template_render is the stable
# marker that also proves implicit std is bundled (no external --std file).
if [ -z "$candidate" ] ||
   ! grep -q '^fn template_render(string, Json) -> string$' <<<"$signatures" ||
   ! "$candidate" --lmcode-schema "$WORK_FOR_COMPILER/probe.schema.json" --json >/dev/null 2>&1
then
  if [ -n "${LMGCP_DEBUG:-}" ]; then
    echo "--- compiler probe diagnostics (candidate=$candidate)" >&2
    grep -Fm3 'template_render' <<<"$signatures" >&2
  fi
  echo "error: no implicit-std lmcode v6 compiler found; install @lmctl-ai/lmctl@0.2.453 so lmscript is on PATH, or set LMSCRIPT_BIN" >&2
  exit 2
fi

# --check proves load_schema/_lmcode_/run_lmcode are the current API.
if ! "$candidate" --check "$WORK_FOR_COMPILER/check.lms" >/dev/null 2>&1; then
  if [ -n "${LMGCP_DEBUG:-}" ]; then
    "$candidate" --check "$WORK_FOR_COMPILER/check.lms" 2>&1 | head -10 >&2
  fi
  echo "error: compiler lacks the current load_schema/_lmcode_/run_lmcode API; install @lmctl-ai/lmctl@0.2.453 or newer" >&2
  exit 2
fi
printf '%s\n' "$candidate"
