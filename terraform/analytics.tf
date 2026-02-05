# =============================================================================
# BigQuery Analytics Infrastructure for FitGlue
# =============================================================================
#
# This file creates:
# 1. BigQuery dataset for analytics
# 2. Log sink to export Cloud Logging data to BigQuery
# 3. Firestore export configuration (scheduled via Cloud Scheduler)
#
# Looker Studio dashboards connect to this BigQuery dataset.
# =============================================================================

# =============================================================================
# BIGQUERY DATASET
# =============================================================================

resource "google_bigquery_dataset" "analytics" {
  dataset_id    = "fitglue_analytics"
  friendly_name = "FitGlue Analytics"
  description   = "Analytics data for FitGlue business dashboards including logs, user metrics, and pipeline execution data"
  location      = var.region

  labels = {
    environment = var.environment
    purpose     = "analytics"
  }

  # Default table expiration: 90 days for log data
  # Can be overridden per-table
  default_table_expiration_ms = 7776000000 # 90 days in ms

  # Access is managed via project-level IAM roles
  # The log sink writer identity is granted access separately below
}

# =============================================================================
# LOG SINK TO BIGQUERY
# =============================================================================
# Exports structured Cloud Logging data to BigQuery for analysis

resource "google_logging_project_sink" "bigquery_analytics" {
  name        = "fitglue-analytics-sink"
  description = "Exports Cloud Function logs to BigQuery for analytics"

  # Export to our analytics dataset
  destination = "bigquery.googleapis.com/projects/${var.project_id}/datasets/${google_bigquery_dataset.analytics.dataset_id}"

  # Filter for relevant logs only (Cloud Functions with structured payloads)
  filter = <<-EOT
    resource.type="cloud_function" AND
    (
      jsonPayload.status IS NOT NULL OR
      jsonPayload.execution_id IS NOT NULL OR
      jsonPayload.provider IS NOT NULL OR
      jsonPayload.user_id IS NOT NULL
    )
  EOT

  # Use partitioned tables for better query performance and cost
  bigquery_options {
    use_partitioned_tables = true
  }

  # Create unique writer identity for this sink
  unique_writer_identity = true
}

# Grant the log sink permission to write to BigQuery
resource "google_bigquery_dataset_iam_member" "log_sink_writer" {
  dataset_id = google_bigquery_dataset.analytics.dataset_id
  role       = "roles/bigquery.dataEditor"
  member     = google_logging_project_sink.bigquery_analytics.writer_identity
}

# =============================================================================
# PLACEHOLDER TABLE FOR WILDCARD VIEWS
# =============================================================================
# BigQuery requires at least one table to match a wildcard pattern before views
# can reference it. The log sink creates cloudfunction_YYYYMMDD tables when
# logs flow in, but on first apply no tables exist yet. This placeholder
# allows the views to be created; it will be ignored once real log data arrives.
# =============================================================================

resource "google_bigquery_table" "cloudfunction_placeholder" {
  dataset_id = google_bigquery_dataset.analytics.dataset_id
  table_id   = "cloudfunction_20200101"

  schema = jsonencode([
    { name = "timestamp", type = "TIMESTAMP", mode = "NULLABLE" },
    { name = "severity", type = "STRING", mode = "NULLABLE" },
    { name = "jsonPayload", type = "STRING", mode = "NULLABLE" },
    {
      name   = "resource",
      type   = "RECORD",
      mode   = "NULLABLE",
      fields = [
        { name = "type", type = "STRING", mode = "NULLABLE" },
        {
          name   = "labels",
          type   = "RECORD",
          mode   = "NULLABLE",
          fields = [{ name = "function_name", type = "STRING", mode = "NULLABLE" }]
        }
      ]
    }
  ])

  labels = {
    purpose = "analytics-placeholder"
  }

  depends_on = [google_bigquery_dataset.analytics]
}

# =============================================================================
# BIGQUERY VIEWS FOR COMMON QUERIES
# =============================================================================

