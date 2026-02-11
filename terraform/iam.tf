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
  role    = "roles/firebasecloudmessaging.admin"
  member  = "serviceAccount:${google_service_account.cloud_function_sa.email}"
}

# Firebase Auth Admin - allows GetUser() for display name lookup AND deleteUser() for account deletion
resource "google_project_iam_member" "cloud_function_sa_firebase_auth_admin" {
  project = var.project_id
  role    = "roles/firebaseauth.admin"
  member  = "serviceAccount:${google_service_account.cloud_function_sa.email}"
}

# AI Platform User - allows enricher to call Vertex AI Imagen API for AI Banner generation
resource "google_project_iam_member" "cloud_function_sa_aiplatform_user" {
  project = var.project_id
  role    = "roles/aiplatform.user"
  member  = "serviceAccount:${google_service_account.cloud_function_sa.email}"
}

# Service Account User - allows Cloud Function SA to create Cloud Tasks with OIDC tokens
# targeting the App Engine default service account
resource "google_project_iam_member" "cloud_function_sa_service_account_user" {
  project = var.project_id
  role    = "roles/iam.serviceAccountUser"
  member  = "serviceAccount:${google_service_account.cloud_function_sa.email}"
}

# Service Account Token Creator - allows getSignedUrl() for GCS objects (data export, etc.)
resource "google_project_iam_member" "cloud_function_sa_token_creator" {
  project = var.project_id
  role    = "roles/iam.serviceAccountTokenCreator"
  member  = "serviceAccount:${google_service_account.cloud_function_sa.email}"
}

resource "google_project_iam_member" "web_deployer_run_viewer" {
  project = var.project_id
  role    = "roles/run.viewer"
  member  = "serviceAccount:circleci-web-deployer@${var.project_id}.iam.gserviceaccount.com"
}

# Service Usage Consumer - allows Firebase CLI to check API enablement status during deploy
resource "google_project_iam_member" "web_deployer_service_usage_consumer" {
  project = var.project_id
  role    = "roles/serviceusage.serviceUsageConsumer"
  member  = "serviceAccount:circleci-web-deployer@${var.project_id}.iam.gserviceaccount.com"
}

# Firebase Rules Admin - allows Firebase CLI to validate and deploy Firestore rules
resource "google_project_iam_member" "web_deployer_firebase_rules_admin" {
  project = var.project_id
  role    = "roles/firebaserules.admin"
  member  = "serviceAccount:circleci-web-deployer@${var.project_id}.iam.gserviceaccount.com"
}
