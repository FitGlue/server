# OpenAPI Client Generation

FitGlue uses `openapi-typescript` and `openapi-fetch` (TypeScript) and `oapi-codegen` (Go) to generate strongly-typed API clients from OpenAPI (Swagger) specifications.

## Directory Structure

Specs are located in `src/openapi/<service_name>/swagger.json`.
Generated clients are output to:
- **TypeScript**: `src/typescript/shared/src/api/<service_name>/schema.ts`
- **Go**: `src/go/pkg/api/<service_name>/client.gen.go`

## Workflow

1.  **Place Spec**: Save the `swagger.json` file in `src/openapi/<service_name>/`.
    *   Example: `src/openapi/hevy/swagger.json`

2.  **Generate Types**: Run the generation command via Makefile.
    ```bash
    make generate
    ```
    This command:
    - Generates TypeScript schemas using `openapi-typescript`.
    - Generates Go client code using `oapi-codegen`.

3.  **TypeScript Usage**:
    *   Import basic schema types from `@fitglue/shared/dist/api/<service>/schema`.
    *   Use a factory function (e.g., `createHevyClient`) to wrap `openapi-fetch` with authentication.

    *Example (`src/typescript/shared/src/api/hevy/client.ts`):*
    ```typescript
    import createClient from "openapi-fetch";
    import type { paths } from "./schema";

    export function createHevyClient(config: { apiKey: string }) {
        const client = createClient<paths>({
            baseUrl: "https://api.hevyapp.com",
            headers: { "api-key": config.apiKey }
        });
        return client;
    }
    ```

    *Consumer Usage:*
    ```typescript
    const client = createHevyClient({ apiKey: '...' });
    const { data } = await client.GET("/v1/workouts/{workoutId}", {
        params: { path: { workoutId: "123" } }
    });
    ```

4.  **Go Usage**:
    *   Import the generated package `github.com/ripixel/fitglue-server/src/go/pkg/api/<service>`.
    *   Use `NewClientWithResponses` to create a client.

    *Example:*
    ```go
    import "github.com/ripixel/fitglue-server/src/go/pkg/api/strava"

    client, _ := strava.NewClientWithResponses("https://www.strava.com/api/v3", strava.WithRequestEditorFn(authMiddleware))
    resp, _ := client.GetActivityByIdWithResponse(ctx, 12345)
    ```

## Spec Maintenance
- Maintain a single source of truth in `src/openapi`.
- Use `components/schemas` for reusable types to ensure clean code generation.
- Validated specs are critical—fix `swagger.json` errors (e.g., path parameters) to ensure generation succeeds.
