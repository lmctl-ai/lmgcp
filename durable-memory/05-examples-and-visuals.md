# 05 — Examples, Cost Labels & `lmctl autorun` (40 Stories)

## Overview & How Examples Run

- **Total count**: **40** self-executing `.lms` cloud stories in [`examples/`](../examples/) with **40** matching PNG visuals in [`docs/images/`](../docs/images/).
  - **14 read-only audits (`$0.00`)**: `01`, `02`, `03`, `08`, `10`, `12`, `23`, `25`, `27`, `32`, `38`, `39` (plus read-only baseline phases in hybrid stories). Zero resources created.
  - **26 mutating ephemeral stories (`$0.0002` – `$0.0027`)**: Create between 1 and 8 `lmgcp_` / `lmgcp-` resources (**88** ephemeral resources total across the suite), verify state/redaction, tear down every resource via `gcp.cleanup_ledger()` (`leftovers = 0`), write `build/evidence/live-cost-<example>.json`, and print `estimated cost: $0.00xx`. Cumulative estimated cost across all 40 examples is **`$0.0218`**.
- **Running locally or via `lmctl autorun`**:
  ```bash
  # Local execution from repo checkout:
  PATH="$PWD/bin:$PATH" lmscript examples/04_gcs_bucket_lifecycle_ephemeral.lms

  # Zero-setup execution from an empty directory (auto-installs lmgcp@latest + lmdraw@latest):
  lmctl autorun https://raw.githubusercontent.com/lmctl-ai/lmgcp/main/examples/04_gcs_bucket_lifecycle_ephemeral.lms
  ```
- **README Cost Labels**: Every example section in [`README.md`](../README.md) includes the picture first, the `lmctl autorun` raw URL, and `"Running this example costs about $X (estimated from price table, not yet measured live)"` with a note on how estimates are computed from the `us-central1` price table.

## Complete Catalog of 40 Examples

