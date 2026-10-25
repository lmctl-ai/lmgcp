# OpenTofu (lmcloud) equivalent for lmgcp example: 24_serverless_event_driven_mesh
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
  default = "serverless-mesh-24"
}

provider "google" {
  project = var.project_id
  region  = var.region
  zone    = var.zone
}

resource "google_storage_bucket" "bucket_lmgcp_mesh_ingest_1" {
  project                     = var.project_id
  name                        = "lmgcp-mesh-ingest"
  location                    = "US-CENTRAL1"
  storage_class               = "STANDARD"
  uniform_bucket_level_access = true
  force_destroy               = true

  labels = {
    lmgcp_test = var.run_id
    managed_by = "lmcloud"
  }
}

resource "google_storage_bucket_object" "storage_object_lmgcp_mesh_ingest_events_batch_01_json_2" {
  bucket       = "lmgcp-mesh-ingest"
  name         = "events/batch-01.json"
  content_type = "application/json"
  content      = "lmgcp_test=${var.run_id}"
}

resource "google_pubsub_topic" "pubsub_topic_lmgcp_mesh_topic_3" {
  project                    = var.project_id
  name                       = "lmgcp-mesh-topic"
  message_retention_duration = "604800s"

  labels = {
    lmgcp_test = var.run_id
    managed_by = "lmcloud"
  }
}

resource "google_pubsub_subscription" "pubsub_sub_lmgcp_mesh_sub_4" {
  project              = var.project_id
  name                 = "lmgcp-mesh-sub"
  topic                = "lmgcp-orders-stream"
  ack_deadline_seconds = 30

  labels = {
    lmgcp_test = var.run_id
    managed_by = "lmcloud"
  }
}

resource "terraform_data" "pubsub_msg_lmgcp_mesh_topic_5" {
  input = {
    kind       = "pubsub_msg"
    subtype    = "publish"
    target     = "lmgcp-mesh-topic"
    lmgcp_test = var.run_id
  }
}

resource "google_cloud_run_v2_service" "cloud_run_lmgcp_mesh_processor_6" {
  project  = var.project_id
  location = "us-central1"
  name     = "lmgcp-mesh-processor"
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

resource "google_bigquery_dataset" "bq_dataset_lmgcp_mesh_dw_7" {
  project                     = var.project_id
  dataset_id                  = "lmgcp_mesh_dw"
  location                    = "US"
  default_table_expiration_ms = 2592000000
  delete_contents_on_destroy  = true

  labels = {
    lmgcp_test = var.run_id
    managed_by = "lmcloud"
  }
}

resource "google_bigquery_table" "bq_table_lmgcp_mesh_dw_lmgcp_ingested_events_8" {
  project             = var.project_id
  dataset_id          = "lmgcp_mesh_dw"
  table_id            = "lmgcp_ingested_events"
  deletion_protection = false

  time_partitioning {
    type  = "DAY"
    field = "ingest_ts"
  }

  labels = {
    lmgcp_test = var.run_id
    managed_by = "lmcloud"
  }
}

output "example_id" {
  value = "24_serverless_event_driven_mesh"
}

output "estimated_cost" {
  value = "$0.0012"
}

output "resource_count" {
  value = "8"
}
