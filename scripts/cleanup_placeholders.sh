#!/bin/bash
set -e

# Script to cleanup secret placeholders by restoring the latest non-placeholder version
# Usage: ./scripts/cleanup_placeholders.sh <environment>
# Example: ./scripts/cleanup_placeholders.sh dev

ENV=${1:-}
PLACEHOLDER="PLACEHOLDER_REPLACE_ME"

# Validate arguments
if [[ ! "$ENV" =~ ^(dev|test|prod)$ ]]; then
  echo "❌ Error: Invalid environment '$ENV'"
  echo "Usage: $0 <dev|test|prod>"
  exit 1
fi

PROJECT_ID="fitglue-server-${ENV}"

echo "🧹 Cleaning up secret placeholders for ${ENV} environment"
echo "Project: $PROJECT_ID"
echo "Placeholder pattern: $PLACEHOLDER"
echo ""

# Get list of all secret names
echo "🔍 Fetching secrets..."
secrets=$(gcloud secrets list --project="$PROJECT_ID" --format="value(name)")

if [ -z "$secrets" ]; then
  echo "✅ No secrets found in project $PROJECT_ID"
  exit 0
fi

for secret in $secrets; do
  echo "---"
  echo "📦 Processing secret: $secret"

  # Get latest version value
  # Note: Some secrets might be empty or restricted, we handle errors gracefully
  latest_val=$(gcloud secrets versions access latest --secret="$secret" --project="$PROJECT_ID" 2>/dev/null || echo "")

  if [ -z "$latest_val" ]; then
    echo "⚠️  Warning: Could not access latest version or secret has no versions. Skipping."
    continue
  fi

  if [ "$latest_val" != "$PLACEHOLDER" ]; then
    echo "✅ Latest version is NOT a placeholder. Skipping."
    continue
  fi

  echo "🧨 Latest version is a placeholder! Searching for previous valid values..."

  # Get list of all versions (highest number first)
  # Versions are usually integers like "1", "2", etc.
  versions=$(gcloud secrets versions list "$secret" --project="$PROJECT_ID" --format="value(name)" --filter="state=ENABLED" --sort-by="~name")

  found_valid=false
  for version_id in $versions; do
    # Try to access this version
    val=$(gcloud secrets versions access "$version_id" --secret="$secret" --project="$PROJECT_ID" 2>/dev/null || echo "")

    if [ -n "$val" ] && [ "$val" != "$PLACEHOLDER" ]; then
      echo "✨ Found valid value in version $version_id. Restoring as newest version..."
      echo -n "$val" | gcloud secrets versions add "$secret" --project="$PROJECT_ID" --data-file=- > /dev/null
      echo "✅ Successfully updated $secret"
      found_valid=true
      break
    fi
  done

  if [ "$found_valid" = false ]; then
    echo "❌ Error: No previous valid (non-placeholder) versions found for $secret."
  fi
done

echo ""
echo "✨ Finished processing all secrets for ${ENV}."
