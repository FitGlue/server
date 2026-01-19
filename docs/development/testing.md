# Testing Guide

This guide covers all testing approaches for FitGlue: unit tests, integration tests, and manual QA procedures.

## Overview

FitGlue uses a multi-layered testing strategy:

| Test Type | Purpose | Frequency |
|-----------|---------|-----------|
| Unit Tests | Validate individual functions and components | Every commit |
| Integration Tests | Validate deployed Cloud Functions and pipelines | Per deployment |
| Manual QA | End-to-end verification with real services | Major features |

---

## 1. Unit Tests

### Running Tests

```bash
# All tests
make test

# Go tests only
make test-go

# TypeScript tests only
make test-ts
```

### Go Unit Tests

Located in `src/go/pkg/**/*_test.go` and function directories.

```bash
cd src/go
go test ./pkg/enricher_providers/... -v
```

### TypeScript Unit Tests

Located in `src/typescript/*/src/**/*.test.ts`.

```bash
cd src/typescript/shared
npm test
```

---

## 2. Integration Tests

Integration tests validate deployed Cloud Functions in GCP environments.

### Test Suites

#### Local Validation (`local.test.ts`)

Tests local Functions Framework instances running on localhost.

- **Purpose**: Fast feedback during local development
- **Triggers**: Direct HTTP calls to all functions (including Pub/Sub-triggered ones via CloudEvent format)
- **Requirements**: Local functions must be running (`./scripts/local_run.sh`)

**Run:**
```bash
./scripts/local_run.sh  # Start local functions
npm run test:local
```

#### Deployed Validation (`deployed.test.ts`)

Tests deployed Cloud Functions in GCP environments (dev, test, prod).

- **Purpose**: Validate deployed infrastructure and end-to-end flows
- **Triggers**:
  - HTTP calls to public endpoints (`hevy-handler`)
  - Pub/Sub message publishing for event-triggered functions (`enricher`, `router`, `strava-uploader`)
- **Requirements**:
  - GCP authentication configured (`gcloud auth application-default login`)
  - `TEST_ENVIRONMENT` set to target environment

**Run:**
```bash
# Authenticate with GCP
gcloud auth application-default login

# Test against Dev environment
npm run test:dev

# Test against Test environment
npm run test:test
```

### Test Run ID Tracking

Each test generates a unique `testRunId` that is propagated through all function executions:

- **HTTP Functions**: Passed via `X-Test-Run-Id` header
- **Pub/Sub Functions**: Passed via message attributes

This enables:
- ✅ Precise verification of test executions
- ✅ Complete cleanup of test data
- ✅ No test pollution between runs
- ✅ Parallel test execution (future)

**Example:**
```typescript
describe('My Test Suite', () => {
  const testRunIds: string[] = []; // Track all test run IDs

  it('should process activity', async () => {
    const testRunId = randomUUID(); // Unique ID per test
    testRunIds.push(testRunId);

    // HTTP request
    await axios.post(endpoint, payload, {
      headers: { 'X-Test-Run-Id': testRunId }
    });

    // Pub/Sub message
    await publishRawActivity(payload, testRunId);

    // Verify by test run ID (not timestamp!)
    await waitForExecutionActivity({
      testRunId,
      minExecutions: 1
    });
  });

  afterAll(async () => {
    // Clean up all executions from all tests
    await cleanupExecutions(testRunIds);
    await cleanupGCSArtifacts(userId);
    await cleanupTestUser(userId);
  });
});
```

### Configuration

#### Environment Variables

Set `TEST_ENVIRONMENT` to control which environment to test:

- `local` (default) - localhost Functions Framework instances
- `dev` - fitglue-server-dev GCP project
- `test` - fitglue-server-test GCP project
- `prod` - fitglue-server-prod GCP project

#### Environment Configuration

Environment-specific settings are defined in `environments.json`:

- Project ID
- Region
- GCS bucket name
- Function endpoints (HTTP-triggered)
- Pub/Sub topic names (event-triggered)

### Architecture

