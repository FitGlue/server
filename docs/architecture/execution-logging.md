# Execution Logging and Framework Architecture

## Overview

FitGlue uses a standardized framework wrapper pattern for all Cloud Functions (both Go and TypeScript) that provides:

- **Automatic execution logging** - All function invocations are logged to Firestore
- **Consistent metadata extraction** - Automatic extraction of `user_id` and `test_run_id`
- **Structured logging** - Pre-configured logger with execution context
- **Error handling** - Automatic success/failure logging

## Framework Wrappers

### Go Framework (`src/go/pkg/framework/wrapper.go`)

**Pattern:**
```go
// Entry point - unchanged signature
func EnrichActivity(ctx context.Context, e event.Event) error {
    svc, err := initService(ctx)
    if err != nil {
        return fmt.Errorf("service init failed: %v", err)
    }
    return framework.WrapCloudEvent("enricher", svc, enrichHandler)(ctx, e)
}

// Handler - receives FrameworkContext
func enrichHandler(ctx context.Context, e event.Event, fwCtx *framework.FrameworkContext) (interface{}, error) {
    // Use fwCtx.Logger (has execution_id, user_id)
    fwCtx.Logger.Info("Starting enrichment")

    // Use fwCtx.Service (DB, Pub, Store, Config)
    userData, err := fwCtx.Service.DB.GetUser(ctx, userId)

    // Return outputs for logging
    return enrichedEvent, nil
}
```

**FrameworkContext:**
```go
type FrameworkContext struct {
    Service     *bootstrap.Service  // DB, Pub, Store, Secrets, Config
    Logger      *slog.Logger        // Pre-configured with execution_id, user_id
    ExecutionID string              // Unique execution identifier
}
```

**Automatic Metadata Extraction:**
- `user_id`: Extracted from Pub/Sub message payload (`user_id` or `userId` field)
- `test_run_id`: Extracted from event extensions
- `trigger_type`: Automatically detected (`http` or `pubsub`)

### TypeScript Framework (`src/typescript/shared/src/framework/index.ts`)

**Pattern:**
```typescript
const handler = async (req: any, res: any, ctx: FrameworkContext) => {
  const { db, logger } = ctx;

  // Use ctx.logger (has executionId, user_id)
  logger.info("Processing request");

  // Business logic
  const result = await processData();

  // Return outputs for logging
  return { status: 'success', result };
};

export const myFunction = createCloudFunction(handler);
```

**FrameworkContext:**
```typescript
interface FrameworkContext {
  db: admin.firestore.Firestore;
  logger: winston.Logger;  // Pre-configured with executionId, user_id
  executionId: string;
}
```

**Automatic Metadata Extraction:**
- **HTTP Requests:**
  - `user_id`: From request body (`user_id` or `userId`)
  - `test_run_id`: From header (`X-Test-Run-Id`)
- **Pub/Sub Messages:**
  - `user_id`: From base64-decoded message data
  - `test_run_id`: From message attributes
  - Automatically detects Pub/Sub vs HTTP

## Execution Records

All function invocations create execution records in Firestore (`executions` collection):

```typescript
{
  execution_id: string;      // Unique identifier
  service: string;           // Function name (e.g., "enricher")
  user_id?: string;          // Extracted from event
  test_run_id?: string;      // For test isolation
  trigger_type: string;      // "http" or "pubsub"
  status: string;            // "STARTED" → "SUCCESS" or "FAILED"
  inputs?: any;              // Function inputs (optional)
  outputs?: any;             // Function outputs (on success)
  error?: string;            // Error message (on failure)
  start_time: Timestamp;
  end_time?: Timestamp;
}
```

## Protobuf Types

All data structures use Protocol Buffers for type safety and consistency:

**Activity Data (`src/proto/activity.proto`):**
```protobuf
message ActivityPayload {
  ActivitySource source = 1;
  string user_id = 2;
  string timestamp = 3;
  string original_payload_json = 4;
  map<string, string> metadata = 5;
}

message EnrichedActivityEvent {
  string user_id = 1;
  string activity_id = 2;
  string gcs_uri = 3;
  string description = 4;
  string metadata_json = 5;
}
```

**Execution Records (`src/proto/execution.proto`):**
```protobuf
message ExecutionRecord {
  string execution_id = 1;
  string service = 2;
  string user_id = 3;
  string test_run_id = 4;
  string trigger_type = 5;
  ExecutionStatus status = 6;
  google.protobuf.Timestamp start_time = 7;
  google.protobuf.Timestamp end_time = 8;
}

enum ExecutionStatus {
  STATUS_UNSPECIFIED = 0;
  STARTED = 1;
  SUCCESS = 2;
  FAILED = 3;
}
```

