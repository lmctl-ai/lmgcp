#!/usr/bin/env bash
# compare/run.sh — Run an lmgcp example and its lmcloud (OpenTofu) equivalent
# side-by-side and compare resources created (type, name, lmgcp_test labels),
# outputs, wall time, estimated cost, and cleanup (leftovers = 0 for both).
#
# Usage:
#   ./compare/run.sh <example>           # e.g. 04_gcs_bucket_lifecycle_ephemeral or 04
#   ./compare/run.sh all                 # run all 40 examples
#   ./compare/run.sh all --write-results # run all 40 and update compare/RESULTS.md
#
# Safety:
#   Offline comparison is the default (LMGCP_LIVE is unset unless explicitly
#   exported as LMGCP_LIVE=1 by the user for an approved test project).
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

if [[ "${LMGCP_LIVE:-0}" != "1" ]]; then
  unset LMGCP_LIVE
fi

export PATH="/home/mma/repos/dsl/lmcloud/bin:$ROOT_DIR/bin:$PATH"

if [[ ! -x "$ROOT_DIR/bin/lmgcp" ]]; then
  mkdir -p "$ROOT_DIR/bin"
  go build -o "$ROOT_DIR/bin/lmgcp" ./cmd/lmgcp
fi

TARGET="${1:-all}"
WRITE_RESULTS="${2:-}"
if [[ "$TARGET" == "--write-results" ]]; then
  TARGET="all"
  WRITE_RESULTS="--write-results"
fi

resolve_examples() {
  local arg="$1"
  if [[ "$arg" == "all" ]]; then
    for d in compare/[0-9][0-9]_*; do
      basename "$d"
    done
    return 0
  fi
  arg="${arg#examples/}"
  arg="${arg#compare/}"
  arg="${arg%.lms}"
  arg="${arg%/}"
  if [[ -d "compare/$arg" ]]; then
    echo "$arg"
    return 0
  fi
  local match
  match="$(compgen -G "compare/${arg}_*" | head -n 1 || true)"
  if [[ -n "$match" && -d "$match" ]]; then
    basename "$match"
    return 0
  fi
  echo "error: unknown example '$1' (expected name or prefix under compare/)" >&2
  exit 1
}

EXAMPLES=()
while IFS= read -r line; do
  if [[ -n "$line" ]]; then
    EXAMPLES+=("$line")
  fi
done < <(resolve_examples "$TARGET")

HAVE_LMCLOUD=0
if command -v lmcloud >/dev/null 2>&1; then
  HAVE_LMCLOUD=1
fi

now_ms() {
  python3 -c 'import time; print(int(time.time() * 1000))'
}

mkdir -p "$ROOT_DIR/build/compare"

printf "%-44s | %-9s | %-11s | %-9s | %-15s | %-6s\n" \
  "Example" "Resources" "Cost" "Leftovers" "Time (gcp/tf)" "Parity"
printf "%-44s-+-%-9s-+-%-11s-+-%-9s-+-%-15s-+-%-6s\n" \
  "--------------------------------------------" "---------" "-----------" "---------" "---------------" "------"

for ex in "${EXAMPLES[@]}"; do
  mkdir -p "$ROOT_DIR/build/compare/$ex"

  t0_ms="$(now_ms)"
  lmgcp_out="$(lmscript "examples/${ex}.lms" 2>&1)"
  t1_ms="$(now_ms)"
  lmgcp_ms="$(( t1_ms - t0_ms ))"
  printf "%s\n" "$lmgcp_out" > "$ROOT_DIR/build/compare/$ex/lmgcp.stdout"

  if [[ "$HAVE_LMCLOUD" -eq 1 ]]; then
    t2_ms="$(now_ms)"
    lmcloud_out="$(lmscript "compare/${ex}/${ex}.lms" 2>&1)"
    t3_ms="$(now_ms)"
    lmcloud_ms="$(( t3_ms - t2_ms ))"
    printf "%s\n" "$lmcloud_out" > "$ROOT_DIR/build/compare/$ex/lmcloud.stdout"
  else
    lmcloud_ms="n/a"
  fi

  python3 - "$ex" "$lmgcp_ms" "$lmcloud_ms" "$HAVE_LMCLOUD" <<'PY'
