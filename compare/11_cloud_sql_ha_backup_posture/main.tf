# OpenTofu (lmcloud) equivalent for lmgcp example: 11_cloud_sql_ha_backup_posture
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
  default = "sql-posture-11"
}

provider "google" {
  project = var.project_id
  region  = var.region
  zone    = var.zone
}

resource "google_sql_database_instance" "sql_instance_lmgcp_sql_demo_1" {
  project             = var.project_id
  name                = "lmgcp-sql-demo"
  region              = var.region
  database_version    = "POSTGRES_15"
  deletion_protection = false

  settings {
    tier = "db-f1-micro"
    user_labels = {
      lmgcp_test = var.run_id
      managed_by = "lmcloud"
    }
  }
}

output "example_id" {
  value = "11_cloud_sql_ha_backup_posture"
}

output "estimated_cost" {
  value = "$0.0027"
}

output "resource_count" {
  value = "1"
}
