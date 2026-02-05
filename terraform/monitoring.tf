# =============================================================================
# GCP Monitoring Dashboards and Alert Policies for FitGlue
# =============================================================================
#
# This file creates:
# 1. Main Operations Dashboard - Overall system health
# 2. Handler Performance Dashboard - Per-function metrics
# 3. Provider Latency Dashboard - API call performance by destination
# 4. Alert policies for critical errors
#
# Log-based metrics extract custom dimensions from structured Cloud Logging.
# =============================================================================

locals {
  # ----- Handler Groups for Dashboard Organization -----

  # TypeScript API Handlers (user-facing)
  ts_api_handlers = [
    "activities-handler",
    "admin-handler",
    "billing-handler",
    "inputs-handler",
    "registry-handler",
    "repost-handler",
    "showcase-handler",
    "user-data-handler",
    "user-integrations-handler",
    "user-pipelines-handler",
    "user-profile-handler",
  ]

  # TypeScript Integration Handlers (webhooks/sources)
  ts_integration_handlers = [
    "fitbit-handler",
    "hevy-handler",
    "oura-handler",
    "polar-handler",
    "strava-handler",
    "wahoo-handler",
    "mobile-sync-handler",
    "integration-request-handler",
  ]

  # TypeScript OAuth Handlers
  ts_oauth_handlers = [
    "fitbit-oauth-handler",
    "google-oauth-handler",
    "oura-oauth-handler",
    "polar-oauth-handler",
    "spotify-oauth-handler",
    "strava-oauth-handler",
    "trainingpeaks-oauth-handler",
    "wahoo-oauth-handler",
  ]

  # Go Pipeline Handlers (enrichment/upload)
  go_pipeline_handlers = [
    "enricher",
    "fit-parser-handler",
    "parkrun-results-source",
  ]

  # Go Uploaders (destination sync)
  go_uploaders = [
    "googlesheets-uploader",
    "hevy-uploader",
    "intervals-uploader",
    "showcase-uploader",
    "strava-uploader",
    "trainingpeaks-uploader",
  ]

  # All handlers combined for alert policies
  all_critical_handlers = concat(
    local.ts_integration_handlers,
    local.go_pipeline_handlers,
    local.go_uploaders
  )
}

# =============================================================================
# LOG-BASED METRICS
# =============================================================================
# These extract custom metrics from structured logs for advanced dashboards.

# --- Pipeline Execution Status Metric ---
# Tracks SUCCESS/FAILED/SKIPPED status from orchestrator logs
resource "google_logging_metric" "pipeline_execution_status" {
  name        = "pipeline_execution_status"
  description = "Pipeline execution outcomes extracted from enricher logs"
  filter      = <<-EOT
    resource.type="cloud_function"
    resource.labels.function_name="enricher"
    jsonPayload.status=~"SUCCESS|FAILED|SKIPPED"
  EOT

  metric_descriptor {
    metric_kind = "DELTA"
    value_type  = "INT64"
    labels {
      key         = "status"
      value_type  = "STRING"
      description = "Execution status (SUCCESS, FAILED, SKIPPED)"
    }
  }

  label_extractors = {
    "status" = "EXTRACT(jsonPayload.status)"
  }
}

# --- Enricher Provider Execution Metric ---
# Tracks individual enricher provider executions from orchestrator logs
resource "google_logging_metric" "enricher_provider_execution" {
  name        = "enricher_provider_execution"
  description = "Enricher provider execution count by name and status"
  filter      = <<-EOT
    resource.type="cloud_function"
    resource.labels.function_name="enricher"
    jsonPayload.message=~"Provider completed|Provider failed|Provider halted"
  EOT

  metric_descriptor {
    metric_kind = "DELTA"
    value_type  = "INT64"
    labels {
      key         = "provider_name"
      value_type  = "STRING"
      description = "Name of the enricher provider"
    }
    labels {
      key         = "status"
      value_type  = "STRING"
      description = "Execution status (SUCCESS, FAILED, SKIPPED)"
    }
  }

  label_extractors = {
    "provider_name" = "EXTRACT(jsonPayload.name)"
    "status"        = "REGEXP_EXTRACT(jsonPayload.message, \"Provider (completed|failed|halted)\")"
  }
}

# --- Enricher Provider Duration Metric ---
# Tracks execution duration of enricher providers
resource "google_logging_metric" "enricher_provider_duration" {
  name        = "enricher_provider_duration"
  description = "Enricher provider execution duration in milliseconds"
  filter      = <<-EOT
    resource.type="cloud_function"
    resource.labels.function_name="enricher"
    jsonPayload.message=~"Provider completed|Provider failed"
    jsonPayload.duration_ms > 0
  EOT

  metric_descriptor {
    metric_kind = "DELTA"
    value_type  = "DISTRIBUTION"
    unit        = "ms"
    labels {
      key         = "provider_name"
      value_type  = "STRING"
      description = "Name of the enricher provider"
    }
  }

  label_extractors = {
    "provider_name" = "EXTRACT(jsonPayload.name)"
  }

  value_extractor = "EXTRACT(jsonPayload.duration_ms)"

  bucket_options {
    explicit_buckets {
      bounds = [50, 100, 250, 500, 1000, 2500, 5000, 10000]
    }
  }
}

