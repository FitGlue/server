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


# -------------- TypeScript Per-Handler ZIPs --------------
# Each handler gets its own ZIP with filemd5() for change detection
# This means only handlers that changed will trigger Cloud Build

resource "google_storage_bucket_object" "activities_handler_zip" {
  name   = "activities-handler-${filemd5("/tmp/fitglue-function-zips/activities-handler.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/activities-handler.zip"
}

resource "google_storage_bucket_object" "admin_handler_zip" {
  name   = "admin-handler-${filemd5("/tmp/fitglue-function-zips/admin-handler.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/admin-handler.zip"
}

resource "google_storage_bucket_object" "auth_hooks_zip" {
  name   = "auth-hooks-${filemd5("/tmp/fitglue-function-zips/auth-hooks.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/auth-hooks.zip"
}

resource "google_storage_bucket_object" "billing_handler_zip" {
  name   = "billing-handler-${filemd5("/tmp/fitglue-function-zips/billing-handler.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/billing-handler.zip"
}

resource "google_storage_bucket_object" "fitbit_handler_zip" {
  name   = "fitbit-handler-${filemd5("/tmp/fitglue-function-zips/fitbit-handler.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/fitbit-handler.zip"
}

resource "google_storage_bucket_object" "fitbit_oauth_handler_zip" {
  name   = "fitbit-oauth-handler-${filemd5("/tmp/fitglue-function-zips/fitbit-oauth-handler.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/fitbit-oauth-handler.zip"
}

resource "google_storage_bucket_object" "hevy_handler_zip" {
  name   = "hevy-handler-${filemd5("/tmp/fitglue-function-zips/hevy-handler.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/hevy-handler.zip"
}

resource "google_storage_bucket_object" "inputs_handler_zip" {
  name   = "inputs-handler-${filemd5("/tmp/fitglue-function-zips/inputs-handler.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/inputs-handler.zip"
}

resource "google_storage_bucket_object" "integration_request_handler_zip" {
  name   = "integration-request-handler-${filemd5("/tmp/fitglue-function-zips/integration-request-handler.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/integration-request-handler.zip"
}

resource "google_storage_bucket_object" "mobile_sync_handler_zip" {
  name   = "mobile-sync-handler-${filemd5("/tmp/fitglue-function-zips/mobile-sync-handler.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/mobile-sync-handler.zip"
}

resource "google_storage_bucket_object" "mock_source_handler_zip" {
  name   = "mock-source-handler-${filemd5("/tmp/fitglue-function-zips/mock-source-handler.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/mock-source-handler.zip"
}

resource "google_storage_bucket_object" "registry_handler_zip" {
  name   = "registry-handler-${filemd5("/tmp/fitglue-function-zips/registry-handler.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/registry-handler.zip"
}

resource "google_storage_bucket_object" "repost_handler_zip" {
  name   = "repost-handler-${filemd5("/tmp/fitglue-function-zips/repost-handler.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/repost-handler.zip"
}

resource "google_storage_bucket_object" "showcase_handler_zip" {
  name   = "showcase-handler-${filemd5("/tmp/fitglue-function-zips/showcase-handler.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/showcase-handler.zip"
}

resource "google_storage_bucket_object" "strava_oauth_handler_zip" {
  name   = "strava-oauth-handler-${filemd5("/tmp/fitglue-function-zips/strava-oauth-handler.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/strava-oauth-handler.zip"
}

resource "google_storage_bucket_object" "user_integrations_handler_zip" {
  name   = "user-integrations-handler-${filemd5("/tmp/fitglue-function-zips/user-integrations-handler.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/user-integrations-handler.zip"
}

resource "google_storage_bucket_object" "user_pipelines_handler_zip" {
  name   = "user-pipelines-handler-${filemd5("/tmp/fitglue-function-zips/user-pipelines-handler.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/user-pipelines-handler.zip"
}

