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

resource "google_storage_bucket_object" "mobile_source_handler_zip" {
  name   = "mobile-source-handler-${filemd5("/tmp/fitglue-function-zips/mobile-source-handler.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/mobile-source-handler.zip"
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

resource "google_storage_bucket_object" "spotify_oauth_handler_zip" {
  name   = "spotify-oauth-handler-${filemd5("/tmp/fitglue-function-zips/spotify-oauth-handler.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/spotify-oauth-handler.zip"
}

resource "google_storage_bucket_object" "trainingpeaks_oauth_handler_zip" {
  name   = "trainingpeaks-oauth-handler-${filemd5("/tmp/fitglue-function-zips/trainingpeaks-oauth-handler.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/trainingpeaks-oauth-handler.zip"
}

resource "google_storage_bucket_object" "wahoo_oauth_handler_zip" {
  name   = "wahoo-oauth-handler-${filemd5("/tmp/fitglue-function-zips/wahoo-oauth-handler.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/wahoo-oauth-handler.zip"
}

resource "google_storage_bucket_object" "wahoo_handler_zip" {
  name   = "wahoo-handler-${filemd5("/tmp/fitglue-function-zips/wahoo-handler.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/wahoo-handler.zip"
}

resource "google_storage_bucket_object" "polar_handler_zip" {
  name   = "polar-handler-${filemd5("/tmp/fitglue-function-zips/polar-handler.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/polar-handler.zip"
}

resource "google_storage_bucket_object" "polar_oauth_handler_zip" {
  name   = "polar-oauth-handler-${filemd5("/tmp/fitglue-function-zips/polar-oauth-handler.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/polar-oauth-handler.zip"
}

resource "google_storage_bucket_object" "oura_handler_zip" {
  name   = "oura-handler-${filemd5("/tmp/fitglue-function-zips/oura-handler.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/oura-handler.zip"
}

resource "google_storage_bucket_object" "oura_oauth_handler_zip" {
  name   = "oura-oauth-handler-${filemd5("/tmp/fitglue-function-zips/oura-oauth-handler.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/oura-oauth-handler.zip"
}

resource "google_storage_bucket_object" "google_oauth_handler_zip" {
  name   = "google-oauth-handler-${filemd5("/tmp/fitglue-function-zips/google-oauth-handler.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/google-oauth-handler.zip"
}

resource "google_storage_bucket_object" "github_oauth_handler_zip" {
  name   = "github-oauth-handler-${filemd5("/tmp/fitglue-function-zips/github-oauth-handler.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/github-oauth-handler.zip"
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

resource "google_storage_bucket_object" "user_data_handler_zip" {
  name   = "user-data-handler-${filemd5("/tmp/fitglue-function-zips/user-data-handler.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/user-data-handler.zip"
}

resource "google_storage_bucket_object" "connection_actions_handler_zip" {
  name   = "connection-actions-handler-${filemd5("/tmp/fitglue-function-zips/connection-actions-handler.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/connection-actions-handler.zip"
}

resource "google_storage_bucket_object" "data_export_handler_zip" {
  name   = "data-export-handler-${filemd5("/tmp/fitglue-function-zips/data-export-handler.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/data-export-handler.zip"
}

resource "google_storage_bucket_object" "auth_email_handler_zip" {
  name   = "auth-email-handler-${filemd5("/tmp/fitglue-function-zips/auth-email-handler.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/auth-email-handler.zip"
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
      GOOGLE_CLOUD_PROJECT   = var.project_id
      GCS_ARTIFACT_BUCKET    = "${var.project_id}-artifacts"
      SHOWCASE_ASSETS_BUCKET = google_storage_bucket.showcase_assets_bucket.name
      LOG_LEVEL              = var.log_level
      ASSETS_BASE_URL        = "https://assets.${var.domain_name}"
      SENTRY_ORG             = var.sentry_org
      SENTRY_PROJECT         = var.sentry_project
      SENTRY_DSN             = var.sentry_dsn
      PARKRUN_FETCHER_URL    = google_cloud_run_v2_service.parkrun_fetcher.uri
    }

    secret_environment_variables {
      key        = "GEMINI_API_KEY"
      project_id = var.project_id
      secret     = google_secret_manager_secret.gemini_api_key.secret_id
      version    = "latest"
    }

    # Spotify secrets (for spotify_tracks enricher)
    secret_environment_variables {
      key        = "SPOTIFY_CLIENT_ID"
      project_id = var.project_id
      secret     = google_secret_manager_secret.spotify_client_id.secret_id
      version    = "latest"
    }

    secret_environment_variables {
      key        = "SPOTIFY_CLIENT_SECRET"
      project_id = var.project_id
      secret     = google_secret_manager_secret.spotify_client_secret.secret_id
      version    = "latest"
    }

    # Fitbit secrets (for fitbit_hr enricher)
    secret_environment_variables {
      key        = "FITBIT_CLIENT_ID"
      project_id = var.project_id
      secret     = google_secret_manager_secret.fitbit_client_id.secret_id
      version    = "latest"
    }

    secret_environment_variables {
      key        = "FITBIT_CLIENT_SECRET"
      project_id = var.project_id
      secret     = google_secret_manager_secret.fitbit_client_secret.secret_id
      version    = "latest"
    }

    service_account_email = google_service_account.cloud_function_sa.email
  }

  event_trigger {
    trigger_region = var.region
    event_type     = "google.cloud.pubsub.topic.v1.messagePublished"
    pubsub_topic   = google_pubsub_topic.pipeline_activity.id
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
      GOOGLE_CLOUD_PROJECT   = var.project_id
      GCS_ARTIFACT_BUCKET    = "${var.project_id}-artifacts"
      SHOWCASE_ASSETS_BUCKET = google_storage_bucket.showcase_assets_bucket.name
      LOG_LEVEL              = var.log_level
      ASSETS_BASE_URL        = "https://assets.${var.domain_name}"
      SENTRY_ORG             = var.sentry_org
      SENTRY_PROJECT         = var.sentry_project
      SENTRY_DSN             = var.sentry_dsn
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
      LOG_LEVEL            = var.log_level
      SENTRY_ORG           = var.sentry_org
      SENTRY_PROJECT       = var.sentry_project
      SENTRY_DSN           = var.sentry_dsn
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

# ----------------- Pipeline Splitter Service -----------------
# Receives raw activities and fans out to per-pipeline messages
resource "google_storage_bucket_object" "pipeline_splitter_zip" {
  name   = "pipeline-splitter-${filemd5("/tmp/fitglue-function-zips/pipeline-splitter.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/pipeline-splitter.zip"
}

resource "google_cloudfunctions2_function" "pipeline_splitter" {
  name     = "pipeline-splitter"
  location = var.region

  build_config {
    runtime     = "go125"
    entry_point = "SplitByPipeline"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.pipeline_splitter_zip.name
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
      SENTRY_ORG           = var.sentry_org
      SENTRY_PROJECT       = var.sentry_project
      SENTRY_DSN           = var.sentry_dsn
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
      SENTRY_ORG           = var.sentry_org
      SENTRY_PROJECT       = var.sentry_project
      SENTRY_DSN           = var.sentry_dsn
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
      SENTRY_ORG           = var.sentry_org
      SENTRY_PROJECT       = var.sentry_project
      SENTRY_DSN           = var.sentry_dsn
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
      GOOGLE_CLOUD_PROJECT   = var.project_id
      SHOWCASE_ASSETS_BUCKET = google_storage_bucket.showcase_assets_bucket.name
      LOG_LEVEL              = var.log_level
      SENTRY_ORG             = var.sentry_org
      SENTRY_PROJECT         = var.sentry_project
      SENTRY_DSN             = var.sentry_dsn
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
      SENTRY_ORG           = var.sentry_org
      SENTRY_PROJECT       = var.sentry_project
      SENTRY_DSN           = var.sentry_dsn
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

# ----------------- TrainingPeaks Uploader (Upload activities to TrainingPeaks) -----------------
resource "google_storage_bucket_object" "trainingpeaks_uploader_zip" {
  name   = "trainingpeaks-uploader-${filemd5("/tmp/fitglue-function-zips/trainingpeaks-uploader.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/trainingpeaks-uploader.zip"
}

resource "google_cloudfunctions2_function" "trainingpeaks_uploader" {
  name     = "trainingpeaks-uploader"
  location = var.region

  build_config {
    runtime     = "go125"
    entry_point = "UploadToTrainingPeaks"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.trainingpeaks_uploader_zip.name
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
      SENTRY_ORG           = var.sentry_org
      SENTRY_PROJECT       = var.sentry_project
      SENTRY_DSN           = var.sentry_dsn
    }

    secret_environment_variables {
      key        = "TRAININGPEAKS_CLIENT_ID"
      project_id = var.project_id
      secret     = google_secret_manager_secret.trainingpeaks_client_id.secret_id
      version    = "latest"
    }

    secret_environment_variables {
      key        = "TRAININGPEAKS_CLIENT_SECRET"
      project_id = var.project_id
      secret     = google_secret_manager_secret.trainingpeaks_client_secret.secret_id
      version    = "latest"
    }

    service_account_email = google_service_account.cloud_function_sa.email
  }

  event_trigger {
    trigger_region = var.region
    event_type     = "google.cloud.pubsub.topic.v1.messagePublished"
    pubsub_topic   = google_pubsub_topic.job_upload_trainingpeaks.id
    retry_policy   = var.retry_policy
  }
}

# ----------------- Intervals.icu Uploader (Upload activities to Intervals.icu) -----------------
resource "google_storage_bucket_object" "intervals_uploader_zip" {
  name   = "intervals-uploader-${filemd5("/tmp/fitglue-function-zips/intervals-uploader.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/intervals-uploader.zip"
}

resource "google_cloudfunctions2_function" "intervals_uploader" {
  name     = "intervals-uploader"
  location = var.region

  build_config {
    runtime     = "go125"
    entry_point = "UploadToIntervals"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.intervals_uploader_zip.name
      }
    }
    environment_variables = {}
  }

  service_config {
    available_memory = "256Mi"
    timeout_seconds  = 60
    environment_variables = {
      GOOGLE_CLOUD_PROJECT = var.project_id
      GCS_ARTIFACT_BUCKET  = "${var.project_id}-artifacts"
      LOG_LEVEL            = var.log_level
      SENTRY_ORG           = var.sentry_org
      SENTRY_PROJECT       = var.sentry_project
      SENTRY_DSN           = var.sentry_dsn
    }
    service_account_email = google_service_account.cloud_function_sa.email
  }

  event_trigger {
    trigger_region = var.region
    event_type     = "google.cloud.pubsub.topic.v1.messagePublished"
    pubsub_topic   = google_pubsub_topic.job_upload_intervals.id
    retry_policy   = var.retry_policy
  }
}

# ----------------- Google Sheets Uploader (Upload activities to Google Sheets) -----------------
resource "google_storage_bucket_object" "googlesheets_uploader_zip" {
  name   = "googlesheets-uploader-${filemd5("/tmp/fitglue-function-zips/googlesheets-uploader.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/googlesheets-uploader.zip"
}

resource "google_cloudfunctions2_function" "googlesheets_uploader" {
  name     = "googlesheets-uploader"
  location = var.region

  build_config {
    runtime     = "go125"
    entry_point = "UploadToGoogleSheets"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.googlesheets_uploader_zip.name
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
      SENTRY_ORG           = var.sentry_org
      SENTRY_PROJECT       = var.sentry_project
      SENTRY_DSN           = var.sentry_dsn
    }

    secret_environment_variables {
      key        = "GOOGLE_CLIENT_ID"
      project_id = var.project_id
      secret     = google_secret_manager_secret.google_client_id.secret_id
      version    = "latest"
    }

    secret_environment_variables {
      key        = "GOOGLE_CLIENT_SECRET"
      project_id = var.project_id
      secret     = google_secret_manager_secret.google_client_secret.secret_id
      version    = "latest"
    }

    service_account_email = google_service_account.cloud_function_sa.email
  }

  event_trigger {
    trigger_region = var.region
    event_type     = "google.cloud.pubsub.topic.v1.messagePublished"
    pubsub_topic   = google_pubsub_topic.job_upload_googlesheets.id
    retry_policy   = var.retry_policy
  }
}