## Function Implementations

### Go Functions

All Go functions follow this pattern:

1. **Entry Point** - Unchanged signature for Cloud Functions compatibility
2. **Service Initialization** - Singleton pattern with `sync.Once`
3. **Framework Wrapper** - Wraps handler with automatic logging
4. **Handler** - Pure business logic with FrameworkContext

**Example (Enricher):**
```go
// 1. Entry point
func EnrichActivity(ctx context.Context, e event.Event) error {
    svc, err := initService(ctx)
    if err != nil {
        return fmt.Errorf("service init failed: %v", err)
    }
    return framework.WrapCloudEvent("enricher", svc, enrichHandler)(ctx, e)
}

// 2. Handler
func enrichHandler(ctx context.Context, e event.Event, fwCtx *framework.FrameworkContext) (interface{}, error) {
    // Parse event
    var msg types.PubSubMessage
    if err := e.DataAs(&msg); err != nil {
        return nil, fmt.Errorf("event.DataAs: %v", err)
    }

    var rawEvent pb.ActivityPayload
    if err := json.Unmarshal(msg.Message.Data, &rawEvent); err != nil {
        return nil, fmt.Errorf("json unmarshal: %v", err)
    }

    // Use framework logger
    fwCtx.Logger.Info("Starting enrichment", "timestamp", rawEvent.Timestamp)

    // Business logic using fwCtx.Service
    fitBytes, err := fit.GenerateFitFile(...)
    if err != nil {
        fwCtx.Logger.Error("FIT generation failed", "error", err)
        return nil, err
    }

    // Return outputs
    return enrichedEvent, nil
}
```

### TypeScript Functions

All TypeScript functions follow this pattern:

1. **Handler Function** - Receives FrameworkContext
2. **Export** - Wrapped with `createCloudFunction()`

**Example (Hevy Handler):**
```typescript
const handler = async (req: any, res: any, ctx: FrameworkContext) => {
  const { db, logger } = ctx;

  // Signature verification
  if (signingSecret && !verifySignature(...)) {
    logger.warn('Invalid signature attempt');
    res.status(401).send('Unauthorized');
    throw new Error('Invalid X-Hevy-Signature');
  }

  // Business logic
  const messagePayload: ActivityPayload = {
    source: ActivitySource.SOURCE_HEVY,
    userId: userId,
    timestamp: timestamp,
    originalPayloadJson: JSON.stringify(workoutData),
    metadata: {}
  };

  const messageId = await pubsub.topic(TOPIC_NAME).publishMessage({
    json: messagePayload,
  });

  logger.info("Processed workout", { messageId, userId });
  res.status(200).send('Processed');

  return { pubsubMessageId: messageId };
};

export const hevyHandler = createCloudFunction(handler);
```

## Testing with Test Run IDs

Integration tests use `test_run_id` for precise tracking and cleanup:

**Test Pattern:**
```typescript
describe('My Test Suite', () => {
  const testRunIds: string[] = [];

  it('should process activity', async () => {
    const testRunId = randomUUID();
    testRunIds.push(testRunId);

    // HTTP request
    await axios.post(endpoint, payload, {
      headers: { 'X-Test-Run-Id': testRunId }
    });

    // Pub/Sub message
    await publishRawActivity(payload, testRunId);

    // Verify by test run ID
    await waitForExecutionActivity({ testRunId, minExecutions: 1 });
  });

  afterAll(async () => {
    // Clean up all executions from all tests
    await cleanupExecutions(testRunIds);
  });
});
```

## Benefits

1. **Consistency** - Same pattern across all functions (Go and TypeScript)
2. **Observability** - All executions logged with consistent metadata
3. **Testability** - Test run IDs enable precise verification and cleanup
4. **Maintainability** - No manual logging code in business logic
5. **Type Safety** - Protobuf types ensure consistency

## Migration Guide

To add a new function:

**Go:**
```go
// 1. Create handler
func myHandler(ctx context.Context, e event.Event, fwCtx *framework.FrameworkContext) (interface{}, error) {
    // Business logic
    return outputs, nil
}

// 2. Wrap in entry point
func MyFunction(ctx context.Context, e event.Event) error {
    svc, _ := initService(ctx)
    return framework.WrapCloudEvent("my-function", svc, myHandler)(ctx, e)
}
```

**TypeScript:**
```typescript
// 1. Create handler
const handler = async (req: any, res: any, ctx: FrameworkContext) => {
  // Business logic
  return outputs;
};

// 2. Export wrapped
export const myFunction = createCloudFunction(handler);
```

No manual execution logging required!
