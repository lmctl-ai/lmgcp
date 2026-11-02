# Go host skeleton lessons (lmcode-go HEAD)

This branch migrated lmgcp from `65f0ef2275b0` (`v0.6.38-12`) to
`77932bf7134a` (`v0.6.38-22`). The reusable pieces are the shared run loop in
`internal/invoke` and the dry-slot pattern in `internal/host/register.go`; copy
those, not the whole provider host.

## Dependency and API cutover

- Bump with `go get github.com/lmctl-ai/lmcode-go@<HEAD sha>` and update `sdk.lock`
  to the same full SHA. `test.sh` asserts the 12-character prefixes match.
- `ReportOK`, `ReportError`, and `ReportAppFailure` no longer exist. Construct
  `lmcode.InvocationResultsFromCommand(files)` only in the `--lmcode` branch
  (never for schema/signature/sweep discovery), call `Select`/`Complete` per
  slot, record failures with `Fail`, and emit exactly one frame with `WriteFrame`.
- `FailureFrame(os.Stdout, phase, err)` is for errors before a collector exists
  (usage, host init, schema contract, membudget init). Do not frame twice.
- Exit codes: 0 success, 2 cli/usage, 3 load/run, 4 app or output failure.
  `Fail` does not exit; `WriteFrame` returns the status.
- Terminal format: `LMCODE_RETURN_BEGIN\n<json>\nLMCODE_RETURN_END\n`. Success is
  `{"ok":true,"result":...}`; multiple files add `"result":[...],"parts":N`.
  Failure keys are compact (`"code":"RUN_HOST"`), not the old spaced JSON.
- `ResultContractFromSchema` returns the zero contract for schemas without
  `result`; pass it to `Select`. `Select` also binds the registry result sink.
- The run loop lives in `internal/invoke.Runner`: read/load/select, optional
  `BeforeRun` (memory reset), optional watchdog, `Program.Run` in **both** real
  and dry mode, auth check with cleanup, failure cleanup/hooks, `Complete`, and
  one `WriteFrame`. Provider hosts supply `Hooks` (cleanup, auth error, watchdog,
  run-error diagnostics) so lmoci/lmazure/lmaws can copy the same control flow.
- Dry mode comes from invocation metadata (`results.Dry()`), not a CLI flag.
  Pass it to `host.NewHost(dry)` so live/ADC setup is skipped; set
  `ctx.SetDry(dry)`; still call `Program.Run`. The SDK substitutes a dummy value
  for any callback without a `DrySlot`, so mutators naturally make zero writes.
  Register **every** read-only snapshot method with `lmcode.Dry(realFn)`: the
  Gcp list/get/report methods and all object accessors/predicates (`name`,
  `vcpus`, `labels`, `risk_score`, ...). The fake transport read is side-effect
  free, and without Dry the SDK returns empty/zero/true dummies. Mutators that
  return objects get `mPreview` callbacks returning a concrete zero object.
  Local-only configuration setters (`set_project/region/zone/run_id`) execute
  under `lmcode.Dry(fn)` so chaining (`set_region(...).region()`) sees the new
  value; `set_auto_cleanup` uses a same-receiver preview. Consuming operations
  such as `pubsub_pull` stay previews. Never skip `Program.Run` in dry mode.
- Watchdog goroutines cannot safely call the collector or provider cleanup
  while `Program.Run` is active. `invoke` emits the collector wire shape with
  the dry/part metadata, never calls `Cleanup`, and exits; normal auth failures
  still call `CleanupLedger`. `Host.fail/clearErr/LastError/AuthError` are
  mutex-guarded so even the old cleanup-vs-read overlap is race-free.
- CLI `--lmcode` accepts repeated selectors exactly as lmscript's default
  builder emits them (`--lmcode f0 --lmcode f1`) and the sample's
  `--first/--last FILE` pairs. Slots keep add order; execution order is stable
  by priority. `InvocationResultsFromCommand` still runs only in this branch.
- Shell flake: never pipe a large captured string into `grep -q` under
  `set -o pipefail` (grep's early exit can SIGPIPE the producer). Use
  `grep -Fq PATTERN <<<"$var"` in test.sh/select-compiler.sh.

## lmscript example migration

- Removed forms: `_dsl_`, `savedsl`, `dsl_cli`. Use
  `s = load_schema(cli="lmgcp" install="github:lmctl-ai/lmgcp@latest")`, then
  `p = _lmcode_ s;` ... `end _lmcode_`, then `r = run_lmcode(p)`.
- `load_schema` clauses are space-separated, not comma-separated:
  `load_schema(cli="lmdraw" install="github:lmctl-ai/lmdraw@latest")`.
- `run_lmcode` returns only the parsed envelope (`ok`, `result`, `code`,
  `message`, `dry`, `parts`). Host stdout is forwarded to lmscript stdout; there
  is no `r.stdout`, so remove `split(r.stdout, ...)` uses and assert `r.ok`.
- Helper names differ from older examples: use `contains(...)`, not
  `str_contains(...)`; inside `_lmcode_`, use `str_contains`/`str_starts_with`
  and `assert_close`, not `assert`.
- The compiler capability probe cannot save bytecode anymore.
  `scripts/select-compiler.sh` validates `--lmcode-signatures` for
  `fn template_render(string, Json) -> string`, `--lmcode-schema --json`, and
  `--check` on a load_schema/`_lmcode_`/run_lmcode probe.
- `compare/run.sh` must use `LMSCRIPT_BIN` (`LMSCRIPT="${LMSCRIPT_BIN:-lmscript}"`);
  a test-only PATH entry for `bin/lmgcp` does not expose lmscript.
- `LMSCRIPT_BIN` must be `@lmctl-ai/lmctl@0.2.453` or newer; 0.2.420 predates
  `load_schema` and 0.2.451 fails the dry envelope with `dry must be boolean`.

## Images and gallery

- `examples/*.lms` stay pure lmgcp and no longer contain draw/plot blocks.
  Restored scenes live in `gallery/NN_<name>.draw.lms`, unchanged inside, with
  the new `load_schema(cli="lmdraw" install=...)` wrapper and optional
  `ARGV[0]` input path.
- `scripts/render-gallery.sh` probes `lmdraw`/`lmplot` for
  `--lmcode-schema --json` plus a valid dry `run_lmcode` envelope. Until both
  pass, it prints `SKIP lmdraw not envelope-capable` and `test.sh` prints
  `ok N # SKIP ...`. Do not force the gallery: the released hosts are still on
  the retired protocol. `docs/images/*.png` remain the last renderings.

## Verification gate

`go vet ./...`, `go test ./...`, and `./test.sh` with both the local mono
`lmscript` and the published `@lmctl-ai/lmctl@0.2.453`. The suite covers the
envelope, dry reads/dummy mutators/zero ledger writes, all 41 examples, the
gallery skip probe, and the `lmcloud` comparison script.
