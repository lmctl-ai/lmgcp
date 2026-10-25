# OpenTofu (lmcloud) equivalent for lmgcp example: 17_firestore_document_workflow
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
  default = "firestore-crud-17"
}

provider "google" {
  project = var.project_id
  region  = var.region
  zone    = var.zone
}

resource "google_firestore_document" "firestore_doc_lmgcp_sess_101_1" {
  project     = var.project_id
  collection  = "lmgcp_sessions"
  document_id = "lmgcp-sess-101"
  fields      = "{\"lmgcp_test\":{\"stringValue\":\"${var.run_id}\"}}"
}

resource "google_firestore_document" "firestore_doc_lmgcp_sess_102_2" {
  project     = var.project_id
  collection  = "lmgcp_sessions"
  document_id = "lmgcp-sess-102"
  fields      = "{\"lmgcp_test\":{\"stringValue\":\"${var.run_id}\"}}"
}

output "example_id" {
  value = "17_firestore_document_workflow"
}

output "estimated_cost" {
  value = "$0.0002"
}

output "resource_count" {
  value = "2"
}
