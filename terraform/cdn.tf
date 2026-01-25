# Cloud Load Balancer for Showcase Assets CDN
# Serves files from showcase_assets_bucket via assets.dev.fitglue.tech

# Backend bucket pointing to the showcase assets GCS bucket
resource "google_compute_backend_bucket" "showcase_assets_cdn" {
  name        = "${var.project_id}-showcase-assets-cdn"
  bucket_name = google_storage_bucket.showcase_assets_bucket.name
  enable_cdn  = true

  cdn_policy {
    cache_mode        = "CACHE_ALL_STATIC"
    default_ttl       = 3600
    max_ttl           = 86400
    client_ttl        = 3600
    negative_caching  = true
    serve_while_stale = 86400
  }
}

# Managed SSL certificate for assets subdomain
# Domain pattern: dev -> assets.dev.fitglue.tech, test -> assets.test.fitglue.tech, prod -> assets.fitglue.tech
resource "google_compute_managed_ssl_certificate" "showcase_assets_cert" {
  name = "${var.project_id}-showcase-assets-cert"

  managed {
    domains = [var.environment == "prod" ? "assets.fitglue.tech" : "assets.${var.environment}.fitglue.tech"]
  }
}

# URL map routing all traffic to the backend bucket
resource "google_compute_url_map" "showcase_assets_lb" {
  name            = "${var.project_id}-showcase-assets-lb"
  default_service = google_compute_backend_bucket.showcase_assets_cdn.id
}

# HTTPS proxy with SSL certificate
resource "google_compute_target_https_proxy" "showcase_assets_https_proxy" {
  name             = "${var.project_id}-showcase-assets-https-proxy"
  url_map          = google_compute_url_map.showcase_assets_lb.id
  ssl_certificates = [google_compute_managed_ssl_certificate.showcase_assets_cert.id]
}

# Global forwarding rule (creates the external IP)
resource "google_compute_global_forwarding_rule" "showcase_assets_https" {
  name        = "${var.project_id}-showcase-assets-https"
  target      = google_compute_target_https_proxy.showcase_assets_https_proxy.id
  port_range  = "443"
  ip_protocol = "TCP"
}

# HTTP to HTTPS redirect
resource "google_compute_url_map" "showcase_assets_http_redirect" {
  name = "${var.project_id}-showcase-assets-http-redirect"

  default_url_redirect {
    https_redirect         = true
    redirect_response_code = "MOVED_PERMANENTLY_DEFAULT"
    strip_query            = false
  }
}

resource "google_compute_target_http_proxy" "showcase_assets_http_proxy" {
  name    = "${var.project_id}-showcase-assets-http-proxy"
  url_map = google_compute_url_map.showcase_assets_http_redirect.id
}

resource "google_compute_global_forwarding_rule" "showcase_assets_http" {
  name        = "${var.project_id}-showcase-assets-http"
  target      = google_compute_target_http_proxy.showcase_assets_http_proxy.id
  port_range  = "80"
  ip_protocol = "TCP"
}

# Output the IP address for DNS configuration
output "showcase_assets_cdn_ip" {
  description = "IP address for assets.dev.fitglue.tech DNS A record"
  value       = google_compute_global_forwarding_rule.showcase_assets_https.ip_address
}
