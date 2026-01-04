resource "google_identity_platform_config" "default" {
  project = var.project_id

  sign_in {
    allow_duplicate_emails = false

    email {
      enabled           = true
      password_required = true
    }

    # Explicitly disable phone number authentication to prevent drift
    phone_number {
      enabled = false
    }
  }

  # Explicitly disable multi-tenancy to prevent drift
  multi_tenant {
    allow_tenants = false
  }
}