resource "google_storage_bucket_object" "user_profile_handler_zip" {
  name   = "user-profile-handler-${filemd5("/tmp/fitglue-function-zips/user-profile-handler.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/user-profile-handler.zip"
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

    secret_environment_variables {
      key        = "GEMINI_API_KEY"
      project_id = var.project_id
      secret     = google_secret_manager_secret.gemini_api_key.secret_id
      version    = "latest"
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

    secret_environment_variables {
      key        = "GEMINI_API_KEY"
      project_id = var.project_id
      secret     = google_secret_manager_secret.gemini_api_key.secret_id
      version    = "latest"
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

# ----------------- Showcase Uploader (Public Shareable URLs) -----------------
resource "google_storage_bucket_object" "showcase_uploader_zip" {
  name   = "showcase-uploader-${filemd5("/tmp/fitglue-function-zips/showcase-uploader.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/showcase-uploader.zip"
}

resource "google_cloudfunctions2_function" "showcase_uploader" {
  name     = "showcase-uploader"
  location = var.region

  build_config {
    runtime     = "go125"
    entry_point = "ShowcaseUpload"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.showcase_uploader_zip.name
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
    pubsub_topic   = google_pubsub_topic.job_upload_showcase.id
    retry_policy   = var.retry_policy
  }
}

# ----------------- Hevy Uploader (Upload activities to Hevy) -----------------
resource "google_storage_bucket_object" "hevy_uploader_zip" {
  name   = "hevy-uploader-${filemd5("/tmp/fitglue-function-zips/hevy-uploader.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/hevy-uploader.zip"
}

resource "google_cloudfunctions2_function" "hevy_uploader" {
  name     = "hevy-uploader"
  location = var.region

  build_config {
    runtime     = "go125"
    entry_point = "UploadToHevy"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.hevy_uploader_zip.name
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
    pubsub_topic   = google_pubsub_topic.job_upload_hevy.id
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
        object = google_storage_bucket_object.mock_source_handler_zip.name
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
  name        = "hevy-handler"
  location    = var.region
  description = "Ingests Hevy webhooks"

  build_config {
    runtime     = "nodejs20"
    entry_point = "hevyHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.hevy_handler_zip.name
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
        object = google_storage_bucket_object.fitbit_handler_zip.name
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

# ----------------- Strava Handler (Webhook Source) -----------------
resource "google_storage_bucket_object" "strava_handler_zip" {
  name   = "strava-handler-${filemd5("/tmp/fitglue-function-zips/strava-handler.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/strava-handler.zip"
}

resource "google_cloudfunctions2_function" "strava_handler" {
  name        = "strava-handler"
  location    = var.region
  description = "Ingests Strava webhooks for activity sync"

  build_config {
    runtime     = "nodejs20"
    entry_point = "stravaWebhookHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.strava_handler_zip.name
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
      key        = "STRAVA_VERIFY_TOKEN"
      project_id = var.project_id
      secret     = google_secret_manager_secret.strava_verify_token.secret_id
      version    = "latest"
    }

    secret_environment_variables {
      key        = "STRAVA_CLIENT_ID"
      project_id = var.project_id
      secret     = google_secret_manager_secret.strava_client_id.secret_id
      version    = "latest"
    }

    secret_environment_variables {
      key        = "STRAVA_CLIENT_SECRET"
      project_id = var.project_id
      secret     = google_secret_manager_secret.strava_client_secret.secret_id
      version    = "latest"
    }

    service_account_email = google_service_account.cloud_function_sa.email
  }
}

resource "google_cloud_run_service_iam_member" "strava_handler_invoker" {
  project  = google_cloudfunctions2_function.strava_handler.project
  location = google_cloudfunctions2_function.strava_handler.location
  service  = google_cloudfunctions2_function.strava_handler.name
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
  source_archive_object = google_storage_bucket_object.auth_hooks_zip.name
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
        object = google_storage_bucket_object.inputs_handler_zip.name
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
        object = google_storage_bucket_object.activities_handler_zip.name
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

# ----------------- User Profile Handler -----------------
resource "google_cloudfunctions2_function" "user_profile_handler" {
  name        = "user-profile-handler"
  location    = var.region
  description = "Handles user profile operations (GET, PATCH, DELETE)"

  build_config {
    runtime     = "nodejs20"
    entry_point = "userProfileHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.user_profile_handler_zip.name
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

resource "google_cloud_run_service_iam_member" "user_profile_handler_invoker" {
  project  = google_cloudfunctions2_function.user_profile_handler.project
  location = google_cloudfunctions2_function.user_profile_handler.location
  service  = google_cloudfunctions2_function.user_profile_handler.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# ----------------- User Integrations Handler -----------------
resource "google_cloudfunctions2_function" "user_integrations_handler" {
  name        = "user-integrations-handler"
  location    = var.region
  description = "Handles user integration management (list, connect, disconnect)"

  build_config {
    runtime     = "nodejs20"
    entry_point = "userIntegrationsHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.user_integrations_handler_zip.name
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
      BASE_URL             = var.base_url
    }

    secret_environment_variables {
      key        = "STRAVA_CLIENT_ID"
      project_id = var.project_id
      secret     = google_secret_manager_secret.strava_client_id.secret_id
      version    = "latest"
    }

    secret_environment_variables {
      key        = "FITBIT_CLIENT_ID"
      project_id = var.project_id
      secret     = google_secret_manager_secret.fitbit_client_id.secret_id
      version    = "latest"
    }

    service_account_email = google_service_account.cloud_function_sa.email
  }
}

resource "google_cloud_run_service_iam_member" "user_integrations_handler_invoker" {
  project  = google_cloudfunctions2_function.user_integrations_handler.project
  location = google_cloudfunctions2_function.user_integrations_handler.location
  service  = google_cloudfunctions2_function.user_integrations_handler.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# ----------------- Billing Handler -----------------
resource "google_cloudfunctions2_function" "billing_handler" {
  name        = "billing-handler"
  location    = var.region
  description = "Handles billing requests"

  build_config {
    runtime     = "nodejs20"
    entry_point = "billingHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.billing_handler_zip.name
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
      BASE_URL             = var.base_url
    }

    service_account_email = google_service_account.cloud_function_sa.email
  }
}

resource "google_cloud_run_service_iam_member" "billing_handler_invoker" {
  project  = google_cloudfunctions2_function.billing_handler.project
  location = google_cloudfunctions2_function.billing_handler.location
  service  = google_cloudfunctions2_function.billing_handler.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# ----------------- Mobile Sync Handler -----------------
resource "google_cloudfunctions2_function" "mobile_sync_handler" {
  name        = "mobile-sync-handler"
  location    = var.region
  description = "Handles mobile sync requests"

  build_config {
    runtime     = "nodejs20"
    entry_point = "mobileSyncHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.mobile_sync_handler_zip.name
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
      BASE_URL             = var.base_url
    }

    service_account_email = google_service_account.cloud_function_sa.email
  }
}

resource "google_cloud_run_service_iam_member" "mobile_sync_handler_invoker" {
  project  = google_cloudfunctions2_function.mobile_sync_handler.project
  location = google_cloudfunctions2_function.mobile_sync_handler.location
  service  = google_cloudfunctions2_function.mobile_sync_handler.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# ----------------- User Pipelines Handler -----------------
resource "google_cloudfunctions2_function" "user_pipelines_handler" {
  name        = "user-pipelines-handler"
  location    = var.region
  description = "Handles user pipeline CRUD operations"

  build_config {
    runtime     = "nodejs20"
    entry_point = "userPipelinesHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.user_pipelines_handler_zip.name
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

resource "google_cloud_run_service_iam_member" "user_pipelines_handler_invoker" {
  project  = google_cloudfunctions2_function.user_pipelines_handler.project
  location = google_cloudfunctions2_function.user_pipelines_handler.location
  service  = google_cloudfunctions2_function.user_pipelines_handler.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# ----------------- Registry Handler -----------------
resource "google_cloudfunctions2_function" "registry_handler" {
  name        = "registry-handler"
  location    = var.region
  description = "Returns FitGlue registry (connections and plugins)"

  build_config {
    runtime     = "nodejs20"
    entry_point = "registryHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.registry_handler_zip.name
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

resource "google_cloud_run_service_iam_member" "registry_handler_invoker" {
  project  = google_cloudfunctions2_function.registry_handler.project
  location = google_cloudfunctions2_function.registry_handler.location
  service  = google_cloudfunctions2_function.registry_handler.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# ----------------- Integration Request Handler -----------------
resource "google_cloudfunctions2_function" "integration_request_handler" {
  name        = "integration-request-handler"
  location    = var.region
  description = "Handles integration requests from users"

  build_config {
    runtime     = "nodejs20"
    entry_point = "integrationRequestHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.integration_request_handler_zip.name
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

resource "google_cloud_run_service_iam_member" "integration_request_handler_invoker" {
  project  = google_cloudfunctions2_function.integration_request_handler.project
  location = google_cloudfunctions2_function.integration_request_handler.location
  service  = google_cloudfunctions2_function.integration_request_handler.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# NOTE: strava-updater removed - UPDATE operations are now handled by strava-uploader via use_update_method flag


# ----------------- Parkrun Results Source -----------------
# Scheduled function to poll for official Parkrun results
resource "google_storage_bucket_object" "parkrun_results_source_zip" {
  name   = "parkrun-results-source-${filemd5("/tmp/fitglue-function-zips/parkrun-results-source.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/parkrun-results-source.zip"
}

resource "google_cloudfunctions2_function" "parkrun_results_source" {
  name     = "parkrun-results-source"
  location = var.region

  build_config {
    runtime     = "go125"
    entry_point = "PollParkrunResults"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.parkrun_results_source_zip.name
      }
    }
    environment_variables = {}
  }

  service_config {
    available_memory = "512Mi"
    timeout_seconds  = 300
    environment_variables = {
      GOOGLE_CLOUD_PROJECT = var.project_id
      LOG_LEVEL            = var.log_level
    }
    service_account_email = google_service_account.cloud_function_sa.email
  }

  event_trigger {
    trigger_region = var.region
    event_type     = "google.cloud.pubsub.topic.v1.messagePublished"
    pubsub_topic   = google_pubsub_topic.parkrun_results_trigger.id
    retry_policy   = var.retry_policy
  }
}

# Cloud Scheduler: Saturday Parkrun results polling (rapid - every 30 min from 10am-2pm UTC)
resource "google_cloud_scheduler_job" "parkrun_results_rapid" {
  name        = "parkrun-results-rapid"
  description = "Poll for Parkrun results every 30 min on Saturday mornings"
  schedule    = "0,30 10-13 * * 6"  # Saturdays 10:00-13:30 UTC
  time_zone   = "UTC"
  region      = var.region

  pubsub_target {
    topic_name = google_pubsub_topic.parkrun_results_trigger.id
    data       = base64encode("{\"trigger\":\"scheduled\",\"mode\":\"rapid\"}")
  }
}

# Cloud Scheduler: Saturday Parkrun results polling (extended - every 2 hr from 2pm-10pm UTC)
resource "google_cloud_scheduler_job" "parkrun_results_extended" {
  name        = "parkrun-results-extended"
  description = "Poll for Parkrun results every 2 hours on Saturday afternoons"
  schedule    = "0 14,16,18,20,22 * * 6"  # Saturdays 14:00-22:00 UTC
  time_zone   = "UTC"
  region      = var.region

  pubsub_target {
    topic_name = google_pubsub_topic.parkrun_results_trigger.id
    data       = base64encode("{\"trigger\":\"scheduled\",\"mode\":\"extended\"}")
  }
}

# Cloud Scheduler: Christmas Day Parkrun results polling
# Runs at 10:00, 10:30, 11:00, 11:30, 12:00, 12:30, 13:00, 14:00, 16:00, 18:00, 20:00, 22:00 UTC on Dec 25
resource "google_cloud_scheduler_job" "parkrun_results_christmas" {
  name        = "parkrun-results-christmas"
  description = "Poll for Parkrun results on Christmas Day"
  schedule    = "0,30 10-13 25 12 *"
  time_zone   = "UTC"
  region      = var.region

  pubsub_target {
    topic_name = google_pubsub_topic.parkrun_results_trigger.id
    data       = base64encode("{\"trigger\":\"scheduled\",\"mode\":\"christmas\"}")
  }
}

# Cloud Scheduler: New Year's Day Parkrun results polling
# Runs at 10:00, 10:30, 11:00, 11:30, 12:00, 12:30, 13:00 UTC on Jan 1
resource "google_cloud_scheduler_job" "parkrun_results_newyear" {
  name        = "parkrun-results-newyear"
  description = "Poll for Parkrun results on New Year's Day"
  schedule    = "0,30 10-13 1 1 *"
  time_zone   = "UTC"
  region      = var.region

  pubsub_target {
    topic_name = google_pubsub_topic.parkrun_results_trigger.id
    data       = base64encode("{\"trigger\":\"scheduled\",\"mode\":\"newyear\"}")
  }
}

# ----------------- Showcase Handler (Public Viewer) -----------------
resource "google_cloudfunctions2_function" "showcase_handler" {
  name        = "showcase-handler"
  location    = var.region
  description = "Public showcase viewer for shareable activity URLs"

  build_config {
    runtime     = "nodejs20"
    entry_point = "showcaseHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.showcase_handler_zip.name
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

# Public access for showcase (unauthenticated)
resource "google_cloud_run_service_iam_member" "showcase_handler_invoker" {
  project  = google_cloudfunctions2_function.showcase_handler.project
  location = google_cloudfunctions2_function.showcase_handler.location
  service  = google_cloudfunctions2_function.showcase_handler.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# ----------------- FIT Parser Handler (Go) -----------------
resource "google_storage_bucket_object" "fit_parser_handler_zip" {
  name   = "fit-parser-handler-${filemd5("/tmp/fitglue-function-zips/fit-parser-handler.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/fit-parser-handler.zip"
}

resource "google_cloudfunctions2_function" "fit_parser_handler" {
  name        = "fit-parser-handler"
  location    = var.region
  description = "Parses FIT files and publishes to pipeline"

  build_config {
    runtime     = "go125"
    entry_point = "ParseFitFile"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.fit_parser_handler_zip.name
      }
    }
    environment_variables = {}
  }

  service_config {
    available_memory = "512Mi"
    timeout_seconds  = 120
    environment_variables = {
      LOG_LEVEL            = var.log_level
      GOOGLE_CLOUD_PROJECT = var.project_id
      ENABLE_PUBLISH       = "true"
    }
    service_account_email = google_service_account.cloud_function_sa.email
  }
}

resource "google_cloud_run_service_iam_member" "fit_parser_handler_invoker" {
  project  = google_cloudfunctions2_function.fit_parser_handler.project
  location = google_cloudfunctions2_function.fit_parser_handler.location
  service  = google_cloudfunctions2_function.fit_parser_handler.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# ----------------- Repost Handler -----------------
resource "google_cloudfunctions2_function" "repost_handler" {
  name        = "repost-handler"
  location    = var.region
  description = "Handles re-post mechanisms for synchronized activities"

  build_config {
    runtime     = "nodejs20"
    entry_point = "repostHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.repost_handler_zip.name
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

resource "google_cloud_run_service_iam_member" "repost_handler_invoker" {
  project  = google_cloudfunctions2_function.repost_handler.project
  location = google_cloudfunctions2_function.repost_handler.location
  service  = google_cloudfunctions2_function.repost_handler.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# ----------------- Admin Handler -----------------
resource "google_cloudfunctions2_function" "admin_handler" {
  name        = "admin-handler"
  location    = var.region
  description = "Consolidated admin operations handler"

  build_config {
    runtime     = "nodejs20"
    entry_point = "adminHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.admin_handler_zip.name
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

resource "google_cloud_run_service_iam_member" "admin_handler_invoker" {
  project  = google_cloudfunctions2_function.admin_handler.project
  location = google_cloudfunctions2_function.admin_handler.location
  service  = google_cloudfunctions2_function.admin_handler.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}
