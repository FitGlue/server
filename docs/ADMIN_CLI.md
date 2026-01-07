# FitGlue Admin CLI

The Admin CLI is a tool for managing users and integrations in the FitGlue system. It interacts directly with the Firestore database and handles tasks like generating API keys and configuring third-party integrations (e.g., Hevy).

## Quick Start

We provide a wrapper script in the root directory for easy execution:

```bash
./fitglue-admin <command> [options]
```

## Commands

### `users:create [userId]`

Creates a new user in the system. If `userId` is omitted, a random UUID will be generated. This command is interactive and will prompt you for necessary details.

**Usage:**
```bash
./fitglue-admin users:create
# OR
./fitglue-admin users:create my-specific-id
```

**Prompts:**
1.  **Ingress API Key**: Do you want to generate an API Key for this user? (Default: Yes)
    *   **Label**: A descriptive name for the key (e.g., "Mobile App").
    *   **Scopes**: Select `read:activity` (required for ingesting data).
**Prompts:**
1.  **Ingress API Key**: Do you want to generate an API Key for this user? (Default: Yes)
    *   **Label**: A descriptive name for the key (e.g., "Mobile App").
    *   **Scopes**: Select `read:activity` (required for ingesting data).

### `users:configure-integration <provider> <userId>`

Configures an integration for a specific user. This command dynamically prompts for fields required by the provider (e.g., API keys, client secrets).

**Usage:**
```bash
./fitglue-admin users:configure-integration hevy my-user-id
```

**Prompts:**
Prompts vary by provider. For Hevy, it will ask for the Hevy API Key.

### `users:list`

Lists all users currently in the Firestore database, along with their creation date and enabled integrations. Useful for auditing and cleanup.

**Usage:**
```bash
./fitglue-admin users:list
```

### `users:clean`

**DANGER:** Permanently deletes **ALL** users from the system. Requires double confirmation.

**Usage:**
### `users:clean`

<span style="color:red">**DANGER ZONE**</span>

Deletes **ALL** users from the Firestore database.
- Requires explicit `yes` confirmation.
- Requires typing `DELETE ALL` to proceed.

```bash
./fitglue-admin users:clean
```

### `users:create-auth <userId>`

Creates a Firebase Auth user for an existing Firestore user ID. This allows the user to authenticate via email/password.

**Usage:**
```bash
./fitglue-admin users:create-auth my-user-id
```

**Prompts:**
1. **Email**: User's email address
2. **Password**: Password (minimum 6 characters)

### `users:get <userId>`

Get detailed information about a specific user, including integrations and pipelines.

**Usage:**
```bash
./fitglue-admin users:get my-user-id
```

### `users:remove-pipeline <userId>`

Remove a processing pipeline from a user.

**Usage:**
```bash
./fitglue-admin users:remove-pipeline my-user-id
```

**Prompts:**
1. **Pipeline Selection**: Choose which pipeline to remove from the list
2. **Confirmation**: Confirm deletion

### `users:replace-pipeline <userId>`

Replace or reconfigure an existing pipeline for a user.

**Usage:**
```bash
./fitglue-admin users:replace-pipeline my-user-id
```

**Prompts:**
1. **Pipeline Selection**: Choose which pipeline to replace
2. **New Configuration**: Configure source, enrichers, and destinations (same as `users:add-pipeline`)

## Activity Management Commands

### `activities:list-processed <userId>`

List all processed activities for a user. Useful for debugging deduplication or finding activities to re-ingest.

**Usage:**
```bash
./fitglue-admin activities:list-processed my-user-id
```

**Output:**
```
Found 3 activities:
--------------------------------------------------
[SOURCE_HEVY] workout-123 (Processed: 2026-01-03T12:00:00Z)
[SOURCE_FITBIT] activity-456 (Processed: 2026-01-03T11:30:00Z)
[SOURCE_HEVY] workout-789 (Processed: 2026-01-03T10:15:00Z)
--------------------------------------------------
```

### `activities:delete-processed <userId> <source> <activityId>`

Delete a processed activity record to allow re-ingestion. This is useful when you want to re-process an activity that was already handled.

**Usage:**
```bash
# Delete a Hevy workout
./fitglue-admin activities:delete-processed my-user-id SOURCE_HEVY workout-123

# Delete a Fitbit activity
./fitglue-admin activities:delete-processed my-user-id SOURCE_FITBIT activity-456
```

## Synchronized Activity Commands

### `synchronized:list <userId>`

List all synchronized activities for a user. These are activities that have been successfully processed and uploaded to destinations.

