# OpenTofu (lmcloud) equivalent for lmgcp example: 39_multi_service_dependency_blast_radius
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

data "google_compute_network" "tier0_hub" {
  project = var.project_id
  name    = "prod-vpc"
}

data "google_pubsub_topic" "event_backbone" {
  project = var.project_id
  name    = "order-events-v1"
}

output "example_id" {
  value = "39_multi_service_dependency_blast_radius"
}

output "estimated_cost" {
  value = "$0.00"
}

output "resource_count" {
  value = "0"
}
