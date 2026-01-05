resource "google_firebase_web_app" "web" {
  provider = google-beta
  project  = var.project_id
  display_name = "fitglue-web"
}
