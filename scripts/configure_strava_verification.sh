#!/bin/bash
set -e

# Script to configure Strava Webhook Verification Token
# Usage: ./scripts/configure_strava_verification.sh <environment>
# Example: ./scripts/configure_strava_verification.sh dev

ENV=${1:-}

# Validate arguments
if [[ ! "$ENV" =~ ^(dev|test|prod)$ ]]; then
  echo "❌ Error: Invalid environment '$ENV'"
  echo "Usage: $0 <dev|test|prod>"
  exit 1
fi

PROJECT_ID="fitglue-server-${ENV}"

echo "🔐 Configuring Strava Webhook Verification Token for ${ENV} environment"
echo "Project: $PROJECT_ID"
echo ""

# Generate a random verification token
VERIFY_TOKEN=$(openssl rand -hex 32)
echo "🔑 Generated verification token: ${VERIFY_TOKEN:0:8}... (truncated for security)"

# Store Secret
echo "📝 Storing strava-verify-token..."
echo -n "$VERIFY_TOKEN" | gcloud secrets versions add "strava-verify-token" \
  --data-file=- \
  --project="$PROJECT_ID" 2>/dev/null || {
  echo "Secret doesn't exist yet, creating it..."
  echo -n "$VERIFY_TOKEN" | gcloud secrets create "strava-verify-token" \
    --data-file=- \
    --project="$PROJECT_ID"
}

echo ""
echo "✅ Successfully configured Strava Verification Token for ${ENV} environment"
echo ""
echo "🔄 Next steps:"
echo "   1. Deploy the strava-handler Cloud Function"
echo "   2. Run: npx ts-node scripts/register-strava-webhook.ts ${ENV}"
echo "      This will register your webhook subscription with Strava"
