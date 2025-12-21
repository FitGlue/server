import { SecretManagerServiceClient } from '@google-cloud/secret-manager';

let client: SecretManagerServiceClient;

function getClient() {
    if (!client) {
        client = new SecretManagerServiceClient();
    }
    return client;
}

/**
 * Fetches the latest version of a secret from Google Secret Manager.
 * @param projectId The Google Cloud Project ID.
 * @param secretName The name of the secret (not the full path).
 * @returns The secret string value.
 */
export async function getSecret(projectId: string, secretName: string): Promise<string> {
    // 1. Local Fallback: Check environment variable first
    // This allows local development without needing authenticated access to Secret Manager
    if (process.env[secretName]) {
        console.log(`[SecretManager] Using local env var for: ${secretName}`);
        return process.env[secretName]!;
    }

    // 2. Cloud Secret Manager
    const name = `projects/${projectId}/secrets/${secretName}/versions/latest`;

    try {
        const [version] = await getClient().accessSecretVersion({
            name: name,
        });

        const payload = version.payload?.data?.toString();
        if (!payload) {
            throw new Error(`Secret payload is empty for ${secretName}`);
        }
        return payload;
    } catch (err: any) {
        console.error(`Failed to fetch secret ${secretName}:`, err);
        throw err;
    }
}