# --- Provider API Latency Metric ---
# Tracks execution time of uploader functions (proxy for external API latency)
resource "google_logging_metric" "provider_api_latency" {
  name        = "provider_api_latency"
  description = "Uploader function execution time as proxy for provider API latency"
  filter      = <<-EOT
    resource.type="cloud_function"
    resource.labels.function_name=~".*-uploader"
  EOT

  metric_descriptor {
    metric_kind = "DELTA"
    value_type  = "DISTRIBUTION"
    unit        = "ms"
    labels {
      key         = "provider"
      value_type  = "STRING"
      description = "Destination provider name"
    }
  }

  label_extractors = {
    "provider" = "REGEXP_EXTRACT(resource.labels.function_name, \"(.*)-uploader\")"
  }

  # Use execution duration from Cloud Functions
  value_extractor = "EXTRACT(jsonPayload.execution_time_ms)"

  bucket_options {
    explicit_buckets {
      bounds = [100, 250, 500, 1000, 2500, 5000, 10000, 30000]
    }
  }
}

# =============================================================================
# MAIN OPERATIONS DASHBOARD
# =============================================================================
resource "google_monitoring_dashboard" "operations" {
  dashboard_json = jsonencode({
    displayName = "FitGlue Operations Overview"
    labels = {
      environment = var.environment
    }
    mosaicLayout = {
      columns = 12
      tiles = concat(
        # ----- Row 1: Key Scorecards -----
        [
          {
            width  = 3
            height = 2
            widget = {
              title = "Total Function Invocations (24h)"
              scorecard = {
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "resource.type=\"cloud_function\" AND metric.type=\"cloudfunctions.googleapis.com/function/execution_count\""
                    aggregation = {
                      alignmentPeriod  = "86400s"
                      perSeriesAligner = "ALIGN_SUM"
                    }
                  }
                }
              }
            }
          },
          {
            xPos   = 3
            width  = 3
            height = 2
            widget = {
              title = "Error Rate (1h)"
              scorecard = {
                timeSeriesQuery = {
                  timeSeriesFilterRatio = {
                    numerator = {
                      filter = "resource.type=\"cloud_function\" AND metric.type=\"cloudfunctions.googleapis.com/function/execution_count\" AND metric.labels.status!=\"ok\""
                      aggregation = {
                        alignmentPeriod  = "3600s"
                        perSeriesAligner = "ALIGN_SUM"
                      }
                    }
                    denominator = {
                      filter = "resource.type=\"cloud_function\" AND metric.type=\"cloudfunctions.googleapis.com/function/execution_count\""
                      aggregation = {
                        alignmentPeriod  = "3600s"
                        perSeriesAligner = "ALIGN_SUM"
                      }
                    }
                  }
                }
                thresholds = [
                  { value = 0.05, color = "YELLOW", direction = "ABOVE" },
                  { value = 0.10, color = "RED", direction = "ABOVE" }
                ]
              }
            }
          },
          {
            xPos   = 6
            width  = 3
            height = 2
            widget = {
              title = "Median Latency (p50)"
              scorecard = {
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "resource.type=\"cloud_function\" AND metric.type=\"cloudfunctions.googleapis.com/function/execution_times\""
                    aggregation = {
                      alignmentPeriod    = "3600s"
                      perSeriesAligner   = "ALIGN_PERCENTILE_50"
                      crossSeriesReducer = "REDUCE_MEAN"
                    }
                  }
                }
              }
            }
          },
          {
            xPos   = 9
            width  = 3
            height = 2
            widget = {
              title = "p95 Latency"
              scorecard = {
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "resource.type=\"cloud_function\" AND metric.type=\"cloudfunctions.googleapis.com/function/execution_times\""
                    aggregation = {
                      alignmentPeriod    = "3600s"
                      perSeriesAligner   = "ALIGN_PERCENTILE_95"
                      crossSeriesReducer = "REDUCE_MEAN"
                    }
                  }
                }
                thresholds = [
                  { value = 30000, color = "YELLOW", direction = "ABOVE" },
                  { value = 60000, color = "RED", direction = "ABOVE" }
                ]
              }
            }
          },
        ],
        # ----- Row 2: Invocations Chart -----
        [
          {
            yPos   = 2
            width  = 6
            height = 4
            widget = {
              title = "Function Invocations Over Time"
              xyChart = {
                dataSets = [{
                  timeSeriesQuery = {
                    timeSeriesFilter = {
                      filter = "resource.type=\"cloud_function\" AND metric.type=\"cloudfunctions.googleapis.com/function/execution_count\""
                      aggregation = {
                        alignmentPeriod    = "300s"
                        perSeriesAligner   = "ALIGN_RATE"
                        crossSeriesReducer = "REDUCE_SUM"
                        groupByFields      = ["resource.labels.function_name"]
                      }
                    }
                  }
                  plotType       = "STACKED_AREA"
                  legendTemplate = "$${resource.labels.function_name}"
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
                      filter = "resource.type=\"cloud_function\" AND metric.type=\"cloudfunctions.googleapis.com/function/execution_count\" AND metric.labels.status!=\"ok\""
                      aggregation = {
                        alignmentPeriod    = "300s"
                        perSeriesAligner   = "ALIGN_RATE"
                        crossSeriesReducer = "REDUCE_SUM"
                        groupByFields      = ["resource.labels.function_name"]
                      }
                    }
                  }
                  plotType       = "STACKED_BAR"
                  legendTemplate = "$${resource.labels.function_name}"
                }]
                yAxis = { label = "Errors/sec" }
              }
            }
          },
        ],
        # ----- Row 3: Latency Distribution -----
        [
          {
            yPos   = 6
            width  = 12
            height = 4
            widget = {
              title = "Latency Distribution (All Functions)"
              xyChart = {
                dataSets = [{
                  timeSeriesQuery = {
                    timeSeriesFilter = {
                      filter = "resource.type=\"cloud_function\" AND metric.type=\"cloudfunctions.googleapis.com/function/execution_times\""
                      aggregation = {
                        alignmentPeriod    = "300s"
                        perSeriesAligner   = "ALIGN_PERCENTILE_99"
                        crossSeriesReducer = "REDUCE_MAX"
                        groupByFields      = ["resource.labels.function_name"]
                      }
                    }
                  }
                  plotType       = "LINE"
                  legendTemplate = "$${resource.labels.function_name} (p99)"
                }]
                yAxis = { label = "Latency (ms)" }
              }
            }
          }
        ],
        # ----- Row 4: Firestore Operations -----
        [
          {
            yPos   = 10
            width  = 6
            height = 4
            widget = {
              title = "Firestore Document Reads"
              xyChart = {
                dataSets = [{
                  timeSeriesQuery = {
                    timeSeriesFilter = {
                      filter = "resource.type=\"firestore_database\" AND metric.type=\"firestore.googleapis.com/document/read_count\""
                      aggregation = {
                        alignmentPeriod  = "300s"
                        perSeriesAligner = "ALIGN_RATE"
                      }
                    }
                  }
                  plotType = "LINE"
                }]
                yAxis = { label = "Reads/sec" }
              }
            }
          },
          {
            xPos   = 6
            yPos   = 10
            width  = 6
            height = 4
            widget = {
              title = "Firestore Document Writes"
              xyChart = {
                dataSets = [{
                  timeSeriesQuery = {
                    timeSeriesFilter = {
                      filter = "resource.type=\"firestore_database\" AND metric.type=\"firestore.googleapis.com/document/write_count\""
                      aggregation = {
                        alignmentPeriod  = "300s"
                        perSeriesAligner = "ALIGN_RATE"
                      }
                    }
                  }
                  plotType = "LINE"
                }]
                yAxis = { label = "Writes/sec" }
              }
            }
          }
        ]
      )
    }
  })
}

