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
