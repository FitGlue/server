import createClient, { Middleware } from "openapi-fetch";
import type { paths, components } from "./schema";

// Utility type to make a specific header optional in the paths definition
// This allows middleware to handle headers (like api-key) without forcing the caller to provide them.
type OmitHeader<T, K extends string> = {
    [Path in keyof T]: {
        [Method in keyof T[Path]]: T[Path][Method] extends { parameters: { header: infer H } }
            ? Omit<T[Path][Method], "parameters"> & {
                  parameters: Omit<T[Path][Method]["parameters"], "header"> & {
                      header?: Omit<H, K> & Partial<Pick<H, Extract<keyof H, K>>>;
                  };
              }
            : T[Path][Method];
    };
};

export type ClientPaths = OmitHeader<paths, "api-key">;
export type HevyClient = ReturnType<typeof createClient<ClientPaths>>;
export type Workout = components['schemas']['Workout'];

export interface HevyClientOptions {
    apiKey: string;
}

const authMiddleware = (apiKey: string): Middleware => ({
    onRequest({ request }) {
        request.headers.set("api-key", apiKey);
        return request;
    },
});

export function createHevyClient(options: HevyClientOptions): HevyClient {
    const client = createClient<ClientPaths>({
        baseUrl: "https://api.hevyapp.com",
    });

    client.use(authMiddleware(options.apiKey));

    return client;
}
