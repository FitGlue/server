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

# Collection group index for activities synced_at ordering
# Enables efficient dashboard activity list with descending date order
resource "google_firestore_index" "activities_synced_at" {
  project          = var.project_id
  database         = google_firestore_database.database.name
  collection       = "activities"
  query_scope      = "COLLECTION_GROUP"

  fields {
    field_path = "synced_at"
    order      = "DESCENDING"
  }
}

# Collection group index for activities by pipeline_execution_id
# Enables efficient lookup of activities by their pipeline execution
resource "google_firestore_index" "activities_pipeline_execution" {
  project          = var.project_id
  database         = google_firestore_database.database.name
  collection       = "activities"
  query_scope      = "COLLECTION_GROUP"

  fields {
    field_path = "pipeline_execution_id"
    order      = "ASCENDING"
  }
}


