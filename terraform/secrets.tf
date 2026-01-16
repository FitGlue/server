# Add IAM binding to allow Cloud Functions (Default SA) to access these secrets
data "google_project" "project" {
}

resource "google_project_iam_member" "secret_accessor" {
  project = var.project_id
  role    = "roles/secretmanager.secretAccessor"
  member  = "serviceAccount:${data.google_project.project.number}-compute@developer.gserviceaccount.com"
}

# OAuth State Secret (for CSRF protection)
resource "google_secret_manager_secret" "oauth_state_secret" {
  secret_id = "oauth-state-secret"
  replication {
    auto {}
  }
}

# Strava OAuth Credentials
resource "google_secret_manager_secret" "strava_client_id" {
  secret_id = "strava-client-id"
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

# Fitbit OAuth Credentials
resource "google_secret_manager_secret" "fitbit_client_id" {
  secret_id = "fitbit-client-id"
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

resource "google_secret_manager_secret" "fitbit_verification_code" {
  secret_id = "fitbit-verification-code"
  replication {
    auto {}
  }
}

# Stripe Billing Secrets
resource "google_secret_manager_secret" "stripe_secret_key" {
  secret_id = "stripe-secret-key"
  replication {
    auto {}
  }
}

resource "google_secret_manager_secret" "stripe_webhook_secret" {
  secret_id = "stripe-webhook-secret"
  replication {
    auto {}
  }
}

resource "google_secret_manager_secret" "stripe_price_id" {
  secret_id = "stripe-price-id"
  replication {
    auto {}
  }
}

