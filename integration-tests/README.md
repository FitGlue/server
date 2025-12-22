# Integration Tests

This directory contains integration tests for the FitGlue application, supporting both local development and deployed environments.

## Test Suites

### Local Validation (`local.test.ts`)
Tests local Functions Framework instances running on localhost.

- **Purpose**: Fast feedback during local development
- **Triggers**: Direct HTTP calls to all functions (including Pub/Sub-triggered ones via CloudEvent format)
- **Requirements**: Local functions must be running (`./scripts/local_run.sh`)

**Run:**
```bash
./scripts/local_run.sh  # Start local functions
npm run test:local
```

### Deployed Validation (`deployed.test.ts`)
Tests deployed Cloud Functions in GCP environments (dev, test, prod).

- **Purpose**: Validate deployed infrastructure and end-to-end flows
- **Triggers**:
  - HTTP calls to public endpoints (`hevy-webhook-handler`)
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

# Test against Test environment (future)
npm run test:test
```

## Configuration

### Environment Variables

Set `TEST_ENVIRONMENT` to control which environment to test:

- `local` (default) - localhost Functions Framework instances
- `dev` - fitglue-server-dev GCP project
- `test` - fitglue-server-test GCP project
- `prod` - fitglue-server-prod GCP project

Example:
```bash
export TEST_ENVIRONMENT=dev
npm run test:deployed
```

### Environment Configuration

Environment-specific settings are defined in `environments.json`:

- Project ID
- Region
- GCS bucket name
- Function endpoints (HTTP-triggered)
- Pub/Sub topic names (event-triggered)

## Architecture

### Configuration (`config.ts`)
Loads environment-specific configuration from `environments.json` based on `TEST_ENVIRONMENT`.

### Test Setup (`setup.ts`)
- Creates test users in Firestore
- Cleans up test data after tests complete
- Uses config module for environment-aware setup

### Pub/Sub Helpers (`pubsub-helpers.ts`)
Utilities for publishing messages to Pub/Sub topics to trigger deployed Cloud Functions:
- `publishRawActivity()` - triggers Enricher
- `publishEnrichedActivity()` - triggers Router
- `publishUploadJob()` - triggers Strava Uploader

### Verification Helpers (`verification-helpers.ts`)
Async utilities for verifying function execution:
- `waitForFirestoreDoc()` - polls for Firestore document creation/updates
- `waitForGcsFile()` - polls for GCS file creation
- `waitForExecutionActivity()` - verifies function execution via execution logs

## Authentication

### Local Tests
No authentication required - tests run against localhost.

### Deployed Tests
Requires GCP Application Default Credentials (ADC):

```bash
gcloud auth application-default login
```

This provides access to:
- Pub/Sub (for publishing messages)
- Firestore (for test user setup/cleanup and verification)
- Cloud Storage (for artifact cleanup and verification)

## Troubleshooting

### "Timeout waiting for execution activity"
- **Cause**: Function didn't execute or took too long
- **Solutions**:
  - Check Cloud Functions logs in GCP Console
  - Verify Pub/Sub topic configuration
  - Increase timeout in test (cold starts can take 30-45s)

### "Pub/Sub topics not configured for this environment"
- **Cause**: Running deployed tests in local environment
- **Solution**: Set `TEST_ENVIRONMENT=dev` (or test/prod)

### "Invalid TEST_ENVIRONMENT"
- **Cause**: Typo in environment name
- **Solution**: Use one of: `local`, `dev`, `test`, `prod`

### Authentication errors
- **Cause**: ADC not configured or expired
- **Solution**: Run `gcloud auth application-default login`

## Best Practices

1. **Run local tests frequently** during development for fast feedback
2. **Run deployed tests** before merging to verify infrastructure changes
3. **Clean up test data** - tests automatically clean up, but verify in GCP Console if tests fail
4. **Monitor costs** - deployed tests publish real Pub/Sub messages and invoke Cloud Functions
5. **Use unique test user IDs** - tests use UUIDs to avoid conflicts

## CI/CD Integration

Integration tests can be run in CI/CD pipelines:

```yaml
# Example CircleCI job
- run:
    name: Integration Tests - Dev
    command: |
      gcloud auth activate-service-account --key-file=${GOOGLE_APPLICATION_CREDENTIALS}
      npm run test:dev
```

Ensure the service account has necessary permissions:
- `roles/pubsub.publisher`
- `roles/datastore.user`
- `roles/storage.objectAdmin` (for test bucket)