# =============================================================================
# PROVIDER LATENCY DASHBOARD
# =============================================================================
resource "google_monitoring_dashboard" "provider_latency" {
  dashboard_json = jsonencode({
    displayName = "FitGlue Provider API Latency"
    labels = {
      environment = var.environment
    }
    mosaicLayout = {
      columns = 12
      tiles = [
        # ----- Latency by Uploader (proxy for API latency) -----
        {
          width  = 12
          height = 6
          widget = {
            title = "Uploader Execution Time by Provider (Proxy for API Latency)"
            xyChart = {
              dataSets = [
                for uploader in local.go_uploaders : {
                  timeSeriesQuery = {
                    timeSeriesFilter = {
                      filter = "resource.type=\"cloud_function\" AND resource.labels.function_name=\"${uploader}\" AND metric.type=\"cloudfunctions.googleapis.com/function/execution_times\""
                      aggregation = {
                        alignmentPeriod  = "300s"
                        perSeriesAligner = "ALIGN_PERCENTILE_95"
                      }
                    }
                  }
                  plotType       = "LINE"
                  legendTemplate = replace(uploader, "-uploader", "")
                }
              ]
              yAxis = { label = "Latency (ms)" }
            }
          }
        },
        # ----- Strava Specific -----
        {
          yPos   = 6
          width  = 4
          height = 4
          widget = {
            title = "Strava Uploader p50/p95/p99"
            xyChart = {
              dataSets = [
                {
                  timeSeriesQuery = {
                    timeSeriesFilter = {
                      filter      = "resource.type=\"cloud_function\" AND resource.labels.function_name=\"strava-uploader\" AND metric.type=\"cloudfunctions.googleapis.com/function/execution_times\""
                      aggregation = { alignmentPeriod = "300s", perSeriesAligner = "ALIGN_PERCENTILE_50" }
                    }
                  }
                  plotType = "LINE", legendTemplate = "p50"
                },
                {
                  timeSeriesQuery = {
                    timeSeriesFilter = {
                      filter      = "resource.type=\"cloud_function\" AND resource.labels.function_name=\"strava-uploader\" AND metric.type=\"cloudfunctions.googleapis.com/function/execution_times\""
                      aggregation = { alignmentPeriod = "300s", perSeriesAligner = "ALIGN_PERCENTILE_95" }
                    }
                  }
                  plotType = "LINE", legendTemplate = "p95"
                },
                {
                  timeSeriesQuery = {
                    timeSeriesFilter = {
                      filter      = "resource.type=\"cloud_function\" AND resource.labels.function_name=\"strava-uploader\" AND metric.type=\"cloudfunctions.googleapis.com/function/execution_times\""
                      aggregation = { alignmentPeriod = "300s", perSeriesAligner = "ALIGN_PERCENTILE_99" }
                    }
                  }
                  plotType = "LINE", legendTemplate = "p99"
                }
              ]
              yAxis = { label = "Latency (ms)" }
            }
          }
        },
        # ----- TrainingPeaks Specific -----
        {
          xPos   = 4
          yPos   = 6
          width  = 4
          height = 4
          widget = {
            title = "TrainingPeaks Uploader p50/p95/p99"
            xyChart = {
              dataSets = [
                {
                  timeSeriesQuery = {
                    timeSeriesFilter = {
                      filter      = "resource.type=\"cloud_function\" AND resource.labels.function_name=\"trainingpeaks-uploader\" AND metric.type=\"cloudfunctions.googleapis.com/function/execution_times\""
                      aggregation = { alignmentPeriod = "300s", perSeriesAligner = "ALIGN_PERCENTILE_50" }
                    }
                  }
                  plotType = "LINE", legendTemplate = "p50"
                },
                {
                  timeSeriesQuery = {
                    timeSeriesFilter = {
                      filter      = "resource.type=\"cloud_function\" AND resource.labels.function_name=\"trainingpeaks-uploader\" AND metric.type=\"cloudfunctions.googleapis.com/function/execution_times\""
                      aggregation = { alignmentPeriod = "300s", perSeriesAligner = "ALIGN_PERCENTILE_95" }
                    }
                  }
                  plotType = "LINE", legendTemplate = "p95"
                },
                {
                  timeSeriesQuery = {
                    timeSeriesFilter = {
                      filter      = "resource.type=\"cloud_function\" AND resource.labels.function_name=\"trainingpeaks-uploader\" AND metric.type=\"cloudfunctions.googleapis.com/function/execution_times\""
                      aggregation = { alignmentPeriod = "300s", perSeriesAligner = "ALIGN_PERCENTILE_99" }
                    }
                  }
                  plotType = "LINE", legendTemplate = "p99"
                }
              ]
              yAxis = { label = "Latency (ms)" }
            }
          }
        },
        # ----- Intervals Specific -----
        {
          xPos   = 8
          yPos   = 6
          width  = 4
          height = 4
          widget = {
            title = "Intervals Uploader p50/p95/p99"
            xyChart = {
              dataSets = [
                {
                  timeSeriesQuery = {
                    timeSeriesFilter = {
                      filter      = "resource.type=\"cloud_function\" AND resource.labels.function_name=\"intervals-uploader\" AND metric.type=\"cloudfunctions.googleapis.com/function/execution_times\""
                      aggregation = { alignmentPeriod = "300s", perSeriesAligner = "ALIGN_PERCENTILE_50" }
                    }
                  }
                  plotType = "LINE", legendTemplate = "p50"
                },
                {
                  timeSeriesQuery = {
                    timeSeriesFilter = {
                      filter      = "resource.type=\"cloud_function\" AND resource.labels.function_name=\"intervals-uploader\" AND metric.type=\"cloudfunctions.googleapis.com/function/execution_times\""
                      aggregation = { alignmentPeriod = "300s", perSeriesAligner = "ALIGN_PERCENTILE_95" }
                    }
                  }
                  plotType = "LINE", legendTemplate = "p95"
                },
                {
                  timeSeriesQuery = {
                    timeSeriesFilter = {
                      filter      = "resource.type=\"cloud_function\" AND resource.labels.function_name=\"intervals-uploader\" AND metric.type=\"cloudfunctions.googleapis.com/function/execution_times\""
                      aggregation = { alignmentPeriod = "300s", perSeriesAligner = "ALIGN_PERCENTILE_99" }
                    }
                  }
                  plotType = "LINE", legendTemplate = "p99"
                }
              ]
              yAxis = { label = "Latency (ms)" }
            }
          }
        },
        # ----- Error Rates by Provider -----
        {
          yPos   = 10
          width  = 12
          height = 4
          widget = {
            title = "Uploader Error Rate by Provider"
            xyChart = {
              dataSets = [
                for uploader in local.go_uploaders : {
                  timeSeriesQuery = {
                    timeSeriesFilterRatio = {
                      numerator = {
                        filter      = "resource.type=\"cloud_function\" AND resource.labels.function_name=\"${uploader}\" AND metric.type=\"cloudfunctions.googleapis.com/function/execution_count\" AND metric.labels.status!=\"ok\""
                        aggregation = { alignmentPeriod = "300s", perSeriesAligner = "ALIGN_SUM" }
                      }
                      denominator = {
                        filter      = "resource.type=\"cloud_function\" AND resource.labels.function_name=\"${uploader}\" AND metric.type=\"cloudfunctions.googleapis.com/function/execution_count\""
                        aggregation = { alignmentPeriod = "300s", perSeriesAligner = "ALIGN_SUM" }
                      }
                    }
                  }
                  plotType       = "LINE"
                  legendTemplate = replace(uploader, "-uploader", "")
                }
              ]
              yAxis = { label = "Error Rate (%)" }
            }
          }
        }
      ]
    }
  })
}

