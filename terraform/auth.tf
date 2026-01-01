resource "google_identity_platform_project_default_config" "default" {
  project = var.project_id

  sign_in {
    allow_duplicate_emails = false

    email {
      enabled = true
      password_required = true
    }
  }
}
