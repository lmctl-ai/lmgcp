# OpenTofu (lmcloud) equivalent for lmgcp example: 04_gcs_bucket_lifecycle_ephemeral
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

resource "google_storage_bucket" "bucket_lmgcp_lifecycle_demo_1" {
  project                     = var.project_id
  name                        = "lmgcp-lifecycle-demo"
  location                    = "US-CENTRAL1"
  storage_class               = "STANDARD"
  uniform_bucket_level_access = true
  force_destroy               = true

  labels = {
    lmgcp_test = var.run_id
    managed_by = "lmcloud"
  }
}

resource "google_storage_bucket_object" "storage_object_lmgcp_lifecycle_demo_2026_10_events_01_json_2" {
  bucket       = "lmgcp-lifecycle-demo"
  name         = "2026/10/events-01.json"
  content_type = "application/json"
  content      = "lmgcp_test=${var.run_id}"
}

resource "google_storage_bucket_object" "storage_object_lmgcp_lifecycle_demo_2026_10_events_02_json_3" {
  bucket       = "lmgcp-lifecycle-demo"
  name         = "2026/10/events-02.json"
  content_type = "application/json"
  content      = "lmgcp_test=${var.run_id}"
}

output "example_id" {
  value = "04_gcs_bucket_lifecycle_ephemeral"
}

output "estimated_cost" {
  value = "$0.0003"
}

output "resource_count" {
  value = "3"
}
