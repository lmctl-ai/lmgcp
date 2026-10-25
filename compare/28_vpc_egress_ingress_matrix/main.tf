# OpenTofu (lmcloud) equivalent for lmgcp example: 28_vpc_egress_ingress_matrix
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

resource "google_compute_network" "vpc_lmgcp_zero_trust_vpc_1" {
  project                 = var.project_id
  name                    = "lmgcp-zero-trust-vpc"
  auto_create_subnetworks = false
  mtu                     = 1460
}

resource "google_compute_subnetwork" "subnet_lmgcp_zero_trust_sub_2" {
  project                  = var.project_id
  region                   = "us-central1"
  name                     = "lmgcp-zero-trust-sub"
  ip_cidr_range            = "10.88.0.0/24"
  network                  = "default"
  private_ip_google_access = true
}

resource "google_compute_firewall" "firewall_lmgcp_allow_internal_mtls_3" {
  project   = var.project_id
  name      = "lmgcp-allow-internal-mtls"
  network   = "default"
  direction = "INGRESS"
  priority  = 900

  allow {
    protocol = "tcp"
    ports    = ["8443"]
  }

  source_ranges = ["10.88.0.0/24"]
}

output "example_id" {
  value = "28_vpc_egress_ingress_matrix"
}

output "estimated_cost" {
  value = "$0.0004"
}

output "resource_count" {
  value = "3"
}
