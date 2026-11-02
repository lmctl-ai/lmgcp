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
