#!/bin/bash
set -e

# Script to configure OAuth state secret for FitGlue
# Usage: ./scripts/configure_oauth_state_secret.sh <environment>
# Example: ./scripts/configure_oauth_state_secret.sh dev

ENV=${1:-}

# Validate arguments
if [[ ! "$ENV" =~ ^(dev|test|prod)$ ]]; then
  echo "❌ Error: Invalid environment '$ENV'"
  echo "Usage: $0 <dev|test|prod>"
  exit 1
fi

PROJECT_ID="fitglue-server-${ENV}"

echo "🔐 Configuring OAuth state secret for ${ENV} environment"
echo "Project: $PROJECT_ID"
echo ""

# Generate a random 32-byte secret
echo "🎲 Generating random secret..."
SECRET=$(openssl rand -hex 32)

# Store the secret
echo "📝 Storing oauth-state-secret..."
echo -n "$SECRET" | gcloud secrets versions add "oauth-state-secret" \
  --data-file=- \
  --project="$PROJECT_ID" 2>/dev/null || {
  echo "Secret doesn't exist yet, creating it..."
  echo -n "$SECRET" | gcloud secrets create "oauth-state-secret" \
    --data-file=- \
    --project="$PROJECT_ID"
}

echo ""
echo "✅ Successfully configured OAuth state secret for ${ENV} environment"
echo ""
echo "🔒 Secret value (save this securely if needed):"
echo "$SECRET"
echo ""
echo "⚠️  This secret is used for CSRF protection. Keep it secure!"
