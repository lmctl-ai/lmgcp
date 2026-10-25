# OpenTofu (lmcloud) equivalent for lmgcp example: 06_cloud_run_canary_rollout
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

resource "google_cloud_run_v2_service" "cloud_run_lmgcp_checkout_api_1" {
  project  = var.project_id
  location = "us-central1"
  name     = "lmgcp-checkout-api"
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
  value = "06_cloud_run_canary_rollout"
}

output "estimated_cost" {
  value = "$0.0004"
}

output "resource_count" {
  value = "1"
}