# =============================================================================
# HANDLER PERFORMANCE DASHBOARD
# =============================================================================
resource "google_monitoring_dashboard" "handler_performance" {
  dashboard_json = jsonencode({
    displayName = "FitGlue Handler Performance"
    labels = {
      environment = var.environment
    }
    mosaicLayout = {
      columns = 12
      tiles = concat(
        # ----- Integration Handlers (Webhooks) -----
        [
          {
            width  = 12
            height = 1
            widget = { title = "", text = { content = "## Integration Handlers (Webhooks)", format = "MARKDOWN" } }
          },
          {
            yPos   = 1
            width  = 6
            height = 4
            widget = {
              title = "Integration Handler Invocations"
              xyChart = {
                dataSets = [
                  for handler in local.ts_integration_handlers : {
                    timeSeriesQuery = {
                      timeSeriesFilter = {
                        filter      = "resource.type=\"cloud_function\" AND resource.labels.function_name=\"${handler}\" AND metric.type=\"cloudfunctions.googleapis.com/function/execution_count\""
                        aggregation = { alignmentPeriod = "300s", perSeriesAligner = "ALIGN_RATE" }
                      }
                    }
                    plotType       = "STACKED_AREA"
                    legendTemplate = handler
                  }
                ]
                yAxis = { label = "Invocations/sec" }
              }
            }
          },
          {
            xPos   = 6
            yPos   = 1
            width  = 6
            height = 4
            widget = {
              title = "Integration Handler Latency (p95)"
              xyChart = {
                dataSets = [
                  for handler in local.ts_integration_handlers : {
                    timeSeriesQuery = {
                      timeSeriesFilter = {
                        filter      = "resource.type=\"cloud_function\" AND resource.labels.function_name=\"${handler}\" AND metric.type=\"cloudfunctions.googleapis.com/function/execution_times\""
                        aggregation = { alignmentPeriod = "300s", perSeriesAligner = "ALIGN_PERCENTILE_95" }
                      }
                    }
                    plotType       = "LINE"
                    legendTemplate = handler
                  }
                ]
                yAxis = { label = "Latency (ms)" }
              }
            }
          },
        ],
        # ----- Go Pipeline Handlers -----
        [
          {
            yPos   = 5
            width  = 12
            height = 1
            widget = { title = "", text = { content = "## Go Pipeline (Enricher + Uploaders)", format = "MARKDOWN" } }
          },
          {
            yPos   = 6
            width  = 6
            height = 4
            widget = {
              title = "Pipeline Handler Invocations"
              xyChart = {
                dataSets = [
                  for handler in concat(local.go_pipeline_handlers, local.go_uploaders) : {
                    timeSeriesQuery = {
                      timeSeriesFilter = {
                        filter      = "resource.type=\"cloud_function\" AND resource.labels.function_name=\"${handler}\" AND metric.type=\"cloudfunctions.googleapis.com/function/execution_count\""
                        aggregation = { alignmentPeriod = "300s", perSeriesAligner = "ALIGN_RATE" }
                      }
                    }
                    plotType       = "STACKED_AREA"
                    legendTemplate = handler
                  }
                ]
                yAxis = { label = "Invocations/sec" }
              }
            }
          },
          {
            xPos   = 6
            yPos   = 6
            width  = 6
            height = 4
            widget = {
              title = "Pipeline Handler Latency (p95)"
              xyChart = {
                dataSets = [
                  for handler in concat(local.go_pipeline_handlers, local.go_uploaders) : {
                    timeSeriesQuery = {
                      timeSeriesFilter = {
                        filter      = "resource.type=\"cloud_function\" AND resource.labels.function_name=\"${handler}\" AND metric.type=\"cloudfunctions.googleapis.com/function/execution_times\""
                        aggregation = { alignmentPeriod = "300s", perSeriesAligner = "ALIGN_PERCENTILE_95" }
                      }
                    }
                    plotType       = "LINE"
                    legendTemplate = handler
                  }
                ]
                yAxis = { label = "Latency (ms)" }
              }
            }
          },
        ],
        # ----- API Handlers -----
        [
          {
            yPos   = 10
            width  = 12
            height = 1
            widget = { title = "", text = { content = "## API Handlers (User-Facing)", format = "MARKDOWN" } }
          },
          {
            yPos   = 11
            width  = 6
            height = 4
            widget = {
              title = "API Handler Invocations"
              xyChart = {
                dataSets = [
                  for handler in local.ts_api_handlers : {
                    timeSeriesQuery = {
                      timeSeriesFilter = {
                        filter      = "resource.type=\"cloud_function\" AND resource.labels.function_name=\"${handler}\" AND metric.type=\"cloudfunctions.googleapis.com/function/execution_count\""
                        aggregation = { alignmentPeriod = "300s", perSeriesAligner = "ALIGN_RATE" }
                      }
                    }
                    plotType       = "STACKED_AREA"
                    legendTemplate = handler
                  }
                ]
                yAxis = { label = "Invocations/sec" }
              }
            }
          },
          {
            xPos   = 6
            yPos   = 11
            width  = 6
            height = 4
            widget = {
              title = "API Handler Latency (p95)"
              xyChart = {
                dataSets = [
                  for handler in local.ts_api_handlers : {
                    timeSeriesQuery = {
                      timeSeriesFilter = {
                        filter      = "resource.type=\"cloud_function\" AND resource.labels.function_name=\"${handler}\" AND metric.type=\"cloudfunctions.googleapis.com/function/execution_times\""
                        aggregation = { alignmentPeriod = "300s", perSeriesAligner = "ALIGN_PERCENTILE_95" }
                      }
                    }
                    plotType       = "LINE"
                    legendTemplate = handler
                  }
                ]
                yAxis = { label = "Latency (ms)" }
              }
            }
          },
        ]
      )
    }
  })
}

