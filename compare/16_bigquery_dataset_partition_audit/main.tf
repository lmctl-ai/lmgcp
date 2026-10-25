# OpenTofu (lmcloud) equivalent for lmgcp example: 16_bigquery_dataset_partition_audit
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
  default = "bq-audit-16"
}

provider "google" {
  project = var.project_id
  region  = var.region
  zone    = var.zone
}

resource "google_bigquery_dataset" "bq_dataset_lmgcp_analytics_ds_1" {
  project                     = var.project_id
  dataset_id                  = "lmgcp_analytics_ds"
  location                    = "US"
  default_table_expiration_ms = 2592000000
  delete_contents_on_destroy  = true

  labels = {
    lmgcp_test = var.run_id
    managed_by = "lmcloud"
  }
}

resource "google_bigquery_table" "bq_table_lmgcp_analytics_ds_lmgcp_session_rollup_2" {
  project             = var.project_id
  dataset_id          = "lmgcp_analytics_ds"
  table_id            = "lmgcp_session_rollup"
  deletion_protection = false

  time_partitioning {
    type  = "DAY"
    field = "ts"
  }

  labels = {
    lmgcp_test = var.run_id
    managed_by = "lmcloud"
  }
}

output "example_id" {
  value = "16_bigquery_dataset_partition_audit"
}

output "estimated_cost" {
  value = "$0.0003"
}

output "resource_count" {
  value = "2"
}
