# Google Sheets Connection Setup

## Prerequisites

1. A [Google Cloud Console](https://console.cloud.google.com) project (the same one FitGlue runs on)
2. `gcloud` CLI authenticated with appropriate permissions

## 1. Enable the Google Sheets API

The Google Sheets API is enabled automatically via Terraform (`apis.tf`). No manual action needed.

## 2. Create OAuth 2.0 Credentials

1. Go to **APIs & Credentials** → **Create Credentials** → **OAuth 2.0 Client ID**
2. Application type: **Web application**
3. Add the **Authorized redirect URI**:
   - Dev: `https://dev.fitglue.tech/auth/google/callback`
   - Prod: `https://fitglue.tech/auth/google/callback`
4. Set the OAuth consent screen scopes:
   - `https://www.googleapis.com/auth/spreadsheets`
   - `https://www.googleapis.com/auth/userinfo.profile`
5. Copy the **Client ID** and **Client Secret**

## 3. Store Secrets

```bash
./scripts/configure_oauth_secrets.sh google dev
```

This stores `google-client-id` and `google-client-secret` in Secret Manager.

## 4. Remove the Coming Soon Flag

In [registry.ts](file:///home/ripixel/dev/fitglue/server/src/typescript/shared/src/plugin/registry.ts), remove `isTemporarilyUnavailable: true` from the Google integration (line ~3748).

## 5. Deploy

```bash
cd terraform && terraform apply -var-file=envs/dev.tfvars
```

This deploys the updated secrets to:
- **google-oauth-handler** — OAuth callback (token exchange)
- **googlesheets-uploader** — Sheets row appender (uses secrets for token refresh via transport layer)

## Architecture Reference

| Component | Path |
|:---|:---|
| OAuth Handler | `server/src/typescript/google-oauth-handler/src/index.ts` |
| Sheets Uploader | `server/src/go/functions/googlesheets-uploader/function.go` |
| Secrets (Terraform) | `server/terraform/secrets.tf` — `google_client_id`, `google_client_secret` |
| OAuth Function (Terraform) | `server/terraform/oauth_functions.tf` |
| Uploader Function (Terraform) | `server/terraform/functions.tf` |
| Firebase Routing | `web/firebase.json` — `/auth/google/callback` |
| Registry Entry | `server/src/typescript/shared/src/plugin/registry.ts` — `id: 'google'` |