# =============================================================================
# ENRICHER PERFORMANCE DASHBOARD
# =============================================================================
# Uses log-based metrics extracted from orchestrator to show per-provider stats
resource "google_monitoring_dashboard" "enricher_performance" {
  dashboard_json = jsonencode({
    displayName = "FitGlue Enricher Performance"
    labels = {
      environment = var.environment
    }
    mosaicLayout = {
      columns = 12
      tiles = [
        # ----- Row 1: Header -----
        {
          width  = 12
          height = 1
          widget = { title = "", text = { content = "## Enricher Provider Execution Metrics\nThese metrics are extracted from enricher function logs using log-based metrics.", format = "MARKDOWN" } }
        },
        # ----- Row 2: Provider Popularity (Execution Count) -----
        {
          yPos   = 1
          width  = 12
          height = 5
          widget = {
            title = "Enricher Provider Executions Over Time"
            xyChart = {
              dataSets = [{
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "metric.type=\"logging.googleapis.com/user/enricher_provider_execution\""
                    aggregation = {
                      alignmentPeriod    = "300s"
                      perSeriesAligner   = "ALIGN_SUM"
                      crossSeriesReducer = "REDUCE_SUM"
                      groupByFields      = ["metric.labels.provider_name"]
                    }
                  }
                }
                plotType       = "STACKED_BAR"
                legendTemplate = "$${metric.labels.provider_name}"
              }]
              yAxis = { label = "Executions" }
            }
          }
        },
        # ----- Row 3: Provider Latency Distribution -----
        {
          yPos   = 6
          width  = 6
          height = 5
          widget = {
            title = "Enricher Provider Latency (p95)"
            xyChart = {
              dataSets = [{
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "metric.type=\"logging.googleapis.com/user/enricher_provider_duration\""
                    aggregation = {
                      alignmentPeriod    = "300s"
                      perSeriesAligner   = "ALIGN_PERCENTILE_95"
                      crossSeriesReducer = "REDUCE_MAX"
                      groupByFields      = ["metric.labels.provider_name"]
                    }
                  }
                }
                plotType       = "LINE"
                legendTemplate = "$${metric.labels.provider_name}"
              }]
              yAxis = { label = "Latency (ms)" }
            }
          }
        },
        # ----- Row 3: Success vs Failure Breakdown -----
        {
          xPos   = 6
          yPos   = 6
          width  = 6
          height = 5
          widget = {
            title = "Enricher Execution Status (Success vs Failed)"
            xyChart = {
              dataSets = [{
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "metric.type=\"logging.googleapis.com/user/enricher_provider_execution\""
                    aggregation = {
                      alignmentPeriod    = "300s"
                      perSeriesAligner   = "ALIGN_SUM"
                      crossSeriesReducer = "REDUCE_SUM"
                      groupByFields      = ["metric.labels.status"]
                    }
                  }
                }
                plotType       = "STACKED_AREA"
                legendTemplate = "$${metric.labels.status}"
              }]
              yAxis = { label = "Executions" }
            }
          }
        },
        # ----- Row 4: Pipeline Inclusion Proxy (activity types) -----
        {
          yPos   = 11
          width  = 12
          height = 1
          widget = { title = "", text = { content = "## Note\nFor exact pipeline inclusion rates, use **BigQuery** with Firestore exports to query `pipelines.enrichers` arrays and count provider occurrences.", format = "MARKDOWN" } }
        }
      ]
    }
  })
}

