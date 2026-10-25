# OpenTofu (lmcloud) equivalent for lmgcp example: 33_bigquery_multi_table_warehouse_lifecycle
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

resource "google_bigquery_dataset" "bq_dataset_lmgcp_finops_mart_1" {
  project                     = var.project_id
  dataset_id                  = "lmgcp_finops_mart"
  location                    = "US"
  default_table_expiration_ms = 2592000000
  delete_contents_on_destroy  = true

  labels = {
    lmgcp_test = var.run_id
    managed_by = "lmcloud"
  }
}

resource "google_bigquery_table" "bq_table_lmgcp_finops_mart_lmgcp_daily_sku_spend_2" {
  project             = var.project_id
  dataset_id          = "lmgcp_finops_mart"
  table_id            = "lmgcp_daily_sku_spend"
  deletion_protection = false

  time_partitioning {
    type  = "DAY"
    field = "usage_date"
  }

  labels = {
    lmgcp_test = var.run_id
    managed_by = "lmcloud"
  }
}

resource "google_bigquery_table" "bq_table_lmgcp_finops_mart_lmgcp_anomaly_alerts_3" {
  project             = var.project_id
  dataset_id          = "lmgcp_finops_mart"
  table_id            = "lmgcp_anomaly_alerts"
  deletion_protection = false

  time_partitioning {
    type  = "DAY"
    field = "detected_at"
  }

  labels = {
    lmgcp_test = var.run_id
    managed_by = "lmcloud"
  }
}

output "example_id" {
  value = "33_bigquery_multi_table_warehouse_lifecycle"
}

output "estimated_cost" {
  value = "$0.0005"
}

output "resource_count" {
  value = "3"
}
