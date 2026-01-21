#!/bin/bash
set -e

# Script to refresh all Cloud Functions (Gen2) to pick up latest secret values
# Usage: ./scripts/redeploy_functions.sh <environment>
# Example: ./scripts/redeploy_functions.sh dev

ENV=${1:-}

# Validate arguments
if [[ ! "$ENV" =~ ^(dev|test|prod)$ ]]; then
  echo "❌ Error: Invalid environment '$ENV'"
  echo "Usage: $0 <dev|test|prod>"
  exit 1
fi

PROJECT_ID="fitglue-server-${ENV}"

echo "🚀 Refreshing all Gen2 Functions (Cloud Run services) for ${ENV} environment"
echo "Project: $PROJECT_ID"
echo "This will create a new revision for each service, forcing it to fetch 'latest' secret versions."
echo ""

# Get list of services and their regions
# For Gen2 functions, the Cloud Run service name is the same as the function name
echo "🔍 Fetching services..."
services=$(gcloud run services list --project="$PROJECT_ID" --format="value(SERVICE,REGION)")

if [ -z "$services" ]; then
  echo "✅ No services found in project $PROJECT_ID"
  exit 0
fi

# Count services for the progress message
count=$(echo "$services" | wc -l)
echo "📦 Found $count services to refresh."
echo ""

while read -r service region; do
  if [ -z "$service" ]; then continue; fi

  echo "---"
  echo "🔄 [${ENV}] Refreshing: $service ($region)"

  # Updating a label triggers a new revision without requiring a code rebuild
  # This is the fastest way to "redeploy" to pick up new secrets or env vars
  gcloud run services update "$service" \
    --region="$region" \
    --project="$PROJECT_ID" \
    --update-labels="redeploy-timestamp=$(date +%s)" \
    --quiet

  echo "✅ Successfully refreshed $service"
done <<< "$services"

echo ""
echo "✨ All services in ${ENV} have been refreshed and are now using the latest secret versions."
