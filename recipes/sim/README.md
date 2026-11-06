# lmgcp recipes against gcpsim

Runnable lmscript recipes that drive `lmgcp` against the local
[gcpsim](https://github.com/lmctl-ai/gcpsim) emulator on `127.0.0.1`, and against
lmgcp's own in-process fake GCP server for the services gcpsim does not implement.
Nothing here contacts Google Cloud: `LMGCP_LIVE` stays unset, `LMGCP_ENDPOINT` must
be a loopback origin, and no Application Default Credentials are read. Every recipe
creates `lmgcp-`prefixed, ledger-owned resources, asserts what the emulator did,
writes a JSON report under `data/sim/<recipe>/` and ends with `PASS <recipe>`.

Two host surfaces are used, both served by the one `lmgcp` binary:

- **lmgcp** (`cli="lmgcp"`): the Google Cloud Go SDK host with its run ledger,
  cost cap, sandbox shape allowlist and `--sweep` recovery.
- **lmgcp cloud**: the shared [lmcloud](https://github.com/lmctl-ai/lmcloud) schema
  (portable bucket, object and queue operations with `last_error()` classification).

## Which emulator serves what

gcpsim 0.2.x (floci-gcp 0.11) implements Cloud Storage, Pub/Sub, Cloud Run v2 and
bearer-token authentication/permission policies in process. The recipes for those
services default to `--endpoint http://127.0.0.1:4588`. Compute Engine, VPC,
Cloud SQL, Secret Manager, KMS, BigQuery, Cloud DNS, Logging sinks, Monitoring and
IAM are not persisted by gcpsim (requests either 404 or fall through to the storage
router), so those recipes default to `--endpoint fake`, which boots lmgcp's built-in
fake GCP REST server. Pass a gcpsim origin to them once gcpsim serves the service.

## Start the emulator

```bash
gcpsim serve                                               # open mode, :4588
lmscript recipes/sim/auth_enforced_denial.lms --prepare    # writes data/sim/auth_enforced_denial/gcpsim-auth.json
gcpsim serve --port 4589 --auth enforce --auth-config data/sim/auth_enforced_denial/gcpsim-auth.json
```

## Run

```bash
export PATH="$PWD/bin:$PATH"            # go build -o bin/lmgcp ./cmd/lmgcp, or the release binary
lmscript recipes/sim/static_site_bucket.lms
lmscript recipes/sim/cloud_run_web_service.lms
lmscript recipes/sim/compute_web_vm.lms   # in-process fake by default
```

Common flags: `--endpoint URL|fake`, `--host PATH` (the schema is read from the
`lmgcp` on PATH), `--project ID` (default `gcpsim-local`), `--token TOKEN` (dummy
bearer for an enforce-mode emulator), `--run ID`, `--out DIR`, `--help`.

`python3 scripts/test-sim-recipes.py` starts gcpsim in open and enforce mode on free
ports, runs every recipe in a temporary directory (fake-backed ones with
`--endpoint fake`) and tears everything down. It takes `--gcpsim`, `--lmgcp`,
`--lmscript` (or `GCPSIM_BIN`, `LMGCP_BIN`, `LMSCRIPT_BIN`) and `--only NAME`.

## Catalog

| Recipe | Host | Target | What it proves |
| --- | --- | --- | --- |
| `static_site_bucket.lms` | lmgcp cloud | gcpsim GCS | Publish a five-file static site with content types and cache metadata, list by prefix, republish with a new ETag, tear down |
| `queue_worker.lms` | lmgcp cloud | gcpsim Pub/Sub | Topic+subscription as one portable queue: enqueue site jobs, drain with ack, nack one for redelivery, end empty |
| `auth_enforced_denial.lms` | lmgcp cloud | gcpsim `--auth enforce` | Author the token/policy file with `_json_` (admin plus restricted deployer); the deployer's own bucket and queue work, objects and deletes on a protected bucket and publishes to a protected topic fail `PERMISSION_DENIED`/403, an unknown token fails `UNAUTHENTICATED`/401; SKIPs on an open-mode emulator |
| `gcs_website_hosting.lms` | lmgcp | gcpsim GCS | Labelled bucket with uniform access, versioning and lifecycle rules, HTML/CSS/JS objects, listing, reads, ledger cleanup, cost report |
| `pubsub_job_pipeline.lms` | lmgcp | gcpsim Pub/Sub | Labelled topic, worker subscription with a dead-letter topic, attribute-tagged publishes, pulls, listings |
| `cloud_run_web_service.lms` | lmgcp | gcpsim Cloud Run v2 | Deploy a container service with cpu/memory/instance caps, read its URL and revision, 90/10 traffic split, labels, list, delete |
| `leftover_sweep.lms` | lmgcp + `lmgcp --sweep` | gcpsim GCS | A host exits without cleanup; the durable recovery journal lets `--sweep --run-id` list, delete and idempotently re-run the cleanup |
| `compute_web_vm.lms` | lmgcp | in-process fake | VPC, subnet, firewall admitting 80/443 from one /32, e2-micro instance, labels, stop/start, disks, reverse-order teardown |
| `sql_secrets_kms.lms` | lmgcp | in-process fake | PostgreSQL Cloud SQL on a sandbox tier, rotating secret metadata, KMS ring and rotating key |
| `bigquery_analytics.lms` | lmgcp | in-process fake | Dataset with a time-partitioned page-view table and an orders table, schema read-back, table drop |
| `dns_and_observability.lms` | lmgcp | in-process fake | Public DNSSEC zone with A/CNAME/TXT records, error log sink to a bucket, structured log entries, latency alert policy |
| `iam_posture_review.lms` | lmgcp | in-process fake | Read-only: service accounts with key ages, role bindings, label compliance, resource graph edges |
| `smoke_test.lms` | lmgcp cloud | gcpsim GCS, Pub/Sub | CI smoke test: emulator answers, bucket/object/queue/message round trips, timed |
| `error_handling.lms` | lmgcp cloud | gcpsim GCS, Pub/Sub | Error contract: idempotent creates and deletes, false-not-fail probes, NOT_FOUND/CONFLICT with HTTP status, pre-flight INVALID_ARGUMENT guards |
| `object_listing_pagination.lms` | lmgcp cloud | gcpsim GCS | 120 objects, continuation-token paging in pages of 50, ascending order, prefix counts, exhaustive list |
| `binary_object_roundtrip.lms` | lmgcp cloud | gcpsim GCS | PNG header, all 256 byte values, empty object, UTF-8 text, 64 KiB file byte-exact with content types and ETags |
| `queue_visibility_timeout.lms` | lmgcp cloud | gcpsim Pub/Sub | In-flight messages invisible; redelivery after the ack deadline is forced with nack(0) because gcpsim does not expire deadlines; drained |
| `upload_thumbnail_pipeline.lms` | lmgcp cloud | gcpsim GCS, Pub/Sub | Uploads enqueue jobs, a worker writes thumbnails with provenance metadata, every upload gets exactly one |
| `blue_green_release.lms` | lmgcp cloud | gcpsim GCS | Two release trees, a current.json pointer, instant switch, verified rollback, prune of the retired release |
| `concurrent_uploaders.lms` | lmgcp cloud | gcpsim GCS | Four host processes upload disjoint prefixes at once (parallel_map), one listing proves nothing lost |
| `persistence_restart.lms` | lmgcp cloud | gcpsim GCS, Pub/Sub | Write phase, emulator restart with --data-dir, verify phase: objects intact, queue present (runner shows it as persistence_restart*) |
| `pubsub_fanout.lms` | lmgcp | gcpsim Pub/Sub | One topic, three subscriptions, every event reaches every subscriber exactly once |
| `cost_cap_guardrail.lms` | lmgcp | in-process fake | A tiny cap refuses a VM before any request, caps above $0.25 are rejected, the raised cap lets the same VM through |
| `artifact_registry_gke.lms` | lmgcp | in-process fake | Immutable-tag Docker repository plus a GKE hardening review (private nodes, workload identity, binary authorization) |

## Emulator findings

Recorded while verifying against gcpsim v0.2.23 (floci-gcp 0.11.0):

- Bucket and topic creation are authorised against the project (`storage.buckets.create` on `projects/PROJECT`), so a policy cannot deny creation by name; the permissions recipe denies object writes, bucket deletion and publishing on protected names instead.
- Cloud Run v2 is real on gcpsim: create returns a completed operation with a
  service URL and revision, traffic and labels update, delete works.
- Object metadata from gcpsim omits the size, so `storage_object_size` reports 0
  for objects written through the SDK; `gcs_website_hosting.lms` prints a note.
- Compute, Cloud SQL, Secret Manager, KMS, BigQuery tables, DNS, logging sinks and
  monitoring either 404 or are swallowed by gcpsim's storage router (`uploadId is
  required`), so their recipes target the in-process fake.
- `gcp.set_auto_cleanup(false)` marks the ledger as retention-only and the sweep then
  retains everything; with an external endpoint auto-cleanup is already off, so the
  sweep recipe simply exits without `cleanup_ledger()`.
- `--sweep` verifies ownership labels only for buckets; other kinds are retained with
  "ownership check unsupported for resource kind".
- Pub/Sub on gcpsim never redelivers a pulled message when its ack deadline expires; only `modifyAckDeadline(0)` (lmcloud `queue_nack(..., 0)`) hands it back. `queue_visibility_timeout.lms` records whether redelivery was natural or forced.
