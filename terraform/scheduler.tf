resource "google_service_account" "scheduler_sa" {
  account_id   = "keiser-scheduler-sa"
  display_name = "Cloud Scheduler SA for Keiser"
}

resource "google_cloud_scheduler_job" "keiser_job" {
  name             = "keiser-poller-job"
  description      = "Triggers Keiser Poller every 15 minutes"
  schedule         = "*/15 * * * *"
  time_zone        = "UTC"
  attempt_deadline = "320s"

  http_target {
    http_method = "POST"
    uri         = google_cloudfunctions2_function.keiser_poller.service_config[0].uri

    oidc_token {
      service_account_email = google_service_account.scheduler_sa.email
    }
  }
}

# IAM Binding to allow Scheduler SA to invoke Function
resource "google_cloud_run_service_iam_member" "scheduler_invoker" {
    project = var.project_id
    location = var.region
    service = google_cloudfunctions2_function.keiser_poller.name
    # Note: Gen 2 names are technically `projects/*/locations/*/functions/*` but IAM binds to the underlying Run service
    role = "roles/run.invoker"
    member = "serviceAccount:${google_service_account.scheduler_sa.email}"
}
