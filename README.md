# FitGlue Server

FitGlue is a serverless activity integration platform that ingests workout data from various sources (Hevy, Keiser), normalizes it, and routes it to destinations (Strava).

## Project Structure

- **functions/**: Cloud Functions (Go & TypeScript).
  - `enricher` (Go): Enriches raw activity data.
  - `router` (Go): Routes enriched data to destinations.
  - `strava-uploader` (Go): Uploads FIT files to Strava.
  - `hevy-handler` (TS): Webhook handler for Hevy workouts.
  - `keiser-poller` (TS): Scheduled job for Keiser machine data.
- **shared/**: Shared libraries and Protocol Buffers.
  - `go/`: Go shared code (Bootstrap, types).
  - `typescript/`: TypeScript shared code (Framework, types).
  - `proto/`: Protobuf definitions.
- **terraform/**: Infrastructure as Code (GCP).
- **scripts/**: Verification and simulation scripts.

## Prerequisites

- **Go**: v1.21+
- **Node.js**: v18+
- **Terraform**: v1.0+
- **gcloud CLI**: Authenticated with your GCP project.
- **Make**: For running unified commands.

## Quick Start (Fresh Setup)

One command to rule them all. If you've just cloned the repo:

1. **Configure Environment**:
    ```bash
    cp .env.example .env
    # Edit .env to add your secrets (for local dev)
    ```

2. **Setup**:
    ```bash
    make setup
    ```
    This single command installs dependencies (npm/go), generates protobuf code, injects shared libraries, and builds all functions.

3. **Verify**:
    ```bash
    make test
    ```

## Unified Operations (Makefile)

We provide a `Makefile` to simplify common development tasks.

### 1. Setup (One-Time)
Installs dependencies, generates code, and builds everything.
```bash
make setup
```

### 2. Build
Builds all TypeScript and Go functions to verify compilation.
```bash
make build-ts
make build-go
# or
make all
```

### 2. Test
Runs all unit tests across the codebase.
```bash
make test
```

### 3. Deploy (Dev)
Deploys the entire stack to the `fitglue-server-dev` environment using Terraform.
> **Note:** Requires active GCP credentials (`gcloud auth application-default login`).
```bash
make deploy-dev
```

### 4. Verify (Dev)
Runs end-to-end verification scripts against the deployed dev environment.
```bash
make verify-dev
```

## Local Development
For running the stack locally using the Functions Framework and emulators, refer to [Local Development Guide](README_LOCAL.md).

## CI/CD Pipeline
We use CircleCI with OIDC authentication for secure, keyless deployments to GCP. The pipeline automatically deploys to Dev on `main` branch commits, with manual approval gates for Test and Prod.

For detailed setup instructions (including configuring OIDC for new environments), see [CI/CD Deployment Guide](docs/CICD.md).
