# OpenTofu (lmcloud) equivalent for lmgcp example: 05_pubsub_dead_letter_pipeline
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

resource "google_pubsub_topic" "pubsub_topic_lmgcp_orders_dlq_1" {
  project                    = var.project_id
  name                       = "lmgcp-orders-dlq"
  message_retention_duration = "604800s"

  labels = {
    lmgcp_test = var.run_id
    managed_by = "lmcloud"
  }
}

resource "google_pubsub_topic" "pubsub_topic_lmgcp_orders_stream_2" {
  project                    = var.project_id
  name                       = "lmgcp-orders-stream"
  message_retention_duration = "604800s"

  labels = {
    lmgcp_test = var.run_id
    managed_by = "lmcloud"
  }
}

resource "google_pubsub_subscription" "pubsub_sub_lmgcp_orders_worker_sub_3" {
  project              = var.project_id
  name                 = "lmgcp-orders-worker-sub"
  topic                = "lmgcp-orders-stream"
  ack_deadline_seconds = 30

  labels = {
    lmgcp_test = var.run_id
    managed_by = "lmcloud"
  }
}

resource "terraform_data" "pubsub_msg_lmgcp_orders_stream_4" {
  input = {
    kind       = "pubsub_msg"
    subtype    = "publish"
    target     = "lmgcp-orders-stream"
    lmgcp_test = var.run_id
  }
}

resource "terraform_data" "pubsub_msg_lmgcp_orders_stream_5" {
  input = {
    kind       = "pubsub_msg"
    subtype    = "publish"
    target     = "lmgcp-orders-stream"
    lmgcp_test = var.run_id
  }
}

resource "terraform_data" "pubsub_msg_lmgcp_orders_stream_6" {
  input = {
    kind       = "pubsub_msg"
    subtype    = "publish"
    target     = "lmgcp-orders-stream"
    lmgcp_test = var.run_id
  }
}

output "example_id" {
  value = "05_pubsub_dead_letter_pipeline"
}

output "estimated_cost" {
  value = "$0.0006"
}

output "resource_count" {
  value = "6"
}
