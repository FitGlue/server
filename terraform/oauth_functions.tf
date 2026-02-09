# ----------------- Strava OAuth Handler -----------------

resource "google_cloudfunctions2_function" "strava_oauth_handler" {
  name        = "strava-oauth-handler"
  location    = var.region
  description = "Handles Strava OAuth callback"

  build_config {
    runtime     = "nodejs22"
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
    runtime     = "nodejs22"
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

# ----------------- Spotify OAuth Handler -----------------

resource "google_cloudfunctions2_function" "spotify_oauth_handler" {
  name        = "spotify-oauth-handler"
  location    = var.region
  description = "Handles Spotify OAuth callback"

  build_config {
    runtime     = "nodejs22"
    entry_point = "spotifyOAuthHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.spotify_oauth_handler_zip.name
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
    service_account_email = google_service_account.cloud_function_sa.email
  }
}

resource "google_cloud_run_service_iam_member" "spotify_oauth_handler_invoker" {
  project  = google_cloudfunctions2_function.spotify_oauth_handler.project
  location = google_cloudfunctions2_function.spotify_oauth_handler.location
  service  = google_cloudfunctions2_function.spotify_oauth_handler.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# ----------------- TrainingPeaks OAuth Handler -----------------

resource "google_cloudfunctions2_function" "trainingpeaks_oauth_handler" {
  name        = "trainingpeaks-oauth-handler"
  location    = var.region
  description = "Handles TrainingPeaks OAuth callback"

  build_config {
    runtime     = "nodejs22"
    entry_point = "trainingPeaksOAuthHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.trainingpeaks_oauth_handler_zip.name
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
}

resource "google_cloud_run_service_iam_member" "trainingpeaks_oauth_handler_invoker" {
  project  = google_cloudfunctions2_function.trainingpeaks_oauth_handler.project
  location = google_cloudfunctions2_function.trainingpeaks_oauth_handler.location
  service  = google_cloudfunctions2_function.trainingpeaks_oauth_handler.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# ----------------- Wahoo OAuth Handler -----------------

resource "google_cloudfunctions2_function" "wahoo_oauth_handler" {
  name        = "wahoo-oauth-handler"
  location    = var.region
  description = "Handles Wahoo OAuth callback"

  build_config {
    runtime     = "nodejs22"
    entry_point = "wahooOAuthHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.wahoo_oauth_handler_zip.name
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

resource "google_cloud_run_service_iam_member" "wahoo_oauth_handler_invoker" {
  project  = google_cloudfunctions2_function.wahoo_oauth_handler.project
  location = google_cloudfunctions2_function.wahoo_oauth_handler.location
  service  = google_cloudfunctions2_function.wahoo_oauth_handler.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# ----------------- Polar OAuth Handler -----------------

resource "google_cloudfunctions2_function" "polar_oauth_handler" {
  name        = "polar-oauth-handler"
  location    = var.region
  description = "Handles Polar OAuth callback"

  build_config {
    runtime     = "nodejs22"
    entry_point = "polarOAuthHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.polar_oauth_handler_zip.name
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

resource "google_cloud_run_service_iam_member" "polar_oauth_handler_invoker" {
  project  = google_cloudfunctions2_function.polar_oauth_handler.project
  location = google_cloudfunctions2_function.polar_oauth_handler.location
  service  = google_cloudfunctions2_function.polar_oauth_handler.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# ----------------- Oura OAuth Handler -----------------

resource "google_cloudfunctions2_function" "oura_oauth_handler" {
  name        = "oura-oauth-handler"
  location    = var.region
  description = "Handles Oura OAuth callback"

  build_config {
    runtime     = "nodejs22"
    entry_point = "ouraOAuthHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.oura_oauth_handler_zip.name
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

resource "google_cloud_run_service_iam_member" "oura_oauth_handler_invoker" {
  project  = google_cloudfunctions2_function.oura_oauth_handler.project
  location = google_cloudfunctions2_function.oura_oauth_handler.location
  service  = google_cloudfunctions2_function.oura_oauth_handler.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# ----------------- Google OAuth Handler -----------------

resource "google_cloudfunctions2_function" "google_oauth_handler" {
  name        = "google-oauth-handler"
  location    = var.region
  description = "Handles Google OAuth callback for Sheets integration"

  build_config {
    runtime     = "nodejs22"
    entry_point = "googleOAuthHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.google_oauth_handler_zip.name
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
}

resource "google_cloud_run_service_iam_member" "google_oauth_handler_invoker" {
  project  = google_cloudfunctions2_function.google_oauth_handler.project
  location = google_cloudfunctions2_function.google_oauth_handler.location
  service  = google_cloudfunctions2_function.google_oauth_handler.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# ----------------- GitHub OAuth Handler -----------------

resource "google_cloudfunctions2_function" "github_oauth_handler" {
  name        = "github-oauth-handler"
  location    = var.region
  description = "Handles GitHub OAuth callback"

  build_config {
    runtime     = "nodejs22"
    entry_point = "githubOAuthHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.github_oauth_handler_zip.name
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

resource "google_cloud_run_service_iam_member" "github_oauth_handler_invoker" {
  project  = google_cloudfunctions2_function.github_oauth_handler.project
  location = google_cloudfunctions2_function.github_oauth_handler.location
  service  = google_cloudfunctions2_function.github_oauth_handler.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}
