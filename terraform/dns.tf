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
}
