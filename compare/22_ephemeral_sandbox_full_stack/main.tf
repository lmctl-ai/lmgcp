# OpenTofu (lmcloud) equivalent for lmgcp example: 22_ephemeral_sandbox_full_stack
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
  default = "fullstack-sandbox-22"
}

provider "google" {
  project = var.project_id
  region  = var.region
  zone    = var.zone
}

resource "google_compute_network" "vpc_lmgcp_sbx_vpc_1" {
  project                 = var.project_id
  name                    = "lmgcp-sbx-vpc"
  auto_create_subnetworks = false
  mtu                     = 1460
}

resource "google_compute_subnetwork" "subnet_lmgcp_sbx_subnet_2" {
  project                  = var.project_id
  region                   = "us-central1"
  name                     = "lmgcp-sbx-subnet"
  ip_cidr_range            = "10.88.0.0/24"
  network                  = "default"
  private_ip_google_access = true
}

resource "google_compute_instance" "compute_instance_lmgcp_sbx_vm_3" {
  project      = var.project_id
  zone         = var.zone
  name         = "lmgcp-sbx-vm"
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

resource "google_storage_bucket" "bucket_lmgcp_sbx_bucket_4" {
  project                     = var.project_id
  name                        = "lmgcp-sbx-bucket"
  location                    = "US-CENTRAL1"
  storage_class               = "STANDARD"
  uniform_bucket_level_access = true
  force_destroy               = true

  labels = {
    lmgcp_test = var.run_id
    managed_by = "lmcloud"
  }
}

resource "google_pubsub_topic" "pubsub_topic_lmgcp_sbx_events_5" {
  project                    = var.project_id
  name                       = "lmgcp-sbx-events"
  message_retention_duration = "604800s"

  labels = {
    lmgcp_test = var.run_id
    managed_by = "lmcloud"
  }
}

resource "google_pubsub_subscription" "pubsub_sub_lmgcp_sbx_sub_6" {
  project              = var.project_id
  name                 = "lmgcp-sbx-sub"
  topic                = "lmgcp-orders-stream"
  ack_deadline_seconds = 30

  labels = {
    lmgcp_test = var.run_id
    managed_by = "lmcloud"
  }
}

resource "google_cloud_run_v2_service" "cloud_run_lmgcp_sbx_worker_7" {
  project  = var.project_id
  location = "us-central1"
  name     = "lmgcp-sbx-worker"
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
  value = "22_ephemeral_sandbox_full_stack"
}

output "estimated_cost" {
  value = "$0.0019"
}

output "resource_count" {
  value = "7"
}
