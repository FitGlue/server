# Services & Stores Architecture

FitGlue separates business logic from data access using a strict **Service vs Store** pattern.

## Philosophy

We strictly separate **Domain Logic** (Services) from **Infrastructure/Persistence** (Stores). This allows us to:
1. **Test Isolated Logic**: Services can be tested with mock Stores without spinning up a real database.
2. **Swap Backends**: If we move from Firestore to SQL, only the Stores change; Services remain untouched.
3. **Enforce Schema**: Stores act as the gatekeepers of database integrity.

## Available Stores

### TypeScript Stores (`shared/src/stores/`)

| Store | Collection | Purpose |
|-------|------------|---------|
| `UserStore` | `users` | User profiles and integrations |
| `PipelineStore` | `users/{userId}/pipelines` | Pipeline configurations |
| `ActivityStore` | `users/{userId}/activities` | Synchronized activities |
| `PendingInputStore` | `users/{userId}/pending_inputs` | Inputs awaiting user response |
| `ExecutionStore` | `users/{userId}/executions` | Function execution logs |
| `ShowcaseStore` | `showcased_activities` | Public activity shares |

### Go Stores (`pkg/store/`)

| Store | Collection | Purpose |
|-------|------------|---------|
| `UserStore` | `users` | User profiles and tokens |
| `PipelineStore` | `users/{userId}/pipelines` | Pipeline configurations |
| `PipelineRunStore` | `users/{userId}/pipeline_runs` | Pipeline execution lifecycle |
| `PendingInputStore` | `users/{userId}/pending_inputs` | Pending user inputs |
| `ActivityStore` | `users/{userId}/activities` | Synchronized activities |

## User Sub-Collections Pattern

All user data is stored in sub-collections for scalability:

```
users/{userId}/
├── pipelines/{pipelineId}        # Pipeline configurations
├── pipeline_runs/{pipelineRunId} # Pipeline execution tracking
├── pending_inputs/{inputId}      # Paused inputs awaiting resolution
├── activities/{activityId}       # Synchronized activities
└── executions/{executionId}      # Function execution logs
```

**Benefits:**
- Efficient queries per user (no cross-user scans)
- Firestore security rules per user
- Scales to millions of users

## Store Responsibilities

### Rules
1. **Strict Typing**: Update methods must accept `Partial<RecordType>` or specific, typed arguments. **NEVER use `any`**.
2. **No Business Logic**: A store should never check permissions or validate business rules.
3. **Encapsulation**: Hide database specifics. A Service should call `store.addPipeline(...)`, not pass Firestore `FieldValue` objects.

### Example: Good Store Method

```typescript
// ✅ Good: Typed, encapsulates complexity
async setIntegration(userId: string, provider: 'strava', data: StravaIntegration): Promise<void> {
  await this.collection().doc(userId).update({
    [`integrations.${provider}`]: data
  });
}
```

### Example: Bad Store Method

```typescript
// ❌ Bad: Accepts 'any', potentially dangerous
async update(userId: string, data: any): Promise<void> {
  await this.collection().doc(userId).update(data);
}
```

## PipelineRunStore

Manages pipeline execution lifecycle tracking:

```go
// pkg/store/pipeline_run_store.go
type PipelineRunStore struct {
    db *firestore.Client
}

func (s *PipelineRunStore) Create(ctx context.Context, userId string, run *pb.PipelineRun) error
func (s *PipelineRunStore) Get(ctx context.Context, userId, runId string) (*pb.PipelineRun, error)
func (s *PipelineRunStore) UpdateStatus(ctx context.Context, userId, runId string, status pb.PipelineRunStatus) error
func (s *PipelineRunStore) AddBoosterExecution(ctx context.Context, userId, runId string, exec *pb.BoosterExecution) error
func (s *PipelineRunStore) UpdateDestinationStatus(ctx context.Context, userId, runId string, dest pb.Destination, status pb.DestinationStatus) error
```

