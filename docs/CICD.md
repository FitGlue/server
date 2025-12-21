# CI/CD Deployment Guide

This guide details the CI/CD pipeline configuration for `fitglue-server`, focusing on the OIDC authentication setup with Google Cloud Platform (GCP).

## Overview

We use **CircleCI** for our CI/CD pipeline, connecting to **GCP** using **OpenID Connect (OIDC)**. OIDC allows CircleCI to authenticate with GCP without managing long-lived service account keys, improving security.

## Pipeline Workflow

The CI/CD pipeline automatically:
1. **Builds and tests** all code on every commit
2. **Deploys to Dev** automatically on `main` branch
3. **Deploys to Test** after manual approval
4. **Deploys to Prod** after manual approval

## Setting Up OIDC for a New Environment

### Step 1: Run the Setup Script

We provide `scripts/setup_oidc.sh` to automate the GCP configuration. Run it for each environment:

```bash
# For Dev (already configured)
./scripts/setup_oidc.sh

# For Test (update script with test project ID)
# Edit scripts/setup_oidc.sh: PROJECT_ID="fitglue-server-test"
./scripts/setup_oidc.sh

# For Prod (update script with prod project ID)
# Edit scripts/setup_oidc.sh: PROJECT_ID="fitglue-server-prod"
./scripts/setup_oidc.sh
```

### Step 2: Enable Required APIs

```bash
gcloud services enable cloudresourcemanager.googleapis.com --project=fitglue-server-<ENV>
gcloud services enable iam.googleapis.com --project=fitglue-server-<ENV>
gcloud services enable iamcredentials.googleapis.com --project=fitglue-server-<ENV>
```

### Step 3: Update CircleCI Config

The `.circleci/config.yml` is already configured for all three environments (dev, test, prod). No changes needed unless you're adding a new environment.

## Critical Configuration Details

### 1. Attribute Mapping
The Workload Identity Provider must map CircleCI token claims to Google attributes:
```
attribute.project_id = assertion['oidc.circleci.com/project-id']
attribute.org_id     = assertion.aud
google.subject       = assertion.sub
```

### 2. Allowed Audiences
**CRITICAL**: The allowed audience must be your **CircleCI Organization ID**, NOT the GCP resource path.
- ✅ **Correct**: `b2fc92f7-4f8d-4676-95b1-94d7f15c0a8e` (CircleCI Org ID)
- ❌ **Wrong**: `//iam.googleapis.com/projects/...`

To find your CircleCI Org ID:
1. Go to CircleCI → Organization Settings
2. Copy the Organization ID

### 3. IAM Binding
The service account must allow the Workload Identity principal to impersonate it:
- **Role**: `roles/iam.workloadIdentityUser`
- **Member**: `principalSet://iam.googleapis.com/projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/${POOL_ID}/attribute.org_id/${CIRCLECI_ORG_ID}`

## How OIDC Authentication Works

The `deploy` job in CircleCI:

1. **Installs gcloud SDK** (Alpine Linux requires manual installation)
2. **Creates credential config** using CircleCI's OIDC token:
   ```bash
   gcloud iam workload-identity-pools create-cred-config \
     "projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/${POOL_ID}/providers/${PROVIDER_ID}" \
     --service-account="${SERVICE_ACCOUNT_EMAIL}" \
     --output-file=/tmp/gcp_cred_config.json \
     --credential-source-file=/tmp/oidc_token.txt
   ```
3. **Authenticates** with GCP:
   ```bash
   gcloud auth login --brief --cred-file=/tmp/gcp_cred_config.json
   ```
4. **Exports credentials** for Terraform:
   ```bash
   export GOOGLE_APPLICATION_CREDENTIALS=/tmp/gcp_cred_config.json
   ```

## Troubleshooting

### "The audience in ID Token does not match the expected audience"
- **Cause**: The GCP Workload Identity Provider's allowed audiences don't include your CircleCI Org ID
- **Fix**: Update the Provider's "Allowed Audiences" to your CircleCI Organization ID

### "Cloud Resource Manager API has not been used"
- **Cause**: The target project hasn't enabled the necessary API
- **Fix**: `gcloud services enable cloudresourcemanager.googleapis.com --project=${PROJECT_ID}`

### "Could not find default credentials"
- **Cause**: `GOOGLE_APPLICATION_CREDENTIALS` isn't set correctly
- **Fix**: Ensure the env var is exported inline with the terraform command (already done in our config)

### gcloud installation fails
- **Cause**: Missing dependencies in Alpine Linux executor
- **Fix**: Ensure `apk add --no-cache python3 py3-pip curl bash` runs before gcloud installation

## References

- [CircleCI OIDC Documentation](https://circleci.com/docs/openid-connect-tokens/)
- [GCP Workload Identity Federation](https://cloud.google.com/iam/docs/workload-identity-federation)
- [CircleCI GCP OIDC Example](https://circleci.com/docs/guides/permissions-authentication/oidc-tokens-with-custom-claims/)
