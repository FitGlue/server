# FitGlue

**FitGlue** is a serverless fitness data aggregation and routing platform built on Google Cloud Platform. It ingests workout data from multiple sources (Hevy, Keiser, Fitbit), enriches it with standardized formats (FIT files), and routes it to connected services like Strava.

## Architecture

FitGlue uses an event-driven, microservices architecture deployed as Google Cloud Functions (Gen 2):

```
┌─────────────────┐
│  Data Sources   │
│  (Hevy, Keiser) │
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
- **Keiser Poller** (TypeScript): Scheduled poller for Keiser M3i bike sessions
- **Enricher** (Go): Converts raw activity data to FIT files and stores in GCS
- **Router** (Go): Routes enriched activities to configured destinations
- **Strava Uploader** (Go): Uploads FIT files to Strava via OAuth

## Features

- 🔄 **Multi-source ingestion**: Hevy webhooks, Keiser polling, extensible for Fitbit/Garmin
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

- **[Execution Logging](docs/EXECUTION_LOGGING.md)** - Framework architecture and patterns
- **[Local Development](docs/LOCAL_DEVELOPMENT.md)** - Running the stack locally
- **[CI/CD Guide](docs/CICD.md)** - Deployment pipeline and infrastructure
- **[Architecture Decisions](docs/DECISIONS.md)** - Key design choices and rationale
- **[Integration Tests](docs/INTEGRATION_TESTS.md)** - Integration tests and verification
- **[Admin CLI](docs/ADMIN_CLI.md)** - Usage guide for user management and keys

## Research

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
│   │   ├── keiser-poller/
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
