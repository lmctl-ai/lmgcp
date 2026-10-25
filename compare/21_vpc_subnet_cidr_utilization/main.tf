# OpenTofu (lmcloud) equivalent for lmgcp example: 21_vpc_subnet_cidr_utilization
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
  default = "vpc-subnet-21"
}

provider "google" {
  project = var.project_id
  region  = var.region
  zone    = var.zone
}

resource "google_compute_network" "vpc_lmgcp_vpc_sandbox_1" {
  project                 = var.project_id
  name                    = "lmgcp-vpc-sandbox"
  auto_create_subnetworks = false
  mtu                     = 1460
}

resource "google_compute_subnetwork" "subnet_lmgcp_subnet_sandbox_2" {
  project                  = var.project_id
  region                   = "us-central1"
  name                     = "lmgcp-subnet-sandbox"
  ip_cidr_range            = "10.88.0.0/24"
  network                  = "default"
  private_ip_google_access = true
}

output "example_id" {
  value = "21_vpc_subnet_cidr_utilization"
}

output "estimated_cost" {
  value = "$0.0003"
}

output "resource_count" {
  value = "2"
}
