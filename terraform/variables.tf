variable "project_id" {
  description = "The ID of the GCP project"
  type        = string
}

variable "region" {
  description = "The GCP region to deploy to"
  type        = string
  default     = "us-central1"
}

variable "log_level" {
  description = "Log level for applications (debug, info, warn, error)"
  type        = string
  default     = "info"
}

variable "retry_policy" {
  description = "Retry policy for Cloud Functions (RETRY_POLICY_RETRY or RETRY_POLICY_DO_NOT_RETRY)"
  type        = string
  default     = "RETRY_POLICY_RETRY"
}

variable "environment" {
  description = "The deployment environment (dev, test, prod)"
  type        = string
}

variable "domain_name" {
  description = "Custom domain for Firebase Hosting"
  type        = string
}

variable "base_url" {
  description = "Base URL for the application (used for OAuth redirects)"
  type        = string
}

