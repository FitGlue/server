import * as admin from 'firebase-admin';
import * as converters from './converters';

// Wrapper to ensure we use the default app instance
const db = () => admin.firestore();

export const getUsersCollection = () =>
  db().collection('users').withConverter(converters.userConverter);

export const getWaitlistCollection = () =>
  db().collection('waitlist').withConverter(converters.waitlistConverter);

export const getIngressApiKeysCollection = () =>
  db().collection('ingress_api_keys').withConverter(converters.apiKeyConverter);

export const getExecutionsCollection = () =>
  db().collection('executions').withConverter(converters.executionConverter);

/**
 * Access integration identity mapping: integrations/{provider}/ids/{externalId}
 */
export const getIntegrationIdentitiesCollection = (provider: string) =>
  db().collection('integrations').doc(provider).collection('ids').withConverter(converters.integrationIdentityConverter);

/**
 * Access user's processed/raw activities: users/{userId}/raw_activities
 */
export const getRawActivitiesCollection = (userId: string) =>
  getUsersCollection().doc(userId).collection('raw_activities').withConverter(converters.processedActivityConverter);
