#!/bin/bash
set -e

# Script to configure Stripe secrets for FitGlue
# Usage: ./scripts/configure_stripe_secrets.sh <environment>
# Example: ./scripts/configure_stripe_secrets.sh prod

ENV=${1:-}

# Validate arguments
if [[ ! "$ENV" =~ ^(dev|test|prod)$ ]]; then
  echo "❌ Error: Invalid environment '$ENV'"
  echo "Usage: $0 <dev|test|prod>"
  exit 1
fi

PROJECT_ID="fitglue-server-${ENV}"

echo "🔐 Configuring Stripe secrets for ${ENV} environment"
echo "Project: $PROJECT_ID"
echo ""

# Prompt for Stripe Secret Key
read -sp "Enter Stripe Secret Key (sk_live_... or sk_test_...): " STRIPE_SECRET_KEY
echo ""
if [ -z "$STRIPE_SECRET_KEY" ]; then
  echo "❌ Error: Stripe Secret Key cannot be empty"
  exit 1
fi

# Prompt for Stripe Webhook Secret
read -sp "Enter Stripe Webhook Secret (whsec_...): " STRIPE_WEBHOOK_SECRET
echo ""
if [ -z "$STRIPE_WEBHOOK_SECRET" ]; then
  echo "❌ Error: Stripe Webhook Secret cannot be empty"
  exit 1
fi

# Prompt for Stripe Price ID
read -p "Enter Stripe Price ID (price_...): " STRIPE_PRICE_ID
if [ -z "$STRIPE_PRICE_ID" ]; then
  echo "❌ Error: Stripe Price ID cannot be empty"
  exit 1
fi

# Store Stripe Secret Key
echo "📝 Storing stripe-secret-key..."
echo -n "$STRIPE_SECRET_KEY" | gcloud secrets versions add "stripe-secret-key" \
  --data-file=- \
  --project="$PROJECT_ID" 2>/dev/null || {
  echo "Secret doesn't exist yet, creating it..."
  echo -n "$STRIPE_SECRET_KEY" | gcloud secrets create "stripe-secret-key" \
    --data-file=- \
    --project="$PROJECT_ID"
}

# Store Stripe Webhook Secret
echo "📝 Storing stripe-webhook-secret..."
echo -n "$STRIPE_WEBHOOK_SECRET" | gcloud secrets versions add "stripe-webhook-secret" \
  --data-file=- \
  --project="$PROJECT_ID" 2>/dev/null || {
  echo "Secret doesn't exist yet, creating it..."
  echo -n "$STRIPE_WEBHOOK_SECRET" | gcloud secrets create "stripe-webhook-secret" \
    --data-file=- \
    --project="$PROJECT_ID"
}

# Store Stripe Price ID
echo "📝 Storing stripe-price-id..."
echo -n "$STRIPE_PRICE_ID" | gcloud secrets versions add "stripe-price-id" \
  --data-file=- \
  --project="$PROJECT_ID" 2>/dev/null || {
  echo "Secret doesn't exist yet, creating it..."
  echo -n "$STRIPE_PRICE_ID" | gcloud secrets create "stripe-price-id" \
    --data-file=- \
    --project="$PROJECT_ID"
}

echo ""
echo "✅ Successfully configured Stripe secrets for ${ENV} environment"
echo ""
echo "🔄 Next steps:"
echo "  1. Deploy the billing handler:"
echo "     cd terraform && terraform apply -var-file=envs/${ENV}.tfvars"
echo "  2. Test the checkout flow at https://${ENV}.fitglue.tech/app/settings/upgrade"