# ----------------- GitHub Uploader (Go) -----------------
resource "google_storage_bucket_object" "github_uploader_zip" {
  name   = "github-uploader-${filemd5("/tmp/fitglue-function-zips/github-uploader.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/github-uploader.zip"
}

resource "google_cloudfunctions2_function" "github_uploader" {
  name     = "github-uploader"
  location = var.region

  build_config {
    runtime     = "go125"
    entry_point = "UploadToGitHub"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.github_uploader_zip.name
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
      SENTRY_ORG           = var.sentry_org
      SENTRY_PROJECT       = var.sentry_project
      SENTRY_DSN           = var.sentry_dsn
    }

    service_account_email = google_service_account.cloud_function_sa.email
  }

  event_trigger {
    trigger_region = var.region
    event_type     = "google.cloud.pubsub.topic.v1.messagePublished"
    pubsub_topic   = google_pubsub_topic.job_upload_github.id
    retry_policy   = var.retry_policy
  }
}

# ----------------- GitHub Handler (Webhook Source) -----------------
resource "google_storage_bucket_object" "github_handler_zip" {
  name   = "github-handler-${filemd5("/tmp/fitglue-function-zips/github-handler.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/github-handler.zip"
}

resource "google_cloudfunctions2_function" "github_handler" {
  name        = "github-handler"
  location    = var.region
  description = "Ingests GitHub webhooks for activity sync"

  build_config {
    runtime     = "nodejs22"
    entry_point = "githubHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.github_handler_zip.name
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
      SENTRY_ORG           = var.sentry_org
      SENTRY_PROJECT       = var.sentry_project
      SENTRY_DSN           = var.sentry_dsn
    }

    secret_environment_variables {
      key        = "GITHUB_WEBHOOK_SECRET"
      project_id = var.project_id
      secret     = google_secret_manager_secret.github_webhook_secret.secret_id
      version    = "latest"
    }

    secret_environment_variables {
      key        = "GITHUB_CLIENT_ID"
      project_id = var.project_id
      secret     = google_secret_manager_secret.github_client_id.secret_id
      version    = "latest"
    }

    secret_environment_variables {
      key        = "GITHUB_CLIENT_SECRET"
      project_id = var.project_id
      secret     = google_secret_manager_secret.github_client_secret.secret_id
      version    = "latest"
    }

    service_account_email = google_service_account.cloud_function_sa.email
  }
}

