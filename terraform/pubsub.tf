resource "google_pubsub_topic" "raw_activity" {
  name    = "topic-raw-activity"
  project = var.project_id

  # Enable message retention for replay (1 hour)
  message_retention_duration = "3600s"
}

resource "google_pubsub_topic" "enriched_activity" {
  name    = "topic-enriched-activity"
  project = var.project_id

  # Enable message retention for replay (1 hour)
  message_retention_duration = "3600s"
}

resource "google_pubsub_topic" "job_upload_strava" {
  name    = "topic-job-upload-strava"
  project = var.project_id

  # Enable message retention for replay (1 hour)
  message_retention_duration = "3600s"
}

resource "google_pubsub_topic" "enrichment_lag" {
  name    = "topic-enrichment-lag"
  project = var.project_id
}

# Mock topic for testing (dev only)
resource "google_pubsub_topic" "job_upload_mock" {
  count   = var.environment == "dev" ? 1 : 0
  name    = "topic-job-upload-mock"
  project = var.project_id
}

# Showcase topic for public shareable activity URLs (all environments)
resource "google_pubsub_topic" "job_upload_showcase" {
  name    = "topic-job-upload-showcase"
  project = var.project_id

  message_retention_duration = "3600s"
}


resource "google_pubsub_subscription" "enrichment_lag_sub" {
  name    = "sub-enrichment-lag"
  topic   = google_pubsub_topic.enrichment_lag.name
  project = var.project_id

  # 20 minutes max retention (or longer to be safe, e.g. 1h, if backoff is long)
  message_retention_duration = "3600s"

  retry_policy {
    # 60s minimum backoff
    minimum_backoff = "60s"
    # 10 minutes max backoff
    maximum_backoff = "600s"
  }

  # Use a Push Subscription to enforce a custom retry policy (backoff).
  # Standard EventArc triggers created by Cloud Functions do not support granular backoff configuration.
  # We use a separate HTTP-triggered function that properly returns HTTP 500 on errors.

  push_config {
    push_endpoint = google_cloudfunctions2_function.enricher_lag.service_config[0].uri

    oidc_token {
      service_account_email = google_service_account.cloud_function_sa.email
    }
  }
}

# NOTE: topic-job-update-strava removed - UPDATE operations now use the standard strava-uploader topic


resource "google_pubsub_topic" "parkrun_results_trigger" {
  name    = "topic-parkrun-results-trigger"
  project = var.project_id
}
