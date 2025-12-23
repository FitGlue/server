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
