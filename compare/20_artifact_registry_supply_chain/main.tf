# OpenTofu (lmcloud) equivalent for lmgcp example: 20_artifact_registry_supply_chain
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
  default = "ar-supplychain-20"
}

provider "google" {
  project = var.project_id
  region  = var.region
  zone    = var.zone
}

resource "google_artifact_registry_repository" "artifact_repo_lmgcp_containers_1" {
  project       = var.project_id
  location      = var.region
  repository_id = "lmgcp-containers"
  format        = "DOCKER"

  labels = {
    lmgcp_test = var.run_id
    managed_by = "lmcloud"
  }
}

output "example_id" {
  value = "20_artifact_registry_supply_chain"
}

output "estimated_cost" {
  value = "$0.0002"
}

output "resource_count" {
  value = "1"
}
