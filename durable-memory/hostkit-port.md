# lmgcp shared schema on hostkit v2

Branch `hostkit-port` from `635a665`.

## Structure

- `cmd/lmgcp/cloud.go` is now a thin `cloudhost.Program{...}.Main` using
  `github.com/lmctl-ai/lmcloud/schema.Bytes` and `hostkit/invoke` hooks.
- `internal/gcp` is the provider adapter: `hostkit.ObjectStore` +
  `hostkit.Queue` over the GCS JSON API and Pub/Sub REST. It keeps only
  provider wire logic and error classification; validation, codec, retry,
  endpoint policy, pagination, abort formatting, dry overlay and registration
  come from hostkit.
- `internal/cloudhost` (the duplicated neutral implementation) is deleted.
- The legacy `lmgcp --lmcode` surface, `internal/host`, `internal/invoke`,
  journal and sweep are untouched.
- `go.mod` uses `replace github.com/lmctl-ai/lmcloud => ../lmcloud` because
  hostkit is not published yet. `lmcloud.lock` and the schema verify script
  remain.

## Verification

- `go vet ./...` PASS
- `go test -race ./...` PASS (includes `internal/gcp` classify table)
- Object storage conformance against cached released gcpsim: 12/12 pass,
  1 open-mode skip.
- Queue conformance against the fixed JVM gcpsim build: 13/13 pass,
  1 open-mode skip.
- Dry-run mutator spy in `test-cloud.sh`: PASS (closed loopback endpoint;
  bucket/object/queue mutators make no provider calls).

## SolReviewer follow-up fixes

- All GCP HTTP now goes through `hostkit.NewHTTPClient` + `RetryPolicy.DoHTTP`
  with the caller context; 503 retries three times, 429 is retried, non-idempotent
  POSTs are not retried after a possible send, and cross-origin redirects are not
  followed. Bodies use `ReadBoundedResponse` (16 MiB), so oversize responses fail
  without buffering past the limit.
- GCP-only neutral validation was removed (hostkit owns rules); aws-/amz-/azure-
  queue names, 255-segment keys, and long prefixes now behave like other hosts.
- Queue receive returns `message.messageId`, not the ackId.
- `LMGCP_LIVE=1` with no loopback endpoint returns UNAUTHENTICATED
  `LiveCredentialsNotImplemented`; non-loopback HTTPS does the same.
- Dead `internal/gcp/register.go` and unused `schema/lmcloud.go` were deleted.
- Transport regressions: 503 attempt count, 10 ms caller deadline, 16 MiB+1
  oversize read, cross-origin redirect not followed.

## Hostkit v3 adoption

- `go.mod` adds `replace github.com/lmctl-ai/lmcode-go => ../lmcode-go-patched`
  (remove when Registry.Enum is upstream and pins bump); hostkit v3 enum dry
  runs now work.
- GCP adapter implements `BucketCreateTokenReader`/`QueueCreateTokenReader`.
  Create requests store `lmcloud_create_token` as GCS bucket labels and Pub/Sub
  topic/subscription labels; readers return that label after a 409/conflict.
- `QueueCapsFor` reports LongPoll=false for the offline fake and true for the
  loopback simulator, so offline wait_seconds uses hostkit's WaitLoop.
- Dry-run spy now executes `cloud.last_error().code() == ErrorCode.OK` and
  asserts under `lmscript --dry-run`.
- Dead cloud-era code removed: `internal/gcp/register.go`, `schema/lmcloud.go`,
  the old `Op` dispatcher, non-wire queue send/receive, `ObjectListAll`,
  `BucketDeleteErr`, and `finishOK`.
- Full test-cloud passes against the fixed JVM gcpsim: object 12/12 (1 skip),
  queue 13/13 (1 skip), dry spy, go vet, go test -race -count=3.

## Token and enforce-mode completion

- Create-token conflict tests: pre-existing bucket/queue without our token
  returns false; a conflict whose stored token matches the per-call token
  returns true; topic and subscription labels are read back for queues.
- gcpsim enforce mode is wired into `test-cloud.sh`: a second JVM sim starts
  with an auth config containing `denied-token` and a deny-all statement for
  its principal; scenarios 12/13 run with
  `LMCLOUD_ENFORCE_DENIED_GCP_TOKEN=denied-token` and pass with no skip.
- The adapter sends `Authorization: Bearer $LMGCP_SIM_TOKEN` on loopback
  requests, per design 02 M3.

## SolReviewer final GCP fixes

- Bucket/queue listing adapted to one-page methods (`BucketListPage`,
  `QueueListPage`) with next tokens; hostkit `ListAll` now enforces repeated
  token/page/deadline limits. Bucket and queue stall tests fail after one page.
- Queue receive derives its wait context from the caller context, uses
  `returnImmediately=true` for wait=0, does not retry a pull after a transport
  failure, and rejects malformed base64 instead of fabricating an ID. Tests:
  10 ms parent deadline, accepted-pull-then-EOF makes exactly one pull.
- Bucket/queue create conflicts are returned to hostkit (not swallowed as
  false/OK), so v3 token read-back decides ownership. Tests cover matching-token
  conflict true and pre-existing/foreign conflict false for bucket and queue.
- `QueueCapsFor` reports offline fake `LongPoll=false` and simulator
  `LongPoll=true`.

## Hostkit gaps observed

- `hostkit` validates the receipt before the queue exists probe, so a
  malformed non-empty receipt on a missing queue needs the adapter to probe
  `LookupQueue` before `Ack`/`Nack` to preserve the M5 NOT_FOUND precedence.
  The GCP adapter does this.
- GCS JSON `HeadObject` has no body distinction between missing bucket and
  missing key; the adapter probes the bucket to attach the right
  `AbsentBucket`/`AbsentObject` qualifier.
- Pub/Sub `:acknowledge` returns success for a missing subscription upstream;
  the adapter probes the queue first so missing queue still yields NOT_FOUND.
