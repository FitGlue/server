# =============================================================================
# GCP Monitoring Dashboards and Alert Policies for FitGlue
# =============================================================================

locals {
  # --- Core 10 Go Services ---
  gateway_services = [
    "api-admin",
    "api-client",
    "api-public",
    "api-webhook"
  ]
  core_services = [
    "activity",
    "billing",
    "destination",
    "pipeline",
    "registry",
    "user"
  ]
  all_monitored_services = concat(local.gateway_services, local.core_services)
}

# =============================================================================
# LOG-BASED METRICS
# =============================================================================

resource "google_logging_metric" "pipeline_execution_status" {
  name        = "pipeline_execution_status"
  description = "Pipeline execution outcomes extracted from pipeline logs"
  filter      = <<-EOT
    resource.type="cloud_run_revision"
    resource.labels.service_name="pipeline"
    jsonPayload.status=~"SUCCESS|FAILED|SKIPPED"
  EOT

  metric_descriptor {
    metric_kind = "DELTA"
    value_type  = "INT64"
    labels {
      key         = "status"
      value_type  = "STRING"
      description = "Execution status"
    }
  }
  label_extractors = {
    "status" = "EXTRACT(jsonPayload.status)"
  }
}

resource "google_logging_metric" "destination_upload_status" {
  name        = "destination_upload_status"
  description = "Destination upload status and volume"
  filter      = <<-EOT
    resource.type="cloud_run_revision"
    resource.labels.service_name="destination"
    jsonPayload.destination != ""
  EOT

  metric_descriptor {
    metric_kind = "DELTA"
    value_type  = "INT64"
    labels {
      key         = "provider"
      value_type  = "STRING"
      description = "Destination provider name"
    }
  }
  label_extractors = {
    "provider" = "EXTRACT(jsonPayload.destination)"
  }
}

resource "google_logging_metric" "webhook_ingress_source" {
  name        = "webhook_ingress_source"
  description = "Webhook ingress volume by source"
  filter      = <<-EOT
    resource.type="cloud_run_revision"
    resource.labels.service_name="api-webhook"
    jsonPayload.provider != ""
  EOT

  metric_descriptor {
    metric_kind = "DELTA"
    value_type  = "INT64"
    labels {
      key         = "source"
      value_type  = "STRING"
      description = "Webhook source provider"
    }
  }
  label_extractors = {
    "source" = "EXTRACT(jsonPayload.provider)"
  }
}

# =============================================================================
# ALERT POLICIES
# =============================================================================

resource "google_monitoring_notification_channel" "email" {
  display_name = "FitGlue Alerts Email"
  type         = "email"
  labels = {
    email_address = "alerts@fitglue.com"
  }
}

resource "google_monitoring_alert_policy" "high_error_rate" {
  display_name = "High Error Rate (>5%)"
  combiner     = "OR"

  conditions {
    display_name = "Error rate exceeds 5%"
    condition_threshold {
      filter          = <<-EOT
        resource.type="cloud_run_revision" AND
        metric.type="run.googleapis.com/request_count" AND
        metric.labels.response_code_class!="2xx"
      EOT
      duration        = "300s"
      comparison      = "COMPARISON_GT"
      threshold_value = 0.05

      aggregations {
        alignment_period     = "300s"
        per_series_aligner   = "ALIGN_RATE"
        cross_series_reducer = "REDUCE_SUM"
        group_by_fields      = ["resource.labels.service_name"]
      }

      denominator_filter = <<-EOT
        resource.type="cloud_run_revision" AND
        metric.type="run.googleapis.com/request_count"
      EOT

      denominator_aggregations {
        alignment_period     = "300s"
        per_series_aligner   = "ALIGN_RATE"
        cross_series_reducer = "REDUCE_SUM"
        group_by_fields      = ["resource.labels.service_name"]
      }
    }
  }

  notification_channels = [google_monitoring_notification_channel.email.id]
  alert_strategy {
    auto_close = "604800s"
  }
}

# We split the 10 services into 2 groups to avoid the 6-condition limit per policy
locals {
  group_1 = slice(local.all_monitored_services, 0, 5)
  group_2 = slice(local.all_monitored_services, 5, 10)
}

resource "google_monitoring_alert_policy" "critical_service_failure_1" {
  display_name = "Critical Service Failure (1/2)"
  combiner     = "OR"
  dynamic "conditions" {
    for_each = local.group_1
    content {
      display_name = "$${conditions.value} errors"
      condition_threshold {
        filter          = "resource.type=\"cloud_run_revision\" AND resource.labels.service_name=\"$${conditions.value}\" AND metric.type=\"run.googleapis.com/request_count\" AND metric.labels.response_code_class!=\"2xx\""
        duration        = "60s"
        comparison      = "COMPARISON_GT"
        threshold_value = 5
        aggregations {
          alignment_period   = "60s"
          per_series_aligner = "ALIGN_SUM"
        }
      }
    }
  }
  notification_channels = [google_monitoring_notification_channel.email.id]
}

