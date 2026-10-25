# OpenTofu (lmcloud) equivalent for lmgcp example: 14_kms_key_rotation_posture
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
  default = "kms-posture-14"
}

provider "google" {
  project = var.project_id
  region  = var.region
  zone    = var.zone
}

resource "google_kms_crypto_key" "kms_key_lmgcp_ephemeral_cmek_1" {
  name            = "lmgcp-ephemeral-cmek"
  key_ring        = "projects/${var.project_id}/locations/${var.region}/keyRings/prod-ring"
  purpose         = "ENCRYPT_DECRYPT"
  rotation_period = "2592000s"

  labels = {
    lmgcp_test = var.run_id
    managed_by = "lmcloud"
  }
}

output "example_id" {
  value = "14_kms_key_rotation_posture"
}

output "estimated_cost" {
  value = "$0.0010"
}

output "resource_count" {
  value = "1"
}
