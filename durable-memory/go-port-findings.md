# Shared lmcloud schema in lmgcp (Phase A WIP)

Branch `shared-schema` cut from `go-lmcode-head` `4ba074b`.

## Phase A status (complete)

- Copied `lmcloud` schema byte-identically from lmcloud `9532d19`; pinned commit and
  SHA-256 in `lmcloud.lock` (`f1cd7471...`).
- Added `lmgcp cloud ...` subcommand using `internal/cloudhost` and the shared
  `invoke.Runner`/`membudget`; legacy `lmgcp ...` is unchanged.
- `--lmcode-schema --json` is byte-identical; `--lmcode-signatures` matches
  lmscript with and without std.
- Trivial real and dry `cloud.provider()` return `"gcp"`.
- Added `test-cloud.sh` and `scripts/sync-lmcloud-schema.sh`.

## Phase B/C status (not implemented)

- Object/queue Cloud methods are registered for signature parity but currently
  return `lmcloud phase A: <op> not implemented`; object-storage 12 and queue 13
  conformance scenarios do not pass yet.
- Next: GCS JSON API storage methods, then Pub/Sub topic+subscription queue
  semantics (expirationPolicy {}, returnImmediately=false pull, unconditional
  modifyAckDeadline, empty-body sentinel, intersection list/exists).

## Spec/sim findings to carry forward

- GCS object paths encode the whole key as one `%2F`-escaped segment for
  GET/PATCH/DELETE; multipart upload carries the raw key in JSON metadata.
- Queue is a topic+subscription pair; both must exist for `queue_exists` and
  `queue_list`; create subscription with `expirationPolicy: {}`.
- Pull must use `returnImmediately: false` under a `max(wait_seconds,1)`
  deadline and always `modifyAckDeadline` before returning messages.
- `set_endpoint` must accept `"storage"` (not just `"gcs"`) per the conformance
  scripts.
- `print(bool|float|int|string)` overloads are required in addition to
  `lmcode.RegisterTesting` for signature parity.