**Usage in Orchestrator:**
```go
// Create run at pipeline start
run := &pb.PipelineRun{
    Id:         pipelineExecutionId,
    PipelineId: payload.PipelineId,
    Status:     pb.PipelineRunStatus_PIPELINE_RUN_STATUS_RUNNING,
}
o.pipelineRunStore.Create(ctx, userId, run)

// Update after each enricher
o.pipelineRunStore.AddBoosterExecution(ctx, userId, runId, &pb.BoosterExecution{
    ProviderName: "weather",
    Status:       "SUCCESS",
    DurationMs:   int32(elapsed.Milliseconds()),
})

// Update destination status
o.pipelineRunStore.UpdateDestinationStatus(ctx, userId, runId, pb.Destination_DESTINATION_STRAVA, pb.DestinationStatus_DESTINATION_STATUS_UPLOADED)
```

## PendingInputStore

Manages inputs awaiting user response:

```go
// pkg/store/pending_input_store.go
func (s *PendingInputStore) Create(ctx context.Context, userId string, input *pb.PendingInput) error
func (s *PendingInputStore) Get(ctx context.Context, userId, inputId string) (*pb.PendingInput, error)
func (s *PendingInputStore) Resolve(ctx context.Context, userId, inputId string, resolvedBy string, response map[string]string) error
func (s *PendingInputStore) ListPending(ctx context.Context, userId string) ([]*pb.PendingInput, error)
```

**Key Fields:**
- `linkedActivityId`: Activity created during initial enrichment
- `originalPayloadUri`: GCS URI for payload retrieval on resume
- `pipelineId`: Pipeline that created this input

## Services (Domain Layer)

Services implement the business capabilities of the application.

### Responsibilities
- Orchestrating workflows (e.g., "Ingest Webhook")
- Validating inputs and business rules
- Calling external APIs
- Calling Stores to persist state

### Rules
1. **No Direct DB Access**: A Service must **never** import `firebase-admin` or call `db.collection()`.
2. **Use Store Methods**: Services should use the specific methods exposed by Stores.

### Example: Good Service Method

```typescript
// ✅ Good: Delegates persistence to Store
async connectStrava(userId: string, token: string): Promise<void> {
  // Business Logic
  if (!this.isValid(token)) throw new Error("Invalid");
  
  // Persistence
  await this.userStore.setIntegration(userId, 'strava', { ... });
}
```

### Example: Bad Service Method

```typescript
// ❌ Bad: Leaks DB details, constructs untyped object
async connectStrava(userId: string, token: string): Promise<void> {
  const updatePayload = { 'integrations.strava': { ... } };
  await this.userStore.update(userId, updatePayload);
}
```

## Firestore Converters

Converters translate between TypeScript objects (camelCase) and Firestore documents (snake_case).

### Critical Pattern: Omit Undefined Values

**Problem:** Firestore rejects `undefined` values by default.

**Solution:**
```typescript
// ✅ Good: Only writes defined fields
toFirestore(model: ExecutionRecord): FirebaseFirestore.DocumentData {
  const data: FirebaseFirestore.DocumentData = {};
  if (model.executionId !== undefined) data.execution_id = model.executionId;
  if (model.service !== undefined) data.service = model.service;
  return data;
}
```

### Store Create vs Update

**`create()`**: Use `.set()` without `{merge: true}`
- Accepts full record type
- TypeScript enforces all required fields
- Fails if document exists

**`update()`**: Use `.update()` with `Partial<>`
- Accepts `Partial<RecordType>`
- Only modifies specified fields
- Fails if document doesn't exist

## AuthorizationService

Centralized access control:

```typescript
// In any handler that accesses user resources
const pipeline = await ctx.stores.pipeline.get(pipelineId);
ctx.services.authorization.requireAccess(ctx.auth.userId, pipeline.userId);
```

### Key Methods

| Method | Purpose |
|--------|---------|
| `requireAccess(userId, resourceOwnerId)` | Verifies user can access a resource |
| `requireAdmin()` | Verifies user has admin privileges |

## Summary Rule of Thumb

* If it involves `FieldValue`, `collection()`, or `where()`, it belongs in a **Store**.
* If it involves `if (user.enabled)`, `throw new Error()`, or `api.fetch()`, it belongs in a **Service**.

## Related Documentation

- [Security](security.md) - Authorization and access control
- [Architecture Overview](overview.md) - System components
- [Execution Logging](execution-logging.md) - Execution tracking
