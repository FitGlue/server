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
  excludes    = ["**/node_modules", "**/dist", "**/build", "**/coverage", "**/.DS_Store", "**/mcp-server"]
}

resource "google_storage_bucket_object" "typescript_source_zip" {
  name   = "typescript-source-${data.archive_file.typescript_source_zip.output_md5}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = data.archive_file.typescript_source_zip.output_path
}


# ----------------- Enricher Service -----------------
resource "google_cloudfunctions2_function" "enricher" {
  name     = "enricher"
  location = var.region

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
    available_memory = "512Mi"
    timeout_seconds  = 300
    environment_variables = {
      GOOGLE_CLOUD_PROJECT = var.project_id
      GCS_ARTIFACT_BUCKET  = "${var.project_id}-artifacts"
      ENABLE_PUBLISH       = "true"
      LOG_LEVEL            = var.log_level
    }
    service_account_email = google_service_account.cloud_function_sa.email
  }

  event_trigger {
    trigger_region = var.region
    event_type     = "google.cloud.pubsub.topic.v1.messagePublished"
    pubsub_topic   = google_pubsub_topic.raw_activity.id
    retry_policy   = var.retry_policy
  }
}

resource "google_cloud_run_service_iam_member" "enricher_invoker" {
  project  = google_cloudfunctions2_function.enricher.project
  location = google_cloudfunctions2_function.enricher.location
  service  = google_cloudfunctions2_function.enricher.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.cloud_function_sa.email}"
}

# ----------------- Enricher Lag Retry Handler -----------------
# This is a separate HTTP-triggered function for the lag topic push subscription.
# Unlike CloudEvent handlers, HTTP handlers properly return HTTP 500 on errors,
# which triggers Pub/Sub retry with backoff.
resource "google_cloudfunctions2_function" "enricher_lag" {
  name     = "enricher-lag"
  location = var.region

  build_config {
    runtime     = "go125"
    entry_point = "EnrichActivityHTTP"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.enricher_zip.name
      }
    }
    environment_variables = {}
  }

  service_config {
    available_memory = "512Mi"
    timeout_seconds  = 300
    environment_variables = {
      GOOGLE_CLOUD_PROJECT = var.project_id
      GCS_ARTIFACT_BUCKET  = "${var.project_id}-artifacts"
      ENABLE_PUBLISH       = "true"
      LOG_LEVEL            = var.log_level
    }
    service_account_email = google_service_account.cloud_function_sa.email
  }

  # No event_trigger - this is an HTTP-triggered function
}

resource "google_cloud_run_service_iam_member" "enricher_lag_invoker" {
  project  = google_cloudfunctions2_function.enricher_lag.project
  location = google_cloudfunctions2_function.enricher_lag.location
  service  = google_cloudfunctions2_function.enricher_lag.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.cloud_function_sa.email}"
}



# ----------------- Router Service -----------------
resource "google_cloudfunctions2_function" "router" {
  name     = "router"
  location = var.region

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
    available_memory = "512Mi"
    timeout_seconds  = 300
    environment_variables = {
      GOOGLE_CLOUD_PROJECT = var.project_id
      GCS_ARTIFACT_BUCKET  = "${var.project_id}-artifacts"
      GCS_ARTIFACT_BUCKET  = "${var.project_id}-artifacts"
      ENABLE_PUBLISH       = "true"
      LOG_LEVEL            = var.log_level
    }
    service_account_email = google_service_account.cloud_function_sa.email
  }

  event_trigger {
    trigger_region = var.region
    event_type     = "google.cloud.pubsub.topic.v1.messagePublished"
    pubsub_topic   = google_pubsub_topic.enriched_activity.id
    retry_policy   = var.retry_policy
  }
}

# ----------------- Strava Uploader -----------------
resource "google_cloudfunctions2_function" "strava_uploader" {
  name     = "strava-uploader"
  location = var.region

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
    available_memory = "512Mi"
    timeout_seconds  = 300
    environment_variables = {
      GOOGLE_CLOUD_PROJECT = var.project_id
      GCS_ARTIFACT_BUCKET  = "${var.project_id}-artifacts"
      LOG_LEVEL            = var.log_level
    }
    service_account_email = google_service_account.cloud_function_sa.email
  }

  event_trigger {
    trigger_region = var.region
    event_type     = "google.cloud.pubsub.topic.v1.messagePublished"
    pubsub_topic   = google_pubsub_topic.job_upload_strava.id
    retry_policy   = var.retry_policy # longer retries for upload failures
  }
}

