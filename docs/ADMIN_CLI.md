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
2.  **Hevy Integration**: Do you want to configure Hevy? (Default: Yes)
3.  **Strava Integration**: Do you want to configure Strava? (Default: No)
    *   **Access Token**: The user's OAuth Access Token.
    *   **Refresh Token**: The user's OAuth Refresh Token.
    *   **Expires At**: Token expiration (Unix timestamp in seconds).
    *   **Athlete ID**: The Strava Athlete ID.
4.  **Fitbit Integration**: Do you want to configure Fitbit? (Default: No)
    *   **Access Token**: The user's OAuth Access Token.
    *   **Refresh Token**: The user's OAuth Refresh Token.
    *   **Expires At**: Token expiration (Unix timestamp in seconds).
    *   **User ID**: The Fitbit User ID.

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
