import { UserIntegrations } from './pb/user';
import { IntegrationAuthType } from './pb/plugin';

// Field that can be configured via CLI/API manually
export interface ConfigurableField {
  field: string;
  name: string;
  type: 'string' | 'boolean' | 'password';
  required: boolean;
}

// Integration definition that aligns with protobuf auth types
export interface IntegrationDefinition {
  key: keyof UserIntegrations;
  displayName: string;
  authType: IntegrationAuthType;
  // For OAuth integrations: which proto field contains the external user ID
  externalUserIdField?: string;
  // Fields that can be configured via CLI
  configurableFields: ConfigurableField[];
}

// Helper type guard
export const isOAuthIntegration = (def: IntegrationDefinition): boolean =>
  def.authType === IntegrationAuthType.INTEGRATION_AUTH_TYPE_OAUTH;

// Registry - single source derives from UserIntegrations proto keys
export const INTEGRATIONS: Record<keyof UserIntegrations, IntegrationDefinition> = {
  hevy: {
    key: 'hevy',
    displayName: 'Hevy',
    authType: IntegrationAuthType.INTEGRATION_AUTH_TYPE_API_KEY,
    configurableFields: [{ field: 'apiKey', name: 'API Key', type: 'password', required: true }],
  },
  mock: {
    key: 'mock',
    displayName: 'Mock',
    authType: IntegrationAuthType.INTEGRATION_AUTH_TYPE_API_KEY,
    configurableFields: [{ field: 'enabled', name: 'Enabled', type: 'boolean', required: true }],
  },
  strava: {
    key: 'strava',
    displayName: 'Strava',
    authType: IntegrationAuthType.INTEGRATION_AUTH_TYPE_OAUTH,
    externalUserIdField: 'athleteId',
    configurableFields: [], // OAuth configured via flow
  },
  fitbit: {
    key: 'fitbit',
    displayName: 'Fitbit',
    authType: IntegrationAuthType.INTEGRATION_AUTH_TYPE_OAUTH,
    externalUserIdField: 'fitbitUserId',
    configurableFields: [], // OAuth configured via flow
  },
  parkrun: {
    key: 'parkrun',
    displayName: 'Parkrun',
    authType: IntegrationAuthType.INTEGRATION_AUTH_TYPE_PUBLIC_ID,
    configurableFields: [{ field: 'athleteId', name: 'Barcode Number', type: 'string', required: true }],
  },
};
