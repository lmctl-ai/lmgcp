#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SRC="${LMCLOUD_DIR:-/home/ubuntu/cloud/lmcloud}/schema/lmcloud.dsl.json"
DST="$ROOT/schema/lmcloud.dsl.json"
if [[ "${1:-}" == "--verify" ]]; then
  cmp -s "$SRC" "$DST" || { echo "lmcloud schema drift: $SRC vs $DST" >&2; exit 1; }
  exit 0
fi
cp "$SRC" "$DST"
sha256sum "$DST"