**Options:**
- `-l, --limit <number>`: Limit results (default: 20)

**Usage:**
```bash
./fitglue-admin synchronized:list my-user-id

# With limit
./fitglue-admin synchronized:list my-user-id --limit 50
```

**Output:**
```
Found 3 synchronized activities:
--------------------------------------------------
[abc-123] Morning Run
  Type: 39, Source: SOURCE_HEVY
  Synced: 2026-01-07T10:30:00Z
  Destinations: strava, mock
--------------------------------------------------
```

### `synchronized:get <userId> <activityId>`

Get detailed information about a specific synchronized activity, including the pipeline execution trace if available.

**Options:**
- `-v, --verbose`: Show full execution trace details including input/output payloads

**Usage:**
```bash
./fitglue-admin synchronized:get my-user-id abc-123

# With verbose output
./fitglue-admin synchronized:get my-user-id abc-123 --verbose
```

**Output:**
```
Synchronized Activity Details:
--------------------------------------------------
Activity ID: abc-123
Title: Morning Run
Description: Great run today!
Type: 39
Source: SOURCE_HEVY
Start Time: 2026-01-07T09:00:00Z
Synced At: 2026-01-07T10:30:00Z
Pipeline ID: pipe_123456
Pipeline Execution ID: exec_789
Destinations:
  strava: 12345678
  mock: mock-abc-123
--------------------------------------------------

Pipeline Execution Trace:
--------------------------------------------------
[hevy-webhook-handler] STATUS_2 (125ms)
  Execution ID: exec_789
  Time: 2026-01-07T10:29:55Z
--------------------------------------------------
[enricher] STATUS_2 (350ms)
  Execution ID: exec_790
  Time: 2026-01-07T10:29:56Z
--------------------------------------------------
```

## Execution Inspection Commands

### `executions:list`

List recent function executions for debugging and auditing.

**Options:**
- `-l, --limit <number>`: Limit results (default: 20).
- `-s, --service <name>`: Filter by service name (e.g., `hevy-webhook-handler`, `enricher`).
- `-st, --status <status>`: Filter by status (`STATUS_STARTED`, `STATUS_SUCCESS`, `STATUS_FAILED`).
- `-u, --user <userId>`: Filter by user ID.

```bash
# List last 20 executions
./fitglue-admin executions:list

# Filter by service
./fitglue-admin executions:list --service hevy-webhook-handler

# Find failed executions
./fitglue-admin executions:list --status STATUS_FAILED
```

### `executions:list-watch`

Watch recent executions in real-time. This command clears the screen and updates the list as new executions occur.

**Options:**
- `-l, --limit <number>`: Limit results (default: 20).
- `-s, --service <name>`: Filter by service name.
- `-st, --status <status>`: Filter by status.
- `-u, --user <userId>`: Filter by user ID.

```bash
# Watch executions for a specific user
./fitglue-admin executions:list-watch --user user-123
```

### `executions:latest`

Get full details of the **single most recent** execution. Supports filtering.

**Options:**
- `-s, --service <name>`: Filter by service name.
- `--status <status>`: Filter by status (e.g. `FAILED`).

```bash
# Show details of the latest execution (whatever it is)
./fitglue-admin executions:latest

# Show details of the latest FAILURE
./fitglue-admin executions:latest --status FAILED
```

### `executions:latest-watch`

Real-time monitor of the **latest single execution**. It auto-updates and redraws the screen whenever a new execution matching the criteria appears. Ideal for debugging a specific flow or waiting for a failure.

**Options:**
- `-s, --service <name>`: Filter by service name.
- `--status <status>`: Filter by status.

```bash
# Watch for ANY new execution and show full details
./fitglue-admin executions:latest-watch

# Watch for FAILURES only (Great for passive monitoring)
./fitglue-admin executions:latest-watch --status FAILED
```

### `executions:get <executionId>`

Get full details for a specific execution, including input/output payloads (if logged).

```bash
./fitglue-admin executions:get <executionId>
```

**Options:**
- `-v, --verbose`: Show full execution details without truncating large arrays or objects (e.g. FIT file data streams).

### `executions:get-by-pipeline <pipelineExecutionId>`

Get all executions associated with a specific pipeline run.

```bash
./fitglue-admin executions:get-by-pipeline <pipelineExecutionId>
```

### `executions:create <executionId>`

**Testing Command:** Create a test execution record with minimal data (useful for debugging execution logging).

**Options:**
- `-s, --service <service>`: Service name (default: `test-service`)
- `-t, --trigger <trigger>`: Trigger type (default: `http`)
- `-u, --user <userId>`: User ID (optional)

