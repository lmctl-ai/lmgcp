#!/usr/bin/env python3
"""Generate schema/lmgcp.dsl.json from lmgcp --lmcode-signatures."""
import json
import re
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
SCHEMA_PATH = ROOT / "schema" / "lmgcp.dsl.json"

RE_CTX = re.compile(r"^context\s+([A-Za-z0-9_]+):\s+([A-Za-z0-9_]+)$")
RE_FN = re.compile(r"^fn\s+([A-Za-z0-9_]+)\(([^)]*)\)\s+->\s+([A-Za-z0-9_]+)$")
RE_METH = re.compile(r"^method\s+([A-Za-z0-9_]+)\.([A-Za-z0-9_]+)\(([^)]*)\)\s+->\s+([A-Za-z0-9_]+)$")


def parse_params(raw: str):
    raw = raw.strip()
    if not raw:
        return []
    return [p.strip() for p in raw.split(",") if p.strip()]


def main():
    bin_path = sys.argv[1] if len(sys.argv) > 1 else str(ROOT / "bin" / "lmgcp")
    out = subprocess.check_output([bin_path, "--lmcode-signatures", "--without-std"], text=True)
    context = {}
    functions = {}
    types = {}

    for line in out.splitlines():
        line = line.strip()
        if not line:
            continue
        m = RE_CTX.match(line)
        if m:
            context[m.group(1)] = m.group(2)
            continue
        m = RE_FN.match(line)
        if m:
            name, params_raw, ret = m.group(1), m.group(2), m.group(3)
            functions.setdefault(name, []).append({
                "params": parse_params(params_raw),
                "returns": ret,
            })
            continue
        m = RE_METH.match(line)
        if m:
            tname, mname, params_raw, ret = m.group(1), m.group(2), m.group(3), m.group(4)
            tentry = types.setdefault(tname, {"methods": {}})
            tentry["methods"].setdefault(mname, []).append({
                "params": parse_params(params_raw),
                "returns": ret,
            })
            continue
        raise ValueError(f"unrecognized signature line: {line}")

    schema = {
        "dsl.schema": 2,
        "name": "lmgcp",
        "context": dict(sorted(context.items())),
        "functions": {k: functions[k] for k in sorted(functions)},
        "types": {
            t: {"methods": {m: types[t]["methods"][m] for m in sorted(types[t]["methods"])}}
            for t in sorted(types)
        },
    }
    SCHEMA_PATH.write_text(json.dumps(schema, indent=2) + "\n", encoding="utf-8", newline="\n")
    print(f"wrote {SCHEMA_PATH} ({len(out.splitlines())} signatures)")


if __name__ == "__main__":
    main()
