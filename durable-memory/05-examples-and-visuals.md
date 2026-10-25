# 05 — Examples & Visuals (26 Stories)

All 26 self-executing `.lms` examples live in `examples/` and render PNGs to `images/` and `docs/images/`:

1. `01_project_inventory_overview.lms` — 10-service estate inventory bar chart across 43 baseline resources.
2. `02_label_compliance_audit.lms` — Mandatory label (`env`, `owner`, `cost_center`, `data_class`) audit scorecard.
3. `03_vpc_firewall_security_review.lms` — Ingress firewall risk scoring (`0.0.0.0/0` SSH/RDP/Postgres detection).
4. `04_gcs_bucket_lifecycle_ephemeral.lms` — Ephemeral GCS bucket lifecycle (`STANDARD` -> `NEARLINE` 30d -> `EXPIRE` 365d).
5. `05_pubsub_dead_letter_pipeline.lms` — Ephemeral Pub/Sub topic, subscription, and dead-letter queue pipeline.
6. `06_cloud_run_canary_rollout.lms` — Progressive Cloud Run v2 traffic shifting (`10% -> 50% -> 100%`).
7. `07_cleanup_sweeper_dry_run.lms` — Age-gated orphan sweeper (`seed_demo_age`, `sweep_dry_run`, `sweep_delete`).
8. `08_vpc_resource_topology_graph.lms` — Multi-VPC dependency graph (`resource_graph_edges`).
9. `09_compute_fleet_rightsizing.lms` — Compute Engine fleet sizing, Shielded vTPM audit & `e2-micro` lifecycle probe.
10. `10_iam_least_privilege_review.lms` — IAM service account key age & primitive `roles/editor` audit.
11. `11_cloud_sql_ha_backup_posture.lms` — Cloud SQL Regional HA, PITR, SSL & Public IP posture + `db-f1-micro` CRUD.
12. `12_gke_cluster_hardening_matrix.lms` — CIS GKE hardening matrix (Autopilot, Private Nodes, Workload Identity, Binary Auth).
13. `13_secret_manager_metadata_audit.lms` — Secret Manager metadata-only rotation & CMEK audit.
14. `14_kms_key_rotation_posture.lms` — Cloud KMS CryptoKey rotation period vs 90-day CIS threshold & HSM posture.
15. `15_cloud_dns_zone_records.lms` — Cloud DNS split-horizon zones, DNSSEC & ephemeral record CRUD.
16. `16_bigquery_dataset_partition_audit.lms` — BigQuery dataset expiration & table time-partitioning audit.
17. `17_firestore_document_workflow.lms` — Cloud Firestore feature-flag inspection & ephemeral `lmgcp_sessions` CRUD.
18. `18_cloud_logging_sink_routing.lms` — Cloud Logging severity distribution, automatic secret redaction & SIEM sink routing.
19. `19_cloud_monitoring_slo_burn_rate.lms` — Cloud Monitoring P95 latency series & ephemeral SLO alert policy.
20. `20_artifact_registry_supply_chain.lms` — Artifact Registry tag immutability & vulnerability scanning posture.
21. `21_vpc_subnet_cidr_utilization.lms` — VPC regional subnet CIDR, Private Google Access & Flow Logs audit.
22. `22_ephemeral_sandbox_full_stack.lms` — 7-resource full-stack sandbox provisioning & LIFO `cleanup_ledger()`.
23. `23_storage_class_tiering_analysis.lms` — GCS storage class tiering (`STANDARD`, `NEARLINE`, `COLDLINE`, `ARCHIVE`) & CMEK.
24. `24_serverless_event_driven_mesh.lms` — End-to-end serverless mesh (`GCS -> Pub/Sub -> Cloud Run -> BigQuery`).
25. `25_multi_region_dr_readiness.lms` — Primary (`us-central1`) vs Recovery (`us-east1`) DR readiness scorecard.
26. `26_safety_guardrails_verification.lms` — Verification suite testing all 7 binding Cloud Safety Guardrails.
