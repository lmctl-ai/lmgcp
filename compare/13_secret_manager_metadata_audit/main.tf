# OpenTofu (lmcloud) equivalent for lmgcp example: 13_secret_manager_metadata_audit
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
  default = "secret-meta-13"
}

provider "google" {
  project = var.project_id
  region  = var.region
  zone    = var.zone
}

resource "google_secret_manager_secret" "secret_lmgcp_secret_meta_probe_1" {
  project   = var.project_id
  secret_id = "lmgcp-secret-meta-probe"

  replication {
    auto {}
  }

  labels = {
    lmgcp_test = var.run_id
    managed_by = "lmcloud"
  }
}

output "example_id" {
  value = "13_secret_manager_metadata_audit"
}

output "estimated_cost" {
  value = "$0.0002"
}

output "resource_count" {
  value = "1"
}
