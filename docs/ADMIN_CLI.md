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

### `users:configure-hevy <userId>`

Configures the Hevy integration for a specific user by setting their Hevy API Key.

**Usage:**
```bash
./fitglue-admin users:configure-hevy my-user-id
```

**Prompts:**
1.  **Hevy API Key**: Enter the user's Hevy API Key.

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

### `executions:get`

Get full details for a specific execution, including input/output payloads (if logged).

```bash
./fitglue-admin executions:get <executionId>
```

### `executions:clean`

<span style="color:red">**DANGER ZONE**</span>

Deletes **ALL** execution logs from the Firestore database.
- Requires explicit `yes` confirmation.
- Requires typing `DELETE ALL` to proceed.

```bash
./fitglue-admin executions:clean
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

### `fitbit:subscribe <userId>`

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


### `users:update <userId>`

Updates the configuration for an existing user. Currently supports updating integration settings.

**Usage:**
```bash
./fitglue-admin users:update my-test-user
```

**Prompts:**
1.  **Hevy Integration**: Update Hevy API Key?
2.  **Strava Integration**: Update Strava credentials? (Access Token, Refresh Token, Expires At, Athlete ID)
3.  **Fitbit Integration**: Update Fitbit credentials? (Access Token, Refresh Token, Expires At, User ID)

### `users:add-pipeline <userId>`

Adds a data processing pipeline to a user. This command allows you to define complex routing and enrichment flows, such as "Hevy -> Fitbit HR Enrichment -> Strava".

**Usage:**
```bash
./fitglue-admin users:add-pipeline my-user-id
```

**Prompts:**
1.  **Source**: Select the data source triggering this pipeline (e.g., `SOURCE_HEVY`, `SOURCE_KEISER`).
2.  **Enrichers**:
    *   Add enrichers in sequence (e.g., first `fitbit-heart-rate`, then `ai-description`).
    *   You can optionally provide a JSON string for specific enricher inputs.
3.  **Destinations**: Select where the final data should be sent (e.g., `strava`).

## GCS Bucket Commands

### `buckets:list`
List all GCS buckets in the project.

```bash
./fitglue-admin buckets:list
```

### `buckets:get <bucketId>`
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