# View: Daily pipeline execution summary
resource "google_bigquery_table" "pipeline_summary_view" {
  dataset_id = google_bigquery_dataset.analytics.dataset_id
  table_id   = "v_pipeline_summary"

  view {
    query          = <<-SQL
      SELECT
        DATE(timestamp) AS date,
        JSON_EXTRACT_SCALAR(jsonPayload, '$.status') AS status,
        COUNT(*) AS execution_count,
        COUNTIF(JSON_EXTRACT_SCALAR(jsonPayload, '$.status') = 'SUCCESS') AS success_count,
        COUNTIF(JSON_EXTRACT_SCALAR(jsonPayload, '$.status') = 'FAILED') AS failed_count,
        COUNTIF(JSON_EXTRACT_SCALAR(jsonPayload, '$.status') = 'SKIPPED') AS skipped_count
      FROM `${var.project_id}.${google_bigquery_dataset.analytics.dataset_id}.cloudfunction_*`
      WHERE
        resource.labels.function_name = 'enricher'
        AND JSON_EXTRACT_SCALAR(jsonPayload, '$.status') IS NOT NULL
      GROUP BY date, status
      ORDER BY date DESC
    SQL
    use_legacy_sql = false
  }

  labels = {
    purpose = "analytics"
  }

  depends_on = [google_bigquery_dataset.analytics, google_bigquery_table.cloudfunction_placeholder]
}

# View: Enricher provider popularity
resource "google_bigquery_table" "enricher_popularity_view" {
  dataset_id = google_bigquery_dataset.analytics.dataset_id
  table_id   = "v_enricher_popularity"

  view {
    query          = <<-SQL
      SELECT
        DATE(timestamp) AS date,
        JSON_EXTRACT_SCALAR(jsonPayload, '$.name') AS provider_name,
        COUNT(*) AS execution_count,
        AVG(CAST(JSON_EXTRACT_SCALAR(jsonPayload, '$.duration_ms') AS FLOAT64)) AS avg_duration_ms,
        APPROX_QUANTILES(CAST(JSON_EXTRACT_SCALAR(jsonPayload, '$.duration_ms') AS FLOAT64), 100)[OFFSET(95)] AS p95_duration_ms
      FROM `${var.project_id}.${google_bigquery_dataset.analytics.dataset_id}.cloudfunction_*`
      WHERE
        resource.labels.function_name = 'enricher'
        AND JSON_EXTRACT_SCALAR(jsonPayload, '$.message') LIKE 'Provider completed%'
        AND JSON_EXTRACT_SCALAR(jsonPayload, '$.name') IS NOT NULL
      GROUP BY date, provider_name
      ORDER BY date DESC, execution_count DESC
    SQL
    use_legacy_sql = false
  }

  labels = {
    purpose = "analytics"
  }

  depends_on = [google_bigquery_dataset.analytics, google_bigquery_table.cloudfunction_placeholder]
}

# View: Daily active users (based on pipeline runs)
resource "google_bigquery_table" "daily_active_users_view" {
  dataset_id = google_bigquery_dataset.analytics.dataset_id
  table_id   = "v_daily_active_users"

  view {
    query          = <<-SQL
      SELECT
        DATE(timestamp) AS date,
        COUNT(DISTINCT JSON_EXTRACT_SCALAR(jsonPayload, '$.user_id')) AS unique_users,
        COUNT(*) AS total_activities
      FROM `${var.project_id}.${google_bigquery_dataset.analytics.dataset_id}.cloudfunction_*`
      WHERE
        resource.labels.function_name = 'enricher'
        AND JSON_EXTRACT_SCALAR(jsonPayload, '$.user_id') IS NOT NULL
      GROUP BY date
      ORDER BY date DESC
    SQL
    use_legacy_sql = false
  }

  labels = {
    purpose = "analytics"
  }

  depends_on = [google_bigquery_dataset.analytics, google_bigquery_table.cloudfunction_placeholder]
}

# View: Weekly growth trends with week-over-week comparison
resource "google_bigquery_table" "weekly_growth_view" {
  dataset_id = google_bigquery_dataset.analytics.dataset_id
  table_id   = "v_weekly_growth"

  view {
    query          = <<-SQL
      WITH weekly_data AS (
        SELECT
          DATE_TRUNC(DATE(timestamp), WEEK) AS week_start,
          COUNT(DISTINCT JSON_EXTRACT_SCALAR(jsonPayload, '$.user_id')) AS unique_users,
          COUNT(*) AS total_activities,
          COUNTIF(JSON_EXTRACT_SCALAR(jsonPayload, '$.status') = 'SUCCESS') AS successful_syncs
        FROM `${var.project_id}.${google_bigquery_dataset.analytics.dataset_id}.cloudfunction_*`
        WHERE
          resource.labels.function_name = 'enricher'
          AND JSON_EXTRACT_SCALAR(jsonPayload, '$.user_id') IS NOT NULL
        GROUP BY week_start
      )
      SELECT
        week_start,
        unique_users,
        total_activities,
        successful_syncs,
        LAG(unique_users) OVER (ORDER BY week_start) AS prev_week_users,
        LAG(total_activities) OVER (ORDER BY week_start) AS prev_week_activities,
        ROUND(SAFE_DIVIDE(unique_users - LAG(unique_users) OVER (ORDER BY week_start), LAG(unique_users) OVER (ORDER BY week_start)) * 100, 1) AS user_growth_pct,
        ROUND(SAFE_DIVIDE(total_activities - LAG(total_activities) OVER (ORDER BY week_start), LAG(total_activities) OVER (ORDER BY week_start)) * 100, 1) AS activity_growth_pct
      FROM weekly_data
      ORDER BY week_start DESC
    SQL
    use_legacy_sql = false
  }

  labels = {
    purpose = "analytics"
  }

  depends_on = [google_bigquery_dataset.analytics, google_bigquery_table.cloudfunction_placeholder]
}