# ----------------- Mock Uploader (Dev Only) -----------------
resource "google_storage_bucket_object" "mock_uploader_zip" {
  count  = var.environment == "dev" ? 1 : 0
  name   = "mock-uploader-${filemd5("/tmp/fitglue-function-zips/mock-uploader.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/mock-uploader.zip"
}

resource "google_cloudfunctions2_function" "mock_uploader" {
  count    = var.environment == "dev" ? 1 : 0
  name     = "mock-uploader"
  location = var.region

  build_config {
    runtime     = "go125"
    entry_point = "MockUpload"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.mock_uploader_zip[0].name
      }
    }
    environment_variables = {}
  }

  service_config {
    available_memory = "256Mi"
    timeout_seconds  = 60
    environment_variables = {
      GOOGLE_CLOUD_PROJECT = var.project_id
      LOG_LEVEL            = var.log_level
    }
    service_account_email = google_service_account.cloud_function_sa.email
  }

  event_trigger {
    trigger_region = var.region
    event_type     = "google.cloud.pubsub.topic.v1.messagePublished"
    pubsub_topic   = google_pubsub_topic.job_upload_mock[0].id
    retry_policy   = var.retry_policy
  }
}

# ----------------- Mock Source Handler (Dev Only) -----------------
resource "google_cloudfunctions2_function" "mock_source_handler" {
  // Needs to be deployed to test and prod otherwise firebase.json will fail with "can't find function"
  // But we do *not* allow this to be run from the web on test/prod by omiting the mock_source_handler_invoker
  name        = "mock-source-handler"
  location    = var.region
  description = "Mocks source events for testing"

  build_config {
    runtime     = "nodejs20"
    entry_point = "mockSourceHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.typescript_source_zip.name
      }
    }
    environment_variables = {}
  }

  service_config {
    available_memory = "256Mi"
    timeout_seconds  = 60
    environment_variables = {
      GOOGLE_CLOUD_PROJECT = var.project_id
      LOG_LEVEL            = var.log_level
    }
    service_account_email = google_service_account.cloud_function_sa.email
  }
}

resource "google_cloud_run_service_iam_member" "mock_source_handler_invoker" {
  count    = var.environment == "dev" ? 1 : 0
  project  = google_cloudfunctions2_function.mock_source_handler.project
  location = google_cloudfunctions2_function.mock_source_handler.location
  service  = google_cloudfunctions2_function.mock_source_handler.name
  role     = "roles/run.invoker"
  member   = "allUsers"
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
    environment_variables = {}
  }

  service_config {
    available_memory = "256Mi"
    timeout_seconds  = 60
    environment_variables = {
      LOG_LEVEL = var.log_level
    }
    service_account_email = google_service_account.cloud_function_sa.email
  }
}

resource "google_cloud_run_service_iam_member" "hevy_handler_invoker" {
  project  = google_cloudfunctions2_function.hevy_handler.project
  location = google_cloudfunctions2_function.hevy_handler.location
  service  = google_cloudfunctions2_function.hevy_handler.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# ----------------- Fitbit Handler -----------------
resource "google_cloudfunctions2_function" "fitbit_handler" {
  name        = "fitbit-handler"
  location    = var.region
  description = "Ingests Fitbit webhooks and data"

  build_config {
    runtime     = "nodejs20"
    entry_point = "fitbitWebhookHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.typescript_source_zip.name
      }
    }
    environment_variables = {}
  }

  service_config {
    available_memory = "512Mi"
    timeout_seconds  = 300
    environment_variables = {
      LOG_LEVEL            = var.log_level
      GOOGLE_CLOUD_PROJECT = var.project_id
    }

    secret_environment_variables {
      key        = "FITBIT_VERIFICATION_CODE"
      project_id = var.project_id
      secret     = google_secret_manager_secret.fitbit_verification_code.secret_id
      version    = "latest"
    }

    secret_environment_variables {
      key        = "FITBIT_CLIENT_SECRET"
      project_id = var.project_id
      secret     = google_secret_manager_secret.fitbit_client_secret.secret_id
      version    = "latest"
    }

    # Secrets needed for Fetch logic (formerly in ingest)
    secret_environment_variables {
      key        = "FITBIT_CLIENT_ID"
      project_id = var.project_id
      secret     = google_secret_manager_secret.fitbit_client_id.secret_id
      version    = "latest"
    }

    service_account_email = google_service_account.cloud_function_sa.email
  }
}

