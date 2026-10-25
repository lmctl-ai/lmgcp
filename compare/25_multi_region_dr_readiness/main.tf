# OpenTofu (lmcloud) equivalent for lmgcp example: 25_multi_region_dr_readiness
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

data "google_compute_subnetwork" "primary_subnet" {
  project = var.project_id
  region  = "us-central1"
  name    = "prod-app-subnet"
}

data "google_compute_subnetwork" "dr_subnet" {
  project = var.project_id
  region  = "us-east1"
  name    = "prod-dr-subnet"
}

output "example_id" {
  value = "25_multi_region_dr_readiness"
}

output "estimated_cost" {
  value = "$0.00"
}

output "resource_count" {
  value = "0"
}
