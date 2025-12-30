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
