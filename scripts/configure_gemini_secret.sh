#!/bin/bash
set -e

# Script to configure Gemini API key for FitGlue AI Description Enricher
# Usage: ./scripts/configure_gemini_secret.sh <environment>
# Example: ./scripts/configure_gemini_secret.sh prod
#
# Get your API key from: https://ai.google.dev/

ENV=${1:-}

# Validate arguments
if [[ ! "$ENV" =~ ^(dev|test|prod)$ ]]; then
  echo "❌ Error: Invalid environment '$ENV'"
  echo "Usage: $0 <dev|test|prod>"
  exit 1
fi

PROJECT_ID="fitglue-server-${ENV}"

echo "🤖 Configuring Gemini API key for ${ENV} environment"
echo "Project: $PROJECT_ID"
echo ""
echo "ℹ️  Get your API key from: https://ai.google.dev/"
echo ""

# Prompt for Gemini API Key
read -sp "Enter Gemini API Key (AIza...): " GEMINI_API_KEY
echo ""
if [ -z "$GEMINI_API_KEY" ]; then
  echo "❌ Error: Gemini API Key cannot be empty"
  exit 1
fi

# Validate key format (Gemini keys start with AIza)
if [[ ! "$GEMINI_API_KEY" =~ ^AIza ]]; then
  echo "⚠️  Warning: Gemini API keys typically start with 'AIza'. Proceeding anyway..."
fi

# Store Gemini API Key
echo "📝 Storing gemini-api-key..."
echo -n "$GEMINI_API_KEY" | gcloud secrets versions add "gemini-api-key" \
  --data-file=- \
  --project="$PROJECT_ID" 2>/dev/null || {
  echo "Secret doesn't exist yet, creating it..."
  echo -n "$GEMINI_API_KEY" | gcloud secrets create "gemini-api-key" \
    --data-file=- \
    --project="$PROJECT_ID"
}

echo ""
echo "✅ Successfully configured Gemini API key for ${ENV} environment"
echo ""
echo "🔄 Next steps:"
echo "  1. Deploy the enricher functions:"
echo "     cd terraform && terraform apply -var-file=envs/${ENV}.tfvars"
echo "  2. The AI Description enricher is now available for Athlete-tier users"
