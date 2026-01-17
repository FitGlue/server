import { UserIntegrations } from './pb/user';

// Base definition
export interface BaseIntegrationDefinition {
  key: keyof UserIntegrations;
  displayName: string;
  type: 'basic' | 'oauth';
  // Fields that can be configured via CLI/API manually
  configurableFields: ConfigurableField[];
}

export interface ConfigurableField {
  field: string;
  name: string;
  type: 'string' | 'boolean' | 'password';
  required: boolean;
}

export interface OAuthIntegrationDefinition extends BaseIntegrationDefinition {
  type: 'oauth';
  // Map specific proto fields (athleteId, fitbitUserId) to a common 'externalUserId'
  // for generic display/handling
  externalUserIdField: string;
}

export interface BasicIntegrationDefinition extends BaseIntegrationDefinition {
  type: 'basic';
}

export type IntegrationDefinition = BasicIntegrationDefinition | OAuthIntegrationDefinition;

// Registry with strict typing
export const INTEGRATIONS: Record<keyof UserIntegrations, IntegrationDefinition> = {
  // Only real integrations, not auxiliary object methods
  hevy: {
    key: 'hevy',
    displayName: 'Hevy',
    type: 'basic',
    configurableFields: [{ field: 'apiKey', name: 'API Key', type: 'password', required: true }]
  },
  mock: {
    key: 'mock',
    displayName: 'Mock',
    type: 'basic',
    configurableFields: [{ field: 'enabled', name: 'Enabled', type: 'boolean', required: true }]
  },
  strava: {
    key: 'strava',
    displayName: 'Strava',
    type: 'oauth',
    externalUserIdField: 'athleteId',
    configurableFields: [] // OAuth usually configured via flow
  },
  fitbit: {
    key: 'fitbit',
    displayName: 'Fitbit',
    type: 'oauth',
    externalUserIdField: 'fitbitUserId',
    configurableFields: []
  },
  parkrun: {
    key: 'parkrun',
    displayName: 'Parkrun',
    type: 'basic',
    configurableFields: [{ field: 'athleteId', name: 'Barcode Number', type: 'string', required: true }]
  }
};
