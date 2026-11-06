#!/usr/bin/env python3
"""Run every recipes/sim/*.lms recipe against freshly started gcpsim emulators.

Starts gcpsim twice on free loopback ports (open mode, and --auth enforce with the token/policy
file that auth_enforced_denial.lms authors), runs each recipe in its own temporary working
directory and asserts the PASS line. Recipes whose services gcpsim does not serve (compute,
Cloud SQL, Secret Manager, KMS, BigQuery, DNS, logging, monitoring, IAM) run against lmgcp's
in-process fake GCP server, which is their documented default.

Binaries: --gcpsim/GCPSIM_BIN (else PATH), --lmgcp/LMGCP_BIN (else PATH; built from ./cmd/lmgcp
into build/sim/ when absent), --lmscript/LMSCRIPT_BIN (else PATH). No cloud credentials are
read and no network leaves the loopback interface.
"""
from __future__ import annotations

import argparse
import json
import os
import shutil
import socket
import subprocess
import sys
import tempfile
import time
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
RECIPES = ROOT / "recipes" / "sim"
FAKE_RECIPES = {"compute_web_vm", "sql_secrets_kms", "bigquery_analytics", "dns_and_observability", "iam_posture_review", "cost_cap_guardrail", "artifact_registry_gke"}
ENFORCE_RECIPES = {"auth_enforced_denial"}


def free_port() -> int:
    with socket.socket() as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


def which(explicit: str | None, env: str, name: str) -> str | None:
    for candidate in (explicit, os.environ.get(env)):
        if candidate:
            return str(Path(candidate).resolve())
    return shutil.which(name)


def start_sim(binary: str, port: int, log: Path, extra: list[str], env: dict[str, str]) -> subprocess.Popen:
    proc = subprocess.Popen([binary, "serve", "--port", str(port), *extra], stdout=log.open("w"), stderr=subprocess.STDOUT, env=env)
    url = f"http://127.0.0.1:{port}/health"
    for _ in range(600):
        if proc.poll() is not None:
            raise SystemExit(f"gcpsim on port {port} exited early; see {log}")
        try:
            with urllib.request.urlopen(url, timeout=1) as resp:
                if resp.status == 200:
                    return proc
        except Exception:
            time.sleep(0.2)
    raise SystemExit(f"gcpsim on port {port} did not become healthy; see {log}")


def build_host() -> str:
    out = ROOT / "build" / "sim" / "lmgcp"
    out.parent.mkdir(parents=True, exist_ok=True)
    env = dict(os.environ, CGO_ENABLED="0", GOTOOLCHAIN=os.environ.get("GOTOOLCHAIN", "auto"))
    subprocess.run(["go", "build", "-trimpath", "-o", str(out), "./cmd/lmgcp"], cwd=ROOT, check=True, env=env)
    return str(out)


