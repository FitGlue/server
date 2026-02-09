#!/bin/bash
set -e

# Script to configure a webhook secret for a FitGlue integration
# Usage: ./scripts/configure_webhook_secret.sh <service> <environment>
# Example: ./scripts/configure_webhook_secret.sh github dev

SERVICE=${1:-}
ENV=${2:-}

# Validate arguments
if [[ ! "$SERVICE" =~ ^(github)$ ]]; then
  echo "❌ Error: Invalid service '$SERVICE'"
  echo "Usage: $0 <github> <dev|test|prod>"
  exit 1
fi

if [[ ! "$ENV" =~ ^(dev|test|prod)$ ]]; then
  echo "❌ Error: Invalid environment '$ENV'"
  echo "Usage: $0 <github> <dev|test|prod>"
  exit 1
fi

PROJECT_ID="fitglue-server-${ENV}"
SECRET_NAME="${SERVICE}-webhook-secret"

echo "🔐 Configuring webhook secret for ${SERVICE} in ${ENV} environment"
echo "Project: $PROJECT_ID"
echo ""

# Generate a random 32-byte secret
echo "🎲 Generating random secret..."
SECRET=$(openssl rand -hex 32)

# Store the secret
echo "📝 Storing ${SECRET_NAME}..."
echo -n "$SECRET" | gcloud secrets versions add "$SECRET_NAME" \
  --data-file=- \
  --project="$PROJECT_ID" 2>/dev/null || {
  echo "Secret doesn't exist yet, creating it..."
  echo -n "$SECRET" | gcloud secrets create "$SECRET_NAME" \
    --data-file=- \
    --project="$PROJECT_ID"
}

echo ""
echo "✅ Successfully configured ${SERVICE} webhook secret for ${ENV} environment"
echo ""
echo "🔒 Secret value (paste this into your GitHub repo webhook settings):"
echo "$SECRET"
echo ""
echo "📋 Next steps:"
echo "  1. Go to your GitHub repo → Settings → Webhooks → Add webhook"
echo "  2. Payload URL: https://fitglue.tech/hooks/github"
echo "  3. Content type: application/json"
echo "  4. Secret: (paste the value above)"
echo "  5. Events: Just the push event"
