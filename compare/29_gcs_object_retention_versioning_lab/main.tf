# OpenTofu (lmcloud) equivalent for lmgcp example: 29_gcs_object_retention_versioning_lab
# Uses the hashicorp/google provider with identical lmgcp_ / lmgcp- prefixes and lmgcp_test labels.

terraform {
  required_version = ">= 1.6.0"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
  }
}

variable "project_id" {
  type    = string
  default = "gcp-proj-masked"
}

variable "region" {
  type    = string
  default = "us-central1"
}

variable "zone" {
  type    = string
  default = "us-central1-a"
}

variable "run_id" {
  type    = string
  default = "run-20261024"
}

provider "google" {
  project = var.project_id
  region  = var.region
  zone    = var.zone
}

resource "google_storage_bucket" "bucket_lmgcp_telemetry_lake_1" {
  project                     = var.project_id
  name                        = "lmgcp-telemetry-lake"
  location                    = "US-CENTRAL1"
  storage_class               = "STANDARD"
  uniform_bucket_level_access = true
  force_destroy               = true

  labels = {
    lmgcp_test = var.run_id
    managed_by = "lmcloud"
  }
}

resource "google_storage_bucket_object" "storage_object_lmgcp_telemetry_lake_raw_2026_10_24_shard_00_jsonl_2" {
  bucket       = "lmgcp-telemetry-lake"
  name         = "raw/2026-10-24/shard-00.jsonl"
  content_type = "application/json"
  content      = "lmgcp_test=${var.run_id}"
}

resource "google_storage_bucket_object" "storage_object_lmgcp_telemetry_lake_raw_2026_10_24_shard_01_jsonl_3" {
  bucket       = "lmgcp-telemetry-lake"
  name         = "raw/2026-10-24/shard-01.jsonl"
  content_type = "application/json"
  content      = "lmgcp_test=${var.run_id}"
}

resource "google_storage_bucket_object" "storage_object_lmgcp_telemetry_lake_curated_2026_10_24_summary_csv_4" {
  bucket       = "lmgcp-telemetry-lake"
  name         = "curated/2026-10-24/summary.csv"
  content_type = "text/csv"
  content      = "lmgcp_test=${var.run_id}"
}

resource "google_storage_bucket_object" "storage_object_lmgcp_telemetry_lake_tmp_scratch_txt_5" {
  bucket       = "lmgcp-telemetry-lake"
  name         = "tmp/scratch.txt"
  content_type = "text/plain"
  content      = "lmgcp_test=${var.run_id}"
}

output "example_id" {
  value = "29_gcs_object_retention_versioning_lab"
}

output "estimated_cost" {
  value = "$0.0005"
}

output "resource_count" {
  value = "5"
}
