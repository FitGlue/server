---
trigger: always_on
---

# FitGlue Codebase Style Guide
This document outlines the architectural patterns, coding standards, and principles used in the FitGlue codebase. Use this as a reference when creating new components or similar projects.
## 1. Architectural Principles
### Monorepo Structure
- **Unified Go Module**: All Go code lives under `src/go` with a single `go.mod` at `github.com/ripixel/fitglue-server/src/go`.
- **TypeScript Workspaces**: All TypeScript code uses npm workspaces under `src/typescript` with a shared library `@fitglue/shared`.
- **No Code Injection**: Shared code is imported directly via Go modules and npm workspaces. No build-time copying.
- **Protocol Buffers as Contract**: Use Protobuf (`.proto` files in `src/proto`) as the single source of truth for data structures passed between services.
### Hybrid Local/Cloud Development
- **Local Compute, Cloud Data**: Run functions locally (`./scripts/local_run.sh`), but connect to a real "Dev" GCP environment for stateful services (Firestore, Storage).
- **Avoid Local Emulators**: Do not emulate Cloud Firestore or Pub/Sub locally. Use real cloud resources with unique IDs.
- **Environment Isolation**: Use distinct GCP projects for each environment (Dev, Test, Prod) managed by Terraform.
## 2. Go Coding Standards
### Structure
- **Entry Point**: `function.go` contains the `functions-framework` handler. It loads config and wires dependencies.
- **Business Logic**: `pkg/` contains core logic, isolated from the cloud framework.
- **Shared Library**: `src/go/pkg/` contains shared utilities, adapters, and types used across all functions.
- **Commands**: Each function has a `cmd/main.go` for local execution via Functions Framework.
### Dependency Injection
- **Service Struct**: Use `bootstrap.Service` struct holding all dependencies (Database, PubSub, Secrets, Config).
- **Initialization**: Initialize once in `init()` for connection pooling, pass to handlers.
- **Interfaces**: Define interfaces in `pkg/` for external dependencies to enable mocking.
### Error Handling & Logging
- **Structured Logging**: Use `log/slog` with JSON output. Always include `execution_id`, `service`, and `user_id`.
- **Fail Fast**: Return errors immediately for non-retriable failures. Log and return 200 for retriable errors (Pub/Sub will retry).
### Testing
- **Unit Tests**: Use `testing` package with table-driven tests.
- **Mocks**: Mock `bootstrap.Service` dependencies to test logic without cloud calls.
- **Integration Tests**: Minimal, relying on "Dev" environment for real component interaction.
## 3. TypeScript Coding Standards
### Workspace Structure
- **Shared Library**: `@fitglue/shared` contains framework wrappers, secret management, and common utilities.
- **Function Packages**: Each function (`hevy-handler`, `keiser-poller`) is a separate workspace package.
- **Build Order**: Always build `shared` first, then dependent packages. The Makefile enforces this.
### Framework Wrapper
- **Context Injection**: Wrap handlers in `createCloudFunction(handler)` from `@fitglue/shared` to inject `db`, `logger`, and `config`.
- **Type Safety**: Use generated Protobuf types for all Pub/Sub messages.
### Security
- **Signature Verification**: Manually verify webhooks (HMAC SHA256) before processing.
- **Secret Management**: Use `getSecret()` from `@fitglue/shared` which falls back to env vars for local dev.
## 4. Infrastructure (Terraform)
### Organization
- **Environment Variables**: Use `terraform/envs/*.tfvars` for environment-specific configuration.
- **Source Management**: Zip function source from `src/go` or `src/typescript` and upload to GCS.
- **Build Configuration**: Set `GOOGLE_BUILDABLE` environment variable for Go functions to specify the package path in monorepo.
### Naming
- **Resource Naming**: `[function-name]` (e.g., `enricher`, `hevy-webhook-handler`).
- **Service Accounts**: Use least-privilege IAM for each function.
## 5. Build & CI/CD
### Makefile Targets
- **`make setup`**: Install all dependencies (Go modules + npm workspaces).
- **`make generate`**: Generate Protocol Buffer code for Go and TypeScript.
- **`make build`**: Compile all code. **Critical**: Builds `@fitglue/shared` first, then dependent packages.
- **`make test`**: Run all unit tests (requires build first).
- **`make lint`**: Run linters for Go and TypeScript.
### CI Pipeline Order
1. `make setup` - Install dependencies and link workspaces
2. `make build` - Compile code (**must** run before tests for TypeScript)
3. `make lint` - Check code quality
4. `make test` - Run tests
### Error Propagation
- All Makefile loops use `|| exit 1` to fail fast on errors.
- No silent failures in CI.
## 6. "The FitGlue Way" Checklist
- [ ] **Is it DRY?** Shared logic should be in `src/go/pkg/` or `@fitglue/shared`.
- [ ] **Is it Protocol First?** Define data shapes in `src/proto/*.proto` first.
- [ ] **Is it Mockable?** Can I test this function without an internet connection?
- [ ] **Is it Observable?** Do logs include the `execution_id`?
- [ ] **Is it Simple?** Did I avoid adding unnecessary dependencies or build complexity?
- [ ] **Is it Built in Order?** For TypeScript, is `@fitglue/shared` built before dependent packages?