| Module | Purpose |
|--------|---------|
| `config.ts` | Loads environment-specific configuration |
| `setup.ts` | Creates test users in Firestore |
| `cleanup.ts` | Deletes execution records, users, and GCS artifacts |
| `pubsub-helpers.ts` | Publishes messages to Pub/Sub topics |
| `verification-helpers.ts` | Waits for and verifies executions |

### Authentication

**Local Tests**: No authentication required.

**Deployed Tests**: Requires GCP Application Default Credentials:
```bash
gcloud auth application-default login
```

---

## 3. Manual QA

Manual QA is used for end-to-end verification with real external services (Hevy, Fitbit, Strava).

### Prerequisites

- Access to `fitglue-admin` CLI
- Running Dev environment (or Prod via correct GCloud project)
- User's **Hevy API Key** (from Hevy App Settings → API)
- **Client IDs** for Strava and Fitbit (from developer portals)

### User Setup

#### Step 1: Create the User & Ingress Key

```bash
./fitglue-admin users:create
```

1. **Copy the User ID** (e.g., `user-123`)
2. **Generate Ingress Key**: Select "Yes"
3. **Label**: e.g., "Manual Test"
4. **Copy the Ingress Key** (`fg_sk_...`)

#### Step 2: Configure Hevy Integration

```bash
./fitglue-admin users:configure-hevy user-123
```
*Enter the user's personal Hevy API Key.*

#### Step 3: Configure Hevy Webhook

Add the webhook in the Hevy App or Dashboard:

- **URL**: `https://[env].fitglue.tech/hooks/hevy` (e.g., `https://dev.fitglue.tech/hooks/hevy`)
- **Secret**: Paste the **Ingress Key** from Step 1

#### Step 4: Connect OAuth Services

**Connect Strava:**
```bash
./fitglue-admin users:connect user-123 strava
```

**Connect Fitbit:**
```bash
./fitglue-admin users:connect user-123 fitbit
```

### Pipeline Configuration

Example: Hevy → Fitbit HR → Strava

```bash
./fitglue-admin users:add-pipeline user-123
```

1. **Source**: Select `SOURCE_HEVY`
2. **Enrichers**: Choose `fitbit-heart-rate`
3. **Destinations**: Check `strava`

### Verification

#### Trigger a Workout

1. **Real**: Finish a workout in Hevy
2. **Simulation**: Use curl to send a dummy webhook

#### Trace Execution

```bash
# Check each stage
./fitglue-admin executions:list -s hevy-handler -u user-123
./fitglue-admin executions:list -s enricher -u user-123
./fitglue-admin executions:list -s router -u user-123
./fitglue-admin executions:list -s strava-uploader-job -u user-123
```

**Success indicators:**
- `STATUS_SUCCESS` on each stage
- `published_count: 1` in enricher output
- Activity appears in Strava feed

---

## Troubleshooting

### Integration Tests

| Issue | Cause | Solution |
|-------|-------|----------|
| "Timeout waiting for execution" | Function didn't execute | Check Cloud Functions logs, increase timeout |
| "Pub/Sub topics not configured" | Wrong environment | Set `TEST_ENVIRONMENT=dev` |
| Authentication errors | ADC expired | Run `gcloud auth application-default login` |
| Execution records not found | test_run_id not extracted | Verify header/attribute passing |

### Manual QA

| Issue | Cause | Solution |
|-------|-------|----------|
| "Provider not found" | Enricher name mismatch | Check registered provider in enricher/function.go |
| No executions listed | Infrastructure error | Check Cloud Logging for 500 errors |
| OAuth failures | Callback URL mismatch | Verify developer portal settings |

---

## Best Practices

1. **Generate test run ID per test** - Not per suite
2. **Track all test run IDs** - For comprehensive cleanup
3. **Verify by test run ID** - Never by timestamp
4. **Clean up by test run ID** - Ensures complete cleanup
5. **Use unique user IDs** - Avoid conflicts between test runs
6. **Monitor costs** - Deployed tests invoke real Cloud Functions

---

## Related Documentation

- [Local Development](local-development.md) - Running the stack locally
- [Execution Logging](../architecture/execution-logging.md) - Framework architecture
- [Admin CLI Reference](../reference/admin-cli.md) - CLI commands
