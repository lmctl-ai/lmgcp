# OpenTofu (lmcloud) equivalent for lmgcp example: 31_cloud_run_multi_region_serverless
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

resource "google_cloud_run_v2_service" "cloud_run_lmgcp_edge_webhook_usc1_1" {
  project  = var.project_id
  location = "us-central1"
  name     = "lmgcp-edge-webhook-usc1"
  ingress  = "INGRESS_TRAFFIC_ALL"

  template {
    containers {
      image = "us-central1-docker.pkg.dev/demo/app:v1"
    }
  }

  labels = {
    lmgcp_test = var.run_id
    managed_by = "lmcloud"
  }
}

resource "google_cloud_run_v2_service" "cloud_run_lmgcp_edge_webhook_use1_2" {
  project  = var.project_id
  location = "us-east1"
  name     = "lmgcp-edge-webhook-use1"
  ingress  = "INGRESS_TRAFFIC_ALL"

  template {
    containers {
      image = "us-central1-docker.pkg.dev/demo/app:v1"
    }
  }

  labels = {
    lmgcp_test = var.run_id
    managed_by = "lmcloud"
  }
}

output "example_id" {
  value = "31_cloud_run_multi_region_serverless"
}

output "estimated_cost" {
  value = "$0.0008"
}

output "resource_count" {
  value = "2"
}
