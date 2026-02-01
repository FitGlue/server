resource "google_firestore_database" "database" {
  project     = var.project_id
  name        = "(default)"
  location_id = var.region
  type        = "FIRESTORE_NATIVE"

  depends_on = [google_project_service.apis]
}

resource "google_firestore_index" "executions_service_timestamp" {
  project    = var.project_id
  database   = google_firestore_database.database.name
  collection = "executions"

  fields {
    field_path = "service"
    order      = "ASCENDING"
  }

  fields {
    field_path = "timestamp"
    order      = "DESCENDING"
  }
}

resource "google_firestore_index" "executions_status_timestamp" {
  project    = var.project_id
  database   = google_firestore_database.database.name
  collection = "executions"

  fields {
    field_path = "status"
    order      = "ASCENDING"
  }

  fields {
    field_path = "timestamp"
    order      = "DESCENDING"
  }
}

resource "google_firestore_index" "executions_user_timestamp" {
  project    = var.project_id
  database   = google_firestore_database.database.name
  collection = "executions"

  fields {
    field_path = "user_id"
    order      = "ASCENDING"
  }

  fields {
    field_path = "timestamp"
    order      = "DESCENDING"
  }
}

resource "google_firestore_field" "executions_expire_at" {
  project    = var.project_id
  database   = google_firestore_database.database.name
  collection = "executions"
  field      = "expire_at"

  ttl_config {}
}

resource "google_firestore_field" "showcased_activities_expires_at" {
  project    = var.project_id
  database   = google_firestore_database.database.name
  collection = "showcased_activities"
  field      = "expires_at"

  ttl_config {}
}

resource "google_firestore_index" "pending_inputs_user_status_created" {
  project    = var.project_id
  database   = google_firestore_database.database.name
  collection = "pending_inputs"

  fields {
    field_path = "user_id"
    order      = "ASCENDING"
  }

  fields {
    field_path = "status"
    order      = "ASCENDING"
  }

  fields {
    field_path = "created_at"
    order      = "DESCENDING"
  }
}


resource "google_firestore_index" "executions_pipeline_timestamp" {
  project    = var.project_id
  database   = google_firestore_database.database.name
  collection = "executions"

  fields {
    field_path = "pipeline_execution_id"
    order      = "ASCENDING"
  }

  fields {
    field_path = "timestamp"
    order      = "DESCENDING"
  }
}

resource "google_firestore_index" "executions_pipeline_timestamp_asc" {
  project    = var.project_id
  database   = google_firestore_database.database.name
  collection = "executions"

  fields {
    field_path = "pipeline_execution_id"
    order      = "ASCENDING"
  }

  fields {
    field_path = "timestamp"
    order      = "ASCENDING"
  }
}

# Loop Prevention Indexes - check if external ID exists as destination
# Note: These are collection group indexes on subcollection 'activities'
# under users/{userId}/activities


# Phase 2 Performance Optimization Indexes
# Added for O(1) activity list queries

# Index for querying unsynchronized activities (executions without a sync)
resource "google_firestore_index" "executions_user_sync_timestamp" {
  project    = var.project_id
  database   = google_firestore_database.database.name
  collection = "executions"

  fields {
    field_path = "user_id"
    order      = "ASCENDING"
  }

  fields {
    field_path = "has_synchronized_activity"
    order      = "ASCENDING"
  }

  fields {
    field_path = "timestamp"
    order      = "DESCENDING"
  }
}

# Note: Single-field collection group indexes are NOT needed here.
# Firestore automatically creates single-field indexes for all fields.
# The previous `activities_synced_at` and `activities_pipeline_execution`
# indexes were rejected by GCP with "this index is not necessary".
#
# If you need COMPOSITE collection group indexes (2+ fields), add them here.
# Example:
# resource "google_firestore_index" "activities_user_synced" {
#   project          = var.project_id
#   database         = google_firestore_database.database.name
#   collection       = "activities"
#   query_scope      = "COLLECTION_GROUP"
#
#   fields {
#     field_path = "user_id"
#     order      = "ASCENDING"
#   }
#
#   fields {
#     field_path = "synced_at"
#     order      = "DESCENDING"
#   }
# }

# Index for parkrun-results-source to query pending inputs by enricher
# This is a collection group query since pending_inputs may be queried across users
resource "google_firestore_index" "pending_inputs_enricher_status" {
  project     = var.project_id
  database    = google_firestore_database.database.name
  collection  = "pending_inputs"
  query_scope = "COLLECTION_GROUP"

  fields {
    field_path = "enricher_provider_id"
    order      = "ASCENDING"
  }

  fields {
    field_path = "status"
    order      = "ASCENDING"
  }
}

# Index for pending_inputs subcollection (users/{userId}/pending_inputs)
# Used by useRealtimeInputs for real-time dashboard queries
resource "google_firestore_index" "pending_inputs_subcollection_status_created" {
  project     = var.project_id
  database    = google_firestore_database.database.name
  collection  = "pending_inputs"
  query_scope = "COLLECTION"

  fields {
    field_path = "status"
    order      = "ASCENDING"
  }

  fields {
    field_path = "created_at"
    order      = "DESCENDING"
  }
}

# -------------------------------------------------------------------
# Pipeline Runs Indexes
# -------------------------------------------------------------------

# Index for listing pipeline runs by status + created_at
# Used by usePipelineRuns for dashboard activity list
resource "google_firestore_index" "pipeline_runs_status_created" {
  project     = var.project_id
  database    = google_firestore_database.database.name
  collection  = "pipeline_runs"
  query_scope = "COLLECTION"

  fields {
    field_path = "status"
    order      = "ASCENDING"
  }

  fields {
    field_path = "created_at"
    order      = "DESCENDING"
  }
}

# Index for filtering by source + created_at
# Used for filtering activities by source (e.g., "From Hevy")
resource "google_firestore_index" "pipeline_runs_source_created" {
  project     = var.project_id
  database    = google_firestore_database.database.name
  collection  = "pipeline_runs"
  query_scope = "COLLECTION"

  fields {
    field_path = "source"
    order      = "ASCENDING"
  }

  fields {
    field_path = "created_at"
    order      = "DESCENDING"
  }
}

# Index for filtering by pipeline_id + created_at
# Used for viewing runs from a specific pipeline config
resource "google_firestore_index" "pipeline_runs_pipeline_created" {
  project     = var.project_id
  database    = google_firestore_database.database.name
  collection  = "pipeline_runs"
  query_scope = "COLLECTION"

  fields {
    field_path = "pipeline_id"
    order      = "ASCENDING"
  }

  fields {
    field_path = "created_at"
    order      = "DESCENDING"
  }
}
