# Architecture Overview

FitGlue is a serverless fitness data aggregation and routing platform built on Google Cloud Platform. It ingests workout data from multiple sources, enriches it through configurable pipelines, and routes it to connected services.

## System Components

```
                                    ┌─────────────────────────────────────┐
                                    │         Google Cloud Platform       │
                                    └─────────────────────────────────────┘
                                                     │
    ┌──────────────────────────────────────────────────────────────────────────────┐
    │                                                                              │
    │  ┌─────────────────┐                                                         │
    │  │   DATA SOURCES  │                                                         │
    │  │                 │                                                         │
    │  │  • Hevy         │                                                         │
    │  │  • Fitbit       │                                                         │
    │  │  • Strava       │                                                         │
    │  │  • Polar        │                                                         │
    │  │  • Oura         │                                                         │
    │  │  • Wahoo        │                                                         │
    │  │  • Apple Health │                                                         │
    │  │  • FIT Upload   │                                                         │
    │  └────────┬────────┘                                                         │
    │           │                                                                  │
    │           ▼                                                                  │
    │  ┌─────────────────┐      ┌──────────────┐      ┌──────────────────┐        │
    │  │ INGESTION LAYER │─────▶│   Pub/Sub    │─────▶│ PIPELINE SPLITTER│        │
    │  │   (Webhooks)    │      │(raw-activity)│      │       Go         │        │
    │  │   TypeScript    │      └──────────────┘      └────────┬─────────┘        │
    │  └─────────────────┘                                     │                  │
    │                                                          │ (per-pipeline)   │
    │                                                          ▼                  │
    │                         ┌──────────────────┐      ┌──────────────────┐      │
    │                         │     ENRICHER     │◀────▶│  External APIs   │      │
    │                         │ (Single Pipeline)│      │ Fitbit,Spotify,  │      │
    │                         │       Go         │      │ Weather,Parkrun  │      │
    │                         └────────┬─────────┘      └──────────────────┘      │
    │                                  │                                          │
    │                                  ▼                                          │
    │  ┌─────────────────┐      ┌──────────────┐      ┌─────────────────┐         │
    │  │  Cloud Storage  │◀─────│    ROUTER    │─────▶│     Pub/Sub     │         │
    │  │   (FIT Files)   │      │      Go      │      │ (Upload Jobs)   │         │
    │  └─────────────────┘      └──────────────┘      └────────┬────────┘         │
    │                                                          │                  │
    │                                                          ▼                  │
    │                                                 ┌──────────────────┐        │
    │                                                 │   DESTINATIONS   │        │
    │                                                 │  • Strava        │        │
    │                                                 │  • TrainingPeaks │        │
    │                                                 │  • Intervals.icu │        │
    │                                                 │  • Hevy          │        │
    │                                                 │  • Showcase      │        │
    │                                                 └──────────────────┘        │
    │                                                                             │
    │  ┌─────────────────┐      ┌──────────────┐      ┌─────────────────┐         │
    │  │    Firestore    │      │ Secret Mgr   │      │     Sentry      │         │
    │  │   (User Data)   │      │  (Secrets)   │      │  (Observability)│         │
    │  └─────────────────┘      └──────────────┘      └─────────────────┘         │
    │                                                                             │
    └─────────────────────────────────────────────────────────────────────────────┘
```

## Data Flow

### 1. Ingestion

Data enters the system through source-specific handlers:

1. **Webhooks** (Hevy, Strava, Polar, Wahoo): External services push data via authenticated webhooks
2. **Polling** (Fitbit, Oura): Notification triggers fetch from APIs
3. **Mobile Push** (Apple Health, Health Connect): Mobile apps push via authenticated API
4. **Direct Upload** (FIT Parser): Users upload FIT files directly

Each handler:
- Validates authentication (HMAC signature, OAuth, or API key)
- Checks for bounceback loops (prevents infinite webhook cycles)
- Transforms source-specific format → `StandardizedActivity` protobuf
- Publishes to `raw-activities` Pub/Sub topic

### 2. Pipeline Splitting (Per-Pipeline Isolation)

The **Pipeline Splitter** function fans out activities to matching pipelines:

1. Receives `RawActivityEvent` from `raw-activities` topic
2. Looks up user's pipelines from Firestore
3. For each matching pipeline:
   - Creates a targeted message with `pipelineId` set
   - Generates unique `pipelineExecutionId`
   - Publishes to `pipeline-activity` topic

This ensures each pipeline processes independently with its own execution trace.

### 3. Enrichment

The **Enricher** function processes exactly **one pipeline per invocation**:

1. Receives targeted `ActivityPayload` from `pipeline-activity` topic
2. Validates `pipelineId` is set (rejects untargeted messages)
3. Runs enrichers sequentially:
   - Each enricher can add/modify metadata, data streams, or artifacts
   - Enrichers can halt the pipeline (e.g., Logic Gate filter)
   - Enrichers can pause for user input (Pending Inputs)
4. Generates FIT file artifact → Cloud Storage
5. Creates `PipelineRun` document for lifecycle tracking
6. Publishes `EnrichedActivityEvent` to `enriched-activities` topic

