# Services & Stores Architecture

FitGlue separates business logic from data access using a strict **Service vs Store** pattern. This guide definitions, responsibilities, and coding standards for this architecture.

## Philosophy

We strictly separate **Domain Logic** (Services) from **Infrastructure/Persistence** (Stores). This allows us to:
1.  **Test Isolated Logic**: Services can be tested with mock Stores without spinning up a real database.
2.  **Swap Backends**: If we move from Firestore to SQL, only the Stores change; Services remain untouched.
3.  **Enforce Schema**: Stores act as the gatekeepers of database integrity.

## Stores (Data Access Layer)

Stores are wrappers around the database driver (e.g., `admin.firestore()`). They provide typed access to collections and documents.

### Responsibilities
-   Generic CRUD operations (`get`, `create`, `delete`).
-   Complex update logic (handling dot-notation keys for nested objects using `FieldValue`).
-   Enforcing correct data types for writes (using Protebuf Generated Types like `UserRecord`).

### Rules
1.  **Strict Typing**: Update methods must accept `Partial<RecordType>` or specific, typed arguments. **NEVER use `any`**.
2.  **No Business Logic**: A store should never check permissions or validate business rules (e.g., "Is this token expired?").
3.  **Encapsulation**: Hide database specifics. A Service should call `store.addPipeline(...)`, not pass a Firestore `FieldValue` object.

### Example: Good Store Method

```typescript
// ✅ Good: Typed, encapsulates complexity
async setIntegration(userId: string, provider: 'strava', data: StravaIntegration): Promise<void> {
  // Encapsulates the dot-notation syntax required by Firestore
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

## Services (Domain Layer)

Services implement the business capabilities of the application.

### Responsibilities
-   Orchestrating workflows (e.g., "Ingest Webhook").
-   Validating inputs and business rules (e.g., "Check expiry").
-   Calling external APIs (via other specialized clients).
-   Calling Stores to persist state.

### Rules
1.  **No Direct DB Access**: A Service must **never** import `firebase-admin` or call `db.collection()`.
2.  **Parity**: Services should use the specific methods exposed by Stores.

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
  const updatePayload = { 'integrations.strava': { ... } }; // Implementation leak!
  await this.userStore.update(userId, updatePayload); // Unsafe!
}
```

## Summary Rule of Thumb

*   If it involves `FieldValue`, `collection()`, or `where()`, it belongs in a **Store**.
*   If it involves `if (user.enabled)`, `throw new Error()`, or `api.fetch()`, it belongs in a **Service**.

## Firestore Converters

Converters translate between TypeScript objects (camelCase) and Firestore documents (snake_case). They must handle partial data gracefully.

### Critical Pattern: Omit Undefined Values

**Problem:** Firestore rejects `undefined` values by default. When creating with partial data, converters must omit undefined fields.

**Solution:**
```typescript
// ✅ Good: Only writes defined fields
toFirestore(model: ExecutionRecord): FirebaseFirestore.DocumentData {
  const data: FirebaseFirestore.DocumentData = {};
  if (model.executionId !== undefined) data.execution_id = model.executionId;
  if (model.service !== undefined) data.service = model.service;
  // ... only include fields that exist
  return data;
}

// ❌ Bad: Writes undefined, causes errors
toFirestore(model: ExecutionRecord): FirebaseFirestore.DocumentData {
  return {
    execution_id: model.executionId,  // undefined if not set!
    service: model.service,
    // ...
  };
}
```

### Why Not Use `ignoreUndefinedProperties`?

- It's a global Firestore setting that masks bugs
- Firestore's default (rejecting undefined) catches typos and missing data
- Proper converters should handle partial data without global settings

### Store Create vs Update

**`create()`**: Use `.set()` **without** `{merge: true}` and **without** `Partial<>`
- Accepts full record type (e.g., `ExecutionRecord`, not `Partial<ExecutionRecord>`)
- TypeScript enforces all required fields are provided
- Optional fields can be omitted (they're marked with `?` in the type)
- Creates new document
- Fails if document exists (prevents accidental overwrites)

**`update()`**: Use `.update()` **with** `Partial<>`
- Accepts `Partial<RecordType>` to allow updating any subset of fields
- Updates existing document
- Only modifies specified fields
- Fails if document doesn't exist

### Defining Required vs Optional Fields

Use Proto3's `optional` keyword in `.proto` files to mark truly optional fields:

```protobuf
message ExecutionRecord {
  string execution_id = 1;  // Required (no optional keyword)
  string service = 2;        // Required

  optional string user_id = 5;      // Optional (has optional keyword)
  optional string error_message = 10; // Optional
}
```

**Makefile configuration:**
- Add `--experimental_allow_proto3_optional` to protoc for Go
- Use `useOptionals=messages` for ts-proto (only marks `optional` fields as optional)

This generates TypeScript types where:
- Required fields have no `?`: `executionId: string`
- Optional fields have `?`: `userId?: string | undefined`
