---
trigger: always_on
---

# FitGlue Codebase Style Guide

This document outlines the architectural patterns, coding standards, and principles used in the FitGlue codebase. Use this as a reference when creating new components or similar projects.

## 1. Architectural Principles

### "Pragmatic Monorepo"
- **No Complex Tooling**: Avoid heavy monorepo tools (Nx, Bazel, Turborepo) unless necessary.
- **Copy-Paste Injection**: Shared code is "injected" (copied) into function directories at build time via [Makefile](cci:7://file:///home/ripixel/dev/fitglue/Makefile:0:0-0:0). This ensures each cloud function is a self-contained, standard project when deployed, reducing runtime complexity.
- **Protocol Buffers as Contract**: Use Protobuf (`.proto` files) as the single source of truth for data structures passed between services (Pub/Sub) and across languages (Go/TypeScript).

### Hybrid Local/Cloud Development
- **Local Compute, Cloud Data**: Run functions and services locally (`make local`), but connect to a real "Dev" GCP environment for stateful services (Firestore, Storage).
- **Avoid Local Emulators**: Do not try to emulate Cloud Firestore or Pub/Sub locally. It is brittle and heavy. Use real cloud resources with unique IDs to prevent collision.
- **Environment Isolation**: Start fresh for every environment (Dev, Test, Prod) using distinct GCP projects managed by Terraform Workspaces.

## 2. Go Coding Standards

### Structure
- **Root**: [function.go](cci:7://file:///home/ripixel/dev/fitglue/functions/enricher/function.go:0:0-0:0) contains the `functions-framework` entry point. It handles config loading and dependency wiring.
- **Business Logic**: `pkg/` contains the core logic, isolated from the cloud framework.
- **Shared Library**: Core logic used across functions lives in `shared/go` and is effectively a "standard library" for the project.

### Dependency Injection
- **Service Struct**: Use a central `bootstrap.Service` struct that holds all dependencies (Database, PubSub, Secrets, Config).
- **Initialization**: Initialize this service once in `init()` for global reuse (connection pooling) but pass it (or its members) into handlers.
- **Interfaces**: Define interfaces for all external dependencies (`Database`, `Publisher`) in `shared/go/interfaces.go` to enable mocking.

### Error Handling & Logging
- **Structured Logging**: Use `log/slog`. Always include `execution_id`, `service`, and `user_id` in logs to trace flows across microservices.
- **Fail Fast**: distinct error handling for retriable vs. non-retriable errors.

### Testing
- **Unit Tests**: Use `testing` package with table-driven tests.
- **Mocks**: Heavily use mocks for `bootstrap.Service` dependencies to test logic without cloud calls.
- **Integration Tests**: Minimal checking that wires real components together, often relying on the "Dev" environment.

## 3. TypeScript Coding Standards

### Framework Wrapper
- **Context Injection**: Do not write raw handlers. Wrap them in a `createCloudFunction(handler)` utility (from `shared/framework`) which injects a `FrameworkContext` containing `db`, `logger`, and `config`.
- **Type Safety**: Use generated Protobuf types for all Pub/Sub messages and data interfaces.

### Security
- **Signature Verification**: Manually verify webhooks (e.g., HMAC SHA256) before processing.
- **Secret Management**: Retrieve secrets from Google Secret Manager at runtime, with fallback to environment variables for local dev.

## 4. Infrastructure (Terraform)

### Organization
- **Workspaces**: Use Terraform Workspaces (`dev`, `test`, `prod`) to map to distinct GCP projects.
- **Variable Files**: Use `envs/dev.tfvars`, etc., to configure environment-specific values.
- **Source Management**: Zip function source code locally and upload to GCS for deployment (vs. Source Repositories). This keeps deployment fast and simple.

### Naming
- **Resource Naming**: `[project]-[function]-[env]` (e.g., `fitglue-server-enricher-dev`).
- **Service Accounts**: Create specific service accounts for each function with least-privilege access.

## 5. "The FitGlue Way" Checklist

- [ ] **Is it DRY?** Shared logic should be in `shared/`.
- [ ] **Is it Protocol First?** Define data shapes in `.proto` first.
- [ ] **Is it Mockable?** Can I test this function without an internet connection?
- [ ] **Is it Observable?** Do logs include the `execution_id`?
- [ ] **Is it Simple?** Did I avoid adding a new heavy dependency (like a new build tool)?
