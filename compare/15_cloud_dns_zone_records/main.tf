# OpenTofu (lmcloud) equivalent for lmgcp example: 15_cloud_dns_zone_records
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
  default = "dns-audit-15"
}

provider "google" {
  project = var.project_id
  region  = var.region
  zone    = var.zone
}

resource "google_dns_managed_zone" "dns_zone_lmgcp_dns_zone_1" {
  project    = var.project_id
  name       = "lmgcp-dns-zone"
  dns_name   = "sandbox.gcp-masked.example."
  visibility = "public"

  labels = {
    lmgcp_test = var.run_id
    managed_by = "lmcloud"
  }
}

resource "google_dns_record_set" "dns_record_lmgcp_dns_zone_api_sandbox_gcp_masked_example_2" {
  project      = var.project_id
  managed_zone = "lmgcp-dns-zone"
  name         = "api.sandbox.gcp-masked.example."
  type         = "A"
  ttl          = 60
  rrdatas      = ["10.10.1.25"]
}

resource "google_dns_record_set" "dns_record_lmgcp_dns_zone_api_sandbox_gcp_masked_example_3" {
  project      = var.project_id
  managed_zone = "lmgcp-dns-zone"
  name         = "api.sandbox.gcp-masked.example."
  type         = "TXT"
  ttl          = 60
  rrdatas      = ["10.10.1.25"]
}

output "example_id" {
  value = "15_cloud_dns_zone_records"
}

output "estimated_cost" {
  value = "$0.0009"
}

output "resource_count" {
  value = "3"
}
