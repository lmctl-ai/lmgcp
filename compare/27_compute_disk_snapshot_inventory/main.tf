# OpenTofu (lmcloud) equivalent for lmgcp example: 27_compute_disk_snapshot_inventory
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

data "google_compute_instance" "prod_api_gw_01" {
  project = var.project_id
  zone    = "us-central1-a"
  name    = "prod-api-gw-01"
}

data "google_compute_instance" "analytics_etl_node_01" {
  project = var.project_id
  zone    = "us-west1-a"
  name    = "analytics-etl-node-01"
}

output "example_id" {
  value = "27_compute_disk_snapshot_inventory"
}

output "estimated_cost" {
  value = "$0.00"
}

output "resource_count" {
  value = "0"
}
