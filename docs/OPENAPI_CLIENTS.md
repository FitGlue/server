# OpenAPI Client Generation

FitGlue uses `openapi-typescript` and `openapi-fetch` to generate strongly-typed API clients from OpenAPI (Swagger) specifications. This ensures end-to-end type safety when interacting with external services like Hevy.

## Workflow

1.  **Place Spec**: Save the `swagger.json` or `openapi.yaml` file in `src/typescript/shared/openapi/<service_name>/`.
    *   Example: `src/typescript/shared/openapi/hevy/swagger.json`

2.  **Generate Types**: Run the generation command via Makefile.
    ```bash
    make generate
    ```
    This command invokes `openapi-typescript` to convert the spec into a TypeScript definition file (e.g., `src/typescript/shared/src/hevy-api/schema.ts`).

    *Makefile Target Example:*
    ```makefile
    # Generate OpenAPI Types (Hevy)
    cd $(TS_SRC_DIR)/shared && npx openapi-typescript openapi/hevy/swagger.json -o src/hevy-api/schema.ts
    ```

3.  **Create Client Factory**: Implement a factory function in `@fitglue/shared` to wrap the `openapi-fetch` client. This allows for centralized configuration (headers, base URL) and middleware (auth).

    *Example (`src/typescript/shared/src/hevy-api/client.ts`):*
    ```typescript
    import createClient, { Middleware } from "openapi-fetch";
    import type { paths } from "./schema"; // Generated types

    export function createHevyClient(config: { apiKey: string }) {
        const client = createClient<paths>({
            baseUrl: "https://api.hevyapp.com"
        });

        // Add Auth Middleware
        const authMiddleware: Middleware = {
            async onRequest({ request }) {
                request.headers.set("api-key", config.apiKey);
                return request;
            },
        };

        client.use(authMiddleware);
        return client;
    }
    ```

4.  **Use in Handler**: Import the factory and use the client with full type support.

    *Example Usage:*
    ```typescript
    const client = createHevyClient({ apiKey: '...' });

    // strongly typed params and response
    const { data, error } = await client.GET("/v1/workouts/{workoutId}", {
        params: {
            path: { workoutId: "123" }
        }
    });

    if (data) {
        console.log(data.title); // Auto-completed!
    }
    ```

## Benefits

*   **Type Safety**: Request parameters and response bodies are strictly typed against the API spec.
*   **Zero Boilerplate**: No need to manually define interfaces for API responses.
*   **Sync**: Re-running `make generate` updates types if the API spec changes.