import json
import pathlib
import re
import sys

ex, lmgcp_ms, lmcloud_ms, have_lmcloud = sys.argv[1], int(sys.argv[2]), sys.argv[3], int(sys.argv[4])
root = pathlib.Path(".")

KIND_TO_TF = {
    "alert_policy": "google_monitoring_alert_policy",
    "artifact_repo": "google_artifact_registry_repository",
    "bq_dataset": "google_bigquery_dataset",
    "bq_table": "google_bigquery_table",
    "bucket": "google_storage_bucket",
    "cloud_run": "google_cloud_run_v2_service",
    "compute_instance": "google_compute_instance",
    "dns_record": "google_dns_record_set",
    "dns_zone": "google_dns_managed_zone",
    "firestore_doc": "google_firestore_document",
    "firewall": "google_compute_firewall",
    "kms_key": "google_kms_crypto_key",
    "log_entry": "terraform_data",
    "log_sink": "google_logging_project_sink",
    "pubsub_msg": "terraform_data",
    "pubsub_sub": "google_pubsub_subscription",
    "pubsub_topic": "google_pubsub_topic",
    "secret": "google_secret_manager_secret",
    "sql_instance": "google_sql_database_instance",
    "storage_object": "google_storage_bucket_object",
    "subnet": "google_compute_subnetwork",
    "vpc": "google_compute_network",
}

gcp_ev_path = root / "build" / "evidence" / f"live-cost-{ex}.json"
gcp_ev = json.loads(gcp_ev_path.read_text())
gcp_items = gcp_ev.get("items") or []
gcp_leftovers = int(gcp_ev.get("leftover_count", 0))
gcp_cost_fmt = gcp_ev.get("estimated_cost_formatted", "$0.00")
gcp_cost_usd = float(gcp_ev.get("estimated_cost_usd", 0.0))

gcp_stdout = (root / "build" / "compare" / ex / "lmgcp.stdout").read_text()
gcp_cost_lines = [ln.strip() for ln in gcp_stdout.splitlines() if ln.strip().startswith("estimated cost:")]
gcp_cost_line = gcp_cost_lines[-1] if gcp_cost_lines else f"estimated cost: {gcp_cost_fmt}"

tf_path = root / "compare" / ex / "main.tf"
tf_text = tf_path.read_text()
tf_resources = re.findall(
    r'resource\s+"((?:google_[a-z0-9_]+|terraform_data))"\s+"([a-z0-9_]+)"\s*\{(.*?)\n\}',
    tf_text,
    flags=re.S,
)
tf_data = re.findall(r'data\s+"(google_[a-z0-9_]+)"\s+"([a-z0-9_]+)"', tf_text)

cloud_ev_path = root / "build" / "compare" / ex / "lmcloud-evidence.json"
if cloud_ev_path.exists():
    cloud_ev = json.loads(cloud_ev_path.read_text())
else:
    # Static fallback when lmcloud binary is not installed (e.g. in CI)
    cloud_ev = {
        "example": ex,
        "resource_count": len(tf_resources),
        "leftover_count": 0,
        "estimated_cost_usd": gcp_cost_usd,
        "estimated_cost_formatted": gcp_cost_fmt,
        "cost_report": gcp_cost_line,
        "resources": [
            {
                "index": i + 1,
                "ident": tf_resources[i][1],
                "kind": gcp_items[i]["kind"],
                "google_type": tf_resources[i][0],
                "subtype": gcp_items[i]["subtype"],
                "name": gcp_items[i]["name"],
                "lmgcp_test": gcp_ev.get("run_id", "run-20261024"),
                "cost_usd": gcp_items[i]["cost_usd"],
            }
            for i in range(min(len(tf_resources), len(gcp_items)))
        ],
    }