# =============================================================================
# BUSINESS GROWTH DASHBOARD
# =============================================================================
# Business metrics dashboard showing activity trends, source distribution, and success rates
resource "google_monitoring_dashboard" "business_growth" {
  dashboard_json = jsonencode({
    displayName = "FitGlue Business Growth"
    labels = {
      environment = var.environment
    }
    mosaicLayout = {
      columns = 12
      tiles = [
        # ----- Row 1: Header -----
        {
          width  = 12
          height = 1
          widget = { title = "", text = { content = "## Business Growth Metrics\nActivity processing, source distribution, and destination success rates. For full analytics, use **BigQuery** + **Looker Studio**.", format = "MARKDOWN" } }
        },
        # ----- Row 2: Activity Volume Trends -----
        {
          yPos   = 1
          width  = 8
          height = 5
          widget = {
            title = "Activities Processed Over Time"
            xyChart = {
              dataSets = [{
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "resource.type=\"cloud_function\" AND resource.labels.function_name=\"enricher\" AND metric.type=\"cloudfunctions.googleapis.com/function/execution_count\""
                    aggregation = {
                      alignmentPeriod  = "3600s"
                      perSeriesAligner = "ALIGN_SUM"
                    }
                  }
                }
                plotType       = "LINE"
                legendTemplate = "Activities"
              }]
              yAxis = { label = "Activities/hour" }
            }
          }
        },
        # ----- Weekly Comparison Scorecard -----
        {
          xPos   = 8
          yPos   = 1
          width  = 4
          height = 5
          widget = {
            title = "This Week vs Last Week"
            scorecard = {
              timeSeriesQuery = {
                timeSeriesFilter = {
                  filter = "resource.type=\"cloud_function\" AND resource.labels.function_name=\"enricher\" AND metric.type=\"cloudfunctions.googleapis.com/function/execution_count\""
                  aggregation = {
                    alignmentPeriod  = "604800s"
                    perSeriesAligner = "ALIGN_SUM"
                  }
                }
              }
              sparkChartView = {
                sparkChartType = "SPARK_BAR"
              }
            }
          }
        },
        # ----- Row 3: Source Distribution -----
        {
          yPos   = 6
          width  = 6
          height = 5
          widget = {
            title = "Activities by Source Integration"
            xyChart = {
              dataSets = [
                for handler in ["strava-handler", "fitbit-handler", "polar-handler", "wahoo-handler", "mobile-sync-handler"] : {
                  timeSeriesQuery = {
                    timeSeriesFilter = {
                      filter = "resource.type=\"cloud_function\" AND resource.labels.function_name=\"${handler}\" AND metric.type=\"cloudfunctions.googleapis.com/function/execution_count\""
                      aggregation = {
                        alignmentPeriod  = "3600s"
                        perSeriesAligner = "ALIGN_SUM"
                      }
                    }
                  }
                  plotType       = "STACKED_BAR"
                  legendTemplate = replace(handler, "-handler", "")
                }
              ]
              yAxis = { label = "Activities" }
            }
          }
        },
        # ----- Destination Success Rates -----
        {
          xPos   = 6
          yPos   = 6
          width  = 6
          height = 5
          widget = {
            title = "Destination Sync Success Rate"
            xyChart = {
              dataSets = [
                for uploader in ["strava-uploader", "trainingpeaks-uploader", "intervals-uploader", "googlesheets-uploader"] : {
                  timeSeriesQuery = {
                    timeSeriesFilterRatio = {
                      numerator = {
                        filter      = "resource.type=\"cloud_function\" AND resource.labels.function_name=\"${uploader}\" AND metric.type=\"cloudfunctions.googleapis.com/function/execution_count\" AND metric.labels.status=\"ok\""
                        aggregation = { alignmentPeriod = "3600s", perSeriesAligner = "ALIGN_SUM" }
                      }
                      denominator = {
                        filter      = "resource.type=\"cloud_function\" AND resource.labels.function_name=\"${uploader}\" AND metric.type=\"cloudfunctions.googleapis.com/function/execution_count\""
                        aggregation = { alignmentPeriod = "3600s", perSeriesAligner = "ALIGN_SUM" }
                      }
                    }
                  }
                  plotType       = "LINE"
                  legendTemplate = replace(uploader, "-uploader", "")
                }
              ]
              yAxis = { label = "Success Rate" }
            }
          }
        },
        # ----- Row 4: Integration Health -----
        {
          yPos   = 11
          width  = 12
          height = 4
          widget = {
            title = "Integration Handler Invocations (Source Health)"
            xyChart = {
              dataSets = [
                for handler in ["strava-handler", "fitbit-handler", "polar-handler", "wahoo-handler", "hevy-handler", "oura-handler"] : {
                  timeSeriesQuery = {
                    timeSeriesFilter = {
                      filter = "resource.type=\"cloud_function\" AND resource.labels.function_name=\"${handler}\" AND metric.type=\"cloudfunctions.googleapis.com/function/execution_count\""
                      aggregation = {
                        alignmentPeriod  = "3600s"
                        perSeriesAligner = "ALIGN_RATE"
                      }
                    }
                  }
                  plotType       = "LINE"
                  legendTemplate = replace(handler, "-handler", "")
                }
              ]
              yAxis = { label = "Invocations/hr" }
            }
          }
        }
      ]
    }
  })
}

