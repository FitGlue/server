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

# Showcase assets bucket - stores generated images (AI banners, route thumbnails, muscle heatmaps)
resource "google_storage_bucket" "showcase_assets_bucket" {
  name     = "fitglue-showcase-assets"
  location = var.region

  uniform_bucket_level_access = true

  # No lifecycle rule - showcase assets are referenced by Showcase pages forever

  cors {
    origin          = ["*"]
    method          = ["GET", "HEAD"]
    response_header = ["Content-Type"]
    max_age_seconds = 3600
  }
}

# Grant public read access to showcase assets (images are public for Showcase viewing)
resource "google_storage_bucket_iam_member" "showcase_assets_public_read" {
  bucket = google_storage_bucket.showcase_assets_bucket.name
  role   = "roles/storage.objectViewer"
  member = "allUsers"
}

# Grant Cloud Functions service account access to write showcase assets
resource "google_storage_bucket_iam_member" "showcase_assets_functions_write" {
  bucket = google_storage_bucket.showcase_assets_bucket.name
  role   = "roles/storage.objectCreator"
  member = "serviceAccount:${var.project_id}@appspot.gserviceaccount.com"
}
