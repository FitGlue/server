/**
 * Reads a secret from environment variables.
 * Secrets are injected via Terraform's secret_environment_variables blocks.
 * @param secretName The name of the environment variable containing the secret.
 * @returns The secret string value.
 * @throws Error if the secret is not found in environment variables.
 */
export function getSecret(secretName: string): string {
    const value = process.env[secretName];

    if (!value) {
        throw new Error(`Secret ${secretName} not found in environment variables`);
    }

    return value;
}
