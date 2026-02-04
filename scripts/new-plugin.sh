#!/bin/bash
#
# FitGlue Plugin Scaffolding Script
#
# Usage:
#   ./scripts/new-plugin.sh source garmin
#   ./scripts/new-plugin.sh enricher weather
#   ./scripts/new-plugin.sh destination runkeeper
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVER_DIR="$(dirname "$SCRIPT_DIR")"
PROTO_DIR="$SERVER_DIR/src/proto"
GO_PKG_DIR="$SERVER_DIR/src/go/pkg"
GO_FUNC_DIR="$SERVER_DIR/src/go/functions"
TS_DIR="$SERVER_DIR/src/typescript"
TERRAFORM_DIR="$SERVER_DIR/terraform"
WEB_DIR="$(dirname "$SERVER_DIR")/web"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

usage() {
    echo "Usage: $0 <type> <name>"
    echo ""
    echo "Types:"
    echo "  source        Create a new ingress source (TypeScript webhook handler)"
    echo "  enricher      Create a new enricher provider (Go pipeline step)"
    echo "  destination   Create a new destination (Go uploader)"
    echo ""
    echo "Examples:"
    echo "  $0 source garmin"
    echo "  $0 enricher weather"
    echo "  $0 destination runkeeper"
    exit 1
}

# Convert name to various cases
to_pascal_case() {
    echo "$1" | sed -r 's/(^|_)([a-z])/\U\2/g' | sed 's/_//g'
}

to_snake_case() {
    echo "$1" | sed 's/[A-Z]/_\L&/g' | sed 's/^_//' | tr '[:upper:]' '[:lower:]'
}

to_upper_snake() {
    echo "$1" | tr '[:lower:]' '[:upper:]' | tr '-' '_'
}

to_kebab_case() {
    echo "$1" | tr '[:upper:]' '[:lower:]' | tr '_' '-'
}