```bash
# Create minimal execution (like logExecutionPending)
./fitglue-admin executions:create test-exec-001 --service my-service --trigger pubsub

# With user ID
./fitglue-admin executions:create test-exec-002 --user user-123
```

### `executions:update <executionId>`

**Testing Command:** Update an existing execution record (useful for testing partial updates).

**Options:**
- `--status <status>`: Status code (0-4, default: 2 = SUCCESS)
- `--error <message>`: Error message (sets status to FAILED)
- `--inputs <json>`: Inputs JSON string
- `--outputs <json>`: Outputs JSON string

```bash
# Mark as successful with output
./fitglue-admin executions:update test-exec-001 --status 2 --outputs '{"result":"success"}'

# Mark as failed
./fitglue-admin executions:update test-exec-001 --error "Something went wrong"
```

### `executions:clean`

<span style="color:red">**DANGER ZONE**</span>

Deletes **ALL** execution logs from the Firestore database.

**Options:**
- `-f, --force`: Skip confirmation prompts (DANGEROUS).

**Usage:**
```bash
./fitglue-admin executions:clean
```

## Execution Replay Commands

### `replay:pubsub <execution-id>`

Replay a Pub/Sub-triggered execution by seeking the subscription to just before the original execution time. Useful for re-processing failed enricher, router, or uploader executions.

**Requirements:**
- `gcloud` CLI authenticated
- Message must be within retention period (1 hour)

**Options:**
- `--yes`: Skip confirmation prompt

**Usage:**
```bash
# Find failed execution
./fitglue-admin executions:list --service enricher --status STATUS_FAILED --limit 1

# Replay with confirmation
./fitglue-admin replay:pubsub enricher-1234567890

# Skip confirmation
./fitglue-admin replay:pubsub enricher-1234567890 --yes
```

**Output:**
```
📋 Replay Details:
   Execution ID: enricher-1234567890
   Service: enricher
   Subscription: eventarc-us-central1-enricher-885833-sub-349
   Original time: 2026-01-03T12:00:00Z
   Seek time: 2026-01-03T11:59:59Z
   Status: STATUS_FAILED

🔄 Proceed with replay? (y/n): y

Executing: gcloud pubsub subscriptions seek ...
✅ Replay initiated. Check logs for new execution.
```

### `replay:webhook <execution-id>`

Replay an HTTP webhook execution by re-POSTing the original payload to the webhook endpoint. Useful for re-processing failed Hevy or Fitbit webhook handlers.

**Options:**
- `--env <env>`: Target environment - `dev`, `test`, or `prod` (default: `dev`)
- `--yes`: Skip confirmation prompt

**Usage:**
```bash
# Find failed webhook
./fitglue-admin executions:list --service hevy-webhook-handler --status STATUS_FAILED --limit 1

# Replay to dev environment
./fitglue-admin replay:webhook hevy-webhook-handler-1234567890 --env dev

# Replay to prod
./fitglue-admin replay:webhook hevy-webhook-handler-1234567890 --env prod --yes
```

**Output:**
```
📋 Replay Details:
   Execution ID: hevy-webhook-handler-1234567890
   Service: hevy-webhook-handler
   Environment: dev
   URL: https://us-central1-fitglue-dev.cloudfunctions.net/hevy-webhook-handler
   Status: STATUS_FAILED
   Payload preview: {"id":"abc123","type":"workout_created"}...

🔄 Proceed with replay? (y/n): y

Sending request...
✅ Success: 200 OK
```

### `users:connect <userId> <provider>`

Generates an OAuth authorization URL for a specific provider (Strava or Fitbit). It now prompts for the **Client ID**, which you can find in your provider developer portal or Google Secret Manager.

**Usage:**
```bash
./fitglue-admin users:connect my-user-id strava
```

**Prompts:**
1.  **Client ID**: Enter the Client ID for the chosen provider.

**Output:**
Prints a URL that you can send to the user (or click yourself) to authorize the application. Upon success, the callback handler will save the tokens to the user's Firestore record.

### `fitbit:subscribe [userId]`

Creates a Fitbit "API Subscription" for the user. this tells Fitbit's servers to send real-time notifications to our Webhook Handler whenever this user syncs new activities. This command **must** be run after the user has connected their Fitbit account.

**Usage:**
```bash
./fitglue-admin fitbit:subscribe my-user-id
```

