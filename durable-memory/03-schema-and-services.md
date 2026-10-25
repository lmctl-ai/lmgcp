# 03 — Schema & Services (370 Signatures)

`schema/lmgcp.dsl.json` defines `dsl_version: "1"`, context `gcp: Gcp`, 19 opaque types (`Gcp`, `ComputeInstance`, `VpcNetwork`, `Subnet`, `FirewallRule`, `StorageBucket`, `PubsubTopic`, `PubsubSubscription`, `CloudRunService`, `SqlInstance`, `BqDataset`, `BqTable`, `ServiceAccount`, `SecretMetadata`, `KmsKey`, `GkeCluster`, `ArtifactRepo`, `DnsZone`, `LogSink`, `AlertPolicy`), plus the `lmcode_std_v1` collection/string/math/file standard library (`StrList`, `IntList`, `FloatList`, `StrMap`) and `assert_close` / `print`.

## Service Breakdown

1. **Core, Safety & Cost (`Gcp`)**: `project`, `masked_project`, `set_project`, `region`, `set_region`, `zone`, `set_zone`, `is_live`, `auth_mode`, `endpoint`, `run_id`, `set_run_id`, `last_error`, `redact`, `set_auto_cleanup`, `ledger_count`, `ledger_entries`, `ledger_owns`, `cost_cap_usd`, `set_cost_cap_usd`, `estimated_cost_usd`, `cost_report`, `write_cost_evidence`, `cleanup_ledger`, `seed_demo_age`, `sweep_dry_run`, `sweep_delete`, `alloc_scratch_mb`, `inventory_summary`, `audit_label_compliance`, `resource_graph_edges`.
2. **Compute Engine (`compute_*`)**: `compute_list_instances`, `compute_get_instance`, `compute_instance_count`, `compute_list_disks`, `compute_create_instance`, `compute_set_instance_labels`, `compute_stop_instance`, `compute_start_instance`, `compute_delete_instance`.
3. **VPC, Subnets & Firewalls (`network_*`)**: `network_list_vpcs`, `network_get_vpc`, `network_create_vpc`, `network_delete_vpc`, `network_list_subnets`, `network_get_subnet`, `network_create_subnet`, `network_delete_subnet`, `network_list_firewalls`, `network_get_firewall`, `network_create_firewall`, `network_delete_firewall`.
4. **Cloud Storage (`storage_*`)**: `storage_list_buckets`, `storage_get_bucket`, `storage_create_bucket`, `storage_set_bucket_lifecycle`, `storage_set_bucket_versioning`, `storage_set_uniform_access`, `storage_set_bucket_labels`, `storage_put_object`, `storage_list_objects`, `storage_read_object`, `storage_object_size`, `storage_delete_object`, `storage_delete_bucket`.
5. **Cloud Pub/Sub (`pubsub_*`)**: `pubsub_list_topics`, `pubsub_get_topic`, `pubsub_create_topic`, `pubsub_set_topic_labels`, `pubsub_delete_topic`, `pubsub_list_subscriptions`, `pubsub_get_subscription`, `pubsub_create_subscription`, `pubsub_publish`, `pubsub_pull`, `pubsub_delete_subscription`.
6. **Cloud Run v2 (`run_*`)**: `run_list_services`, `run_get_service`, `run_deploy_service`, `run_set_traffic_split`, `run_set_labels`, `run_delete_service`.
7. **Cloud SQL, Firestore & BigQuery (`sql_*`, `firestore_*`, `bq_*`)**: Full inspection and guarded ephemeral CRUD.
8. **IAM, Secret Manager Metadata & KMS (`iam_*`, `secrets_*`, `kms_*`)**: Least-privilege auditing, metadata-only secret inspection, and KMS key rotation checks.
9. **GKE, Artifact Registry, Cloud DNS, Logging & Monitoring (`gke_*`, `ar_*`, `dns_*`, `logging_*`, `monitoring_*`)**: Cluster hardening checks, container supply-chain verification, split-horizon DNS, log sink routing, and metric time-series queries.

Compiler >=0.2.420 resolves the SDK standard implicitly; schema generation uses `--dsl-signatures --without-std`, embeds host-only bytes, and tests retain full plus host-only registry parity.
