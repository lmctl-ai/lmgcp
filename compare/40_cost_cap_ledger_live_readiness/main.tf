# OpenTofu (lmcloud) equivalent for lmgcp example: 40_cost_cap_ledger_live_readiness
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
  default = "cost-readiness-40"
}

provider "google" {
  project = var.project_id
  region  = var.region
  zone    = var.zone
}

resource "google_compute_network" "vpc_lmgcp_probe_vpc_1" {
  project                 = var.project_id
  name                    = "lmgcp-probe-vpc"
  auto_create_subnetworks = false
  mtu                     = 1460
}

resource "google_compute_subnetwork" "subnet_lmgcp_probe_sub_2" {
  project                  = var.project_id
  region                   = "us-central1"
  name                     = "lmgcp-probe-sub"
  ip_cidr_range            = "10.88.0.0/24"
  network                  = "default"
  private_ip_google_access = true
}

resource "google_storage_bucket" "bucket_lmgcp_probe_bkt_3" {
  project                     = var.project_id
  name                        = "lmgcp-probe-bkt"
  location                    = "US-CENTRAL1"
  storage_class               = "STANDARD"
  uniform_bucket_level_access = true
  force_destroy               = true

  labels = {
    lmgcp_test = var.run_id
    managed_by = "lmcloud"
  }
}

resource "google_pubsub_topic" "pubsub_topic_lmgcp_probe_topic_4" {
  project                    = var.project_id
  name                       = "lmgcp-probe-topic"
  message_retention_duration = "604800s"

  labels = {
    lmgcp_test = var.run_id
    managed_by = "lmcloud"
  }
}

resource "google_cloud_run_v2_service" "cloud_run_lmgcp_probe_run_5" {
  project  = var.project_id
  location = "us-central1"
  name     = "lmgcp-probe-run"
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

resource "google_compute_instance" "compute_instance_lmgcp_probe_vm_6" {
  project      = var.project_id
  zone         = var.zone
  name         = "lmgcp-probe-vm"
  machine_type = "e2-micro"

  boot_disk {
    initialize_params {
      image = "debian-cloud/debian-12"
      size  = 20
    }
  }

  network_interface {
    network = "default"
  }

  labels = {
    lmgcp_test = var.run_id
    managed_by = "lmcloud"
  }
}

output "example_id" {
  value = "40_cost_cap_ledger_live_readiness"
}

output "estimated_cost" {
  value = "$0.0018"
}

output "resource_count" {
  value = "6"
}
