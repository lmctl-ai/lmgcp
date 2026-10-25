# OpenTofu (lmcloud) equivalent for lmgcp example: 19_cloud_monitoring_slo_burn_rate
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
  default = "monitoring-slo-19"
}

provider "google" {
  project = var.project_id
  region  = var.region
  zone    = var.zone
}

resource "google_monitoring_alert_policy" "alert_policy_lmgcp_slo_latency_alert_1" {
  project      = var.project_id
  display_name = "lmgcp-slo-latency-alert"
  combiner     = "OR"
  enabled      = true

  user_labels = {
    lmgcp_test = var.run_id
    managed_by = "lmcloud"
  }
}

output "example_id" {
  value = "19_cloud_monitoring_slo_burn_rate"
}

output "estimated_cost" {
  value = "$0.0002"
}

output "resource_count" {
  value = "1"
}
