# OpenTofu (lmcloud) equivalent for lmgcp example: 10_iam_least_privilege_review
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

data "google_service_account" "sa_prod_api" {
  project    = var.project_id
  account_id = "sa-prod-api"
}

data "google_service_account" "sa_legacy_ci" {
  project    = var.project_id
  account_id = "sa-legacy-ci"
}

output "example_id" {
  value = "10_iam_least_privilege_review"
}

output "estimated_cost" {
  value = "$0.00"
}

output "resource_count" {
  value = "0"
}
