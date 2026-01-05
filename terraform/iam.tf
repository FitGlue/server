resource "google_project_iam_member" "cloud_function_sa_datastore_user" {
  project = var.project_id
  role    = "roles/datastore.user"
  member  = "serviceAccount:${google_service_account.cloud_function_sa.email}"
}

resource "google_project_iam_member" "cloud_function_sa_pubsub_publisher" {
  project = var.project_id
  role    = "roles/pubsub.publisher"
  member  = "serviceAccount:${google_service_account.cloud_function_sa.email}"
}

resource "google_project_iam_member" "cloud_function_sa_secret_accessor" {
  project = var.project_id
  role    = "roles/secretmanager.secretAccessor"
  member  = "serviceAccount:${google_service_account.cloud_function_sa.email}"
}

resource "google_project_iam_member" "cloud_function_sa_storage_admin" {
  project = var.project_id
  role    = "roles/storage.objectAdmin"
  member  = "serviceAccount:${google_service_account.cloud_function_sa.email}"
}

resource "google_project_iam_member" "cloud_function_sa_fcm_admin" {
  project = var.project_id
  role    = "roles/firebase.messaging.admin"
  member  = "serviceAccount:${google_service_account.cloud_function_sa.email}"
}