**Available Enrichers:**
- **Data**: Fitbit HR, FIT File HR, Spotify Tracks, Weather, Running Dynamics
- **Stats**: Heart Rate Summary, Pace/Speed/Power/Cadence Summary, Elevation, Training Load, Personal Records
- **Visual**: Muscle Heatmap, Muscle Heatmap Image, Route Thumbnail
- **Detection**: Parkrun, Location Naming, Condition Matcher
- **Transform**: Type Mapper, Auto Increment, Logic Gate, Activity Filter
- **Input**: User Input, Hybrid Race Tagger
- **AI**: AI Companion, AI Banner

### 4. Routing

The **Router** function distributes enriched activities:

1. Receives `EnrichedActivityEvent` from Pub/Sub
2. Reads destination list from the event payload
3. Publishes destination-specific upload jobs to topics:
   - `topic-job-upload-strava`
   - `topic-job-upload-trainingpeaks`
   - `topic-job-upload-intervals`
   - etc.

### 5. Egress

Destination-specific uploaders handle delivery:

- **Strava Uploader**: Uploads FIT file, updates title/description
- **TrainingPeaks Uploader**: Creates/updates workouts
- **Intervals.icu Uploader**: Uploads activities
- **Hevy Uploader**: Syncs back to Hevy
- **Showcase Uploader**: Creates public activity pages

Each uploader:
- Updates `PipelineRun` with destination status (pending → success/failed)
- Supports retry via `useUpdateMethod` flag
- Records external IDs for deduplication

## Pending Inputs

Enrichers can pause pipeline execution to request user input:

1. Enricher throws `WaitForInputError` with required fields
2. Orchestrator creates `PendingInput` document with `linkedActivityId`
3. Original payload stored in GCS (`originalPayloadUri`)
4. User resolves via `inputs-handler` API
5. Handler republishes with `isResume=true` and `activityId` set
6. Enricher calls `EnrichResume()` to continue

Supports auto-population (e.g., Parkrun results polling).

## Data Model

### User Sub-Collections

All user data is stored in sub-collections for scalability:

```
users/{userId}/
  ├── pipelines/{pipelineId}          # Pipeline configurations
  ├── pipeline_runs/{pipelineRunId}   # Execution lifecycle tracking
  ├── pending_inputs/{inputId}        # Paused pipeline inputs
  ├── activities/{activityId}         # Synchronized activities
  └── executions/{executionId}        # Function execution logs
```

### PipelineRun Entity

Tracks complete pipeline execution lifecycle:

```protobuf
message PipelineRun {
  string id = 1;                      // pipelineExecutionId
  string pipeline_id = 2;
  string activity_id = 3;
  PipelineRunStatus status = 4;       // RUNNING → SUCCESS/FAILED
  repeated BoosterExecution boosters = 5;
  repeated DestinationOutcome destinations = 6;
  string original_payload_uri = 7;    // GCS URI for retry
}
```

## Plugin Architecture

FitGlue uses a type-safe, self-registering plugin system:

| Plugin Type | Language | Purpose | Location |
|-------------|----------|---------|----------|
| **Source** | TypeScript | Ingests data from external services | `src/typescript/{name}-handler/` |
| **Enricher** | Go | Transforms/enhances activities in pipeline | `src/go/functions/enricher/providers/` |
| **Destination** | Go | Uploads processed activities | `src/go/functions/{name}-uploader/` |
| **Integration** | TypeScript | OAuth connections (sources + destinations) | `src/typescript/shared/src/plugin/registry.ts` |

All plugins register with the central **Plugin Registry**, which exposes configuration schemas via `GET /api/registry`.

See [Plugin System](plugin-system.md) and [Registry Reference](../reference/registry.md) for details.

## Observability

### Sentry Integration

All functions integrate with Sentry for error tracking:
- Go: `SentryHandler` wraps slog to capture errors
- TypeScript: `createCloudFunction` wrapper captures exceptions
- Sensitive headers filtered (Authorization, Cookie)

### Execution Logging

All function executions are logged to Firestore for debugging:
- Inputs/outputs captured as JSON
- Lifecycle: PENDING → STARTED → SUCCESS/FAILURE
- Can be disabled per-handler for high-volume endpoints

## Key Technologies

| Component | Technology |
|-----------|------------|
| Compute | Cloud Functions Gen 2 (Cloud Run) |
| Messaging | Cloud Pub/Sub |
| Database | Cloud Firestore |
| Storage | Cloud Storage |
| Secrets | Secret Manager |
| Observability | Sentry |
| Infrastructure | Terraform |

## Environments

| Environment | Project ID | URL |
|-------------|------------|-----|
| Dev | `fitglue-server-dev` | `https://dev.fitglue.tech` |
| Test | `fitglue-server-test` | `https://test.fitglue.tech` |
| Prod | `fitglue-server-prod` | `https://fitglue.tech` |

## Related Documentation

- [Plugin System](plugin-system.md) - How plugins work
- [Services & Stores](services-and-stores.md) - Business logic architecture
- [Execution Logging](execution-logging.md) - Observability framework
- [Security](security.md) - Authorization and access control
- [Connectors](connectors.md) - Source integration patterns