# View: Destination success rates by provider
resource "google_bigquery_table" "destination_success_view" {
  dataset_id = google_bigquery_dataset.analytics.dataset_id
  table_id   = "v_destination_success"

  view {
    query          = <<-SQL
      SELECT
        DATE(timestamp) AS date,
        resource.labels.function_name AS uploader,
        REPLACE(resource.labels.function_name, '-uploader', '') AS provider,
        COUNT(*) AS total_uploads,
        COUNTIF(severity = 'ERROR' OR severity = 'CRITICAL') AS failed_uploads,
        ROUND((1 - SAFE_DIVIDE(COUNTIF(severity = 'ERROR' OR severity = 'CRITICAL'), COUNT(*))) * 100, 1) AS success_rate_pct
      FROM `${var.project_id}.${google_bigquery_dataset.analytics.dataset_id}.cloudfunction_*`
      WHERE
        resource.labels.function_name LIKE '%-uploader'
      GROUP BY date, uploader, provider
      ORDER BY date DESC, total_uploads DESC
    SQL
    use_legacy_sql = false
  }

  labels = {
    purpose = "analytics"
  }

  depends_on = [google_bigquery_dataset.analytics, google_bigquery_table.cloudfunction_placeholder]
}

# View: Activity source distribution
resource "google_bigquery_table" "source_distribution_view" {
  dataset_id = google_bigquery_dataset.analytics.dataset_id
  table_id   = "v_source_distribution"

  view {
    query          = <<-SQL
      SELECT
        DATE(timestamp) AS date,
        COALESCE(JSON_EXTRACT_SCALAR(jsonPayload, '$.source'), resource.labels.function_name) AS source,
        COUNT(*) AS activity_count,
        COUNT(DISTINCT JSON_EXTRACT_SCALAR(jsonPayload, '$.user_id')) AS unique_users
      FROM `${var.project_id}.${google_bigquery_dataset.analytics.dataset_id}.cloudfunction_*`
      WHERE
        resource.labels.function_name IN ('strava-handler', 'fitbit-handler', 'polar-handler', 'wahoo-handler', 'oura-handler', 'hevy-handler', 'mobile-sync-handler')
      GROUP BY date, source
      ORDER BY date DESC, activity_count DESC
    SQL
    use_legacy_sql = false
  }

  labels = {
    purpose = "analytics"
  }

  depends_on = [google_bigquery_dataset.analytics, google_bigquery_table.cloudfunction_placeholder]
}

