# OpenTofu (lmcloud) equivalent for lmgcp example: 34_firestore_multi_collection_state_store
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

resource "google_firestore_document" "firestore_doc_wf_101_1" {
  project     = var.project_id
  collection  = "lmgcp_workflows"
  document_id = "wf-101"
  fields      = "{\"lmgcp_test\":{\"stringValue\":\"${var.run_id}\"}}"
}

resource "google_firestore_document" "firestore_doc_wf_102_2" {
  project     = var.project_id
  collection  = "lmgcp_workflows"
  document_id = "wf-102"
  fields      = "{\"lmgcp_test\":{\"stringValue\":\"${var.run_id}\"}}"
}

resource "google_firestore_document" "firestore_doc_lock_shard_0_3" {
  project     = var.project_id
  collection  = "lmgcp_locks"
  document_id = "lock-shard-0"
  fields      = "{\"lmgcp_test\":{\"stringValue\":\"${var.run_id}\"}}"
}

output "example_id" {
  value = "34_firestore_multi_collection_state_store"
}

output "estimated_cost" {
  value = "$0.0003"
}

output "resource_count" {
  value = "3"
}
