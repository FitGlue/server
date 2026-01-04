resource "google_storage_bucket" "artifacts_bucket" {
  name     = "${var.project_id}-artifacts"
  location = var.region

  uniform_bucket_level_access = true

  lifecycle_rule {
    condition {
      age = 7
    }
    action {
      type = "Delete"
    }
  }
}
