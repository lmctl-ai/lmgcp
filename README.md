# lmgcp — Google Cloud Platform Scripting Host for `lmscript`

![GCP Resource Dependency Graph & Multi-VPC Topology](docs/images/08_vpc_resource_topology_graph.png)
*Run this story: `lmctl autorun https://raw.githubusercontent.com/lmctl-ai/lmgcp/main/examples/08_vpc_resource_topology_graph.lms`*

**`lmgcp`** is a single-binary Google Cloud Platform (`cloud.google.com/go` + `google.golang.org/api`, Apache-2.0) scripting host built on [`lmcode-go`](https://github.com/lmctl-ai/lmcode-go) for `lmscript` (`lmctl`). It exposes **365 typed DSL signatures** across **16 Google Cloud services**—Compute Engine, VPC Networks/Subnets/Firewalls, Cloud Storage (GCS), Pub/Sub, Cloud Run v2, Cloud SQL, Cloud Firestore, BigQuery, IAM, Secret Manager (metadata-only), Cloud KMS, GKE, Artifact Registry, Cloud DNS, Cloud Logging, and Cloud Monitoring—with strict built-in **Cloud Safety Guardrails** and an in-process **Fake GCP REST Server** for zero-cost offline verification.

---

## Visual Gallery (26 Self-Executing `.lms` Cloud Stories)

Every image below is generated directly by its `.lms` script in [`examples/`](examples/) against the deterministic in-process GCP REST emulator and rendered via [`lmdraw`](https://github.com/lmctl-ai/lmdraw).

### 1. Multi-Service Estate Inventory & Topology

| Estate Inventory Overview | Label & Tag Compliance Audit |
|---|---|
| ![Project Inventory Overview](docs/images/01_project_inventory_overview.png)<br>*`lmctl autorun https://raw.githubusercontent.com/lmctl-ai/lmgcp/main/examples/01_project_inventory_overview.lms`* | ![Label Compliance Audit](docs/images/02_label_compliance_audit.png)<br>*`lmctl autorun https://raw.githubusercontent.com/lmctl-ai/lmgcp/main/examples/02_label_compliance_audit.lms`* |

| VPC Firewall Security Review | Multi-VPC Topology & Dependency Graph |
|---|---|
| ![VPC Firewall Security Review](docs/images/03_vpc_firewall_security_review.png)<br>*`lmctl autorun https://raw.githubusercontent.com/lmctl-ai/lmgcp/main/examples/03_vpc_firewall_security_review.lms`* | ![VPC Resource Topology Graph](docs/images/08_vpc_resource_topology_graph.png)<br>*`lmctl autorun https://raw.githubusercontent.com/lmctl-ai/lmgcp/main/examples/08_vpc_resource_topology_graph.lms`* |

### 2. Storage, Messaging, Serverless & Ephemeral Lifecycles

| GCS Bucket Lifecycle & Tiering | Pub/Sub Dead-Letter Queue Pipeline |
|---|---|
| ![GCS Bucket Lifecycle](docs/images/04_gcs_bucket_lifecycle_ephemeral.png)<br>*`lmctl autorun https://raw.githubusercontent.com/lmctl-ai/lmgcp/main/examples/04_gcs_bucket_lifecycle_ephemeral.lms`* | ![Pub/Sub Dead-Letter Pipeline](docs/images/05_pubsub_dead_letter_pipeline.png)<br>*`lmctl autorun https://raw.githubusercontent.com/lmctl-ai/lmgcp/main/examples/05_pubsub_dead_letter_pipeline.lms`* |

| Cloud Run Progressive Canary Rollout | Orphaned Resource Sweeper (`--sweep`) |
|---|---|
| ![Cloud Run Canary Rollout](docs/images/06_cloud_run_canary_rollout.png)<br>*`lmctl autorun https://raw.githubusercontent.com/lmctl-ai/lmgcp/main/examples/06_cloud_run_canary_rollout.lms`* | ![Cleanup Sweeper Dry Run](docs/images/07_cleanup_sweeper_dry_run.png)<br>*`lmctl autorun https://raw.githubusercontent.com/lmctl-ai/lmgcp/main/examples/07_cleanup_sweeper_dry_run.lms`* |

### 3. Compute, IAM, Databases & Containers Security Posture

| Compute Engine Fleet Rightsizing | IAM Service Account Key & Role Audit |
|---|---|
| ![Compute Fleet Rightsizing](docs/images/09_compute_fleet_rightsizing.png)<br>*`lmctl autorun https://raw.githubusercontent.com/lmctl-ai/lmgcp/main/examples/09_compute_fleet_rightsizing.lms`* | ![IAM Least Privilege Review](docs/images/10_iam_least_privilege_review.png)<br>*`lmctl autorun https://raw.githubusercontent.com/lmctl-ai/lmgcp/main/examples/10_iam_least_privilege_review.lms`* |

| Cloud SQL HA, PITR & SSL Posture | GKE CIS Cluster Hardening Matrix |
|---|---|
| ![Cloud SQL HA Backup Posture](docs/images/11_cloud_sql_ha_backup_posture.png)<br>*`lmctl autorun https://raw.githubusercontent.com/lmctl-ai/lmgcp/main/examples/11_cloud_sql_ha_backup_posture.lms`* | ![GKE Cluster Hardening Matrix](docs/images/12_gke_cluster_hardening_matrix.png)<br>*`lmctl autorun https://raw.githubusercontent.com/lmctl-ai/lmgcp/main/examples/12_gke_cluster_hardening_matrix.lms`* |

### 4. Secrets, KMS, DNS, BigQuery & Firestore Workflows

| Secret Manager Metadata-Only Audit | Cloud KMS Key Rotation & HSM Posture |
|---|---|
| ![Secret Manager Metadata Audit](docs/images/13_secret_manager_metadata_audit.png)<br>*`lmctl autorun https://raw.githubusercontent.com/lmctl-ai/lmgcp/main/examples/13_secret_manager_metadata_audit.lms`* | ![KMS Key Rotation Posture](docs/images/14_kms_key_rotation_posture.png)<br>*`lmctl autorun https://raw.githubusercontent.com/lmctl-ai/lmgcp/main/examples/14_kms_key_rotation_posture.lms`* |

| Cloud DNS Split-Horizon & DNSSEC | BigQuery Time-Partitioned Warehouse |
|---|---|
| ![Cloud DNS Zone Records](docs/images/15_cloud_dns_zone_records.png)<br>*`lmctl autorun https://raw.githubusercontent.com/lmctl-ai/lmgcp/main/examples/15_cloud_dns_zone_records.lms`* | ![BigQuery Dataset Partition Audit](docs/images/16_bigquery_dataset_partition_audit.png)<br>*`lmctl autorun https://raw.githubusercontent.com/lmctl-ai/lmgcp/main/examples/16_bigquery_dataset_partition_audit.lms`* |

| Cloud Firestore Document Workflow | Cloud Logging Severity & SIEM Routing |
|---|---|
| ![Firestore Document Workflow](docs/images/17_firestore_document_workflow.png)<br>*`lmctl autorun https://raw.githubusercontent.com/lmctl-ai/lmgcp/main/examples/17_firestore_document_workflow.lms`* | ![Cloud Logging Sink Routing](docs/images/18_cloud_logging_sink_routing.png)<br>*`lmctl autorun https://raw.githubusercontent.com/lmctl-ai/lmgcp/main/examples/18_cloud_logging_sink_routing.lms`* |

### 5. Observability, Supply Chain, DR & Safety Guardrails

| Cloud Monitoring P95 Latency & SLO Burn | Artifact Registry Supply-Chain Security |
|---|---|
| ![Cloud Monitoring SLO Burn Rate](docs/images/19_cloud_monitoring_slo_burn_rate.png)<br>*`lmctl autorun https://raw.githubusercontent.com/lmctl-ai/lmgcp/main/examples/19_cloud_monitoring_slo_burn_rate.lms`* | ![Artifact Registry Supply Chain](docs/images/20_artifact_registry_supply_chain.png)<br>*`lmctl autorun https://raw.githubusercontent.com/lmctl-ai/lmgcp/main/examples/20_artifact_registry_supply_chain.lms`* |

| VPC Subnet CIDR, PGA & Flow Logs | Full-Stack Ephemeral Sandbox Teardown |
|---|---|
| ![VPC Subnet CIDR Utilization](docs/images/21_vpc_subnet_cidr_utilization.png)<br>*`lmctl autorun https://raw.githubusercontent.com/lmctl-ai/lmgcp/main/examples/21_vpc_subnet_cidr_utilization.lms`* | ![Ephemeral Sandbox Full Stack](docs/images/22_ephemeral_sandbox_full_stack.png)<br>*`lmctl autorun https://raw.githubusercontent.com/lmctl-ai/lmgcp/main/examples/22_ephemeral_sandbox_full_stack.lms`* |

| GCS Storage Class Tiering & CMEK | Serverless Event-Driven Mesh |
|---|---|
| ![Storage Class Tiering Analysis](docs/images/23_storage_class_tiering_analysis.png)<br>*`lmctl autorun https://raw.githubusercontent.com/lmctl-ai/lmgcp/main/examples/23_storage_class_tiering_analysis.lms`* | ![Serverless Event Driven Mesh](docs/images/24_serverless_event_driven_mesh.png)<br>*`lmctl autorun https://raw.githubusercontent.com/lmctl-ai/lmgcp/main/examples/24_serverless_event_driven_mesh.lms`* |

| Multi-Region DR Readiness Scorecard | Cloud Safety Guardrails Verification |
|---|---|
| ![Multi Region DR Readiness](docs/images/25_multi_region_dr_readiness.png)<br>*`lmctl autorun https://raw.githubusercontent.com/lmctl-ai/lmgcp/main/examples/25_multi_region_dr_readiness.lms`* | ![Safety Guardrails Verification](docs/images/26_safety_guardrails_verification.png)<br>*`lmctl autorun https://raw.githubusercontent.com/lmctl-ai/lmgcp/main/examples/26_safety_guardrails_verification.lms`* |

---

## Binding Cloud Safety Guardrails

`lmgcp` enforces **7 mandatory safety controls** inside [`internal/safety/`](internal/safety/) and [`internal/host/`](internal/host/):

1. **Fake-by-Default Endpoint Gate**: By default (`LMGCP_LIVE` unset or `0`), `lmgcp` boots an in-process loopback GCP REST emulator (`internal/fakegcp`) and never contacts `*.googleapis.com` or reads ambient `gcloud` credentials. Live GCP calls require explicit `LMGCP_LIVE=1`.
2. **Zero Secret Exposure & Automatic Redaction**: `AccessSecretVersion` is intentionally omitted from the DSL surface (Secret Manager is metadata-only). All strings and errors pass through `safety.RedactSecrets()`, scrubbing `ya29.*` OAuth tokens, `AIza*` API keys, PEM blocks, JWTs, and password parameters into `[REDACTED]`.
3. **Project & Account ID Masking**: `gcp.project()` and `gcp.masked_project()` mask project identifiers (`gcp-proj-****8f2a`) so real project or billing IDs never appear in logs, CSVs, or rendered PNGs.
4. **Mandatory `lmgcp_` / `lmgcp-` Prefix & Run Labels**: Every mutating `create_*` / `deploy_*` call validates that the target resource name begins with `lmgcp_` or `lmgcp-` and attaches `lmgcp_test=<run-id>` and `lmgcp_created_at=<epoch>`.
5. **Compute & SQL Cost/Shape Guardrails**: Only small/free-tier test shapes are permitted (`e2-micro`, `e2-small`, `e2-medium`, `f1-micro`, `g1-small`, `db-f1-micro`, `db-g1-small`). GPU shapes (`a2-*`, `g2-*`) and large shapes (`n2-standard-64`, `db-n1-highmem-32`) are rejected pre-flight.
6. **RunLedger Ownership Enforcement**: Mutating or deleting a resource requires **both** the `lmgcp_` / `lmgcp-` prefix **and** ownership in the current run's `RunLedger`. Pre-existing resources cannot be modified or deleted. `gcp.cleanup_ledger()` tears down all run-owned resources in reverse dependency order (LIFO).
7. **Orphan Sweeper (`--sweep`)**: `lmgcp --sweep [--older-than-hours N] [--delete]` scans for stale `lmgcp-*` resources carrying `lmgcp_test` labels and supports dry-run listing (default) or age-gated deletion.

---

## Covered Google Cloud Go SDK Services (16 APIs / 365 DSL Signatures)

| Service Family | Go SDK Package (`google.golang.org/api/...` & `cloud.google.com/go/...`) | DSL Types | Representative Methods on `gcp` Context |
|---|---|---|---|
| **Auth & Metadata** | `cloud.google.com/go/auth`, `cloud.google.com/go/compute/metadata` | `Gcp` | `masked_project`, `auth_mode`, `is_live`, `redact`, `cleanup_ledger`, `sweep_dry_run`, `sweep_delete` |
| **Compute Engine** | `google.golang.org/api/compute/v1` | `ComputeInstance` | `compute_list_instances`, `compute_get_instance`, `compute_create_instance`, `compute_stop_instance`, `compute_delete_instance` |
| **VPC, Subnets & Firewalls** | `google.golang.org/api/compute/v1` | `VpcNetwork`, `Subnet`, `FirewallRule` | `network_list_vpcs`, `network_create_vpc`, `network_list_subnets`, `network_create_subnet`, `network_list_firewalls`, `network_get_firewall` |
| **Cloud Storage (GCS)** | `google.golang.org/api/storage/v1` | `StorageBucket` | `storage_list_buckets`, `storage_create_bucket`, `storage_set_bucket_lifecycle`, `storage_put_object`, `storage_delete_bucket` |
| **Cloud Pub/Sub** | `google.golang.org/api/pubsub/v1` | `PubsubTopic`, `PubsubSubscription` | `pubsub_create_topic`, `pubsub_create_subscription`, `pubsub_publish`, `pubsub_pull`, `pubsub_delete_topic` |
| **Cloud Run v2** | `google.golang.org/api/run/v2` | `CloudRunService` | `run_list_services`, `run_deploy_service`, `run_set_traffic_split`, `run_delete_service` |
| **Cloud SQL** | `google.golang.org/api/sqladmin/v1` | `SqlInstance` | `sql_list_instances`, `sql_get_instance`, `sql_create_instance`, `sql_delete_instance` |
| **Cloud Firestore** | `google.golang.org/api/firestore/v1` | `StrMap` | `firestore_list_collections`, `firestore_put_doc`, `firestore_get_doc`, `firestore_delete_doc` |
| **BigQuery** | `google.golang.org/api/bigquery/v2` | `BqDataset`, `BqTable` | `bq_list_datasets`, `bq_create_dataset`, `bq_create_table`, `bq_get_table`, `bq_delete_dataset` |
| **Cloud IAM** | `google.golang.org/api/iam/v1`, `cloudresourcemanager/v1` | `ServiceAccount` | `iam_list_service_accounts`, `iam_get_service_account`, `iam_list_bindings` |
| **Secret Manager (Meta)** | `google.golang.org/api/secretmanager/v1` | `SecretMetadata` | `secrets_list`, `secrets_get_metadata`, `secrets_create_metadata`, `secrets_delete` |
| **Cloud KMS** | `google.golang.org/api/cloudkms/v1` | `KmsKey` | `kms_list_keyrings`, `kms_list_keys`, `kms_get_key`, `kms_create_key` |
| **GKE (Kubernetes)** | `google.golang.org/api/container/v1` | `GkeCluster` | `gke_list_clusters`, `gke_get_cluster` |
| **Artifact Registry** | `google.golang.org/api/artifactregistry/v1` | `ArtifactRepo` | `ar_list_repositories`, `ar_get_repository`, `ar_create_repository`, `ar_delete_repository` |
| **Cloud DNS** | `google.golang.org/api/dns/v1` | `DnsZone` | `dns_list_zones`, `dns_create_zone`, `dns_add_record`, `dns_list_records`, `dns_delete_zone` |
| **Cloud Logging & Monitoring** | `google.golang.org/api/logging/v2`, `monitoring/v3` | `LogSink`, `AlertPolicy` | `logging_severity_counts`, `logging_create_sink`, `monitoring_query_series`, `monitoring_create_alert_policy` |

---

## Quickstart & CLI Reference

```bash
# Build single native executable
go build -o bin/lmgcp ./cmd/lmgcp

# Inspect CLI flags, version, schema, and 365 DSL signatures
./bin/lmgcp --version
./bin/lmgcp --help
./bin/lmgcp --dsl-schema --json
./bin/lmgcp --dsl-signatures
./bin/lmgcp --notices

# Run full offline test suite + all 26 examples (uses in-process fake GCP server)
./test.sh

# Dry-run or execute the orphaned lmgcp-* resource sweeper
./bin/lmgcp --sweep --older-than-hours 1
./bin/lmgcp --sweep --older-than-hours 1 --delete
```

### Optional Live GCP Testing (Requires Explicit Opt-In)

By default, `lmgcp` never touches a live GCP project. To test against a dedicated non-production GCP project:

```bash
export GOOGLE_CLOUD_PROJECT="your-sandbox-project-id"
export GOOGLE_CLOUD_REGION="us-central1"
export GOOGLE_CLOUD_ZONE="us-central1-a"
export LMGCP_LIVE=1

# Verify Application Default Credentials (ADC) are active
gcloud auth application-default print-access-token >/dev/null

# Run read-only inventory & compliance stories against live GCP
lmscript examples/01_project_inventory_overview.lms
lmscript examples/02_label_compliance_audit.lms

# Sweep any orphaned lmgcp-* test resources (dry-run first, then delete)
./bin/lmgcp --sweep --older-than-hours 0
./bin/lmgcp --sweep --older-than-hours 0 --delete
```

---

## License

MIT License — see [`THIRD_PARTY_NOTICES`](THIRD_PARTY_NOTICES) for Apache-2.0 and BSD-3-Clause notices of the Google Cloud Go SDK and transitive dependencies.