def run_persistence(sim: str, lmscript: str, hostargs: list[str], env: dict[str, str], work: Path, timeout: int) -> bool:
    """persistence_restart.lms across a real emulator restart: write with --data-dir, stop, start again, verify."""
    data_dir = work / "persist-data"
    data_dir.mkdir()
    out_dir = work / "persist-out"
    out_dir.mkdir()
    cwd = work / "persistence_restart_restart"
    cwd.mkdir()
    port = free_port()
    recipe = str(RECIPES / "persistence_restart.lms")

    def phase(name: str) -> subprocess.CompletedProcess[str]:
        cmd = [lmscript, recipe, "--phase", name, "--endpoint", f"http://127.0.0.1:{port}", "--out", str(out_dir), "--run", "restart1", *hostargs]
        return subprocess.run(cmd, cwd=cwd, env=env, text=True, capture_output=True, timeout=timeout)

    proc = start_sim(sim, port, work / "gcpsim-persist-1.log", ["--data-dir", str(data_dir)], env)
    try:
        wrote = phase("write")
    finally:
        proc.terminate()
        proc.wait(timeout=30)
    if wrote.returncode != 0:
        (cwd / "stdout.txt").write_text(wrote.stdout + wrote.stderr)
        return False
    proc = start_sim(sim, port, work / "gcpsim-persist-2.log", ["--data-dir", str(data_dir)], env)
    try:
        verified = phase("verify")
    finally:
        proc.terminate()
        proc.wait(timeout=30)
    ok = verified.returncode == 0 and "PASS persistence_restart" in verified.stdout
    if not ok:
        (cwd / "stdout.txt").write_text(verified.stdout + verified.stderr)
    return ok


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--gcpsim")
    ap.add_argument("--lmgcp")
    ap.add_argument("--lmscript")
    ap.add_argument("--only", action="append", default=[], help="recipe stem to run (repeatable)")
    ap.add_argument("--timeout", type=int, default=420)
    args = ap.parse_args()

    gcpsim = which(args.gcpsim, "GCPSIM_BIN", "gcpsim")
    lmscript = which(args.lmscript, "LMSCRIPT_BIN", "lmscript")
    lmgcp = which(args.lmgcp, "LMGCP_BIN", "lmgcp") or build_host()
    missing = [n for n, v in (("gcpsim", gcpsim), ("lmscript", lmscript)) if not v]
    if missing:
        print("missing executables: " + ", ".join(missing) + " (pass --<name> or set <NAME>_BIN; gcpsim releases: https://github.com/lmctl-ai/gcpsim/releases)", file=sys.stderr)
        return 2

    recipes = sorted(p for p in RECIPES.glob("*.lms") if not args.only or p.stem in args.only)
    if not recipes:
        print("no recipes selected", file=sys.stderr)
        return 2

    work = Path(tempfile.mkdtemp(prefix="lmgcp-sim-recipes-"))
    procs: list[subprocess.Popen] = []
    base_env = {k: v for k, v in os.environ.items() if not k.startswith(("LMGCP_", "LMCLOUD_", "GOOGLE_", "CLOUDSDK_"))}
    base_env["PATH"] = str(Path(lmgcp).parent) + os.pathsep + base_env.get("PATH", "")
    failures: list[str] = []
    try:
        open_port, enforce_port = free_port(), free_port()
        procs.append(start_sim(gcpsim, open_port, work / "gcpsim-open.log", [], base_env))
        print(f"gcpsim open mode on {open_port}")
        prep = work / "prepare"
        prep.mkdir()
        subprocess.run([lmscript, str(RECIPES / "auth_enforced_denial.lms"), "--prepare", "--out", str(prep)], cwd=prep, env=base_env, check=True, capture_output=True, text=True)
        auth_config = prep / "gcpsim-auth.json"
        procs.append(start_sim(gcpsim, enforce_port, work / "gcpsim-enforce.log", ["--auth", "enforce", "--auth-config", str(auth_config)], base_env))
        print(f"gcpsim enforce mode on {enforce_port} with {auth_config.name}")

        for recipe in recipes:
            endpoint = "fake" if recipe.stem in FAKE_RECIPES else f"http://127.0.0.1:{enforce_port if recipe.stem in ENFORCE_RECIPES else open_port}"
            cwd = work / recipe.stem
            cwd.mkdir()
            cmd = [lmscript, str(recipe), "--endpoint", endpoint, "--host", lmgcp]
            started = time.monotonic()
            result = None
            try:
                result = subprocess.run(cmd, cwd=cwd, env=base_env, text=True, capture_output=True, timeout=args.timeout)
                ok = result.returncode == 0 and f"PASS {recipe.stem}" in result.stdout
                detail = "" if ok else ((result.stdout + result.stderr).strip().splitlines() or ["(no output)"])[-1]
            except subprocess.TimeoutExpired:
                ok, detail = False, "timeout"
            print(f"{'ok' if ok else 'FAIL':4} {recipe.stem:26} {time.monotonic() - started:6.1f}s" + ("" if ok else f"  {detail[:200]}"))
            if not ok:
                failures.append(recipe.stem)
                (cwd / "stdout.txt").write_text(result.stdout if result else "")

        if not args.only or "persistence_restart" in args.only:
            started = time.monotonic()
            ok = run_persistence(gcpsim, lmscript, ["--host", lmgcp], base_env, work, args.timeout)
            print(f"{'ok' if ok else 'FAIL':4} {'persistence_restart*':26} {time.monotonic() - started:6.1f}s" + ("" if ok else "  (write, emulator restart with --data-dir, verify)"))
            if not ok:
                failures.append("persistence_restart(restart)")
    finally:
        for proc in procs:
            proc.terminate()
        for proc in procs:
            try:
                proc.wait(timeout=20)
            except subprocess.TimeoutExpired:
                proc.kill()
    print(json.dumps({"recipes": len(recipes), "failed": failures, "work": str(work)}))
    if failures:
        print(f"FAIL {len(failures)}/{len(recipes)} recipes; logs under {work}", file=sys.stderr)
        return 1
    shutil.rmtree(work, ignore_errors=True)
    print(f"PASS {len(recipes)} sim recipes")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