| # | Script (`examples/`) | Category | Resources Created | Estimated Cost |
| :---: | :--- | :--- | :---: | :---: |
| 01 | `01_project_inventory_overview.lms` | Estate Inventory | 0 (read-only) | `$0.00` |
| 02 | `02_label_compliance_audit.lms` | Label Compliance | 0 (read-only) | `$0.00` |
| 03 | `03_vpc_firewall_security_review.lms` | Network Security | 0 (read-only) | `$0.00` |
| 04 | `04_gcs_bucket_lifecycle_ephemeral.lms` | Cloud Storage (GCS) | 3 (1 bucket, 2 objects) | `$0.0003` |
| 05 | `05_pubsub_dead_letter_pipeline.lms` | Pub/Sub Messaging | 6 (2 topics, 2 subs, 2 msgs) | `$0.0006` |
| 06 | `06_cloud_run_canary_rollout.lms` | Cloud Run v2 | 1 (1 service, 3 traffic shifts) | `$0.0004` |
| 07 | `07_cleanup_sweeper_dry_run.lms` | Orphan Sweeper | 4 (VM, bucket, topic, Run) | `$0.0015` |
| 08 | `08_vpc_resource_topology_graph.lms` | Topology Graph | 0 (read-only) | `$0.00` |
| 09 | `09_compute_fleet_rightsizing.lms` | Compute Engine | 1 (`e2-micro` VM) | `$0.0009` |
| 10 | `10_iam_least_privilege_review.lms` | IAM Hygiene (Read-Only) | 0 (read-only) | `$0.00` |
| 11 | `11_cloud_sql_ha_backup_posture.lms` | Cloud SQL | 1 (`db-f1-micro` instance) | `$0.0027` |
| 12 | `12_gke_cluster_hardening_matrix.lms` | GKE Security | 0 (read-only) | `$0.00` |
| 13 | `13_secret_manager_metadata_audit.lms` | Secret Manager Metadata | 1 (1 secret metadata) | `$0.0002` |
| 14 | `14_kms_key_rotation_posture.lms` | Cloud KMS | 1 (1 CryptoKey, 30d rotation) | `$0.0010` |
| 15 | `15_cloud_dns_zone_records.lms` | Cloud DNS | 3 (1 zone, 2 record sets) | `$0.0009` |
| 16 | `16_bigquery_dataset_partition_audit.lms` | BigQuery | 2 (1 dataset, 1 DAY table) | `$0.0003` |
| 17 | `17_firestore_document_workflow.lms` | Cloud Firestore | 2 (2 docs in `lmgcp_runs`) | `$0.0002` |
| 18 | `18_cloud_logging_sink_routing.lms` | Cloud Logging | 2 (1 sink, 1 log entry) | `$0.0002` |
| 19 | `19_cloud_monitoring_slo_burn_rate.lms` | Cloud Monitoring | 1 (1 SLO alert policy) | `$0.0002` |
| 20 | `20_artifact_registry_supply_chain.lms` | Artifact Registry | 1 (1 Docker repository) | `$0.0002` |
| 21 | `21_vpc_subnet_cidr_utilization.lms` | VPC Networking | 2 (1 VPC, 1 subnet) | `$0.0003` |
| 22 | `22_ephemeral_sandbox_full_stack.lms` | Full-Stack Blueprint | 7 (VPC, subnet, FW, GCS, Pub/Sub, Run) | `$0.0019` |
| 23 | `23_storage_class_tiering_analysis.lms` | Storage FinOps | 0 (read-only) | `$0.00` |
| 24 | `24_serverless_event_driven_mesh.lms` | Event-Driven Mesh | 8 (bucket, object, topic, sub, Run, msgs) | `$0.0012` |
| 25 | `25_multi_region_dr_readiness.lms` | DR Readiness | 0 (read-only) | `$0.00` |
| 26 | `26_safety_guardrails_verification.lms` | Safety Guardrails | 1 (1 valid VM + 5 blocked calls) | `$0.0009` |
| 27 | `27_compute_disk_snapshot_inventory.lms` | Compute Storage Audit | 0 (read-only) | `$0.00` |
| 28 | `28_vpc_egress_ingress_matrix.lms` | Micro-Segmentation | 3 (1 VPC, 1 subnet, 1 firewall) | `$0.0004` |
| 29 | `29_gcs_object_retention_versioning_lab.lms` | GCS Versioning Lab | 5 (1 bucket, 4 objects) | `$0.0005` |
| 30 | `30_pubsub_fanout_attribute_routing.lms` | Pub/Sub Fan-Out | 8 (1 topic, 2 subs, 5 msgs) | `$0.0008` |
| 31 | `31_cloud_run_multi_region_serverless.lms` | Multi-Region Cloud Run | 2 (2 services: `usc1` + `use1`) | `$0.0008` |
| 32 | `32_iam_binding_blast_radius_audit.lms` | IAM Blast-Radius (Read-Only) | 0 (read-only) | `$0.00` |
| 33 | `33_bigquery_multi_table_warehouse_lifecycle.lms` | BigQuery Warehouse | 3 (1 dataset, 2 DAY tables) | `$0.0005` |
| 34 | `34_firestore_multi_collection_state_store.lms` | Firestore State Store | 3 (3 docs across 2 collections) | `$0.0003` |
| 35 | `35_kms_multi_keyring_envelope_audit.lms` | KMS Envelope Audit | 2 (2 HSM CryptoKeys: 30d, 60d) | `$0.0020` |
| 36 | `36_dns_split_horizon_microservice_mesh.lms` | Cloud DNS Split-Horizon | 4 (1 private zone, 3 records) | `$0.0010` |
| 37 | `37_logging_monitoring_incident_pipeline.lms` | Observability Pipeline | 5 (1 sink, 1 alert, 3 log entries) | `$0.0006` |
| 38 | `38_cost_free_cis_posture_scorecard.lms` | CIS Benchmark Scorecard | 0 (read-only) | `$0.00` |
| 39 | `39_multi_service_dependency_blast_radius.lms` | Dependency Blast-Radius | 0 (read-only) | `$0.00` |
| 40 | `40_cost_cap_ledger_live_readiness.lms` | Cost Ledger & Cap Gate | 6 (6 created + 7th SQL blocked by cap) | `$0.0018` |
