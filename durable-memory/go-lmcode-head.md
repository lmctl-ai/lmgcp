# Go host skeleton lessons (lmcode-go HEAD)

Branch `go-lmcode-head` migrated lmgcp from `65f0ef2275b0` (`v0.6.38-12`) to
`77932bf7134a` (`v0.6.38-22`) and is the reference for the shared Go host skeleton.

## Dependency and API cutover

- Bump with `go get github.com/lmctl-ai/lmcode-go@<HEAD sha>` and update `sdk.lock`
  to the same full SHA. `test.sh` asserts the 12-character prefixes match.
- `ReportOK`, `ReportError`, and `ReportAppFailure` no longer exist. Hosts now
  construct `lmcode.InvocationResultsFromCommand(files)` (or
  `InvocationResultsFromManifest(files, nil)` for standalone argument-order
  slots), call `Select`/`Complete` per slot, record failures with `Fail`, and emit
  exactly one terminal frame with `WriteFrame(os.Stdout)`.
- `FailureFrame(os.Stdout, phase, err)` is for errors before a collector exists
  (usage, host init, schema contract). Do not write a second frame afterwards.
- Exit codes are fixed: 0 success, 2 cli/usage, 3 load/run, 4 app or output
  failure. `Fail` does not exit; `WriteFrame` returns the status.
- The terminal format is `LMCODE_RETURN_BEGIN\n<json>\nLMCODE_RETURN_END\n`.
  Success is `{"ok":true,"result":...}`; multiple files produce a builder frame
  with `"result":[...],"parts":N`. Failure keys are compact (`"code":"RUN_HOST"`),
  not the old spaced JSON.
- `ResultContractFromSchema` returns the zero contract for schemas without
  `result`; pass it to `Select`. `Select` also binds the registry result sink.
- Dry mode comes from invocation metadata (`results.Dry()`), not an ambient
  variable. Set `ctx.SetDry(...)`; if the host has no `DrySlot` coverage, skip
  `Program.Run` entirely and still `Complete` the slot so the envelope carries
  `"dry":true` and a null result. This preserves the old lmgcp `--dry-run`
  no-write behavior without claiming computed previews.
- Watchdog goroutines cannot safely call the collector. Emit
  `FailureFrame(os.Stdout,"run",err)` and exit from the watchdog only, or the
  main path may double-frame.

## lmscript example migration

- Removed forms: `_dsl_`, `savedsl`, `dsl_cli`. Use
  `s = load_schema(cli="lmgcp")` (or `schema="file"`), then
  `p = _lmcode_ s;` ... `end _lmcode_`, then `r = run_lmcode(p)`.
- `load_schema` clauses are space-separated, not comma-separated:
  `load_schema(cli="lmdraw" install="github:lmctl-ai/lmdraw@latest")`.
- `run_lmcode` returns only the parsed envelope (`ok`, `result`, `code`,
  `message`, `dry`, `parts`). Host stdout is forwarded to lmscript stdout; there
  is no `r.stdout`, so remove `split(r.stdout, ...)` uses and assert `r.ok`.
- Helper names differ from older examples: use `contains(...)`, not
  `str_contains(...)`.
- The compiler capability probe cannot save bytecode anymore. `select-compiler.sh`
  now validates `--lmcode-signatures` for the v3 std marker
  `fn template_render(string, Json) -> string` plus `--lmcode-schema --json`.
- `compare/run.sh` must use `LMSCRIPT_BIN` (`LMSCRIPT="${LMSCRIPT_BIN:-lmscript}"`);
  a test-only PATH entry for `bin/lmgcp` does not expose lmscript.

## Known ecosystem blocker

- `lmdraw` and `lmplot` binaries still speak the retired `--dsl`/`--schema`
  protocol and cannot read the new v6 invocation envelope. Their scenes were
  removed from `examples/*.lms` (the lmgcp portions and cost evidence remain);
  they should return once those hosts are ported. `docs/images/*.png` remain the
  last committed renderings.

## Verification gate

`go vet ./...`, `go test ./...`, and
`LMSCRIPT_BIN=/home/ubuntu/lmctl-mono/lmscript/bin/lmscript ./test.sh`.
The suite now checks the envelope in the corrupt-bytecode and memory-budget
cases and still runs all 41 examples plus the `lmcloud` comparison script.
