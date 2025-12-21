# Local Development

This guide explains how to run the entire FitGlue stack locally without deploying to Google Cloud.

## Prerequisites
See [README.md](README.md) for core prerequisites.
- **Functions Framework**: Installed via `npm` (TS) and `go.mod` (Go).

**Tip:** Run `make setup` first to install all dependencies and generate necessary code.

## 1. Configuration (`.env`)

Create a `.env` file to configure local secrets (bypassing Google Secret Manager).

```bash
cp .env.example .env
```

Edit `.env` to set your mock secrets:
```bash
GOOGLE_CLOUD_PROJECT=fitglue-local
HEVY_SIGNING_SECRET=local-secret
```

## 2. Starting Services

You can start all 5 services simultaneously using the orchestration script.

```bash
make local
```

This will spin up:
- **Hevy Handler** (:8080)
- **Enricher** (:8081)
- **Router** (:8082)
- **Strava Uploader** (:8083)
- **Keiser Poller** (:8084)

Logs are written to individual log files in the root directory (`hevy.log`, `enricher.log`, etc.).
Press **Ctrl+C** to stop all services.

### Manual Start (Debugging)
If you need to debug a single service, you can run it in isolation:

| Service | Port | Command |
|---------|------|---------|
| Hevy Handler | 8080 | `cd functions/hevy-handler && npm run dev` |
| Enricher | 8081 | `cd functions/enricher && go run cmd/main.go` |
| Router | 8082 | `cd functions/router && go run cmd/main.go` |
| Strava Uploader | 8083 | `cd functions/strava-uploader && go run cmd/main.go` |
| Keiser Poller | 8084 | `cd functions/keiser-poller && PORT=8084 npm run dev` |

## 3. Triggering Events (Simulations)

We provide Node.js scripts to simulate various events in the pipeline. These scripts construct the correct CloudEvent or HTTP payloads expected by the functions.

### A. Ingestion Layer
**Simulate Hevy Webhook**
Sends a signed JSON payload to the Hevy Handler.
```bash
node scripts/trigger_hevy.js
```

**Simulate Keiser Poll**
Trigger the Keiser Poller schedule.
```bash
node scripts/trigger_keiser.js
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
