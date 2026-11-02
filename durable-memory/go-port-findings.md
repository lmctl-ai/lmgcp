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

## Phase B status (complete)

- Object storage implemented with the GCS JSON API: bucket CRUD/list, multipart
  object upload, alt=media download, metadata head/list with pagination,
  idempotent delete, validation, error mapping.
- lmcloud conformance: 12/12 pass in open mode; scenario 12 skips because the
  shared gcpsim instance is in `--auth open` mode.

## Phase C status (WIP — sim gap)

- Queue implemented as Pub/Sub topic+subscription pairs, including
  `expirationPolicy: {}`, topic+subscription intersection for exists/list,
  `returnImmediately:false` pull under `max(wait,1)` deadline, empty-body
  sentinel, and acknowledge.
- Current conformance: 5/13 pass (02, 08, 10, 11, 13-skip); scenarios 01, 03,
  04, 05, 06, 07, 09, 12 fail.
- Simulator gap: gcpsim REST `POST /v1/projects/{p}/subscriptions/{s}:modifyAckDeadline`
  returns HTTP 404 even for a valid in-flight ackId (while `:acknowledge` and
  `:pull` work; the native binary contains gRPC ModifyAckDeadline). This blocks
  visibility/nack/receive-with-messages scenarios. Recorded, not edited in
  gcpsim. Workaround options: use the pubsub gRPC client for queue receive/nack,
  or fix gcpsim REST.
- Also pending: queue 01 exists/list ordering once modify is fixed; queue 12
  payload-size guard depends on receive.

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
