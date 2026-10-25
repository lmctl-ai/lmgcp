# OpenTofu (lmcloud) equivalent for lmgcp example: 37_logging_monitoring_incident_pipeline
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

resource "terraform_data" "log_entry_lmgcp_incident_log_1" {
  input = {
    kind       = "log_entry"
    subtype    = "INFO"
    target     = "lmgcp-incident-log"
    lmgcp_test = var.run_id
  }
}

resource "terraform_data" "log_entry_lmgcp_incident_log_2" {
  input = {
    kind       = "log_entry"
    subtype    = "WARNING"
    target     = "lmgcp-incident-log"
    lmgcp_test = var.run_id
  }
}

resource "terraform_data" "log_entry_lmgcp_incident_log_3" {
  input = {
    kind       = "log_entry"
    subtype    = "ERROR"
    target     = "lmgcp-incident-log"
    lmgcp_test = var.run_id
  }
}

resource "google_logging_project_sink" "log_sink_lmgcp_incident_sink_4" {
  project     = var.project_id
  name        = "lmgcp-incident-sink"
  destination = "bigquery.googleapis.com/projects/${var.project_id}/datasets/security_telemetry"
  filter      = "severity>=WARNING"
}

resource "google_monitoring_alert_policy" "alert_policy_lmgcp_incident_slo_alert_5" {
  project      = var.project_id
  display_name = "lmgcp-incident-slo-alert"
  combiner     = "OR"
  enabled      = true

  user_labels = {
    lmgcp_test = var.run_id
    managed_by = "lmcloud"
  }
}

output "example_id" {
  value = "37_logging_monitoring_incident_pipeline"
}

output "estimated_cost" {
  value = "$0.0006"
}

output "resource_count" {
  value = "5"
}
