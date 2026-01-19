# ----------------- Strava OAuth Handler -----------------

resource "google_cloudfunctions2_function" "strava_oauth_handler" {
  name        = "strava-oauth-handler"
  location    = var.region
  description = "Handles Strava OAuth callback"

  build_config {
    runtime     = "nodejs20"
    entry_point = "stravaOAuthHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.strava_oauth_handler_zip.name
      }
    }
    environment_variables = {}
  }

  service_config {
    available_memory = "256Mi"
    timeout_seconds  = 60
    environment_variables = {
      LOG_LEVEL            = var.log_level
      BASE_URL             = local.base_url
      GOOGLE_CLOUD_PROJECT = var.project_id
    }
    secret_environment_variables {
      key        = "OAUTH_STATE_SECRET"
      project_id = var.project_id
      secret     = google_secret_manager_secret.oauth_state_secret.secret_id
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

resource "google_cloud_run_service_iam_member" "strava_oauth_handler_invoker" {
  project  = google_cloudfunctions2_function.strava_oauth_handler.project
  location = google_cloudfunctions2_function.strava_oauth_handler.location
  service  = google_cloudfunctions2_function.strava_oauth_handler.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# ----------------- Fitbit OAuth Handler -----------------

resource "google_cloudfunctions2_function" "fitbit_oauth_handler" {
  name        = "fitbit-oauth-handler"
  location    = var.region
  description = "Handles Fitbit OAuth callback"

  build_config {
    runtime     = "nodejs20"
    entry_point = "fitbitOAuthHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.fitbit_oauth_handler_zip.name
      }
    }
    environment_variables = {}
  }

  service_config {
    available_memory = "256Mi"
    timeout_seconds  = 60
    environment_variables = {
      LOG_LEVEL            = var.log_level
      BASE_URL             = local.base_url
      GOOGLE_CLOUD_PROJECT = var.project_id
    }
    secret_environment_variables {
      key        = "OAUTH_STATE_SECRET"
      project_id = var.project_id
      secret     = google_secret_manager_secret.oauth_state_secret.secret_id
      version    = "latest"
    }
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
}

resource "google_cloud_run_service_iam_member" "fitbit_oauth_handler_invoker" {
  project  = google_cloudfunctions2_function.fitbit_oauth_handler.project
  location = google_cloudfunctions2_function.fitbit_oauth_handler.location
  service  = google_cloudfunctions2_function.fitbit_oauth_handler.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}
