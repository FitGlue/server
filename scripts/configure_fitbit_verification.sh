#!/bin/bash
set -e

# Script to configure Fitbit Webhook Verification Code
# Usage: ./scripts/configure_fitbit_verification.sh <environment>
# Example: ./scripts/configure_fitbit_verification.sh dev

ENV=${1:-}

# Validate arguments
if [[ ! "$ENV" =~ ^(dev|test|prod)$ ]]; then
  echo "❌ Error: Invalid environment '$ENV'"
  echo "Usage: $0 <dev|test|prod>"
  exit 1
fi

PROJECT_ID="fitglue-server-${ENV}"

echo "🔐 Configuring Fitbit Verification Code for ${ENV} environment"
echo "Project: $PROJECT_ID"
echo ""

# Prompt for Verification Code (hidden input)
echo "Enter the verification code you want to use (must verify against Fitbit Dev Portal):"
read -sp "Verification Code: " VERIFY_CODE
echo ""

if [ -z "$VERIFY_CODE" ]; then
  echo "❌ Error: Verification Code cannot be empty"
  exit 1
fi

# Store Secret
echo "📝 Storing fitbit-verification-code..."
echo -n "$VERIFY_CODE" | gcloud secrets versions add "fitbit-verification-code" \
  --data-file=- \
  --project="$PROJECT_ID" 2>/dev/null || {
  echo "Secret doesn't exist yet, creating it..."
  echo -n "$VERIFY_CODE" | gcloud secrets create "fitbit-verification-code" \
    --data-file=- \
    --project="$PROJECT_ID"
}

echo ""
echo "✅ Successfully configured Fitbit Verification Code for ${ENV} environment"
echo ""
echo "🔄 Next step:"
echo "   Go to Fitbit Dev Portal -> Subscription/Subscriber Interface"
echo "   Set Verification Code to: (your entered code)"
echo "   Set Endpoint URL to your deployed fitbit-webhook-handler URL"
