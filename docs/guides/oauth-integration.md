# OAuth Integration Guide

This document explains how to configure and use OAuth 2.0 authentication for Strava and Fitbit integrations in FitGlue.

## Overview

FitGlue uses OAuth 2.0 to securely connect user accounts from Strava and Fitbit. The OAuth flow:

1. User initiates connection (via admin CLI or future web dashboard)
2. User is redirected to provider's authorization page
3. User grants permissions
4. Provider redirects back to FitGlue with authorization code
5. FitGlue exchanges code for access/refresh tokens
6. Tokens are stored in Firestore and linked to the user's account

## Security Features

- **CSRF Protection**: State tokens are HMAC-signed with a secret and expire after 10 minutes
- **Token Storage**: Access/refresh tokens are stored in Firestore with expiration timestamps
- **Identity Mapping**: External user IDs are mapped to FitGlue user IDs for webhook lookups
- **HTTPS Only**: All OAuth callbacks must use HTTPS in production

## Setup Instructions

### 1. Register OAuth Applications

#### Strava
1. Go to https://www.strava.com/settings/api
2. Click "Create App"
3. Fill in application details:
   - **Application Name**: FitGlue
   - **Category**: Training
   - **Website**: https://fitglue.tech
   - **Authorization Callback Domain**: `fitglue.tech` (for prod) or `dev.fitglue.tech` (for dev)
4. Save the **Client ID** and **Client Secret**

#### Fitbit
1. Go to https://dev.fitbit.com/apps
2. Click "Register a New App"
3. Fill in application details:
   - **Application Name**: FitGlue
   - **OAuth 2.0 Application Type**: Server
   - **Callback URL**: `https://fitglue.tech/auth/fitbit/callback` (prod) or `https://dev.fitglue.tech/auth/fitbit/callback` (dev)
4. Save the **Client ID** and **Client Secret**

### 2. Store Secrets in Google Secret Manager

We provide helper scripts to simplify secret configuration:

#### Configure OAuth State Secret (One-time per environment)

This generates a cryptographically secure random secret for CSRF protection:

```bash
./scripts/configure_oauth_state_secret.sh dev
./scripts/configure_oauth_state_secret.sh test
./scripts/configure_oauth_state_secret.sh prod
```

#### Configure OAuth Credentials

For each provider and environment, run:

```bash
# Development
./scripts/configure_oauth_secrets.sh strava dev
./scripts/configure_oauth_secrets.sh fitbit dev

# Test
./scripts/configure_oauth_secrets.sh strava test
./scripts/configure_oauth_secrets.sh fitbit test

# Production
./scripts/configure_oauth_secrets.sh strava prod
./scripts/configure_oauth_secrets.sh fitbit prod
```

The script will prompt you for the Client ID and Client Secret (hidden input) and automatically create or update the secrets in Google Secret Manager.

**Note**: Terraform creates the secret *containers*, but the actual secret *values* are populated manually via these scripts. This keeps sensitive data out of Terraform state files.

### 3. Deploy OAuth Handlers

OAuth handlers are part of `service.api.webhook` (callback endpoints) and `service.user` (token storage). Deploy via Terraform:

```bash
cd terraform
terraform apply -var-file=envs/dev.tfvars
```

This deploys all Cloud Run services including the OAuth callback endpoints:
- `https://dev.fitglue.tech/auth/strava/callback`
- `https://dev.fitglue.tech/auth/fitbit/callback`

## Usage

### Via Admin CLI

1. **Create a user** (if not already exists):
   ```bash
   fitglue-admin users:create
   ```

2. **Generate OAuth URL**:
   ```bash
   fitglue-admin users:connect <userId> strava
   # or
   fitglue-admin users:connect <userId> fitbit
   ```

3. **Visit the URL** in a browser and authorize the application

4. **Verify tokens stored**:
   ```bash
   fitglue-admin users:list
   ```

### Via Web Dashboard

The web dashboard provides "Connect Strava" / "Connect Fitbit" buttons that:
1. Call `service.api.client` to generate a state token
2. Redirect the user to the OAuth authorization URL
3. Handle the callback automatically via `service.api.webhook`

## Token Refresh

OAuth tokens expire and must be refreshed periodically:

- **Strava**: Tokens expire after 6 hours
- **Fitbit**: Tokens expire after 8 hours

**TODO**: Implement automatic token refresh logic in a scheduled Cloud Function or during API calls when a 401 error is detected.

## Firestore Schema

### User Record
```
users/{userId}
  integrations:
    strava:
      enabled: true
      access_token: "..."
      refresh_token: "..."
      expires_at: Timestamp
      athlete_id: 12345
    fitbit:
      enabled: true
      access_token: "..."
      refresh_token: "..."
      expires_at: Timestamp
      fitbit_user_id: "ABC123"
```

### Identity Mapping
```
integrations/strava/ids/{athleteId}
  userId: "uuid-..."
  createdAt: Timestamp

integrations/fitbit/ids/{fitbitUserId}
  userId: "uuid-..."
  createdAt: Timestamp
```

## Troubleshooting

### "Invalid state token" error
- State tokens expire after 10 minutes. Generate a new URL.
- Ensure the `oauth-state-secret` is the same across all function instances.

### "Failed to exchange code for tokens"
- Verify client ID and secret are correct in Secret Manager.
- Check that the redirect URI matches exactly what's registered with the provider.
- Ensure the authorization code hasn't been used already (codes are single-use).

### Tokens not appearing in Firestore
- Check Cloud Function logs for errors.
- Verify the user ID in the state token matches an existing user.
- Ensure the function has Firestore write permissions.

## Security Considerations

1. **Never log tokens**: Access/refresh tokens should never appear in logs.
2. **Rotate secrets regularly**: Change the `oauth-state-secret` periodically.
3. **Validate redirect URIs**: Ensure OAuth apps only allow approved callback URLs.
4. **Use HTTPS**: Never use OAuth over HTTP in production.
5. **Limit scopes**: Only request the minimum permissions needed.

## API Scopes

### Strava
- `read`: Read public profile data
- `activity:read_all`: Read all activity data (including private activities)

### Fitbit
- `activity`: Read activity data
- `heartrate`: Read heart rate data
- `profile`: Read profile information

## Future Enhancements

- [ ] Rate limiting and retry logic for API calls
- [ ] Granular scope management per integration
