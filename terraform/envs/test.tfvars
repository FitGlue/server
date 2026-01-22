project_id  = "fitglue-server-test"
region      = "us-central1"
environment = "test"
log_level    = "info"
retry_policy = "RETRY_POLICY_DO_NOT_RETRY"
domain_name  = "test.fitglue.tech"
base_url     = "https://test.fitglue.tech"

# Sentry Configuration
release_version = "test"
sentry_org      = "fitglue"  # Replace with your Sentry organization slug
sentry_project  = "server"
sentry_dsn      = "https://4d64d33ef9f4877b7b18645930a9ec79@o4510752869318656.ingest.de.sentry.io/4510752888520784"
