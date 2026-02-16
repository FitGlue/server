project_id  = "fitglue-server-prod"
region      = "us-central1"
environment = "prod"
log_level    = "info"
retry_policy = "RETRY_POLICY_DO_NOT_RETRY"
domain_name  = "fitglue.tech"
base_url     = "https://fitglue.tech"

# Sentry Configuration
sentry_org      = "fitglue"  # Replace with your Sentry organization slug
sentry_project  = "server"
sentry_dsn      = "https://4d64d33ef9f4877b7b18645930a9ec79@o4510752869318656.ingest.de.sentry.io/4510752888520784"
