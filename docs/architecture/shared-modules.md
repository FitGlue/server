# Shared Modules Architecture

The `@fitglue/shared` TypeScript package contains code shared across all Cloud Function handlers. To optimize CI/CD build times, we use a **module-based architecture** with **smart pruning**.

## Overview

```
src/typescript/shared/src/
├── config.ts           # Environment configuration
├── errors/             # Error types and codes
├── execution/          # Execution logging
├── framework/          # Cloud Function wrapper (createCloudFunction)
├── routing/            # Request routing utilities
├── storage/            # Firestore stores
├── domain/
│   ├── services/       # Business logic services
│   ├── tier.ts         # User tier logic
│   └── file-parsers/   # TCX parser etc.
├── infrastructure/
│   ├── crypto/         # Encryption utilities
│   ├── http/           # HTTP error handling
│   ├── oauth/          # OAuth token management
│   ├── pubsub/         # Cloud Event publisher
│   ├── secrets/        # Secret Manager client
│   └── sentry/         # Error reporting
├── integrations/       # External API clients
│   ├── fitbit/
│   ├── hevy/
│   ├── strava/
│   └── ... (one per integration)
├── plugin/             # Plugin registry
├── services/           # Activity counter service
└── types/
    ├── pb/             # Protobuf-generated types
    └── *.ts            # Other type definitions
```

## Import Conventions

### Recommended: Module-Level Imports

Use specific module imports to enable smart pruning:

```typescript
// ✅ GOOD - Specific modules, enables pruning
import { createCloudFunction, FrameworkContext } from '@fitglue/shared/framework';
import { UserStore, ActivityStore } from '@fitglue/shared/storage';
import { createStravaClient } from '@fitglue/shared/integrations/strava';
import { StandardizedActivity } from '@fitglue/shared/types/pb/standardized_activity';
```

### Legacy: Barrel Import

The root barrel import works but includes ALL modules:

```typescript
// ⚠️ AVOID - Imports everything, no pruning benefit
import { createCloudFunction, UserStore, createStravaClient } from '@fitglue/shared';
```

Handlers using only barrel imports will rebuild whenever ANY shared code changes.

### Deep Imports

Deep imports into `/dist/` also work and enable pruning:

```typescript
// ✅ OK - Works, enables pruning
import { UserService } from '@fitglue/shared/dist/domain/services/user';
import { SynchronizedActivity } from '@fitglue/shared/dist/types/pb/user';
```

## Module Definitions

Modules are defined in `server/scripts/shared_modules.json`. Each module specifies:

- **paths**: Source directories/files in the module
- **depends_on**: Other modules this one requires
- **always_include**: Whether to include in all builds

### Module List

| Module | Description | Common Exports |
|--------|-------------|----------------|
| `core` | Always included | `HttpError`, `config` |
| `framework` | Cloud Function wrapper | `createCloudFunction`, `FrameworkContext` |
| `storage` | Firestore stores | `UserStore`, `ActivityStore`, etc. |
| `services` | Business logic | `UserService`, `ExecutionService` |
| `types-pb` | Protobuf types | All types from `types/pb/` |
| `routing` | Request routing | `routeRequest`, `RouteMatch` |
| `plugin` | Plugin registry | `getRegistry`, `PluginManifest` |
| `execution` | Execution logging | `ExecutionStore` |
| `infra-oauth` | OAuth utilities | `validateOAuthState`, `storeOAuthTokens` |
| `infra-pubsub` | Pub/Sub | `CloudEventPublisher` |
| `int-*` | Integration clients | `createStravaClient`, `createFitbitClient`, etc. |

## Smart Pruning

### How It Works

1. **Analysis**: `analyze_ts_imports.py` extracts imports from each handler
2. **Resolution**: Imports map to module IDs via `shared_modules.json`
3. **Dependencies**: Transitive dependencies are included
4. **Pruning**: Only required modules are copied to the ZIP

### Build Output

When pruning is enabled, build output shows modules per handler:

```
Creating zip for strava-handler...
  strava-handler: 12 modules, 15 paths
Creating zip for registry-handler...
  registry-handler: 5 modules, 8 paths
```

### Disable Pruning

For debugging, disable pruning:

```bash
python3 scripts/build_typescript_zips.py --no-prune
```

## Validation

### Module Config Validation

The `lint-shared-modules` Makefile target validates `shared_modules.json`:

```bash
make lint-shared-modules
```

This runs automatically in CI and checks:
- All defined paths exist
- All dependencies reference valid modules
- All import patterns reference valid modules
- New directories have module definitions (warning)

### Codebase Lint Rules

The `lint-codebase` script includes two rules for shared imports:

| Rule | Type | Description |
|------|------|-------------|
| **T19** | Warning | Flags root barrel imports (`@fitglue/shared`) - should use module imports |
| **T20** | Error | Ensures all modules have `index.ts` barrel exports |

Run with:
```bash
make lint  # or
npx ts-node scripts/lint-codebase.ts --verbose
```

T19 warnings show which handlers need migration to module imports

## Adding a New Module

1. **Create the code** in `shared/src/{module}/`
2. **Add barrel export** in `shared/src/{module}/index.ts`
3. **Update `shared_modules.json`**:
   ```json
   "my-module": {
     "paths": ["src/my-module"],
     "depends_on": ["core", "types-pb"]
   }
   ```
4. **Add import pattern**:
   ```json
   "@fitglue/shared/dist/my-module": ["my-module"]
   ```
5. **Run validation**: `make lint-shared-modules`

## Migrating Handlers

To migrate a handler from barrel to module imports:

1. **Identify imports**: Check what the handler imports from `@fitglue/shared`
2. **Map to modules**: Determine which modules provide those exports
3. **Update imports**: Change to module-specific imports
4. **Test**: Ensure handler still builds and tests pass

Example migration:

```typescript
// Before
import { createCloudFunction, UserStore, HttpError } from '@fitglue/shared';

// After
import { createCloudFunction } from '@fitglue/shared/framework';
import { UserStore } from '@fitglue/shared/storage';
import { HttpError } from '@fitglue/shared/errors';
```

## Related Documentation

- [CI/CD](../infrastructure/cicd.md) - Build pipeline details
- [Services & Stores](services-and-stores.md) - Business logic architecture
