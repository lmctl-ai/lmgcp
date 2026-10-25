# OpenTofu (lmcloud) equivalent for lmgcp example: 35_kms_multi_keyring_envelope_audit
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

resource "google_kms_crypto_key" "kms_key_lmgcp_envelope_dek_01_1" {
  name            = "lmgcp-envelope-dek-01"
  key_ring        = "projects/${var.project_id}/locations/${var.region}/keyRings/prod-ring"
  purpose         = "ENCRYPT_DECRYPT"
  rotation_period = "2592000s"

  labels = {
    lmgcp_test = var.run_id
    managed_by = "lmcloud"
  }
}

resource "google_kms_crypto_key" "kms_key_lmgcp_artifact_sign_01_2" {
  name            = "lmgcp-artifact-sign-01"
  key_ring        = "projects/${var.project_id}/locations/${var.region}/keyRings/prod-ring"
  purpose         = "ENCRYPT_DECRYPT"
  rotation_period = "2592000s"

  labels = {
    lmgcp_test = var.run_id
    managed_by = "lmcloud"
  }
}

output "example_id" {
  value = "35_kms_multi_keyring_envelope_audit"
}

output "estimated_cost" {
  value = "$0.0020"
}

output "resource_count" {
  value = "2"
}
