# 05 — Examples & Visuals (40 Stories)

All 40 self-executing `.lms` examples live in `examples/`, render PNGs to `images/` and `docs/images/`, write JSON cost evidence to `build/evidence/live-cost-<example>.json`, and print `estimated cost: $0.00xx` (or `estimated cost: $0.00` for cost-free read-only audits) at the end:

1. `01_project_inventory_overview.lms` — 10-service estate inventory bar chart across 43 baseline resources (`$0.00`).
2. `02_label_compliance_audit.lms` — Mandatory label (`env`, `owner`, `cost_center`, `data_class`) audit scorecard (`$0.00`).
3. `03_vpc_firewall_security_review.lms` — Ingress firewall risk scoring (`0.0.0.0/0` SSH/RDP/Postgres detection) (`$0.00`).
4. `04_gcs_bucket_lifecycle_ephemeral.lms` — Ephemeral GCS bucket lifecycle (`STANDARD` -> `NEARLINE` 30d -> `EXPIRE` 365d) (`$0.0003`).
5. `05_pubsub_dead_letter_pipeline.lms` — Ephemeral Pub/Sub topic, subscription, and dead-letter queue pipeline (`$0.0006`).
6. `06_cloud_run_canary_rollout.lms` — Progressive Cloud Run v2 traffic shifting (`10% -> 50% -> 100%`) (`$0.0004`).
7. `07_cleanup_sweeper_dry_run.lms` — Age-gated orphan sweeper (`seed_demo_age`, `sweep_dry_run`, `sweep_delete`) (`$0.0015`).
8. `08_vpc_resource_topology_graph.lms` — Multi-VPC dependency graph (`resource_graph_edges`) (`$0.00`).
9. `09_compute_fleet_rightsizing.lms` — Compute Engine fleet sizing, Shielded vTPM audit & `e2-micro` lifecycle probe (`$0.0009`).
10. `10_iam_least_privilege_review.lms` — IAM service account key age & primitive `roles/editor` audit (`$0.00`).
11. `11_cloud_sql_ha_backup_posture.lms` — Cloud SQL Regional HA, PITR, SSL & Public IP posture + `db-f1-micro` CRUD (`$0.0027`).
12. `12_gke_cluster_hardening_matrix.lms` — CIS GKE hardening matrix (Autopilot, Private Nodes, Workload Identity, Binary Auth) (`$0.00`).
13. `13_secret_manager_metadata_audit.lms` — Secret Manager metadata-only rotation & CMEK audit (`$0.0002`).
14. `14_kms_key_rotation_posture.lms` — Cloud KMS CryptoKey rotation period vs 90-day CIS threshold & HSM posture (`$0.0010`).
15. `15_cloud_dns_zone_records.lms` — Cloud DNS split-horizon zones, DNSSEC & ephemeral record CRUD (`$0.0009`).
16. `16_bigquery_dataset_partition_audit.lms` — BigQuery dataset expiration & table time-partitioning audit (`$0.0003`).
17. `17_firestore_document_workflow.lms` — Cloud Firestore feature-flag inspection & ephemeral `lmgcp_sessions` CRUD (`$0.0002`).
18. `18_cloud_logging_sink_routing.lms` — Cloud Logging severity distribution, automatic secret redaction & SIEM sink routing (`$0.0002`).
19. `19_cloud_monitoring_slo_burn_rate.lms` — Cloud Monitoring P95 latency series & ephemeral SLO alert policy (`$0.0002`).
20. `20_artifact_registry_supply_chain.lms` — Artifact Registry tag immutability & vulnerability scanning posture (`$0.0002`).
21. `21_vpc_subnet_cidr_utilization.lms` — VPC regional subnet CIDR, Private Google Access & Flow Logs audit (`$0.0003`).
22. `22_ephemeral_sandbox_full_stack.lms` — 7-resource full-stack sandbox provisioning & LIFO `cleanup_ledger()` (`$0.0019`).
23. `23_storage_class_tiering_analysis.lms` — GCS storage class tiering (`STANDARD`, `NEARLINE`, `COLDLINE`, `ARCHIVE`) & CMEK (`$0.00`).
24. `24_serverless_event_driven_mesh.lms` — End-to-end serverless mesh (`GCS -> Pub/Sub -> Cloud Run -> BigQuery`) (`$0.0012`).
25. `25_multi_region_dr_readiness.lms` — Primary (`us-central1`) vs Recovery (`us-east1`) DR readiness scorecard (`$0.00`).
26. `26_safety_guardrails_verification.lms` — Verification suite testing all 8 binding Cloud Safety & Cost Guardrails (`$0.0009`).
27. `27_compute_disk_snapshot_inventory.lms` — Cost-free read-only Compute persistent disk & deletion protection inventory (`$0.00`).
28. `28_vpc_egress_ingress_matrix.lms` — VPC firewall priority matrix & ephemeral zero-trust mTLS rule lifecycle (`$0.0004`).
29. `29_gcs_object_retention_versioning_lab.lms` — GCS partitioned object ingestion, prefix filtering, redaction & versioning lab (`$0.0005`).
30. `30_pubsub_fanout_attribute_routing.lms` — Pub/Sub multi-subscription fan-out & attribute routing pipeline (`$0.0008`).
31. `31_cloud_run_multi_region_serverless.lms` — Cloud Run v2 multi-region active-active serverless deployment (`$0.0008`).
32. `32_iam_binding_blast_radius_audit.lms` — Cost-free read-only IAM policy binding & principal blast-radius audit (`$0.00`).
33. `33_bigquery_multi_table_warehouse_lifecycle.lms` — BigQuery multi-table FinOps mart & time-partitioning lifecycle (`$0.0005`).
34. `34_firestore_multi_collection_state_store.lms` — Cloud Firestore multi-collection workflow & distributed lock store (`$0.0003`).
35. `35_kms_multi_keyring_envelope_audit.lms` — Cloud KMS multi-KeyRing envelope encryption & 30d/60d HSM key rotation audit (`$0.0020`).
36. `36_dns_split_horizon_microservice_mesh.lms` — Cloud DNS split-horizon microservice discovery lifecycle (`$0.0010`).
37. `37_logging_monitoring_incident_pipeline.lms` — Integrated Cloud Logging & Monitoring incident detection pipeline (`$0.0006`).
38. `38_cost_free_cis_posture_scorecard.lms` — Cost-free read-only CIS Google Cloud Foundations posture scorecard (`$0.00`).
39. `39_multi_service_dependency_blast_radius.lms` — Cost-free read-only multi-service dependency fan-out & blast-radius graph (`$0.00`).
40. `40_cost_cap_ledger_live_readiness.lms` — RunLedger cumulative cost trajectory & per-example cap enforcement probe (`$0.0018`).
