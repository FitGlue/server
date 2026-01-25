resource "google_firebase_web_app" "web" {
  provider     = google-beta
  project      = var.project_id
  display_name = "fitglue-web-app"
}

data "google_firebase_web_app_config" "web" {
  provider   = google-beta
  web_app_id = google_firebase_web_app.web.app_id
}

resource "google_firebase_hosting_site" "default" {
  provider = google-beta
  project  = var.project_id
  site_id  = var.project_id # Default site
  app_id   = google_firebase_web_app.web.app_id
}
