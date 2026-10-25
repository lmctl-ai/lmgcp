# OpenTofu (lmcloud) equivalent for lmgcp example: 38_cost_free_cis_posture_scorecard
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

data "google_project" "cis_target" {
  project_id = var.project_id
}

data "google_container_cluster" "cis_gke" {
  project  = var.project_id
  location = var.region
  name     = "prod-autopilot-core"
}

output "example_id" {
  value = "38_cost_free_cis_posture_scorecard"
}

output "estimated_cost" {
  value = "$0.00"
}

output "resource_count" {
  value = "0"
}
