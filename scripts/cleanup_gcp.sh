#!/bin/bash
PROJECT="fitglue-server-dev"

echo "Deleting Pub/Sub Topics..."
gcloud pubsub topics delete topic-raw-activity --project=$PROJECT --quiet || true
gcloud pubsub topics delete topic-enriched-activity --project=$PROJECT --quiet || true
gcloud pubsub topics delete topic-job-upload-strava --project=$PROJECT --quiet || true
gcloud pubsub topics delete topic-job-upload-other --project=$PROJECT --quiet || true

echo "Deleting Secrets..."
gcloud secrets delete hevy-api-key --project=$PROJECT --quiet || true
gcloud secrets delete keiser-credentials --project=$PROJECT --quiet || true
gcloud secrets delete fitbit-client-secret --project=$PROJECT --quiet || true
gcloud secrets delete strava-client-secret --project=$PROJECT --quiet || true

echo "Deleting Buckets..."
# Removing all objects first
gcloud storage rm --recursive gs://fitglue-server-dev-functions-source/ --project=$PROJECT || true
gcloud storage buckets delete gs://fitglue-server-dev-functions-source --project=$PROJECT --quiet || true

gcloud storage rm --recursive gs://fitglue-server-dev-artifacts/ --project=$PROJECT || true
gcloud storage buckets delete gs://fitglue-server-dev-artifacts --project=$PROJECT --quiet || true

echo "Deleting Cloud Functions..."
gcloud functions delete enricher --project=$PROJECT --region=us-central1 --gen2 --quiet || true
gcloud functions delete router --project=$PROJECT --region=us-central1 --gen2 --quiet || true
gcloud functions delete strava-uploader --project=$PROJECT --region=us-central1 --gen2 --quiet || true
gcloud functions delete keiser-poller --project=$PROJECT --region=us-central1 --gen2 --quiet || true
gcloud functions delete hevy-webhook-handler --project=$PROJECT --region=us-central1 --gen2 --quiet || true

echo "Deleting Cloud Scheduler Jobs..."
gcloud scheduler jobs delete keiser-poller-job --project=$PROJECT --location=us-central1 --quiet || true

echo "Deleting Service Accounts..."
gcloud iam service-accounts delete keiser-scheduler-sa@$PROJECT.iam.gserviceaccount.com --project=$PROJECT --quiet || true

echo "Cleanup complete."
