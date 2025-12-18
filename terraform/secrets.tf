resource "google_secret_manager_secret" "hevy_api_key" {
  secret_id = "hevy-api-key"
  replication {
    auto {}
  }
}

resource "google_secret_manager_secret" "keiser_credentials" {
  secret_id = "keiser-credentials" # JSON blob: {username, password}
  replication {
    auto {}
  }
}

resource "google_secret_manager_secret" "fitbit_client_secret" {
  secret_id = "fitbit-client-secret"
  replication {
    auto {}
  }
}

resource "google_secret_manager_secret" "strava_client_secret" {
  secret_id = "strava-client-secret"
  replication {
    auto {}
  }
}

# Add IAM binding to allow Cloud Functions to access these secrets (later)
