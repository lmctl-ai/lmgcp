# 03 — Schema, Types & Curated Service Set

## Schema & Signature Counts

- **Schema file**: [`schema/lmgcp.dsl.json`](../schema/lmgcp.dsl.json) (`dsl_version: "1"`, `compiler_min_version: "0.2.420"`, context `gcp: Gcp`).
- **Host object types (`20`)**: `Gcp`, `AlertPolicy`, `ArtifactRepo`, `BqDataset`, `BqTable`, `CloudRunService`, `ComputeInstance`, `DnsZone`, `FirewallRule`, `GkeCluster`, `KmsKey`, `LogSink`, `PubsubSubscription`, `PubsubTopic`, `SecretMetadata`, `ServiceAccount`, `SqlInstance`, `StorageBucket`, `Subnet`, `VpcNetwork`.
- **Host-only signatures (`298`)**: `./bin/lmgcp --dsl-signatures --without-std` outputs the 298 GCP host signatures embedded in `schema/lmgcp.dsl.json`.
- **Total runtime signatures (`370`)**: `./bin/lmgcp --dsl-signatures` outputs all 370 registered signatures (298 host signatures + 72 `lmcode_std_v1` collection/string/math/file/JSON and testing built-ins `StrList`, `IntList`, `FloatList`, `StrMap`, `assert_close`, `print`). Compiler `lmscript >= 0.2.420` supplies the built-in standard library implicitly.

## Curated 16-Domain GCP Service Set (`internal/host/host.go`)

1. **Core, Safety, Ledger & Cross-Service Audits (`Gcp`)**:
   - Project/region/zone/mode: `project`, `masked_project`, `set_project`, `region`, `set_region`, `zone`, `set_zone`, `is_live`, `auth_mode`, `endpoint`, `run_id`, `set_run_id`, `last_error`, `redact`, `alloc_scratch_mb`.
   - Ledger & cost: `set_auto_cleanup`, `ledger_count`, `ledger_entries`, `ledger_owns`, `cost_cap_usd`, `set_cost_cap_usd`, `estimated_cost_usd`, `cost_report`, `write_cost_evidence`, `cleanup_ledger`.
   - Sweeper & estate posture: `seed_demo_age`, `sweep_dry_run`, `sweep_delete`, `inventory_summary`, `audit_label_compliance`, `resource_graph_edges`.
2. **Compute Engine (`compute_*`)**: `compute_list_instances`, `compute_get_instance`, `compute_list_disks`, `compute_create_instance`, `compute_set_instance_labels`, `compute_stop_instance`, `compute_start_instance`, `compute_delete_instance`.
3. **VPC Networks, Subnets & Firewalls (`network_*`)**: `network_list_vpcs`, `network_get_vpc`, `network_create_vpc`, `network_delete_vpc`, `network_list_subnets`, `network_get_subnet`, `network_create_subnet`, `network_delete_subnet`, `network_list_firewalls`, `network_get_firewall`, `network_create_firewall`, `network_delete_firewall`.
4. **Cloud Storage (`storage_*`)**: `storage_list_buckets`, `storage_get_bucket`, `storage_create_bucket`, `storage_set_bucket_lifecycle`, `storage_put_object`, `storage_list_objects`, `storage_read_object`, `storage_delete_object`, `storage_delete_bucket`.
5. **Cloud Pub/Sub (`pubsub_*`)**: `pubsub_list_topics`, `pubsub_get_topic`, `pubsub_create_topic`, `pubsub_delete_topic`, `pubsub_list_subscriptions`, `pubsub_get_subscription`, `pubsub_create_subscription`, `pubsub_set_dead_letter`, `pubsub_publish`, `pubsub_pull`, `pubsub_delete_subscription`.
6. **Cloud Run v2 (`run_*`)**: `run_list_services`, `run_get_service`, `run_deploy_service`, `run_set_canary_traffic`, `run_delete_service`.
7. **Cloud SQL (`sql_*`)**: `sql_list_instances`, `sql_get_instance`, `sql_create_instance`, `sql_delete_instance`.
8. **BigQuery (`bq_*`)**: `bq_list_datasets`, `bq_get_dataset`, `bq_create_dataset`, `bq_delete_dataset`, `bq_get_table`, `bq_create_table`, `bq_delete_table`.
9. **Cloud Firestore (`firestore_*`)**: `firestore_list_docs`, `firestore_get_doc`, `firestore_put_doc`, `firestore_delete_doc`.
10. **IAM Hygiene — Read-Only (`iam_*`)**: `iam_list_service_accounts`, `iam_get_service_account`, `iam_list_bindings`.
11. **Secret Manager Metadata (`secrets_*`)**: `secrets_list`, `secrets_get_metadata`, `secrets_create_metadata`, `secrets_delete` (metadata only; secret payload values are never accessed).
12. **Cloud KMS (`kms_*`)**: `kms_list_keyrings`, `kms_list_keys`, `kms_get_key`, `kms_create_key`.
13. **Google Kubernetes Engine — Read-Only (`gke_*`)**: `gke_list_clusters`, `gke_get_cluster`.
14. **Artifact Registry (`ar_*`)**: `ar_list_repositories`, `ar_get_repository`, `ar_create_repository`, `ar_delete_repository`.
15. **Cloud DNS (`dns_*`)**: `dns_list_zones`, `dns_get_zone`, `dns_create_zone`, `dns_delete_zone`, `dns_list_records`, `dns_add_record`, `dns_delete_record`.
16. **Cloud Logging & Cloud Monitoring (`logging_*`, `monitoring_*`)**: `logging_list_sinks`, `logging_get_sink`, `logging_create_sink`, `logging_delete_sink`, `logging_write_entry`, `logging_severity_counts`, `monitoring_list_metrics`, `monitoring_query_series`, `monitoring_list_alert_policies`, `monitoring_get_alert_policy`, `monitoring_create_alert_policy`, `monitoring_delete_alert_policy`.