resource "google_monitoring_alert_policy" "critical_service_failure_2" {
  display_name = "Critical Service Failure (2/2)"
  combiner     = "OR"
  dynamic "conditions" {
    for_each = local.group_2
    content {
      display_name = "$${conditions.value} errors"
      condition_threshold {
        filter          = "resource.type=\"cloud_run_revision\" AND resource.labels.service_name=\"$${conditions.value}\" AND metric.type=\"run.googleapis.com/request_count\" AND metric.labels.response_code_class!=\"2xx\""
        duration        = "60s"
        comparison      = "COMPARISON_GT"
        threshold_value = 5
        aggregations {
          alignment_period   = "60s"
          per_series_aligner = "ALIGN_SUM"
        }
      }
    }
  }
  notification_channels = [google_monitoring_notification_channel.email.id]
}

resource "google_monitoring_alert_policy" "high_latency" {
  display_name = "High Function Latency (p95 > 30s)"
  combiner     = "OR"

  conditions {
    display_name = "Function latency exceeds 30 seconds"
    condition_threshold {
      filter          = "resource.type=\"cloud_run_revision\" AND metric.type=\"run.googleapis.com/request_latencies\""
      duration        = "300s"
      comparison      = "COMPARISON_GT"
      threshold_value = 30000

      aggregations {
        alignment_period     = "300s"
        per_series_aligner   = "ALIGN_PERCENTILE_95"
        cross_series_reducer = "REDUCE_MAX"
        group_by_fields      = ["resource.labels.service_name"]
      }
    }
  }

  notification_channels = [google_monitoring_notification_channel.email.id]

  alert_strategy {
    auto_close = "86400s"
  }
}

# =============================================================================
# MAIN OPERATIONS DASHBOARD
# =============================================================================
resource "google_monitoring_dashboard" "operations" {
  dashboard_json = jsonencode({
    displayName = "FitGlue Operations Overview"
    labels      = { environment = var.environment }
    mosaicLayout = {
      columns = 12
      tiles = [
        # ----- Key Scorecards -----
        {
          width  = 4
          height = 2
          widget = {
            title = "Total Function Invocations (24h)"
            scorecard = {
              timeSeriesQuery = {
                timeSeriesFilter = {
                  filter      = "resource.type=\"cloud_run_revision\" AND metric.type=\"run.googleapis.com/request_count\""
                  aggregation = { alignmentPeriod = "86400s", perSeriesAligner = "ALIGN_SUM" }
                }
              }
            }
          }
        },
        {
          xPos   = 4
          width  = 4
          height = 2
          widget = {
            title = "Error Rate (1h)"
            scorecard = {
              timeSeriesQuery = {
                timeSeriesFilterRatio = {
                  numerator = {
                    filter      = "resource.type=\"cloud_run_revision\" AND metric.type=\"run.googleapis.com/request_count\" AND metric.labels.response_code_class!=\"2xx\""
                    aggregation = { alignmentPeriod = "3600s", perSeriesAligner = "ALIGN_SUM" }
                  }
                  denominator = {
                    filter      = "resource.type=\"cloud_run_revision\" AND metric.type=\"run.googleapis.com/request_count\""
                    aggregation = { alignmentPeriod = "3600s", perSeriesAligner = "ALIGN_SUM" }
                  }
                }
              }
            }
          }
        },
        {
          xPos   = 8
          width  = 4
          height = 2
          widget = {
            title = "Overall Median Latency (p50)"
            scorecard = {
              timeSeriesQuery = {
                timeSeriesFilter = {
                  filter      = "resource.type=\"cloud_run_revision\" AND metric.type=\"run.googleapis.com/request_latencies\""
                  aggregation = { alignmentPeriod = "3600s", perSeriesAligner = "ALIGN_PERCENTILE_50", crossSeriesReducer = "REDUCE_MEAN" }
                }
              }
            }
          }
        },
        # ----- Invocations Chart -----
        {
          yPos   = 2
          width  = 6
          height = 4
          widget = {
            title = "Function Invocations Over Time (Gateways)"
            xyChart = {
              dataSets = [{
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter      = "resource.type=\"cloud_run_revision\" AND metric.type=\"run.googleapis.com/request_count\""
                    aggregation = { alignmentPeriod = "300s", perSeriesAligner = "ALIGN_RATE", crossSeriesReducer = "REDUCE_SUM", groupByFields = ["resource.labels.service_name"] }
                  }
                }
                plotType       = "STACKED_AREA"
                legendTemplate = "$${resource.labels.service_name}"
              }]
              yAxis = { label = "Invocations/sec" }
            }
          }
        },
        {
          xPos   = 6
          yPos   = 2
          width  = 6
          height = 4
          widget = {
            title = "Errors by Function"
            xyChart = {
              dataSets = [{
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter      = "resource.type=\"cloud_run_revision\" AND metric.type=\"run.googleapis.com/request_count\" AND metric.labels.response_code_class!=\"2xx\""
                    aggregation = { alignmentPeriod = "300s", perSeriesAligner = "ALIGN_RATE", crossSeriesReducer = "REDUCE_SUM", groupByFields = ["resource.labels.service_name"] }
                  }
                }
                plotType       = "STACKED_BAR"
                legendTemplate = "$${resource.labels.service_name}"
              }]
              yAxis = { label = "Errors/sec" }
            }
          }
        }
      ]
    }
  })
}

