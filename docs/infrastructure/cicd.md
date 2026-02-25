# CI/CD Deployment Guide

This guide details the CI/CD pipeline for `fitglue-server`, including OIDC authentication with GCP and Cloud Run deployment.

## Overview

We use **CircleCI** for CI/CD, connecting to **GCP** using **OpenID Connect (OIDC)**. OIDC allows CircleCI to authenticate with GCP without managing long-lived service account keys.

## Pipeline Workflow

The CI/CD pipeline automatically:
1. **Lints codebase** — Runs `make lint` (Go formatting, vet, proto-JSON check)
2. **Runs tests** — `make test` for unit tests; fails on any test failure
3. **Enforces coverage** — `make test-coverage` enforces 80% minimum per package
4. **Builds Docker images** — `make docker` for all 10 Cloud Run services
5. **Deploys to Dev** — automatically on `main` branch merge
6. **Deploys to Test** — automatically after Dev deployment succeeds
7. **Deploys to Prod** — after manual approval gate

### Codebase Linter

The `make lint` step runs:

| Check | Description |
|-------|-------------|
| Go formatting | All Go files pass `gofmt -l` |
| Go vet | `go vet` on all packages except generated integrations |
| Proto-JSON misuse | `scripts/lint-proto-json.sh` — no raw JSON marshalling of proto messages |

### Docker Build

All 10 services share a single multi-stage `Dockerfile` with `SERVICE_NAME` as a build argument:

```bash
docker build -t fitglue-activity --build-arg SERVICE_NAME=activity .
docker build -t fitglue-api-client --build-arg SERVICE_NAME=api-client .
# ... etc for all 10 services
```

The Docker image compiles the Go binary for the named service, producing a minimal image (~15MB). No ZIPs, no pruning — Cloud Run receives the container image directly.

### Cloud Run Deployment

Terraform deploys each service as a Cloud Run revision:

```bash
cd terraform
terraform apply -var-file=envs/dev.tfvars
```

Cloud Run automatically performs rolling updates with health-check gating.

All three environments (Dev, Test, Prod) use separate GCP projects with OIDC authentication.

## Setting Up OIDC for a New Environment

### Step 1: Run the Setup Script

We provide `scripts/setup_oidc.sh` to automate the GCP configuration. The script accepts the environment as a parameter:

```bash
# Usage: ./scripts/setup_oidc.sh <environment>
# Valid environments: dev, test, prod

./scripts/setup_oidc.sh dev   # For Dev
./scripts/setup_oidc.sh test  # For Test
./scripts/setup_oidc.sh prod  # For Prod
```

The script will automatically:
- Validate the environment parameter
- Fetch the project number dynamically
- **Enable required APIs** (IAM Credentials, Cloud Resource Manager, IAM, Cloud Functions, Cloud Run, Cloud Build, Artifact Registry, Eventarc, Pub/Sub, Firestore, Cloud Scheduler, Cloud Storage)
- Create the Workload Identity Pool
- Create the OIDC Provider with allowed audiences
- Create the CircleCI deployer service account
- **Grant necessary IAM permissions**:
  - `roles/editor` - Broad permissions for most resources
  - `roles/datastore.owner` - Firestore database creation
  - `roles/run.admin` - Cloud Run IAM policy management
  - `roles/resourcemanager.projectIamAdmin` - Project-level IAM bindings

### Step 2: Verify OIDC Provider Configuration (Optional)

After running the script, you can optionally verify the OIDC provider configuration:

1. Go to [IAM & Admin → Workload Identity Federation](https://console.cloud.google.com/iam-admin/workload-identity-pools)
2. Select the `circleci-pool`
3. Click on `circleci-provider`
4. Verify **Allowed audiences** contains your CircleCI Organization ID: `ecdc6640-c8ad-40c7-8710-b28261eb9107`

> **Note**: The setup script automatically configures the allowed audiences, so this verification step is optional.

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
- ✅ **Correct**: `ecdc6640-c8ad-40c7-8710-b28261eb9107` (CircleCI Org ID)
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
