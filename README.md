# FitGlue

**FitGlue** is a serverless fitness data aggregation and routing platform built on Google Cloud Platform. It ingests workout data from multiple sources (Hevy, Fitbit), enriches it with standardized formats (FIT files), and routes it to connected services like Strava.

## Architecture

FitGlue uses an event-driven, microservices architecture deployed as Google Cloud Functions (Gen 2):

```
┌─────────────────┐
│  Data Sources   │
│     (Hevy)      │
└────────┬────────┘
         │
         ▼
┌─────────────────┐      ┌──────────────┐
│ Ingestion Layer │─────▶│  Pub/Sub     │
│ (Webhooks/Poll) │      │ (Raw Events) │
└─────────────────┘      └──────┬───────┘
                                │
                                ▼
                         ┌──────────────┐      ┌─────────────────┐
                         │  Enricher    │◀────▶│ External Data   │
                         │ (FIT Gen)    │      │ (e.g. FitBit)   │
                         └──────┬───────┘      └─────────────────┘
                                │
                                ▼
                         ┌──────────────┐
                         │    Router    │
                         └──────┬───────┘
                                │
                                ▼
                         ┌──────────────┐
                         │    Egress    │
                         │ Destinations │
                         │ (e.g. Strava)│
                         └──────────────┘
```

### Components

- **Hevy Handler** (TypeScript): Webhook receiver for Hevy workout data
- **Enricher** (Go): Converts raw activity data to FIT files and stores in GCS
- **Router** (Go): Routes enriched activities to configured destinations
- **Strava Uploader** (Go): Uploads FIT files to Strava via OAuth

### Enrichment Pipeline

The Enricher supports a flexible, configurable pipeline of providers that transform and enhance activities:

| Provider | Purpose | Output |
|----------|---------|--------|
| **Metadata Passthrough** | Preserves source metadata | Name, Description |
| **Workout Summary** | Exercise breakdown with sets/reps/weight | Description text |
| **Muscle Heatmap** | Visual muscle activation chart | Description (emoji bars) |
| **Virtual GPS** | Synthetic GPS routes for indoor activities | Lat/Long streams |
| **Source Link** | Links back to original workout | Description (URL) |
| **Fitbit Heart Rate** | Fetches HR data from Fitbit API | Heart rate stream |

Pipelines are configured per-user via the Admin CLI. See [Enricher Testing Guide](docs/ENRICHER_TESTING.md) for details.

## Features

- 🔄 **Multi-source ingestion**: Hevy webhooks, extensible for Fitbit/Garmin/other sources
- 📦 **Standardized output**: Generates industry-standard FIT files
- 🚀 **Serverless**: Auto-scaling Cloud Functions with Pub/Sub event routing
- 🔐 **Secure**: Secret Manager integration, HMAC signature verification
- 🧪 **Testable**: Comprehensive unit tests, integration tests with test run ID tracking
- 📊 **Observable**: Structured logging, automatic execution tracking, Firestore audit logs

- 🎯 **Framework-driven**: Consistent execution logging across all functions (Go & TypeScript)
## Tech Stack

- **Languages**: Go 1.25, TypeScript 5.x
- **Infrastructure**: Terraform, Google Cloud Functions v2
- **Storage**: Cloud Storage (FIT files), Firestore (metadata)
- **Messaging**: Cloud Pub/Sub
- **CI/CD**: CircleCI with OIDC authentication

## Documentation

### Getting Started
- **[Local Development](docs/LOCAL_DEVELOPMENT.md)** - Running the stack locally
- **[Admin CLI](docs/ADMIN_CLI.md)** - User management and pipeline configuration

### Enrichment & Testing
- **[Enricher Configuration](docs/ENRICHER_CONFIG.md)** - Configuring enrichment pipelines
- **[Enricher Testing](docs/ENRICHER_TESTING.md)** - Testing guide for enrichment providers
- **[Manual QA Guide](docs/MANUAL_QA_GUIDE.md)** - End-to-end manual testing procedures
- **[Integration Tests](docs/INTEGRATION_TESTS.md)** - Automated integration tests

### Infrastructure & Architecture
- **[CI/CD Guide](docs/CICD.md)** - Deployment pipeline and infrastructure
- **[Execution Logging](docs/EXECUTION_LOGGING.md)** - Framework architecture and patterns
- **[Architecture Decisions](docs/DECISIONS.md)** - Key design choices and rationale

### Specialized Guides
- **[FIT Generation](docs/FIT_GENERATION.md)** - Guide to generating FIT files and test data
- **[OAuth Integration](docs/OAUTH.md)** - OAuth flow for Strava and Fitbit
- **[OpenAPI Clients](docs/OPENAPI_CLIENTS.md)** - API client generation
- **[Fitbit Setup](docs/FITBIT_SETUP.md)** - Step-by-step Fitbit configuration

### Research

- **[Initial Research](docs/research/INITIAL_RESEARCH.md)** - Background and feasibility analysis
- **[API Integration Research](docs/research/API_INTEGRATION_RESEARCH.md)** - Specific API integrations

## Quick Start

### Prerequisites

- Go 1.25+
- Node.js 20+
- `protoc` (Protocol Buffers compiler)
- Google Cloud SDK (for deployment)

### Setup

```bash
# Install dependencies and generate code
make setup

# Build all services
make build

# Run tests
make test

# Start local development environment
make local
```

See [Local Development](docs/LOCAL_DEVELOPMENT.md) for detailed instructions.

## Project Structure

```
fitglue-server/
├── src/
│   ├── go/                 # Go monorepo
│   │   ├── functions/      # Cloud Functions
│   │   └── pkg/            # Shared libraries
│   ├── typescript/         # TypeScript workspace
│   │   ├── hevy-handler/
│   │   └── shared/         # @fitglue/shared
│   └── proto/              # Protocol Buffer definitions
├── terraform/              # Infrastructure as Code
├── scripts/                # Local development scripts
├── integration-tests/      # E2E tests
└── docs/                   # Documentation
```

## Contributing

This is a personal project, but suggestions and feedback are welcome via issues.

## License

MIT
