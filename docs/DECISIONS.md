# Architecture & Workflow Decisions

## 001 - Hybrid Local Development Strategy (2025-12-19)

### Context
We needed a local development environment that is fast, isolated, but realistic.
Full local emulation (Firestore Emulator, Pub/Sub Emulator) requires heavy dependencies (Java JDK) not currently present.
Creating separate GCP projects for every developer is heavyweight.

### Decision
We adopted a **Hybrid Cloud/Local** workflow:

1.  **Run Local:** Services run locally via `make local` (Go binaries).
2.  **Real Database (Dev):** We use a shared "Dev" GCP Project (`fitglue-server`) for Firestore.
    - *Mitigation:* Developers use unique inputs/IDs during manual testing to collision.
    - *Test Isolation:* Integration tests generate random UUIDs (`user_test_<uuid>`) and clean up after themselves.
3.  **Mocked Pub/Sub:** We use a `LogPublisher` (No-Op) adapter by default.
    - *Why:* Prevents "Topic Pollution" where one dev's test event triggers another dev's local subscriber.
    - *Mechanism:* `ENABLE_PUBLISH` env var. Default is `false` (Log Only).
4.  **Future:** Formal "Test" and "Prod" environments will be separate GCP Projects managed by Terraform.

### Consequences
- **Pros:** Zero setup cost (no Java/Emulators). Wiring verification is fast. No topic collisions.
- **Cons:** Requires internet. "Dev" database is shared (but handled via unique IDs).

## 002 - Infrastructure & Environment Isolation (2025-12-19)

### Context
We need to support "Dev", "Test", and "Prod" environments.
We initially considered using the existing `fitglue-server` for Dev, but decided to start fresh to ensure consistency across all three envs.

### Decision
We will use **Terraform Workspaces** coupled with **tfvars** files.
We will create **3 Fresh GCP Projects** (leaving `fitglue-server` dormant).

1.  **Project Mapping:**
    - Workspace `dev` -> `fitglue-server-dev`
    - Workspace `test` -> `fitglue-server-test`
    - Workspace `prod` -> `fitglue-server-prod`
2.  **Configuration:**
    - `envs/dev.tfvars`, `envs/test.tfvars`, `envs/prod.tfvars`
3.  **State Management:**
    - Local State with Workspaces (`terraform.tfstate.d/`).

## 003 - Naming Convention (2025-12-19)

### Standard
`[project]-[purpose]-[environment]`

### Examples
- `fitglue-server-dev` (Backend Functions - Dev)
- `fitglue-server-prod` (Backend Functions - Prod)
- (Future) `fitglue-web-prod` (Frontend - Prod)

### Rationale
Allows grouping by project (`fitglue`), purpose (`server`/`app`), and environment (`dev`/`prod`) systematically.

## 004 - Monorepo Structure (2025-12-21)

### Context
We initially used a polyrepo-style structure with multiple `go.mod` files and specialized `make` targets to inject shared code into each function's directory. This caused:
-   Persistent `go.sum` checksum mismatches in CI.
-   Complexity in local development (workspace vs replacement directives).
-   "Missing module" errors because of tangled dependency graphs.

### Decision
We refactored the repository into a **Monorepo** structure:
-   `src/go`: Single Go module (`github.com/ripixel/fitglue-server/src/go`) containing all backend functions and shared packages.
-   `src/typescript`: TypeScript functions using npm workspaces.
-   `src/proto`: Unified Protocol Buffers.

### Consequences
-   **Pros:** Simplified build (standard Go tooling works). No code injection/sed hacking. Single source of truth for dependencies.
-   **Cons:** Deployments upload the entire module context (negligible size impact).