**Output:**
```json
{
  "apiSubscriptions": [
    {
      "collectionType": "activities",
      "ownerId": "USER_ID",
      "ownerType": "user",
      "subscriberId": "fitglue-activities",
      "subscriptionId": "fitglue-activities-USER_ID"
    }
  ]
}
```
If the user is already subscribed, it will print a success message (treating 409 Conflict as success).

### `users:delete <userId>`

Permanently deletes a user and their associated root document. Note that subcollections may need manual cleanup in a production environment.

**Usage:**
```bash
./fitglue-admin users:delete my-user-id
```



### `users:add-pipeline <userId>`

Adds a data processing pipeline to a user. This command allows you to define complex routing and enrichment flows, such as "Hevy -> Fitbit HR Enrichment -> Strava".

**Usage:**
```bash
./fitglue-admin users:add-pipeline my-user-id
```

**Prompts:**
1.  **Source**: Select the data source triggering this pipeline (e.g., `SOURCE_HEVY`).
2.  **Enrichers**:
    *   Add enrichers in sequence (e.g., first `fitbit-heart-rate`, then `ai-description`).
    *   **Activity Filter**: Prompts to configure conditional logic. Uses **checkboxes** to select activity types for exclusion/inclusion.
    *   **Parkrun**: Prompts for *Enable Titling* and *Tags* (default `Parkrun,Race`).
    *   You can optionally provide a JSON string for specific enricher inputs.
3.  **Destinations**: Select where the final data should be sent (e.g., `strava`).

## Infrastructure Commands

### `terraform:unlock <environment>`

Attempt to find and clear a Terraform state lock for a specific environment (`dev`, `test`, `prod`).

This command is useful when a Terraform run is interrupted and leaves the state locked. It re-initializes Terraform for the target environment and uses a lightweight `state list` check to detect locks without requiring deployment artifacts (ZIPs).

**Usage:**
```bash
./fitglue-admin terraform:unlock dev
```

**Prompt:**
If a lock is detected, it will display the Lock ID and ask for confirmation before force-unlocking.

## GCS Bucket Commands

### `buckets:list`
List all GCS buckets in the project.

```bash
./fitglue-admin buckets:list
```

### `buckets:get <bucketName>`
Get details about a specific GCS bucket.

```bash
./fitglue-admin buckets:get my-bucket-id
```

### `buckets:from-execution <executionId>`
Get details of the bucket associated with a specific execution. This command will look for a `fit_file_uri` in the execution record (or its inputs/outputs) and then inspect the corresponding bucket.

```bash
./fitglue-admin buckets:from-execution my-execution-id
```

## File Commands

### `files:download <bucketOrUri> [remotePath] [localPath]`
Download a file from GCS. You can usually `gs://` URI or specify bucket and path separately.

**Defaults:**
If `localPath` is not provided, the file is saved to `server/downloads/<filename>`.

**Usage:**
```bash
# Using URI (easies) -> downloads to server/downloads/file.fit
./fitglue-admin files:download gs://my-bucket/path/to/file.fit

# Specifying destination
./fitglue-admin files:download gs://my-bucket/file.fit ./my-custom-path/file.fit

# Using separate arguments
./fitglue-admin files:download my-bucket path/to/file.fit
```

### `files:download-execution <executionId> [localPath]`
Automatic download helper. Scans a specific execution for ANY `gs://` URIs (in inputs, outputs, etc.). If multiple are found, it prompts you to choose which one to download.

**Defaults:**
If `localPath` is not provided, the file is saved to `server/downloads/<filename>`.

```bash
./fitglue-admin files:download-execution my-execution-id
```

## Input Management Commands
These commands manage the `User Input` enricher workflow, where pipelines pause for manual input.

### `inputs:list <userId>`

List all pending inputs for a user (activities with `STATUS_WAITING`).

```bash
./fitglue-admin inputs:list my-user-id
```

### `inputs:get <activityId>`

Get details of a specific pending input requirement, including which fields are requested.

```bash
./fitglue-admin inputs:get my-activity-id
```

### `inputs:resolve <activityId>`

Provide the required input to resume the pipeline. You can do this interactively or via flags.

**Interactive:**
```bash
./fitglue-admin inputs:resolve my-activity-id
# Prompts for 'title', 'description' etc.
```

**Non-Interactive:**
```bash
./fitglue-admin inputs:resolve my-activity-id --data '{"title": "My Run", "description": "Good vibes"}'
```

## Development

The CLI source code is located in `src/typescript/admin-cli`.

To build it manually:
```bash
npm run build --workspace=admin-cli
```

To run it via npm (without the wrapper):
```bash
npm start --prefix src/typescript/admin-cli -- <command>
```
