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
  dry/part metadata (including `part:0` for a one-slot builder), never calls
  `Cleanup`, and exits; a stdout write failure exits 4. The hard-abort message
  includes the durable recovery run ID/path. Normal auth failures still call
  `CleanupLedger`. `Host.fail/clearErr/LastError/AuthError` are mutex-guarded.
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

## Durable recovery contract (required before LIVE)

- Allocate a collision-resistant run ID matching
  `^run-[0-9]+-[0-9a-f]{12}$` before any mutation; never reuse a demo ID.
  Validate the run ID and bind the journal to provider/account/tenant/scope/
  endpoint on every record at load; reject mismatches and fail closed.
- Journal directories are leaf directories the host created with 0700; refuse
  existing group/world-accessible or foreign-owned dirs. Journal files are
  regular files opened with O_NOFOLLOW, owned by the user, 0600. Never
  MkdirAll/chmod an arbitrary parent. The no-config fallback is per-user 0700.
- Journal **intent before the create request**, then fsync `confirmed` with the
  concrete location/provider ID/real creation timestamp. A journal write failure
  poisons further mutations. Journal files are append-only JSONL, 0600 in a 0700
  directory (`LMGCP_STATE_DIR`, `LMGCP_LEDGER`).
- `--sweep --run-id ID` or `--sweep --ledger PATH` loads ownership, includes
  pending intents, computes real age from persisted timestamps (fake age only
  for the seeded fake epoch), and deletes in reverse journal order.
- Definite create failures (any 4xx, including 409) write a `failed` record and
  drop the pending intent; only unknown outcomes (timeout/5xx/transport) stay
  pending. `set_run_id` is refused once the journal is attached.
- Before ANY sweep delete, GET the remote resource and verify ownership:
  `lmgcp_test == journal run ID`, and (live) created-at not before the intent.
  Kinds without a verifier are retained. Never trust a prefix alone.
- Tombstone only after confirmed deletion or typed `googleapi.Error` 404 with a
  resource-specific reason (`notFound`). Never classify absence from message
  text; wrong endpoint/proxy/permission errors are retained/uncertain. Sweep
  reports deleted and absent separately.
- Torn tail: append records the prior offset and truncates back on write
  failure; load ignores an invalid final line without a trailing newline but
  fails on any other invalid line.
- The active run holds an exclusive flock; a second writer or sweep gets a clear
  error. `--force-stale` is an operator escape hatch, not normal operation.
  Buckets are emptied (objects and versions) before delete; KMS CryptoKeys are
  retained in live mode; bq_table and firestore_doc have cleanup paths.
  `set_auto_cleanup(false)` persists retention.
- Dry invocations create no live journal. A hard abort skips provider cleanup
  but leaves the fsynced journal and prints its run ID/path; a separate sweep
  process reclaims it. Tests cover intent-without-confirm, abort/recovery,
  unowned/failed-create safety, crafted records, torn tails, locking, typed
  absence, real kill -9 recovery through gcpsim, unrelated same-prefix safety,
  and an idempotent second sweep.

## Per-host edit checklist (copying the skeleton)

1. Module/cmd/import paths, embedded VERSION/notices, provider description,
   SDK pins; keep selector parsing, result order, dry/preview policy, and frame
   tests unchanged.
2. Exact provider schema bytes and result contract; rename the context binding;
   keep `--lmcode-schema --json` byte-identical.
3. Replace `internal/host/register.go` domain types, `zeroHostObject` cases, and
   explicit `mSnapshot`/`mRead`/`mPreview`/`mLocal`/`mSelf` purity declarations;
   never infer purity from the receiver type.
4. Replace `internal/host/host.go` clients/auth/transport and `LM<HOST>_LIVE`,
   endpoint, memory, state-dir and ledger variables. Preserve dry-before-auth,
   loopback-only endpoint guards, and fake-by-default.
5. Supply provider cleanup/auth hooks to `internal/invoke`; fix the recovery
   journal metadata, every resource kind delete/retain decision, and the
   provider-specific retention/ownership rules in `internal/safety`.
6. Rename `membudget.EnvVarName`, `LMGCP_STATE_DIR` and `LMGCP_LEDGER` per
   provider. Keep the supervisor/recovery outside the worker memory budget.
   Do not copy GCP ADC, project masking, `lmgcp_` prefixes, the fixed demo
   epoch, or fake-only age logic.
7. Port ownership verification per SDK: OCI tags + timeCreated; Azure tags;
   AWS tags (S3 bucket tags). Typed not-found must be provider-specific; OCI
   `NotAuthorizedOrNotFound` is ambiguous and must be retained. Poll async
   delete operations before tombstoning. Empty sub-resources (S3 objects and
   versions, Azure blobs, OCI objects) and delete non-default record sets. The
   journal lock, run-ID validation, truncate-on-failure and tolerant-tail rules
   are shared; do not reimplement them per host.

## LIVE gate

Do not enable LIVE in another host until it has: durable journal with
intent-before-create, cross-process `--sweep` recovery, real timestamps,
retain/tombstone semantics, every kind covered, one-slot builder abort metadata,
and an abort/recovery acceptance test against a persistent loopback emulator.
Until the SDK exposes cooperative VM/request cancellation, the watchdog may not
wait for `Program.Run`; recovery is via the durable journal and explicit sweep.

## Verification gate

`go vet ./...`, `go test -race -count=3 ./...`, and `./test.sh` with both the
local mono `lmscript` and the published `@lmctl-ai/lmctl@0.2.453` (3 runs each).
The 65-check suite covers the envelope, dry reads/dummy mutators/zero ledger
writes, repeated/selector builders, one-slot builder abort metadata, dry and
real memory aborts, durable journal recovery through a separate `--sweep`
process, optional gcpsim kill -9 recovery, all 41 examples, the gallery skip probe, and the `lmcloud` comparison.