# View: Executive summary - key metrics for business dashboards
resource "google_bigquery_table" "executive_summary_view" {
  dataset_id = google_bigquery_dataset.analytics.dataset_id
  table_id   = "v_executive_summary"

  view {
    query          = <<-SQL
      SELECT
        'today' AS period,
        COUNT(DISTINCT CASE WHEN DATE(timestamp) = CURRENT_DATE() THEN JSON_EXTRACT_SCALAR(jsonPayload, '$.user_id') END) AS active_users,
        COUNTIF(DATE(timestamp) = CURRENT_DATE()) AS activities_processed,
        COUNTIF(DATE(timestamp) = CURRENT_DATE() AND JSON_EXTRACT_SCALAR(jsonPayload, '$.status') = 'SUCCESS') AS successful_syncs,
        COUNTIF(DATE(timestamp) = CURRENT_DATE() AND JSON_EXTRACT_SCALAR(jsonPayload, '$.status') = 'FAILED') AS failed_syncs
      FROM `${var.project_id}.${google_bigquery_dataset.analytics.dataset_id}.cloudfunction_*`
      WHERE resource.labels.function_name = 'enricher'

      UNION ALL

      SELECT
        'this_week' AS period,
        COUNT(DISTINCT CASE WHEN DATE(timestamp) >= DATE_SUB(CURRENT_DATE(), INTERVAL 7 DAY) THEN JSON_EXTRACT_SCALAR(jsonPayload, '$.user_id') END) AS active_users,
        COUNTIF(DATE(timestamp) >= DATE_SUB(CURRENT_DATE(), INTERVAL 7 DAY)) AS activities_processed,
        COUNTIF(DATE(timestamp) >= DATE_SUB(CURRENT_DATE(), INTERVAL 7 DAY) AND JSON_EXTRACT_SCALAR(jsonPayload, '$.status') = 'SUCCESS') AS successful_syncs,
        COUNTIF(DATE(timestamp) >= DATE_SUB(CURRENT_DATE(), INTERVAL 7 DAY) AND JSON_EXTRACT_SCALAR(jsonPayload, '$.status') = 'FAILED') AS failed_syncs
      FROM `${var.project_id}.${google_bigquery_dataset.analytics.dataset_id}.cloudfunction_*`
      WHERE resource.labels.function_name = 'enricher'

      UNION ALL

      SELECT
        'this_month' AS period,
        COUNT(DISTINCT CASE WHEN DATE(timestamp) >= DATE_SUB(CURRENT_DATE(), INTERVAL 30 DAY) THEN JSON_EXTRACT_SCALAR(jsonPayload, '$.user_id') END) AS active_users,
        COUNTIF(DATE(timestamp) >= DATE_SUB(CURRENT_DATE(), INTERVAL 30 DAY)) AS activities_processed,
        COUNTIF(DATE(timestamp) >= DATE_SUB(CURRENT_DATE(), INTERVAL 30 DAY) AND JSON_EXTRACT_SCALAR(jsonPayload, '$.status') = 'SUCCESS') AS successful_syncs,
        COUNTIF(DATE(timestamp) >= DATE_SUB(CURRENT_DATE(), INTERVAL 30 DAY) AND JSON_EXTRACT_SCALAR(jsonPayload, '$.status') = 'FAILED') AS failed_syncs
      FROM `${var.project_id}.${google_bigquery_dataset.analytics.dataset_id}.cloudfunction_*`
      WHERE resource.labels.function_name = 'enricher'
    SQL
    use_legacy_sql = false
  }

  labels = {
    purpose = "analytics"
  }

  depends_on = [google_bigquery_dataset.analytics, google_bigquery_table.cloudfunction_placeholder]
}

# =============================================================================
# FIRESTORE EXPORT TO BIGQUERY (via Cloud Scheduler)
# =============================================================================
# Note: Firestore native export requires creating exports to GCS first,
# then loading into BigQuery. This is typically done via:
# 1. Cloud Scheduler triggers a Cloud Function
# 2. Function runs: gcloud firestore export gs://bucket/path --collection-ids=users,pipelines
# 3. BigQuery Data Transfer loads from GCS
#
# For FitGlue, we recommend using the Firestore BigQuery Extension or
# setting up a scheduled export. Below is the infrastructure for the storage.

# GCS bucket for Firestore exports
resource "google_storage_bucket" "firestore_exports" {
  name     = "${var.project_id}-firestore-exports"
  location = var.region

  # Lifecycle rule to clean up old exports after 7 days
  lifecycle_rule {
    condition {
      age = 7
    }
    action {
      type = "Delete"
    }
  }

  labels = {
    purpose     = "firestore-exports"
    environment = var.environment
  }
}

# =============================================================================
# OUTPUT USEFUL INFORMATION
# =============================================================================

output "bigquery_dataset_id" {
  description = "BigQuery dataset ID for analytics"
  value       = google_bigquery_dataset.analytics.dataset_id
}

output "bigquery_analytics_url" {
  description = "URL to access the analytics dataset in BigQuery Console"
  value       = "https://console.cloud.google.com/bigquery?project=${var.project_id}&d=${google_bigquery_dataset.analytics.dataset_id}&p=${var.project_id}&page=dataset"
}

output "looker_studio_connection_info" {
  description = "Information for connecting Looker Studio to BigQuery"
  value = {
    project_id = var.project_id
    dataset_id = google_bigquery_dataset.analytics.dataset_id
    views = [
      "v_pipeline_summary",
      "v_enricher_popularity",
      "v_daily_active_users",
      "v_weekly_growth",
      "v_destination_success",
      "v_source_distribution",
      "v_executive_summary"
    ]
  }
}