# =============================================================================
# ALERT POLICIES
# =============================================================================

# Notification channel - Update with your email or Slack webhook
resource "google_monitoring_notification_channel" "email" {
  display_name = "FitGlue Alerts Email"
  type         = "email"
  labels = {
    email_address = "alerts@fitglue.com" # TODO: Update with actual email
  }
}

# --- High Error Rate Alert ---
resource "google_monitoring_alert_policy" "high_error_rate" {
  display_name = "High Error Rate (>5%)"
  combiner     = "OR"

  conditions {
    display_name = "Error rate exceeds 5%"
    condition_threshold {
      filter          = <<-EOT
        resource.type="cloud_function" AND
        metric.type="cloudfunctions.googleapis.com/function/execution_count" AND
        metric.labels.status!="ok"
      EOT
      duration        = "300s"
      comparison      = "COMPARISON_GT"
      threshold_value = 0.05

      aggregations {
        alignment_period     = "300s"
        per_series_aligner   = "ALIGN_RATE"
        cross_series_reducer = "REDUCE_SUM"
        group_by_fields      = ["resource.labels.function_name"]
      }

      denominator_filter = <<-EOT
        resource.type="cloud_function" AND
        metric.type="cloudfunctions.googleapis.com/function/execution_count"
      EOT

      denominator_aggregations {
        alignment_period     = "300s"
        per_series_aligner   = "ALIGN_RATE"
        cross_series_reducer = "REDUCE_SUM"
        group_by_fields      = ["resource.labels.function_name"]
      }
    }
  }

  notification_channels = [google_monitoring_notification_channel.email.id]

  alert_strategy {
    auto_close = "604800s" # 7 days
  }

  documentation {
    content   = "Error rate for a Cloud Function has exceeded 5% over the last 5 minutes. Check Cloud Logging and Sentry for details."
    mime_type = "text/markdown"
  }
}

