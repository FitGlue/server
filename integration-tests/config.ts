import environments from './environments.json';

export type Environment = 'local' | 'dev' | 'test' | 'prod';

export interface TestConfig {
  environment: Environment;
  projectId: string;
  region: string;
  gcsBucket: string;
  endpoints?: {
    hevyWebhook: string;
    enricher?: string;
    router?: string;
    stravaUploader?: string;
  };
  topics?: {
    rawActivity: string;
    enrichedActivity: string;
    uploadStrava: string;
  };
}

/**
 * Get test configuration based on TEST_ENVIRONMENT env var
 * Defaults to 'local' if not set
 */
export function getConfig(): TestConfig {
  const env = (process.env.TEST_ENVIRONMENT || 'local') as Environment;

  if (!['local', 'dev', 'test', 'prod'].includes(env)) {
    throw new Error(
      `Invalid TEST_ENVIRONMENT: ${env}. Must be one of: local, dev, test, prod`
    );
  }

  const envConfig: any = environments[env];

  return {
    environment: env,
    projectId: envConfig.projectId,
    region: envConfig.region,
    gcsBucket: envConfig.gcsBucket,
    endpoints: envConfig.endpoints,
    topics: envConfig.topics,
  };
}

export const config = getConfig();
