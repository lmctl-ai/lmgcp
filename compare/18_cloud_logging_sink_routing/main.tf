# OpenTofu (lmcloud) equivalent for lmgcp example: 18_cloud_logging_sink_routing
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
  default = "logging-sink-18"
}

provider "google" {
  project = var.project_id
  region  = var.region
  zone    = var.zone
}

resource "terraform_data" "log_entry_lmgcp_audit_log_1" {
  input = {
    kind       = "log_entry"
    subtype    = "WARNING"
    target     = "lmgcp-audit-log"
    lmgcp_test = var.run_id
  }
}

resource "google_logging_project_sink" "log_sink_lmgcp_siem_sink_2" {
  project     = var.project_id
  name        = "lmgcp-siem-sink"
  destination = "bigquery.googleapis.com/projects/${var.project_id}/datasets/security_telemetry"
  filter      = "severity>=WARNING"
}

output "example_id" {
  value = "18_cloud_logging_sink_routing"
}

output "estimated_cost" {
  value = "$0.0002"
}

output "resource_count" {
  value = "2"
}
