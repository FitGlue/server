# Fitbit Integration Setup Guide

This guide walks you through the complete setup to enable Fitbit activity syncing, including Webhook configuration and user subscriptions.

## 1. Prerequisites

Ensure you have deployed the latest Terraform configuration:
```bash
cd server/terraform
terraform apply -var-file=envs/dev.tfvars
```

This must deploy the `fitbit-webhook-handler` and `fitbit-ingest` functions.

## 2. Configure Secrets

We need to set the verification code that Fitbit will use to verify our webhook endpoint.

1.  Pick a random string (e.g., `my-secret-verify-code-123`).
2.  Run the configuration script:
    ```bash
    ./server/scripts/configure_fitbit_verification.sh dev
    ```
    *   Enter your chosen string when prompted.

## 3. Configure Fitbit Developer Portal

1.  Log in to [dev.fitbit.com/apps](https://dev.fitbit.com/apps).
2.  Click **Manage** on your existing app (or create one).
3.  Click **Edit Application**.
4.  Ensure **OAuth 2.0 Application Type** is **Server**.
5.  Set **Callback URL**: `https://<YOUR_DOMAIN>/auth/fitbit/callback` (or your OAuth Handler URL).
6.  Click **Save**.
7.  Click on the **Subscription/Subscriber Interface** tab (or "Manage" button next to "Webhook").
8.  **Endpoint URL**: Enter the URL of your deployed `fitbit-webhook-handler`.
    *   Find it via: `gcloud functions describe fitbit-webhook-handler --region=us-central1 --format='value(serviceConfig.uri)'`
9.  **Verification Code**: Enter the *same string* you set in Step 2.
10. Click **Apply** (or Verify).
    *   Fitbit will send a `GET` request to your handler.
    *   If correct, the handler returns `204 No Content` and Fitbit saves the endpoint.

## 4. User Setup (Connect & Subscribe)

Now that the system is listening, you need to connect a user and subscribe to their updates.

### A. Connect User (OAuth)
If the user hasn't authenticated yet:

```bash
# Get the Auth URL
./fitglue-admin users:connect <USER_ID> fitbit
# (Follow the link, authorize, and ensure the callback succeeds)
```

### B. Subscribe to Updates (Critical Step)
Updates are **not** automatic until you explicitly subscribe to the `activities` collection for that user.

```bash
./fitglue-admin fitbit:subscribe <USER_ID>
```

**Expected Output:**
```
✅ Subscription created successfully!
{
  "apiSubscriptions": [ ... ]
}
```

## 5. End-to-End Test

1.  **Trigger**: Sync your Fitbit device (or log a manual activity in the Fitbit app).
2.  **Wait**: Fitbit usually pushes the webhook notification within 5-10 seconds.
3.  **Verify**:
    *   Check `fitbit-webhook-handler` logs: "Received X updates".
    *   Check `fitbit-ingest` logs: "Fetching TCX for activity...", "Published activity...".
    *   Check `enricher` logs: "Enriching activity...".

DONE! 🚀
