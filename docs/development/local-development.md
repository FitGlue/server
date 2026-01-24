# Local Development

This guide explains how to run the entire FitGlue stack locally without deploying to Google Cloud.

## Prerequisites

- **Go 1.25+**: [Install Go](https://go.dev/doc/install)
- **Node.js 20+**: [Install Node](https://nodejs.org/)
- **Protocol Buffers Compiler**: `brew install protobuf` (macOS) or `sudo apt-get install protobuf-compiler` (Linux)
- **Functions Framework**: Installed automatically via `make setup`

## 1. Initial Setup

Install all dependencies and generate Protocol Buffer code:

```bash
make setup
```

This will:
- Download Go module dependencies
- Install npm packages (workspace mode)
- Link the `@fitglue/shared` TypeScript library

## 2. Configuration (`.env`)

Create a `.env` file to configure local secrets (bypassing Google Secret Manager).

```bash
cp .env.example .env
```

Edit `.env` to set your mock secrets:

```bash
GOOGLE_CLOUD_PROJECT=fitglue-local
HEVY_SIGNING_SECRET=local-secret
```

## 3. Starting Services

### All Services at Once

Start all 5 services simultaneously using the orchestration script:

```bash
./scripts/local_run.sh
```

This will spin up:
- **Hevy Handler** (`:8080`) - Webhook receiver
- **Enricher** (`:8081`) - FIT file generator
- **Router** (`:8082`) - Activity router
- **Strava Uploader** (`:8083`) - Strava integration

Logs are written to individual log files in the root directory (`hevy.log`, `enricher.log`, etc.).

Press **Ctrl+C** to stop all services.

### Manual Start (Debugging)

If you need to debug a single service, run it in isolation:

| Service | Port | Command |
|---------|------|---------|
| Hevy Handler | 8080 | `cd src/typescript/hevy-handler && npm run dev` |
| Enricher | 8081 | `cd src/go/functions/enricher && FUNCTION_TARGET=EnrichActivity go run cmd/main.go` |
| Router | 8082 | `cd src/go/functions/router && FUNCTION_TARGET=RouteActivity go run cmd/main.go` |
| Strava Uploader | 8083 | `cd src/go/functions/strava-uploader && FUNCTION_TARGET=UploadToStrava go run cmd/main.go` |

## 4. Triggering Events (Simulations)

We provide Node.js scripts to simulate various events in the pipeline. These scripts construct the correct CloudEvent or HTTP payloads expected by the functions.

### A. Ingestion Layer

**Simulate Hevy Webhook**

Sends a signed JSON payload to the Hevy Handler.

```bash
node scripts/trigger_hevy.js
```



### B. Transformation Layer

**Simulate Raw Activity Event**

Injects a Pub/Sub message (RawActivity protobuffer) into the Enricher.

```bash
node scripts/trigger_enricher.js
```

### C. Routing & Egress

**Simulate Enrichment Complete**

Injects an EnrichedActivity event into the Router.

```bash
node scripts/trigger_router.js
```

**Simulate Strava Upload Job**

Injects an upload job directly to the Strava Uploader.

```bash
node scripts/trigger_uploader.js
```

## 5. Running Tests

### All Tests

```bash
make test
```

### Go Tests Only

```bash
make test-go
```

### TypeScript Tests Only

```bash
make test-ts
```

### Integration Tests

Integration tests require all services to be running locally:

```bash
# Terminal 1: Start services
./scripts/local_run.sh

# Terminal 2: Run integration tests
cd integration-tests
npm test
```

## 6. Building

### Build All

```bash
make build
```

### Build Go Services

```bash
make build-go
```

### Build TypeScript Services

```bash
make build-ts
```

## 7. Linting

```bash
make lint
```

## 8. Cleaning

Remove build artifacts:

```bash
make clean
```

## Troubleshooting

### "Cannot find module '@fitglue/shared'"

Run `make setup` to ensure workspace linking is correct.

### "Firestore credentials not found" in tests

This is expected. Tests use mocks and don't require real GCP credentials.

### Port already in use

Kill existing processes:

```bash
lsof -ti:8080 | xargs kill -9  # Replace 8080 with the conflicting port
```

## Next Steps

- See [CI/CD Guide](../infrastructure/cicd.md) for deployment instructions
- See [OpenAPI Clients](../guides/openapi-clients.md) for external API integration patterns
- See [Architecture Decisions](../decisions/ADR.md) for design rationale
