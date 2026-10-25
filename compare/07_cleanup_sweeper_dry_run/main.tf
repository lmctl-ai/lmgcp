# OpenTofu (lmcloud) equivalent for lmgcp example: 07_cleanup_sweeper_dry_run
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
  default = "sweeper-demo-07"
}

provider "google" {
  project = var.project_id
  region  = var.region
  zone    = var.zone
}

resource "google_compute_instance" "compute_instance_lmgcp_stale_vm_48h_1" {
  project      = var.project_id
  zone         = var.zone
  name         = "lmgcp-stale-vm-48h"
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

resource "google_storage_bucket" "bucket_lmgcp_stale_bkt_36h_2" {
  project                     = var.project_id
  name                        = "lmgcp-stale-bkt-36h"
  location                    = "US-CENTRAL1"
  storage_class               = "STANDARD"
  uniform_bucket_level_access = true
  force_destroy               = true

  labels = {
    lmgcp_test = var.run_id
    managed_by = "lmcloud"
  }
}

resource "google_pubsub_topic" "pubsub_topic_lmgcp_recent_topic_8h_3" {
  project                    = var.project_id
  name                       = "lmgcp-recent-topic-8h"
  message_retention_duration = "604800s"

  labels = {
    lmgcp_test = var.run_id
    managed_by = "lmcloud"
  }
}

resource "google_cloud_run_v2_service" "cloud_run_lmgcp_fresh_svc_1h_4" {
  project  = var.project_id
  location = "us-central1"
  name     = "lmgcp-fresh-svc-1h"
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

output "example_id" {
  value = "07_cleanup_sweeper_dry_run"
}

output "estimated_cost" {
  value = "$0.0015"
}

output "resource_count" {
  value = "4"
}
