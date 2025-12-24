output "fitglue_tech_name_servers" {
  description = "Name servers for the environment's zone. Configure these at your registrar (for Prod) or in the Prod zone's NS records (for Dev/Test)."
  value       = google_dns_managed_zone.main.name_servers
}