# --- Critical Function Failure Alerts ---
# Split into multiple policies due to GCP limit of 6 conditions per policy

resource "google_monitoring_alert_policy" "critical_pipeline_failure" {
  display_name = "Critical Pipeline Handler Failure"
  combiner     = "OR"

  dynamic "conditions" {
    for_each = local.go_pipeline_handlers
    content {
      display_name = "${conditions.value} errors"
      condition_threshold {
        filter          = <<-EOT
          resource.type="cloud_function" AND
          resource.labels.function_name="${conditions.value}" AND
          metric.type="cloudfunctions.googleapis.com/function/execution_count" AND
          metric.labels.status!="ok"
        EOT
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

  alert_strategy {
    auto_close = "86400s"
  }

  documentation {
    content   = "A critical pipeline function has experienced more than 5 errors in the last minute."
    mime_type = "text/markdown"
  }
}

resource "google_monitoring_alert_policy" "critical_uploader_failure" {
  display_name = "Critical Uploader Failure"
  combiner     = "OR"

  dynamic "conditions" {
    for_each = local.go_uploaders
    content {
      display_name = "${conditions.value} errors"
      condition_threshold {
        filter          = <<-EOT
          resource.type="cloud_function" AND
          resource.labels.function_name="${conditions.value}" AND
          metric.type="cloudfunctions.googleapis.com/function/execution_count" AND
          metric.labels.status!="ok"
        EOT
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

  alert_strategy {
    auto_close = "86400s"
  }

  documentation {
    content   = "An uploader function has experienced more than 5 errors in the last minute."
    mime_type = "text/markdown"
  }
}

resource "google_monitoring_alert_policy" "critical_integration_failure" {
  display_name = "Critical Integration Handler Failure"
  combiner     = "OR"

  dynamic "conditions" {
    for_each = local.ts_integration_handlers
    content {
      display_name = "${conditions.value} errors"
      condition_threshold {
        filter          = <<-EOT
          resource.type="cloud_function" AND
          resource.labels.function_name="${conditions.value}" AND
          metric.type="cloudfunctions.googleapis.com/function/execution_count" AND
          metric.labels.status!="ok"
        EOT
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

  alert_strategy {
    auto_close = "86400s"
  }

  documentation {
    content   = "An integration handler has experienced more than 5 errors in the last minute."
    mime_type = "text/markdown"
  }
}

# --- High Latency Alert ---
resource "google_monitoring_alert_policy" "high_latency" {
  display_name = "High Function Latency (p95 > 30s)"
  combiner     = "OR"

  conditions {
    display_name = "Function latency exceeds 30 seconds"
    condition_threshold {
      filter          = "resource.type=\"cloud_function\" AND metric.type=\"cloudfunctions.googleapis.com/function/execution_times\""
      duration        = "300s"
      comparison      = "COMPARISON_GT"
      threshold_value = 30000 # 30 seconds in ms

      aggregations {
        alignment_period     = "300s"
        per_series_aligner   = "ALIGN_PERCENTILE_95"
        cross_series_reducer = "REDUCE_MAX"
        group_by_fields      = ["resource.labels.function_name"]
      }
    }
  }

  notification_channels = [google_monitoring_notification_channel.email.id]

  alert_strategy {
    auto_close = "86400s"
  }

  documentation {
    content   = "A Cloud Function's p95 latency has exceeded 30 seconds. This may indicate API timeouts or resource contention."
    mime_type = "text/markdown"
  }
}