# ============================================================================
# SOURCE SCAFFOLDING (TypeScript)
# ============================================================================
create_source() {
    local name="$1"
    local pascal_name=$(to_pascal_case "$name")
    local kebab_name=$(to_kebab_case "$name")
    local upper_name=$(to_upper_snake "$name")

    local handler_dir="$TS_DIR/${kebab_name}-handler"

    if [[ -d "$handler_dir" ]]; then
        echo -e "${RED}Error: Handler already exists at $handler_dir${NC}"
        exit 1
    fi

    echo -e "${GREEN}Creating source: $name${NC}"

    # Create directory structure
    mkdir -p "$handler_dir/src"

    # package.json
    cat > "$handler_dir/package.json" << EOF
{
  "name": "${kebab_name}-handler",
  "version": "1.0.0",
  "main": "dist/index.js",
  "scripts": {
    "build": "tsc",
    "gcp-build": "npm run build",
    "test": "jest",
    "lint": "eslint src --ext .ts",
  },
  "dependencies": {
    "@fitglue/shared": "^1.0.0",
  },
  "devDependencies": {
    "typescript": "^5.3.3"
  }
}
EOF

    # tsconfig.json
    cat > "$handler_dir/tsconfig.json" << EOF
{
  "compilerOptions": {
    "module": "commonjs",
    "noImplicitReturns": true,
    "noUnusedLocals": true,
    "outDir": "dist",
    "sourceMap": true,
    "strict": true,
    "target": "es2017",
    "moduleResolution": "node",
    "esModuleInterop": true,
    "skipLibCheck": true
  },
  "compileOnSave": true,
  "include": [
    "src"
  ]
}

EOF

    # src/index.ts
    cat > "$handler_dir/src/index.ts" << EOF
import { createCloudFunction, createWebhookProcessor, ApiKeyStrategy } from '@fitglue/shared';
import { ${pascal_name}Connector } from './connector';

/**
 * ${pascal_name} Webhook Handler
 *
 * Receives webhooks from ${pascal_name} and ingests activities.
 */
export const ${name}WebhookHandler = createCloudFunction(
    createWebhookProcessor(${pascal_name}Connector),
    {
        auth: {
            strategies: [new ApiKeyStrategy()],
            requiredScopes: ['read:activity']
        }
    }
);
EOF

    # src/connector.ts
    cat > "$handler_dir/src/connector.ts" << EOF
import {
  BaseConnector,
  ConnectorConfig,
  IngestStrategy,
  StandardizedActivity,
  CloudEventSource,
  ActivitySource,
  FrameworkContext
} from '@fitglue/shared';

export interface ${pascal_name}ConnectorConfig extends ConnectorConfig {
  // Add ${name}-specific config fields here
  apiKey?: string;
}

/**
 * ${pascal_name} Connector
 *
 * Handles webhook ingestion and activity mapping for ${pascal_name}.
 *
 * TODO:
 * 1. Implement extractId() to get activity ID from webhook payload
 * 2. Implement fetchAndMap() to fetch full activity and convert to StandardizedActivity
 * 3. Add CloudEventSource.CLOUD_EVENT_SOURCE_${upper_name} to events.proto
 * 4. Add ActivitySource.SOURCE_${upper_name} to activity.proto
 */
export class ${pascal_name}Connector extends BaseConnector<${pascal_name}ConnectorConfig, unknown> {
  readonly name = '${name}';
  readonly strategy: IngestStrategy = 'webhook';

  // TODO: Add these to proto files and regenerate
  readonly cloudEventSource = CloudEventSource.CLOUD_EVENT_SOURCE_UNSPECIFIED;
  readonly activitySource = ActivitySource.SOURCE_UNSPECIFIED;

  constructor(context: FrameworkContext) {
    super(context);
    context.logger.debug(\`${pascal_name}Connector: initialized\`);
  }

  extractId(body: unknown): string | null {
    // TODO: Extract activity ID from webhook payload
    // Example: return (body as { id?: string }).id || null;
    return null;
  }

  async fetchAndMap(activityId: string, config: ${pascal_name}ConnectorConfig): Promise<StandardizedActivity[]> {
    // TODO: Fetch activity from ${pascal_name} API and map to StandardizedActivity
    this.context.logger.debug(\`${pascal_name}Connector: fetching activity \${activityId}\`);

    throw new Error('Not implemented: fetchAndMap');
  }
}
EOF

    echo -e "${GREEN}✓ Created $handler_dir${NC}"

    # Add export to index.js
    cat >> "$TS_DIR/index.js" << EOF

exports.${name}WebhookHandler = (req, res) => {
  const handler = require('./${kebab_name}-handler/build/index');
  return handler.${name}WebhookHandler(req, res);
};
EOF
    echo -e "${GREEN}✓ Added export to index.js${NC}"

    # Add Terraform config
    cat >> "$TERRAFORM_DIR/functions.tf" << EOF

# ----------------- ${pascal_name} Handler -----------------
resource "google_cloudfunctions2_function" "${snake_name}_handler" {
  name        = "${kebab_name}-handler"
  location    = var.region
  description = "${pascal_name} webhook handler"

  build_config {
    runtime     = "nodejs22"
    entry_point = "${name}WebhookHandler"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.typescript_source_zip.name
      }
    }
    environment_variables = {}
  }

  service_config {
    available_memory = "256Mi"
    timeout_seconds  = 60
    environment_variables = {
      LOG_LEVEL            = var.log_level
      GOOGLE_CLOUD_PROJECT = var.project_id
    }
    service_account_email = google_service_account.cloud_function_sa.email
  }
}

resource "google_cloud_run_service_iam_member" "${snake_name}_handler_invoker" {
  project  = google_cloudfunctions2_function.${snake_name}_handler.project
  location = google_cloudfunctions2_function.${snake_name}_handler.location
  service  = google_cloudfunctions2_function.${snake_name}_handler.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}
EOF
    echo -e "${GREEN}✓ Added Terraform config to functions.tf${NC}"

    echo ""
    echo -e "${YELLOW}Next steps:${NC}"
    echo "  1. Add CloudEventSource.CLOUD_EVENT_SOURCE_${upper_name} to src/proto/events.proto"
    echo "  2. Add ActivitySource.SOURCE_${upper_name} to src/proto/activity.proto"
    echo "  3. Run 'make generate' to regenerate types"
    echo "  4. Implement the connector in $handler_dir/src/connector.ts"
    echo "  5. Add Firebase rewrite to web/firebase.json for /api/${kebab_name}/webhook"
}

# ============================================================================
# ENRICHER SCAFFOLDING (Go)
# ============================================================================
create_enricher() {
    local name="$1"
    local pascal_name=$(to_pascal_case "$name")
    local snake_name=$(to_snake_case "$name")
    local upper_name=$(to_upper_snake "$name")

    local provider_file="$GO_PKG_DIR/enricher_providers/${snake_name}.go"

    if [[ -f "$provider_file" ]]; then
        echo -e "${RED}Error: Provider already exists at $provider_file${NC}"
        exit 1
    fi

    echo -e "${GREEN}Creating enricher: $name${NC}"

    # Find next available enum value
    local last_enum=$(grep -E "ENRICHER_PROVIDER_[A-Z_]+ = [0-9]+" "$PROTO_DIR/user.proto" | \
                      grep -v MOCK | \
                      sed 's/.*= \([0-9]*\).*/\1/' | \
                      sort -n | tail -1)
    local next_enum=$((last_enum + 1))

    echo "  Using enum value: $next_enum"

    # Add enum to user.proto (before MOCK which is 99)
    sed -i "/ENRICHER_PROVIDER_MOCK = 99;/i\\  ENRICHER_PROVIDER_${upper_name} = ${next_enum};" "$PROTO_DIR/user.proto"

    echo -e "${GREEN}✓ Added ENRICHER_PROVIDER_${upper_name} = ${next_enum} to user.proto${NC}"

    # Create Go provider file
    cat > "$provider_file" << EOF
package enricher_providers

import (
	"context"

	"github.com/fitglue/server/src/go/pkg/plugin"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

func init() {
	// Register manifest for Discovery API
	plugin.RegisterManifest(pb.EnricherProviderType_ENRICHER_PROVIDER_${upper_name}, &pb.PluginManifest{
		Id:          "${snake_name}",
		Type:        pb.PluginType_PLUGIN_TYPE_ENRICHER,
		Name:        "${pascal_name}",
		Description: "TODO: Add description for ${pascal_name} enricher",
		Icon:        "✨",
		Enabled:     true,
		ConfigSchema: []*pb.ConfigFieldSchema{
			// TODO: Add configuration fields
			// {
			// 	Key:         "example_field",
			// 	Label:       "Example Field",
			// 	Description: "An example configuration field",
			// 	FieldType:   pb.ConfigFieldType_CONFIG_FIELD_TYPE_STRING,
			// 	Required:    false,
			// },
		},
	})
	Register(New${pascal_name}Provider())
}

// ${pascal_name}Provider enriches activities with TODO
type ${pascal_name}Provider struct{}

func New${pascal_name}Provider() *${pascal_name}Provider {
	return &${pascal_name}Provider{}
}

func (p *${pascal_name}Provider) Name() string {
	return "${snake_name}"
}

func (p *${pascal_name}Provider) ProviderType() pb.EnricherProviderType {
	return pb.EnricherProviderType_ENRICHER_PROVIDER_${upper_name}
}

func (p *${pascal_name}Provider) Enrich(
	ctx context.Context,
	activity *pb.StandardizedActivity,
	user *pb.UserRecord,
	inputConfig map[string]string,
	doNotRetry bool,
) (*EnrichmentResult, error) {
	// TODO: Implement enrichment logic

	return &EnrichmentResult{
		// Title:       "New Title",
		// Description: "Additional description",
		Metadata: map[string]string{
			"enricher": "${snake_name}",
		},
	}, nil
}
EOF

    echo -e "${GREEN}✓ Created $provider_file${NC}"

    # Run make generate
    echo ""
    echo "Running 'make generate' to regenerate types..."
    (cd "$SERVER_DIR" && make generate)

    echo ""
    echo -e "${GREEN}✓ Enricher scaffolding complete!${NC}"
    echo ""
    echo -e "${YELLOW}Next steps:${NC}"
    echo "  1. Implement the Enrich() method in $provider_file"
    echo "  2. Add config fields to the manifest if needed"
    echo "  3. Run 'make test-go' to verify"
}

# ============================================================================
# DESTINATION SCAFFOLDING (Go)
# ============================================================================
create_destination() {
    local name="$1"
    local pascal_name=$(to_pascal_case "$name")
    local snake_name=$(to_snake_case "$name")
    local upper_name=$(to_upper_snake "$name")
    local kebab_name=$(to_kebab_case "$name")

    local uploader_dir="$GO_FUNC_DIR/${kebab_name}-uploader"

    if [[ -d "$uploader_dir" ]]; then
        echo -e "${RED}Error: Uploader already exists at $uploader_dir${NC}"
        exit 1
    fi

    echo -e "${GREEN}Creating destination: $name${NC}"

    # Add enum to events.proto
    # Find the Destination enum and add new value
    local last_dest_enum=$(grep -E "DESTINATION_[A-Z_]+ = [0-9]+" "$PROTO_DIR/events.proto" | \
                           grep -v MOCK | \
                           sed 's/.*= \([0-9]*\).*/\1/' | \
                           sort -n | tail -1)
    local next_dest_enum=$((last_dest_enum + 1))

    # Find DESTINATION_MOCK line and insert before it
    if grep -q "DESTINATION_MOCK" "$PROTO_DIR/events.proto"; then
        sed -i "/DESTINATION_MOCK/i\\  DESTINATION_${upper_name} = ${next_dest_enum};" "$PROTO_DIR/events.proto"
        echo -e "${GREEN}✓ Added DESTINATION_${upper_name} = ${next_dest_enum} to events.proto${NC}"
    else
        echo -e "${YELLOW}Note: Could not find DESTINATION_MOCK. Please add DESTINATION_${upper_name} manually.${NC}"
    fi

    # Create directory
    mkdir -p "$uploader_dir"

    # Create Go uploader function
    cat > "$uploader_dir/function.go" << EOF
package ${snake_name}uploader

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	shared "github.com/fitglue/server/src/go/pkg"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

func init() {
	functions.HTTP("${pascal_name}Uploader", handler)
}

func handler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	logger := slog.Default().With("function", "${snake_name}-uploader")

	// Decode the enriched activity from request body
	var payload struct {
		Activity *pb.StandardizedActivity \`json:"activity"\`
		UserID   string                   \`json:"user_id"\`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		logger.Error("Failed to decode request", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if payload.Activity == nil {
		http.Error(w, "Missing activity", http.StatusBadRequest)
		return
	}

	logger.Info("Uploading activity to ${pascal_name}",
		"activity_id", payload.Activity.SourceActivityId,
		"user_id", payload.UserID,
	)

	// TODO: Implement upload to ${pascal_name}
	// 1. Get user credentials from Firestore
	// 2. Convert StandardizedActivity to ${pascal_name} format
	// 3. Upload via ${pascal_name} API

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "${pascal_name} upload not yet implemented",
	})
}
EOF

    echo -e "${GREEN}✓ Created $uploader_dir/function.go${NC}"

    # Add Terraform config for Go uploader
    cat >> "$TERRAFORM_DIR/functions.tf" << EOF

# ${pascal_name} Uploader uses pre-built zip with correct structure
resource "google_storage_bucket_object" "${snake_name}_uploader_zip" {
  name   = "${kebab_name}-uploader-\${filemd5("/tmp/fitglue-function-zips/${kebab_name}-uploader.zip")}.zip"
  bucket = google_storage_bucket.source_bucket.name
  source = "/tmp/fitglue-function-zips/${kebab_name}-uploader.zip"
}

# ----------------- ${pascal_name} Uploader -----------------
resource "google_cloudfunctions2_function" "${snake_name}_uploader" {
  name     = "${kebab_name}-uploader"
  location = var.region

  build_config {
    runtime     = "go125"
    entry_point = "${pascal_name}Uploader"
    source {
      storage_source {
        bucket = google_storage_bucket.source_bucket.name
        object = google_storage_bucket_object.${snake_name}_uploader_zip.name
      }
    }
    environment_variables = {}
  }

  service_config {
    available_memory = "512Mi"
    timeout_seconds  = 300
    environment_variables = {
      GOOGLE_CLOUD_PROJECT = var.project_id
      GCS_ARTIFACT_BUCKET  = "\${var.project_id}-artifacts"
      LOG_LEVEL            = var.log_level
    }
    service_account_email = google_service_account.cloud_function_sa.email
  }

  event_trigger {
    trigger_region = var.region
    event_type     = "google.cloud.pubsub.topic.v1.messagePublished"
    pubsub_topic   = google_pubsub_topic.enriched_activity.id
    retry_policy   = var.retry_policy
  }
}
EOF
    echo -e "${GREEN}✓ Added Terraform config to functions.tf${NC}"

    # Run make generate
    echo ""
    echo "Running 'make generate' to regenerate types..."
    (cd "$SERVER_DIR" && make generate)

    echo ""
    echo -e "${GREEN}✓ Destination scaffolding complete!${NC}"
    echo ""
    echo -e "${YELLOW}Next steps:${NC}"
    echo "  1. Implement the upload logic in $uploader_dir/function.go"
    echo "  2. Add routing logic to router/function.go for DESTINATION_${upper_name}"
    echo "  3. Run 'make build' to verify"
}

# ============================================================================
# MAIN
# ============================================================================
if [[ $# -lt 2 ]]; then
    usage
fi

TYPE="$1"
NAME="$2"

# Validate name (lowercase letters, numbers, underscores)
if [[ ! "$NAME" =~ ^[a-z][a-z0-9_]*$ ]]; then
    echo -e "${RED}Error: Name must be lowercase letters, numbers, and underscores, starting with a letter${NC}"
    exit 1
fi

case "$TYPE" in
    source)
        create_source "$NAME"
        ;;
    enricher)
        create_enricher "$NAME"
        ;;
    destination)
        create_destination "$NAME"
        ;;
    *)
        echo -e "${RED}Error: Unknown type '$TYPE'${NC}"
        usage
        ;;
esac
