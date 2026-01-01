resource "google_pubsub_topic" "raw_activity" {
  name    = "topic-raw-activity"
  project = var.project_id
}

resource "google_pubsub_topic" "enriched_activity" {
  name    = "topic-enriched-activity"
  project = var.project_id
}

resource "google_pubsub_topic" "job_upload_strava" {
  name    = "topic-job-upload-strava"
  project = var.project_id
}

# Future extensibility
resource "google_pubsub_topic" "job_upload_other" {
  name    = "topic-job-upload-other"
  project = var.project_id
}

resource "google_pubsub_topic" "fitbit_updates" {
  name    = "topic-fitbit-updates"
  project = var.project_id
}

resource "google_pubsub_topic" "enrichment_lag" {
  name    = "topic-enrichment-lag"
  project = var.project_id
}

resource "google_pubsub_subscription" "enrichment_lag_sub" {
  name  = "sub-enrichment-lag"
  topic = google_pubsub_topic.enrichment_lag.name
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
  # We manually configure this subscription to push to the function's endpoint.

  push_config {
    push_endpoint = google_cloudfunctions2_function.enricher.service_config[0].uri

    # Auth is handled by the OIDC token automatically if we configure it?
    # Actually, simpler: The function is internal-only usually?
    # We will need the service account.
    oidc_token {
      service_account_email = google_service_account.cloud_function_sa.email
    }
  }
}