# =============================================================================
# LATENCIES DASHBOARD
# =============================================================================
resource "google_monitoring_dashboard" "service_latencies" {
  dashboard_json = jsonencode({
    displayName = "FitGlue Service Latencies (Gateways & Core)"
    labels      = { environment = var.environment }
    mosaicLayout = {
      columns = 12
      tiles = [
        {
          width  = 12
          height = 5
          widget = {
            title = "Gateway API Latency (p95)"
            xyChart = {
              dataSets = [
                for svc in local.gateway_services : {
                  timeSeriesQuery = {
                    timeSeriesFilter = {
                      filter      = "resource.type=\"cloud_run_revision\" AND resource.labels.service_name=\"${svc}\" AND metric.type=\"run.googleapis.com/request_latencies\""
                      aggregation = { alignmentPeriod = "300s", perSeriesAligner = "ALIGN_PERCENTILE_95" }
                    }
                  }
                  plotType       = "LINE"
                  legendTemplate = svc
                }
              ]
              yAxis = { label = "Latency (ms)" }
            }
          }
        },
        {
          yPos   = 5
          width  = 12
          height = 5
          widget = {
            title = "Core Domain Latency (p95)"
            xyChart = {
              dataSets = [
                for svc in local.core_services : {
                  timeSeriesQuery = {
                    timeSeriesFilter = {
                      filter      = "resource.type=\"cloud_run_revision\" AND resource.labels.service_name=\"${svc}\" AND metric.type=\"run.googleapis.com/request_latencies\""
                      aggregation = { alignmentPeriod = "300s", perSeriesAligner = "ALIGN_PERCENTILE_95" }
                    }
                  }
                  plotType       = "LINE"
                  legendTemplate = svc
                }
              ]
              yAxis = { label = "Latency (ms)" }
            }
          }
        }
      ]
    }
  })
}

# =============================================================================
# BUSINESS METRICS DASHBOARD
# =============================================================================
resource "google_monitoring_dashboard" "business_metrics" {
  dashboard_json = jsonencode({
    displayName = "FitGlue Business Growth & Processing"
    labels      = { environment = var.environment }
    mosaicLayout = {
      columns = 12
      tiles = [
        {
          width  = 6
          height = 5
          widget = {
            title = "Webhook Ingress Volume by Provider"
            xyChart = {
              dataSets = [{
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter      = "metric.type=\"logging.googleapis.com/user/webhook_ingress_source\""
                    aggregation = { alignmentPeriod = "3600s", perSeriesAligner = "ALIGN_SUM", crossSeriesReducer = "REDUCE_SUM", groupByFields = ["metric.labels.source"] }
                  }
                }
                plotType       = "STACKED_BAR"
                legendTemplate = "$${metric.labels.source}"
              }]
              yAxis = { label = "Activities" }
            }
          }
        },
        {
          xPos   = 6
          width  = 6
          height = 5
          widget = {
            title = "Destination Sync Uploads by Provider"
            xyChart = {
              dataSets = [{
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter      = "metric.type=\"logging.googleapis.com/user/destination_upload_status\""
                    aggregation = { alignmentPeriod = "3600s", perSeriesAligner = "ALIGN_SUM", crossSeriesReducer = "REDUCE_SUM", groupByFields = ["metric.labels.provider"] }
                  }
                }
                plotType       = "STACKED_BAR"
                legendTemplate = "$${metric.labels.provider}"
              }]
              yAxis = { label = "Uploads" }
            }
          }
        }
      ]
    }
  })
}

# =============================================================================
# ERRORS AND TRACES HUB
# =============================================================================
resource "google_monitoring_dashboard" "errors_traces" {
  dashboard_json = jsonencode({
    displayName = "FitGlue Errors & Traces Hub"
    labels      = { environment = var.environment }
    mosaicLayout = {
      columns = 12
      tiles = [
        {
          width  = 12
          height = 5
          widget = {
            title = "High Level Error Landscape (Any non-2xx responses)"
            xyChart = {
              dataSets = [{
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter      = "resource.type=\"cloud_run_revision\" AND metric.type=\"run.googleapis.com/request_count\" AND metric.labels.response_code_class!=\"2xx\""
                    aggregation = { alignmentPeriod = "300s", perSeriesAligner = "ALIGN_RATE", crossSeriesReducer = "REDUCE_SUM", groupByFields = ["resource.labels.service_name", "metric.labels.response_code_class"] }
                  }
                }
                plotType       = "STACKED_BAR"
                legendTemplate = "$${resource.labels.service_name} ($${metric.labels.response_code_class})"
              }]
              yAxis = { label = "Errors/sec" }
            }
          }
        },
        {
          yPos   = 5
          width  = 12
          height = 7
          widget = {
            title = "Root Cause Error Logs (Traces with severity >= ERROR from all Services)"
            logsPanel = {
              filter = "resource.type=\"cloud_run_revision\" severity>=ERROR"
              resourceNames = [
                "projects/${var.project_id}"
              ]
            }
          }
        }
      ]
    }
  })
}
