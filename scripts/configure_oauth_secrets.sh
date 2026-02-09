#!/bin/bash
set -e

# Script to configure OAuth secrets for FitGlue
# Usage: ./scripts/configure_oauth_secrets.sh <service> <environment>
# Example: ./scripts/configure_oauth_secrets.sh strava dev

SERVICE=${1:-}
ENV=${2:-}

# Validate arguments
if [[ ! "$SERVICE" =~ ^(strava|fitbit|google)$ ]]; then
  echo "❌ Error: Invalid service '$SERVICE'"
  echo "Usage: $0 <strava|fitbit|google> <dev|test|prod>"
  exit 1
fi

if [[ ! "$ENV" =~ ^(dev|test|prod)$ ]]; then
  echo "❌ Error: Invalid environment '$ENV'"
  echo "Usage: $0 <strava|fitbit|google> <dev|test|prod>"
  exit 1
fi

PROJECT_ID="fitglue-server-${ENV}"

echo "🔐 Configuring OAuth secrets for ${SERVICE} in ${ENV} environment"
echo "Project: $PROJECT_ID"
echo ""

# Prompt for Client ID
read -p "Enter ${SERVICE} Client ID: " CLIENT_ID
if [ -z "$CLIENT_ID" ]; then
  echo "❌ Error: Client ID cannot be empty"
  exit 1
fi

# Prompt for Client Secret (hidden input)
read -sp "Enter ${SERVICE} Client Secret: " CLIENT_SECRET
echo ""
if [ -z "$CLIENT_SECRET" ]; then
  echo "❌ Error: Client Secret cannot be empty"
  exit 1
fi

# Store Client ID
echo "📝 Storing ${SERVICE}-client-id..."
echo -n "$CLIENT_ID" | gcloud secrets versions add "${SERVICE}-client-id" \
  --data-file=- \
  --project="$PROJECT_ID" 2>/dev/null || {
  echo "Secret doesn't exist yet, creating it..."
  echo -n "$CLIENT_ID" | gcloud secrets create "${SERVICE}-client-id" \
    --data-file=- \
    --project="$PROJECT_ID"
}

# Store Client Secret
echo "📝 Storing ${SERVICE}-client-secret..."
echo -n "$CLIENT_SECRET" | gcloud secrets versions add "${SERVICE}-client-secret" \
  --data-file=- \
  --project="$PROJECT_ID" 2>/dev/null || {
  echo "Secret doesn't exist yet, creating it..."
  echo -n "$CLIENT_SECRET" | gcloud secrets create "${SERVICE}-client-secret" \
    --data-file=- \
    --project="$PROJECT_ID"
}

echo ""
echo "✅ Successfully configured ${SERVICE} OAuth secrets for ${ENV} environment"
echo ""
echo "🔄 Next steps:"
echo "  1. If this is the first time, also run:"
echo "     ./scripts/configure_oauth_state_secret.sh ${ENV}"
echo "  2. Deploy the OAuth handlers:"
echo "     cd terraform && terraform apply -var-file=envs/${ENV}.tfvars"
