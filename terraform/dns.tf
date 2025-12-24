locals {
  # Prod gets the root domain, others get subdomains
  dns_zone_name = var.environment == "prod" ? "fitglue-tech" : "${var.environment}-fitglue-tech"
  dns_name      = var.environment == "prod" ? "fitglue.tech." : "${var.environment}.fitglue.tech."
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
