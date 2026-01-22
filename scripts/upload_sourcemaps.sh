#!/bin/bash
set -e

# Usage: ./scripts/upload_sourcemaps.sh <RELEASE_VERSION>
RELEASE=$1

if [ -z "$RELEASE" ]; then
  echo "Error: Release version argument is required"
  exit 1
fi

if [ -z "$SENTRY_AUTH_TOKEN" ]; then
  echo "Error: SENTRY_AUTH_TOKEN environment variable is required"
  exit 1
fi

ORG="fitglue"
PROJECT="server"
TS_ROOT="src/typescript"

# Create release if it doesn't exist
echo "Creating release $RELEASE..."
npx @sentry/cli releases new "$RELEASE" --org "$ORG" --project "$PROJECT" || true

# Iterate over handlers
for handler_dir in $TS_ROOT/*-handler; do
  if [ -d "$handler_dir" ]; then
    HANDLER_NAME=$(basename "$handler_dir")

    # Determine build directory (default to build, check tsconfig if you want robustness later)
    BUILD_DIR="$handler_dir/build"

    if [ ! -d "$BUILD_DIR" ]; then
      echo "Warning: No build directory found for $HANDLER_NAME. Skipping."
      continue
    fi

    echo "Uploading source maps for $HANDLER_NAME..."

    # URL Prefix strategy:
    # Runtime path: /workspace/{handler-name}/build/index.js
    # Sentry Artifact: ~/{handler-name}/build/index.js
    URL_PREFIX="~/$HANDLER_NAME/build"

    # Upload everything in build dir (js and maps)
    npx @sentry/cli releases files "$RELEASE" upload-sourcemaps "$BUILD_DIR" \
      --url-prefix "$URL_PREFIX" \
      --org "$ORG" \
      --project "$PROJECT" \
      --ext js --ext map
  fi
done

echo "Finalizing release $RELEASE..."
npx @sentry/cli releases finalize "$RELEASE" --org "$ORG" --project "$PROJECT"
echo "Source map upload complete!"
