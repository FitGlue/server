resource "google_storage_bucket" "source_bucket" {
  name     = "${var.project_id}-functions-source"
  location = var.region
}

# Enricher uses pre-built zip with correct structure
resource "google_storage_bucket_object" "enricher_zip" {
  name   = "enricher-${filemd5("/tmp/fitglue-function-zips/enricher.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/enricher.zip"
}


# Router uses pre-built zip with correct structure
resource "google_storage_bucket_object" "router_zip" {
  name   = "router-${filemd5("/tmp/fitglue-function-zips/router.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/router.zip"
}


# Strava Uploader uses pre-built zip with correct structure
resource "google_storage_bucket_object" "strava_uploader_zip" {
  name   = "strava-uploader-${filemd5("/tmp/fitglue-function-zips/strava-uploader.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/strava-uploader.zip"
}


# -------------- TypeScript Source Archive --------------
data "archive_file" "typescript_source_zip" {
  type        = "zip"
  source_dir  = "../src/typescript"
  output_path = "/tmp/typescript-source.zip"
  excludes    = ["node_modules", "build"]
}

resource "google_storage_bucket_object" "typescript_source_zip" {
  name   = "typescript-source-${data.archive_file.typescript_source_zip.output_md5}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = data.archive_file.typescript_source_zip.output_path
}

# ----------------- Hevy Webhook Handler -----------------

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
        object = google_storage_bucket_object.typescript_source_zip.name
      }
    }
  }

  service_config {
    available_memory   = "256Mi"
    timeout_seconds    = 60
  }
}

resource "google_cloud_run_service_iam_member" "hevy_handler_invoker" {
  project  = google_cloudfunctions2_function.hevy_handler.project
  location = google_cloudfunctions2_function.hevy_handler.location
  service  = google_cloudfunctions2_function.hevy_handler.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# ----------------- Keiser Poller -----------------

resource "google_cloudfunctions2_function" "keiser_poller" {
  name        = "keiser-poller"
  location    = var.region

  build_config {
    runtime     = "nodejs20"
    entry_point = "keiserPoller"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.typescript_source_zip.name
      }
    }
  }

  service_config {
    available_memory   = "256Mi"
    timeout_seconds    = 60
  }
}

# ----------------- Enricher Service -----------------
resource "google_cloudfunctions2_function" "enricher" {
  name        = "enricher"
  location    = var.region

  build_config {
    runtime     = "go125"
    entry_point = "EnrichActivity"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.enricher_zip.name
      }
    }
    environment_variables = {}
  }

  service_config {
    available_memory   = "512Mi"
    timeout_seconds    = 300
  }

  event_trigger {
    trigger_region = var.region
    event_type     = "google.cloud.pubsub.topic.v1.messagePublished"
    pubsub_topic   = google_pubsub_topic.raw_activity.id
    retry_policy   = "RETRY_POLICY_RETRY"
  }
}




# ----------------- Router Service -----------------
resource "google_cloudfunctions2_function" "router" {
  name        = "router"
  location    = var.region

  build_config {
    runtime     = "go125"
    entry_point = "RouteActivity"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.router_zip.name
      }
    }
    environment_variables = {}
  }

  service_config {
    available_memory   = "512Mi"
    timeout_seconds    = 300
  }

  event_trigger {
    trigger_region = var.region
    event_type     = "google.cloud.pubsub.topic.v1.messagePublished"
    pubsub_topic   = google_pubsub_topic.enriched_activity.id
    retry_policy   = "RETRY_POLICY_RETRY"
  }
}

# ----------------- Strava Uploader -----------------
resource "google_cloudfunctions2_function" "strava_uploader" {
  name        = "strava-uploader"
  location    = var.region

  build_config {
    runtime     = "go125"
    entry_point = "UploadToStrava"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.strava_uploader_zip.name
      }
    }
    environment_variables = {}
  }

  service_config {
    available_memory   = "512Mi"
    timeout_seconds    = 300
  }

  event_trigger {
    trigger_region = var.region
    event_type     = "google.cloud.pubsub.topic.v1.messagePublished"
    pubsub_topic   = google_pubsub_topic.job_upload_strava.id
    retry_policy   = "RETRY_POLICY_RETRY" # longer retries for upload failures
  }
}
