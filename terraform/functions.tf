resource "google_storage_bucket" "source_bucket" {
  name     = "${var.project_id}-functions-source"
  location = var.region
}

# ----------------- Hevy Webhook Handler -----------------
data "archive_file" "hevy_handler_zip" {
  type        = "zip"
  source_dir  = "../functions/hevy-handler"
  output_path = "/tmp/hevy-handler.zip"
  excludes    = ["node_modules", "build"]
}

resource "google_storage_bucket_object" "hevy_handler_zip" {
  name   = "hevy-handler-${data.archive_file.hevy_handler_zip.output_md5}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = data.archive_file.hevy_handler_zip.output_path
}

resource "google_cloudfunctions2_function" "hevy_handler" {
  name        = "hevy-webhook-handler"
  location    = var.region
  description = "Ingests Hevy webhooks"

  build_config {
    runtime     = "nodejs20"
    entry_point = "hevyWebhookHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.hevy_handler_zip.name
      }
    }
  }

  service_config {
    max_instance_count = 10
    available_memory   = "256Mi"
    timeout_seconds    = 60
    environment_variables = {
      HEVY_SIGNING_SECRET = "projects/${var.project_id}/secrets/hevy-api-key/versions/latest"
    }
  }
}

# ----------------- Keiser Poller -----------------
data "archive_file" "keiser_poller_zip" {
  type        = "zip"
  source_dir  = "../functions/keiser-poller"
  output_path = "/tmp/keiser-poller.zip"
}

resource "google_storage_bucket_object" "keiser_poller_zip" {
  name   = "keiser-poller-${data.archive_file.keiser_poller_zip.output_md5}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = data.archive_file.keiser_poller_zip.output_path
}

resource "google_cloudfunctions2_function" "keiser_poller" {
  name        = "keiser-poller"
  location    = var.region

  build_config {
    runtime     = "nodejs20"
    entry_point = "keiserPoller"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.keiser_poller_zip.name
      }
    }
  }

  service_config {
    available_memory   = "256Mi"
    timeout_seconds    = 60
  }
}

# ----------------- Enricher Service -----------------
data "archive_file" "enricher_zip" {
  type        = "zip"
  source_dir  = "../functions/enricher"
  output_path = "/tmp/enricher.zip"
}

resource "google_storage_bucket_object" "enricher_zip" {
  name   = "enricher-${data.archive_file.enricher_zip.output_md5}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = data.archive_file.enricher_zip.output_path
}

resource "google_cloudfunctions2_function" "enricher" {
  name        = "enricher"
  location    = var.region

  build_config {
    runtime     = "go121"
    entry_point = "EnrichActivity"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.enricher_zip.name
      }
    }
  }

  service_config {
    available_memory   = "512Mi"
    timeout_seconds    = 300
    environment_variables = {
        FITBIT_SECRET_ID = "projects/${var.project_id}/secrets/fitbit-client-secret/versions/latest"
    }
  }

  event_trigger {
    trigger_region = var.region
    event_type     = "google.cloud.pubsub.topic.v1.messagePublished"
    pubsub_topic   = google_pubsub_topic.raw_activity.id
    retry_policy   = "RETRY_POLICY_RETRY"
  }
}

# ----------------- Router Service -----------------
data "archive_file" "router_zip" {
  type        = "zip"
  source_dir  = "../functions/router"
  output_path = "/tmp/router.zip"
}

resource "google_storage_bucket_object" "router_zip" {
  name   = "router-${data.archive_file.router_zip.output_md5}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = data.archive_file.router_zip.output_path
}

resource "google_cloudfunctions2_function" "router" {
  name        = "router"
  location    = var.region

  build_config {
    runtime     = "go121"
    entry_point = "RouteActivity"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.router_zip.name
      }
    }
  }

  event_trigger {
    trigger_region = var.region
    event_type     = "google.cloud.pubsub.topic.v1.messagePublished"
    pubsub_topic   = google_pubsub_topic.enriched_activity.id
    retry_policy   = "RETRY_POLICY_RETRY"
  }
}

# ----------------- Strava Uploader -----------------
data "archive_file" "strava_uploader_zip" {
  type        = "zip"
  source_dir  = "../functions/strava-uploader"
  output_path = "/tmp/strava-uploader.zip"
}

resource "google_storage_bucket_object" "strava_uploader_zip" {
  name   = "strava-uploader-${data.archive_file.strava_uploader_zip.output_md5}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = data.archive_file.strava_uploader_zip.output_path
}

resource "google_cloudfunctions2_function" "strava_uploader" {
  name        = "strava-uploader"
  location    = var.region

  build_config {
    runtime     = "go121"
    entry_point = "UploadToStrava"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.strava_uploader_zip.name
      }
    }
  }

  event_trigger {
    trigger_region = var.region
    event_type     = "google.cloud.pubsub.topic.v1.messagePublished"
    pubsub_topic   = google_pubsub_topic.job_upload_strava.id
    retry_policy   = "RETRY_POLICY_EXTENDED" # longer retries for upload failures
  }
}
