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

# Version config bucket - stores unified FitGlue version across web/server repos
resource "google_storage_bucket" "version_config_bucket" {
  name     = "${var.project_id}-version-config"
  location = var.region

  uniform_bucket_level_access = true

  versioning {
    enabled = true
  }
}

# Grant CI service account access to read/write version config
resource "google_storage_bucket_iam_member" "version_config_ci_access" {
  bucket = google_storage_bucket.version_config_bucket.name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:circleci-deployer@${var.project_id}.iam.gserviceaccount.com"
}

# Grant Web CI service account read access to version config
resource "google_storage_bucket_iam_member" "version_config_web_ci_access" {
  bucket = google_storage_bucket.version_config_bucket.name
  role   = "roles/storage.objectViewer"
  member = "serviceAccount:circleci-web-deployer@${var.project_id}.iam.gserviceaccount.com"
}
