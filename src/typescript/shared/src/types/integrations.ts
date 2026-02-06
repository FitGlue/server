import { UserIntegrations } from './pb/user';
import { IntegrationAuthType } from './pb/plugin';

// Field that can be configured via CLI/API manually
export interface ConfigurableField {
  field: string;
  name: string;
  type: 'string' | 'boolean' | 'password';
  required: boolean;
}

// One-off action that can be triggered for an integration
export interface ActionDefinition {
  id: string;
  label: string;
  description: string;
  icon: string;
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
  // One-off actions available for this integration
  actions?: ActionDefinition[];
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
    externalUserIdField: 'apiKey',
    configurableFields: [{ field: 'apiKey', name: 'API Key', type: 'password', required: true }],
    actions: [
      {
        id: 'import_strength_prs',
        label: 'Import Strength PRs',
        description: 'Import 1RM and volume records from your last 12 months of Hevy workouts',
        icon: '💪',
      },
    ],
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
    actions: [
      {
        id: 'import_cardio_prs',
        label: 'Import Cardio PRs',
        description: 'Fetch your fastest 5K, 10K, and half marathon times from Strava history',
        icon: '🏃',
      },
    ],
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
    externalUserIdField: 'athleteId',
    configurableFields: [{ field: 'athleteId', name: 'Barcode Number', type: 'string', required: true }],
  },
  spotify: {
    key: 'spotify',
    displayName: 'Spotify',
    authType: IntegrationAuthType.INTEGRATION_AUTH_TYPE_OAUTH,
    externalUserIdField: 'spotifyUserId',
    configurableFields: [], // OAuth configured via flow
  },
  trainingpeaks: {
    key: 'trainingpeaks',
    displayName: 'TrainingPeaks',
    authType: IntegrationAuthType.INTEGRATION_AUTH_TYPE_OAUTH,
    externalUserIdField: 'athleteId',
    configurableFields: [], // OAuth configured via flow
  },
  intervals: {
    key: 'intervals',
    displayName: 'Intervals.icu',
    authType: IntegrationAuthType.INTEGRATION_AUTH_TYPE_API_KEY,
    externalUserIdField: 'athleteId',
    configurableFields: [{ field: 'apiKey', name: 'API Key', type: 'password', required: true }],
  },
  google: {
    key: 'google',
    displayName: 'Google',
    authType: IntegrationAuthType.INTEGRATION_AUTH_TYPE_OAUTH,
    externalUserIdField: 'googleUserId',
    configurableFields: [], // OAuth configured via flow
  },
  oura: {
    key: 'oura',
    displayName: 'Oura',
    authType: IntegrationAuthType.INTEGRATION_AUTH_TYPE_OAUTH,
    externalUserIdField: 'ouraUserId',
    configurableFields: [], // OAuth configured via flow
  },
  polar: {
    key: 'polar',
    displayName: 'Polar Flow',
    authType: IntegrationAuthType.INTEGRATION_AUTH_TYPE_OAUTH,
    externalUserIdField: 'polarUserId',
    configurableFields: [], // OAuth configured via flow
  },
  wahoo: {
    key: 'wahoo',
    displayName: 'Wahoo',
    authType: IntegrationAuthType.INTEGRATION_AUTH_TYPE_OAUTH,
    externalUserIdField: 'wahooUserId',
    configurableFields: [], // OAuth configured via flow
  },
};