resource "google_cloud_run_service_iam_member" "github_handler_invoker" {
  project  = google_cloudfunctions2_function.github_handler.project
  location = google_cloudfunctions2_function.github_handler.location
  service  = google_cloudfunctions2_function.github_handler.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# ----------------- Mock Source Handler (Dev Only) -----------------
resource "google_cloudfunctions2_function" "mock_source_handler" {
  // Needs to be deployed to test and prod otherwise firebase.json will fail with "can't find function"
  // But we do *not* allow this to be run from the web on test/prod by omiting the mock_source_handler_invoker
  name        = "mock-source-handler"
  location    = var.region
  description = "Mocks source events for testing"

  build_config {
    runtime     = "nodejs22"
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
      SENTRY_ORG           = var.sentry_org
      SENTRY_PROJECT       = var.sentry_project
      SENTRY_DSN           = var.sentry_dsn
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
    runtime     = "nodejs22"
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
      LOG_LEVEL      = var.log_level
      SENTRY_ORG     = var.sentry_org
      SENTRY_PROJECT = var.sentry_project
      SENTRY_DSN     = var.sentry_dsn
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
    runtime     = "nodejs22"
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
      SENTRY_ORG           = var.sentry_org
      SENTRY_PROJECT       = var.sentry_project
      SENTRY_DSN           = var.sentry_dsn
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
    runtime     = "nodejs22"
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
      SENTRY_ORG           = var.sentry_org
      SENTRY_PROJECT       = var.sentry_project
      SENTRY_DSN           = var.sentry_dsn
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

# ----------------- Wahoo Handler (Webhook Source) -----------------
resource "google_cloudfunctions2_function" "wahoo_handler" {
  name        = "wahoo-handler"
  location    = var.region
  description = "Ingests Wahoo webhooks for workout sync"

  build_config {
    runtime     = "nodejs22"
    entry_point = "wahooWebhookHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.wahoo_handler_zip.name
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
      SENTRY_ORG           = var.sentry_org
      SENTRY_PROJECT       = var.sentry_project
      SENTRY_DSN           = var.sentry_dsn
    }

    secret_environment_variables {
      key        = "WAHOO_CLIENT_ID"
      project_id = var.project_id
      secret     = google_secret_manager_secret.wahoo_client_id.secret_id
      version    = "latest"
    }

    secret_environment_variables {
      key        = "WAHOO_CLIENT_SECRET"
      project_id = var.project_id
      secret     = google_secret_manager_secret.wahoo_client_secret.secret_id
      version    = "latest"
    }

    service_account_email = google_service_account.cloud_function_sa.email
  }
}

resource "google_cloud_run_service_iam_member" "wahoo_handler_invoker" {
  project  = google_cloudfunctions2_function.wahoo_handler.project
  location = google_cloudfunctions2_function.wahoo_handler.location
  service  = google_cloudfunctions2_function.wahoo_handler.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# ----------------- Auth Hooks -----------------
# Triggered by Eventarc (Firebase Auth User Created)
# NOTE: Using Gen 1 function because Gen 2 (Eventarc) does not natively support async Firebase Auth triggers yet
resource "google_cloudfunctions_function" "auth_on_create" {
  name        = "auth-on-create"
  description = "Triggered when a new user is created in Firebase Auth"
  runtime     = "nodejs22"

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
    SENTRY_ORG           = var.sentry_org
    SENTRY_PROJECT       = var.sentry_project
    SENTRY_DSN           = var.sentry_dsn
  }

  service_account_email = google_service_account.cloud_function_sa.email
}

# ----------------- Inputs Handler -----------------
resource "google_cloudfunctions2_function" "inputs_handler" {
  name        = "inputs-handler"
  location    = var.region
  description = "Handles pending user input resolutions"

  build_config {
    runtime     = "nodejs22"
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
      SENTRY_ORG           = var.sentry_org
      SENTRY_PROJECT       = var.sentry_project
      SENTRY_DSN           = var.sentry_dsn
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
    runtime     = "nodejs22"
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
      ENVIRONMENT          = var.environment
      SENTRY_ORG           = var.sentry_org
      SENTRY_PROJECT       = var.sentry_project
      SENTRY_DSN           = var.sentry_dsn
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
    runtime     = "nodejs22"
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
      SENTRY_ORG           = var.sentry_org
      SENTRY_PROJECT       = var.sentry_project
      SENTRY_DSN           = var.sentry_dsn
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
# terraform-lint: needs-all-oauth-secrets
resource "google_cloudfunctions2_function" "user_integrations_handler" {
  name        = "user-integrations-handler"
  location    = var.region
  description = "Handles user integration management (list, connect, disconnect)"

  build_config {
    runtime     = "nodejs22"
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
      SENTRY_ORG           = var.sentry_org
      SENTRY_PROJECT       = var.sentry_project
      SENTRY_DSN           = var.sentry_dsn
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

    secret_environment_variables {
      key        = "GOOGLE_CLIENT_ID"
      project_id = var.project_id
      secret     = google_secret_manager_secret.google_client_id.secret_id
      version    = "latest"
    }

    secret_environment_variables {
      key        = "GITHUB_CLIENT_ID"
      project_id = var.project_id
      secret     = google_secret_manager_secret.github_client_id.secret_id
      version    = "latest"
    }

    secret_environment_variables {
      key        = "SPOTIFY_CLIENT_ID"
      project_id = var.project_id
      secret     = google_secret_manager_secret.spotify_client_id.secret_id
      version    = "latest"
    }

    secret_environment_variables {
      key        = "TRAININGPEAKS_CLIENT_ID"
      project_id = var.project_id
      secret     = google_secret_manager_secret.trainingpeaks_client_id.secret_id
      version    = "latest"
    }

    secret_environment_variables {
      key        = "WAHOO_CLIENT_ID"
      project_id = var.project_id
      secret     = google_secret_manager_secret.wahoo_client_id.secret_id
      version    = "latest"
    }

    secret_environment_variables {
      key        = "POLAR_CLIENT_ID"
      project_id = var.project_id
      secret     = google_secret_manager_secret.polar_client_id.secret_id
      version    = "latest"
    }

    secret_environment_variables {
      key        = "OURA_CLIENT_ID"
      project_id = var.project_id
      secret     = google_secret_manager_secret.oura_client_id.secret_id
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
    runtime     = "nodejs22"
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
      SENTRY_ORG           = var.sentry_org
      SENTRY_PROJECT       = var.sentry_project
      SENTRY_DSN           = var.sentry_dsn
    }

    secret_environment_variables {
      key        = "STRIPE_SECRET_KEY"
      project_id = var.project_id
      secret     = google_secret_manager_secret.stripe_secret_key.secret_id
      version    = "latest"
    }

    secret_environment_variables {
      key        = "STRIPE_WEBHOOK_SECRET"
      project_id = var.project_id
      secret     = google_secret_manager_secret.stripe_webhook_secret.secret_id
      version    = "latest"
    }

    secret_environment_variables {
      key        = "STRIPE_PRICE_ID"
      project_id = var.project_id
      secret     = google_secret_manager_secret.stripe_price_id.secret_id
      version    = "latest"
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

# ----------------- User Data Handler -----------------
resource "google_cloudfunctions2_function" "user_data_handler" {
  name        = "user-data-handler"
  location    = var.region
  description = "Handles user enricher data (counters, personal records) management"

  build_config {
    runtime     = "nodejs22"
    entry_point = "userDataHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.user_data_handler_zip.name
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
      SENTRY_ORG           = var.sentry_org
      SENTRY_PROJECT       = var.sentry_project
      SENTRY_DSN           = var.sentry_dsn
    }
    service_account_email = google_service_account.cloud_function_sa.email
  }
}

resource "google_cloud_run_service_iam_member" "user_data_handler_invoker" {
  project  = google_cloudfunctions2_function.user_data_handler.project
  location = google_cloudfunctions2_function.user_data_handler.location
  service  = google_cloudfunctions2_function.user_data_handler.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# ----------------- Mobile Sync Handler -----------------
resource "google_cloudfunctions2_function" "mobile_sync_handler" {
  name        = "mobile-sync-handler"
  location    = var.region
  description = "Handles mobile sync requests"

  build_config {
    runtime     = "nodejs22"
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
      SENTRY_ORG           = var.sentry_org
      SENTRY_PROJECT       = var.sentry_project
      SENTRY_DSN           = var.sentry_dsn
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

# ----------------- Mobile Source Handler -----------------
# Pub/Sub-triggered: consumes topic-mobile-activity, maps to StandardizedActivity,
# publishes to topic-raw-activity
resource "google_cloudfunctions2_function" "mobile_source_handler" {
  name        = "mobile-source-handler"
  location    = var.region
  description = "Processes mobile health activities into StandardizedActivity format"

  build_config {
    runtime     = "nodejs22"
    entry_point = "mobileSourceHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.mobile_source_handler_zip.name
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
      GCS_ARTIFACT_BUCKET  = "${var.project_id}-artifacts"
      SENTRY_ORG           = var.sentry_org
      SENTRY_PROJECT       = var.sentry_project
      SENTRY_DSN           = var.sentry_dsn
    }

    service_account_email = google_service_account.cloud_function_sa.email
  }

  event_trigger {
    trigger_region = var.region
    event_type     = "google.cloud.pubsub.topic.v1.messagePublished"
    pubsub_topic   = google_pubsub_topic.mobile_activity.id
    retry_policy   = var.retry_policy
  }
}

resource "google_cloud_run_service_iam_member" "mobile_source_handler_invoker" {
  project  = google_cloudfunctions2_function.mobile_source_handler.project
  location = google_cloudfunctions2_function.mobile_source_handler.location
  service  = google_cloudfunctions2_function.mobile_source_handler.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.cloud_function_sa.email}"
}

# ----------------- User Pipelines Handler -----------------
resource "google_cloudfunctions2_function" "user_pipelines_handler" {
  name        = "user-pipelines-handler"
  location    = var.region
  description = "Handles user pipeline CRUD operations"

  build_config {
    runtime     = "nodejs22"
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
      SENTRY_ORG           = var.sentry_org
      SENTRY_PROJECT       = var.sentry_project
      SENTRY_DSN           = var.sentry_dsn
      GITHUB_HANDLER_URL   = google_cloudfunctions2_function.github_handler.service_config[0].uri
    }

    secret_environment_variables {
      key        = "GITHUB_WEBHOOK_SECRET"
      project_id = var.project_id
      secret     = google_secret_manager_secret.github_webhook_secret.secret_id
      version    = "latest"
    }

    secret_environment_variables {
      key        = "GITHUB_CLIENT_ID"
      project_id = var.project_id
      secret     = google_secret_manager_secret.github_client_id.secret_id
      version    = "latest"
    }

    secret_environment_variables {
      key        = "GITHUB_CLIENT_SECRET"
      project_id = var.project_id
      secret     = google_secret_manager_secret.github_client_secret.secret_id
      version    = "latest"
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
    runtime     = "nodejs22"
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
      SENTRY_ORG           = var.sentry_org
      SENTRY_PROJECT       = var.sentry_project
      SENTRY_DSN           = var.sentry_dsn
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
    runtime     = "nodejs22"
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
      SENTRY_ORG           = var.sentry_org
      SENTRY_PROJECT       = var.sentry_project
      SENTRY_DSN           = var.sentry_dsn
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


# ----------------- Parkrun Fetcher (Playwright) -----------------
# Cloud Run service with headless browser to bypass AWS WAF
#
# ⚠️  MANUAL DEPLOYMENT REQUIRED ⚠️
# This function's Docker image is NOT deployed via Terraform/CI.
# The Playwright container requires manual deployment when the Dockerfile changes.
#
# To deploy/update, run from server directory:
#   gcloud builds submit \
#     --tag us-central1-docker.pkg.dev/fitglue-server-dev/cloud-run-source-deploy/parkrun-fetcher:latest \
#     --project fitglue-server-dev \
#     src/typescript/parkrun-fetcher
#
# For prod:
#   gcloud builds submit \
#     --tag us-central1-docker.pkg.dev/fitglue-server-prod/cloud-run-source-deploy/parkrun-fetcher:latest \
#     --project fitglue-server-prod \
#     src/typescript/parkrun-fetcher

resource "google_cloud_run_v2_service" "parkrun_fetcher" {
  name                = "parkrun-fetcher"
  location            = var.region
  ingress             = "INGRESS_TRAFFIC_ALL" # Security enforced via IAM, not network
  deletion_protection = false

  template {
    containers {
      # Image must be pre-deployed manually - see comments above
      image = "${var.region}-docker.pkg.dev/${var.project_id}/cloud-run-source-deploy/parkrun-fetcher:latest"

      resources {
        limits = {
          cpu    = "1"
          memory = "1Gi"
        }
      }

      ports {
        container_port = 8080
      }
    }

    scaling {
      min_instance_count = 0
      max_instance_count = 2
    }

    timeout         = "60s"
    service_account = google_service_account.cloud_function_sa.email
  }

  # Ignore changes to the image since it's deployed manually
  lifecycle {
    ignore_changes = [
      template[0].containers[0].image,
    ]
  }
}

resource "google_cloud_run_v2_service_iam_member" "parkrun_fetcher_invoker" {
  project  = google_cloud_run_v2_service.parkrun_fetcher.project
  location = google_cloud_run_v2_service.parkrun_fetcher.location
  name     = google_cloud_run_v2_service.parkrun_fetcher.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.cloud_function_sa.email}"
}

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
      SENTRY_ORG           = var.sentry_org
      SENTRY_PROJECT       = var.sentry_project
      SENTRY_DSN           = var.sentry_dsn
      PARKRUN_FETCHER_URL  = google_cloud_run_v2_service.parkrun_fetcher.uri
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
  schedule    = "0,30 10-13 * * 6" # Saturdays 10:00-13:30 UTC
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
  schedule    = "0 14,16,18,20,22 * * 6" # Saturdays 14:00-22:00 UTC
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
    runtime     = "nodejs22"
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
      SENTRY_ORG           = var.sentry_org
      SENTRY_PROJECT       = var.sentry_project
      SENTRY_DSN           = var.sentry_dsn
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

# ----------------- Showcase Management Handler (Authenticated Profile CRUD) -----------------
resource "google_storage_bucket_object" "showcase_management_handler_zip" {
  name   = "showcase-management-handler-${filemd5("/tmp/fitglue-function-zips/showcase-management-handler.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/showcase-management-handler.zip"
}

resource "google_cloudfunctions2_function" "showcase_management_handler" {
  name        = "showcase-management-handler"
  location    = var.region
  description = "Authenticated showcase profile management (CRUD, slug, picture upload)"

  build_config {
    runtime     = "nodejs22"
    entry_point = "showcaseManagementHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.showcase_management_handler_zip.name
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
      SENTRY_ORG           = var.sentry_org
      SENTRY_PROJECT       = var.sentry_project
      SENTRY_DSN           = var.sentry_dsn
      ASSETS_BASE_URL      = "https://assets.${var.domain_name}"
    }
    service_account_email = google_service_account.cloud_function_sa.email
  }
}

# Public access (Firebase Auth is handled at app level)
resource "google_cloud_run_service_iam_member" "showcase_management_handler_invoker" {
  project  = google_cloudfunctions2_function.showcase_management_handler.project
  location = google_cloudfunctions2_function.showcase_management_handler.location
  service  = google_cloudfunctions2_function.showcase_management_handler.name
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
      SENTRY_ORG           = var.sentry_org
      SENTRY_PROJECT       = var.sentry_project
      SENTRY_DSN           = var.sentry_dsn
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
    runtime     = "nodejs22"
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
      SENTRY_ORG           = var.sentry_org
      SENTRY_PROJECT       = var.sentry_project
      SENTRY_DSN           = var.sentry_dsn
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
    runtime     = "nodejs22"
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
      SENTRY_ORG           = var.sentry_org
      SENTRY_PROJECT       = var.sentry_project
      SENTRY_DSN           = var.sentry_dsn
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

# ----------------- Polar Handler (Webhook Source) -----------------
resource "google_cloudfunctions2_function" "polar_handler" {
  name        = "polar-handler"
  location    = var.region
  description = "Ingests Polar webhooks for activity sync"

  build_config {
    runtime     = "nodejs22"
    entry_point = "polarWebhookHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.polar_handler_zip.name
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
      SENTRY_ORG           = var.sentry_org
      SENTRY_PROJECT       = var.sentry_project
      SENTRY_DSN           = var.sentry_dsn
    }

    secret_environment_variables {
      key        = "POLAR_CLIENT_ID"
      project_id = var.project_id
      secret     = google_secret_manager_secret.polar_client_id.secret_id
      version    = "latest"
    }

    secret_environment_variables {
      key        = "POLAR_CLIENT_SECRET"
      project_id = var.project_id
      secret     = google_secret_manager_secret.polar_client_secret.secret_id
      version    = "latest"
    }

    service_account_email = google_service_account.cloud_function_sa.email
  }
}

resource "google_cloud_run_service_iam_member" "polar_handler_invoker" {
  project  = google_cloudfunctions2_function.polar_handler.project
  location = google_cloudfunctions2_function.polar_handler.location
  service  = google_cloudfunctions2_function.polar_handler.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# ----------------- Oura Handler (Webhook Source) -----------------
resource "google_cloudfunctions2_function" "oura_handler" {
  name        = "oura-handler"
  location    = var.region
  description = "Ingests Oura webhooks for sleep/activity sync"

  build_config {
    runtime     = "nodejs22"
    entry_point = "ouraWebhookHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.oura_handler_zip.name
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
      SENTRY_ORG           = var.sentry_org
      SENTRY_PROJECT       = var.sentry_project
      SENTRY_DSN           = var.sentry_dsn
    }

    secret_environment_variables {
      key        = "OURA_CLIENT_ID"
      project_id = var.project_id
      secret     = google_secret_manager_secret.oura_client_id.secret_id
      version    = "latest"
    }

    secret_environment_variables {
      key        = "OURA_CLIENT_SECRET"
      project_id = var.project_id
      secret     = google_secret_manager_secret.oura_client_secret.secret_id
      version    = "latest"
    }

    service_account_email = google_service_account.cloud_function_sa.email
  }
}

resource "google_cloud_run_service_iam_member" "oura_handler_invoker" {
  project  = google_cloudfunctions2_function.oura_handler.project
  location = google_cloudfunctions2_function.oura_handler.location
  service  = google_cloudfunctions2_function.oura_handler.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# ----------------- Registration Summary Handler -----------------
# Daily email summary of new user registrations
resource "google_storage_bucket_object" "registration_summary_handler_zip" {
  name   = "registration-summary-handler-${filemd5("/tmp/fitglue-function-zips/registration-summary-handler.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/registration-summary-handler.zip"
}

resource "google_cloudfunctions2_function" "registration_summary_handler" {
  name        = "registration-summary-handler"
  location    = var.region
  description = "Daily email summary of new user registrations"

  build_config {
    runtime     = "nodejs22"
    entry_point = "registrationSummaryHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.registration_summary_handler_zip.name
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
    pubsub_topic   = google_pubsub_topic.registration_summary_trigger.id
    retry_policy   = var.retry_policy
  }
}

# Cloud Scheduler: Daily registration summary at 8:00 AM UTC
resource "google_cloud_scheduler_job" "registration_summary_daily" {
  name        = "registration-summary-daily"
  description = "Send daily summary of new user registrations"
  schedule    = "0 8 * * *"  # 8:00 AM UTC daily
  time_zone   = "UTC"
  region      = var.region

  pubsub_target {
    topic_name = google_pubsub_topic.registration_summary_trigger.id
    data       = base64encode("{\"trigger\":\"scheduled\"}")
  }
}

# ----------------- Connection Actions Handler -----------------
# Handles one-off integration actions like importing historical PRs
resource "google_cloudfunctions2_function" "connection_actions_handler" {
  name        = "connection-actions-handler"
  location    = var.region
  description = "Handles connection-specific actions (import historical PRs, etc.)"

  build_config {
    runtime     = "nodejs22"
    entry_point = "connectionActionsHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.connection_actions_handler_zip.name
      }
    }
    environment_variables = {}
  }

  service_config {
    available_memory = "512Mi"
    timeout_seconds  = 300  # 5 minutes for large import jobs
    environment_variables = {
      LOG_LEVEL            = var.log_level
      GOOGLE_CLOUD_PROJECT = var.project_id
      SENTRY_ORG           = var.sentry_org
      SENTRY_PROJECT       = var.sentry_project
      SENTRY_DSN           = var.sentry_dsn
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

resource "google_cloud_run_service_iam_member" "connection_actions_handler_invoker" {
  project  = google_cloudfunctions2_function.connection_actions_handler.project
  location = google_cloudfunctions2_function.connection_actions_handler.location
  service  = google_cloudfunctions2_function.connection_actions_handler.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# Allow the Cloud Function SA to invoke itself (for Cloud Tasks OIDC callbacks)
resource "google_cloud_run_service_iam_member" "connection_actions_handler_self_invoker" {
  project  = google_cloudfunctions2_function.connection_actions_handler.project
  location = google_cloudfunctions2_function.connection_actions_handler.location
  service  = google_cloudfunctions2_function.connection_actions_handler.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.cloud_function_sa.email}"
}

# ----------------- Cloud Tasks Queue for Connection Actions -----------------
resource "google_cloud_tasks_queue" "connection_actions" {
  name     = "connection-actions"
  location = var.region

  depends_on = [google_project_service.apis["cloudtasks.googleapis.com"]]

  rate_limits {
    max_dispatches_per_second = 5
    max_concurrent_dispatches = 2
  }

  retry_config {
    max_attempts       = 3
    min_backoff        = "10s"
    max_backoff        = "300s"
    max_doublings      = 4
  }
}

# Grant the Cloud Function SA permission to enqueue tasks
resource "google_project_iam_member" "cloud_function_sa_tasks_enqueuer" {
  project = var.project_id
  role    = "roles/cloudtasks.enqueuer"
  member  = "serviceAccount:${google_service_account.cloud_function_sa.email}"
}

# ----------------- Data Export Handler -----------------
resource "google_cloudfunctions2_function" "data_export_handler" {
  name        = "data-export-handler"
  location    = var.region
  description = "Handles GDPR data export and per-run data download"

  build_config {
    runtime     = "nodejs22"
    entry_point = "dataExportHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.data_export_handler_zip.name
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
      GCS_ARTIFACT_BUCKET  = "${var.project_id}-artifacts"
      SENTRY_ORG           = var.sentry_org
      SENTRY_PROJECT       = var.sentry_project
      SENTRY_DSN           = var.sentry_dsn
    }

    secret_environment_variables {
      key        = "EMAIL_APP_PASSWORD"
      project_id = var.project_id
      secret     = google_secret_manager_secret.email_app_password.secret_id
      version    = "latest"
    }

    service_account_email = google_service_account.cloud_function_sa.email
  }
}

resource "google_cloud_run_service_iam_member" "data_export_handler_invoker" {
  project  = google_cloudfunctions2_function.data_export_handler.project
  location = google_cloudfunctions2_function.data_export_handler.location
  service  = google_cloudfunctions2_function.data_export_handler.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# Allow the Cloud Function SA to invoke itself (for Cloud Tasks OIDC callbacks)
resource "google_cloud_run_service_iam_member" "data_export_handler_self_invoker" {
  project  = google_cloudfunctions2_function.data_export_handler.project
  location = google_cloudfunctions2_function.data_export_handler.location
  service  = google_cloudfunctions2_function.data_export_handler.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.cloud_function_sa.email}"
}

# ----------------- Auth Email Handler -----------------
# Sends branded authentication emails (verification, password reset, email change)
resource "google_cloudfunctions2_function" "auth_email_handler" {
  name        = "auth-email-handler"
  location    = var.region
  description = "Sends branded authentication emails via custom templates"

  build_config {
    runtime     = "nodejs22"
    entry_point = "authEmailHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.auth_email_handler_zip.name
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
      SENTRY_ORG           = var.sentry_org
      SENTRY_PROJECT       = var.sentry_project
      SENTRY_DSN           = var.sentry_dsn
    }

    secret_environment_variables {
      key        = "EMAIL_APP_PASSWORD"
      project_id = var.project_id
      secret     = google_secret_manager_secret.email_app_password.secret_id
      version    = "latest"
    }

    service_account_email = google_service_account.cloud_function_sa.email
  }
}

resource "google_cloud_run_service_iam_member" "auth_email_handler_invoker" {
  project  = google_cloudfunctions2_function.auth_email_handler.project
  location = google_cloudfunctions2_function.auth_email_handler.location
  service  = google_cloudfunctions2_function.auth_email_handler.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# ----------------- Cloud Tasks Queue for Data Export -----------------
resource "google_cloud_tasks_queue" "data_export" {
  name     = "data-export"
  location = var.region

  depends_on = [google_project_service.apis["cloudtasks.googleapis.com"]]

  rate_limits {
    max_dispatches_per_second = 2
    max_concurrent_dispatches = 1
  }

  retry_config {
    max_attempts       = 3
    min_backoff        = "30s"
    max_backoff        = "600s"
    max_doublings      = 3
  }
}
