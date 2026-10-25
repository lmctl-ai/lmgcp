# OpenTofu (lmcloud) equivalent for lmgcp example: 36_dns_split_horizon_microservice_mesh
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

resource "google_dns_managed_zone" "dns_zone_lmgcp_mesh_internal_1" {
  project    = var.project_id
  name       = "lmgcp-mesh-internal"
  dns_name   = "sandbox.gcp-masked.example."
  visibility = "private"

  labels = {
    lmgcp_test = var.run_id
    managed_by = "lmcloud"
  }
}

resource "google_dns_record_set" "dns_record_lmgcp_mesh_internal_checkout_mesh_lmgcp_internal_2" {
  project      = var.project_id
  managed_zone = "lmgcp-mesh-internal"
  name         = "checkout.mesh.lmgcp.internal."
  type         = "A"
  ttl          = 60
  rrdatas      = ["10.10.1.25"]
}

resource "google_dns_record_set" "dns_record_lmgcp_mesh_internal_payments_mesh_lmgcp_internal_3" {
  project      = var.project_id
  managed_zone = "lmgcp-mesh-internal"
  name         = "payments.mesh.lmgcp.internal."
  type         = "A"
  ttl          = 60
  rrdatas      = ["10.10.1.25"]
}

resource "google_dns_record_set" "dns_record_lmgcp_mesh_internal__policy_mesh_lmgcp_internal_4" {
  project      = var.project_id
  managed_zone = "lmgcp-mesh-internal"
  name         = "_Policy.mesh.lmgcp.internal."
  type         = "TXT"
  ttl          = 60
  rrdatas      = ["10.10.1.25"]
}

output "example_id" {
  value = "36_dns_split_horizon_microservice_mesh"
}

output "estimated_cost" {
  value = "$0.0010"
}

output "resource_count" {
  value = "4"
}
