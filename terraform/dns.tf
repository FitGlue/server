locals {
  # Prod gets the root domain, others get subdomains
  dns_zone_name = var.environment == "prod" ? "fitglue-tech" : "${var.environment}-fitglue-tech"
  dns_name      = var.environment == "prod" ? "fitglue.tech." : "${var.environment}.fitglue.tech."

  # Base URL for OAuth redirects
  base_url = var.environment == "prod" ? "https://fitglue.tech" : "https://${var.environment}.fitglue.tech"
}

resource "google_dns_managed_zone" "main" {
  name        = local.dns_zone_name
  dns_name    = local.dns_name
  description = "Managed zone for ${local.dns_name}"
  visibility  = "public"

  labels = {
    managed-by  = "terraform"
    environment = var.environment
  }

  depends_on = [
    google_project_service.apis
  ]
}

# Delegation for Dev subdomain (only in Prod)
resource "google_dns_record_set" "dev_delegation" {
  count        = var.environment == "prod" ? 1 : 0
  managed_zone = google_dns_managed_zone.main.name
  project      = var.project_id
  name         = "dev.fitglue.tech."
  type         = "NS"
  ttl          = 300

  rrdatas = [
    "ns-cloud-d1.googledomains.com.",
    "ns-cloud-d2.googledomains.com.",
    "ns-cloud-d3.googledomains.com.",
    "ns-cloud-d4.googledomains.com.",
  ]
}

# Delegation for Test subdomain (only in Prod)
# TODO: Update these nameservers after deploying Test environment
resource "google_dns_record_set" "test_delegation" {
  count        = var.environment == "prod" ? 1 : 0
  managed_zone = google_dns_managed_zone.main.name
  project      = var.project_id
  name         = "test.fitglue.tech."
  type         = "NS"
  ttl          = 300

  rrdatas = [
    "ns-cloud-a1.googledomains.com.",
    "ns-cloud-a2.googledomains.com.",
    "ns-cloud-a3.googledomains.com.",
    "ns-cloud-a4.googledomains.com.",
  ]
}

# Firebase Hosting Custom Domain
# Note: Firebase will provide DNS instructions after creation
# The custom domain resource will output the required DNS records
resource "google_firebase_hosting_custom_domain" "main" {
  provider      = google-beta
  project       = var.project_id
  site_id       = var.project_id
  custom_domain = var.domain_name

  # Don't wait for DNS verification in Terraform
  # We'll configure DNS manually based on Firebase's instructions
  wait_dns_verification = false

  depends_on = [
    google_dns_managed_zone.main
  ]
}
# DNS Records for Firebase Hosting
# All environments are zone apexes, so they must use A records (not CNAME)

# A record for Firebase Hosting (all environments)
resource "google_dns_record_set" "firebase_a" {
  managed_zone = google_dns_managed_zone.main.name
  name         = "${var.domain_name}."
  type         = "A"
  ttl          = 300
  rrdatas      = ["199.36.158.100"]
}

# TXT record for domain verification (all environments)
resource "google_dns_record_set" "firebase_txt" {
  managed_zone = google_dns_managed_zone.main.name
  name         = "${var.domain_name}."
  type         = "TXT"
  ttl          = 300
  rrdatas      = ["hosting-site=${var.project_id}"]
}

# Assets subdomain CNAME - points to GCS bucket for showcase assets (banners, thumbnails)
# Format: assets.{env}.fitglue.tech -> c.storage.googleapis.com
resource "google_dns_record_set" "assets_cname" {
  managed_zone = google_dns_managed_zone.main.name
  name         = "assets.${var.domain_name}."
  type         = "CNAME"
  ttl          = 300
  rrdatas      = ["c.storage.googleapis.com."]
}