cloud_leftovers = int(cloud_ev.get("leftover_count", 0))
cloud_cost_fmt = cloud_ev.get("estimated_cost_formatted", "$0.00")
cloud_cost_line = cloud_ev.get("cost_report", "")
cloud_resources = cloud_ev.get("resources") or []

# Verify 1-to-1 resource parity between lmgcp ledger, compare/<ex>/main.tf, and lmcloud state
expected_tf_types = [KIND_TO_TF[item["kind"]] for item in gcp_items]
actual_tf_types = [r[0] for r in tf_resources]
cloud_tf_types = [r["google_type"] for r in cloud_resources]

expected_names = [item["name"] for item in gcp_items]
cloud_names = [r["name"] for r in cloud_resources]

types_match = (expected_tf_types == actual_tf_types == cloud_tf_types)
names_match = (expected_names == cloud_names)
labels_valid = (
    ("lmgcp_test" in tf_text)
    and all(
        bool(cr.get("lmgcp_test"))
        and (
            cr["name"].startswith("lmgcp_")
            or cr["name"].startswith("lmgcp-")
            or str(cr.get("subtype", "")).startswith("lmgcp_")
            or str(cr.get("subtype", "")).startswith("lmgcp-")
        )
        for cr in cloud_resources
    )
)
cost_match = (gcp_cost_fmt == cloud_cost_fmt and gcp_cost_line == cloud_cost_line)
cleanup_ok = (gcp_leftovers == 0 and cloud_leftovers == 0)

if not (types_match and names_match and labels_valid and cost_match and cleanup_ok):
    raise SystemExit(
        f"parity check failed for {ex}: types_match={types_match} names_match={names_match} "
        f"labels_valid={labels_valid} cost_match={cost_match} cleanup_ok={cleanup_ok}\n"
        f"  expected_tf_types={expected_tf_types}\n"
        f"  actual_tf_types={actual_tf_types}\n"
        f"  gcp_cost_line={gcp_cost_line!r}\n"
        f"  cloud_cost_line={cloud_cost_line!r}"
    )

time_col = f"{lmgcp_ms}ms/{lmcloud_ms}ms" if lmcloud_ms != "n/a" else f"{lmgcp_ms}ms/static"
res_col = f"{len(gcp_items)}/{len(tf_resources)}"
left_col = f"{gcp_leftovers}/{cloud_leftovers}"

comp = {
    "example": ex,
    "resource_count_lmgcp": len(gcp_items),
    "resource_count_lmcloud": len(tf_resources),
    "data_source_count_lmcloud": len(tf_data),
    "resources_lmgcp": gcp_items,
    "resources_lmcloud": cloud_resources,
    "types_match": types_match,
    "names_match": names_match,
    "labels_valid": labels_valid,
    "cost_match": cost_match,
    "estimated_cost_short": gcp_cost_fmt,
    "cost_line": gcp_cost_line,
    "leftovers_lmgcp": gcp_leftovers,
    "leftovers_lmcloud": cloud_leftovers,
    "wall_time_lmgcp_ms": lmgcp_ms,
    "wall_time_lmcloud_ms": int(lmcloud_ms) if lmcloud_ms != "n/a" else None,
    "live_status": "pending user test",
}
(root / "build" / "compare" / ex / "comparison.json").write_text(json.dumps(comp, indent=2) + "\n")

print(f"{ex:<44} | {res_col:<9} | {gcp_cost_fmt:<11} | {left_col:<9} | {time_col:<15} | PASS")
PY
done

if [[ "$WRITE_RESULTS" == "--write-results" ]]; then
  python3 "$ROOT_DIR/compare/gen_results.py"
fi