resource "google_cloud_run_service_iam_member" "fitbit_handler_invoker" {
  project  = google_cloudfunctions2_function.fitbit_handler.project
  location = google_cloudfunctions2_function.fitbit_handler.location
  service  = google_cloudfunctions2_function.fitbit_handler.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# ----------------- Auth Hooks -----------------
# Triggered by Eventarc (Firebase Auth User Created)
# NOTE: Using Gen 1 function because Gen 2 (Eventarc) does not natively support async Firebase Auth triggers yet
resource "google_cloudfunctions_function" "auth_on_create" {
  name        = "auth-on-create"
  description = "Triggered when a new user is created in Firebase Auth"
  runtime     = "nodejs20"

  available_memory_mb   = 256
  source_archive_bucket = google_storage_bucket.source_bucket.name
  source_archive_object = google_storage_bucket_object.typescript_source_zip.name
  entry_point           = "authOnCreate"

  event_trigger {
    event_type = "providers/firebase.auth/eventTypes/user.create"
    resource   = "projects/${var.project_id}"
    failure_policy {
      retry = var.retry_policy == "RETRY_POLICY_RETRY"
    }
  }

  environment_variables = {
    LOG_LEVEL            = var.log_level
    GOOGLE_CLOUD_PROJECT = var.project_id
  }

  service_account_email = google_service_account.cloud_function_sa.email
}

# ----------------- Waitlist Handler -----------------
resource "google_cloudfunctions2_function" "waitlist_handler" {
  name        = "waitlist-handler"
  location    = var.region
  description = "Public waitlist submission handler"

  build_config {
    runtime     = "nodejs20"
    entry_point = "waitlistHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.typescript_source_zip.name
      }
    }
    environment_variables = {}
  }

  service_config {
    available_memory = "256Mi"
    timeout_seconds  = 60
    environment_variables = {
      LOG_LEVEL            = var.log_level
      GOOGLE_CLOUD_PROJECT = var.project_id
    }
    service_account_email = google_service_account.cloud_function_sa.email
  }
}

# Public access for waitlist
resource "google_cloud_run_service_iam_member" "waitlist_handler_invoker" {
  project  = google_cloudfunctions2_function.waitlist_handler.project
  location = google_cloudfunctions2_function.waitlist_handler.location
  service  = google_cloudfunctions2_function.waitlist_handler.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# ----------------- Inputs Handler -----------------
resource "google_cloudfunctions2_function" "inputs_handler" {
  name        = "inputs-handler"
  location    = var.region
  description = "Handles pending user input resolutions"

  build_config {
    runtime     = "nodejs20"
    entry_point = "inputsHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.typescript_source_zip.name
      }
    }
    environment_variables = {}
  }

  service_config {
    available_memory = "256Mi"
    timeout_seconds  = 60
    environment_variables = {
      LOG_LEVEL            = var.log_level
      GOOGLE_CLOUD_PROJECT = var.project_id
      PUBSUB_TOPIC         = google_pubsub_topic.raw_activity.name
    }
    service_account_email = google_service_account.cloud_function_sa.email
  }
}

resource "google_cloud_run_service_iam_member" "inputs_handler_invoker" {
  project  = google_cloudfunctions2_function.inputs_handler.project
  location = google_cloudfunctions2_function.inputs_handler.location
  service  = google_cloudfunctions2_function.inputs_handler.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# ----------------- Activities Handler -----------------
resource "google_cloudfunctions2_function" "activities_handler" {
  name        = "activities-handler"
  location    = var.region
  description = "Handles activities listing and statistics"

  build_config {
    runtime     = "nodejs20"
    entry_point = "activitiesHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.typescript_source_zip.name
      }
    }
    environment_variables = {}
  }

  service_config {
    available_memory = "256Mi"
    timeout_seconds  = 60
    environment_variables = {
      LOG_LEVEL            = var.log_level
      GOOGLE_CLOUD_PROJECT = var.project_id
    }
    service_account_email = google_service_account.cloud_function_sa.email
  }
}

resource "google_cloud_run_service_iam_member" "activities_handler_invoker" {
  project  = google_cloudfunctions2_function.activities_handler.project
  location = google_cloudfunctions2_function.activities_handler.location
  service  = google_cloudfunctions2_function.activities_handler.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}
