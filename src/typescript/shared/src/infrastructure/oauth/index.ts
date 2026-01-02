import * as admin from 'firebase-admin';
import * as crypto from 'crypto';
import { getSecret } from '../../infrastructure/secrets';

/**
 * Generate a signed OAuth state token containing the user ID
 * @param userId - The FitGlue user ID
 * @returns Base64-encoded signed state token
 */
export async function generateOAuthState(userId: string): Promise<string> {
  if (!process.env.OAUTH_STATE_SECRET && !process.env.GOOGLE_CLOUD_PROJECT) {
    throw new Error('Missing configuration: OAUTH_STATE_SECRET or GOOGLE_CLOUD_PROJECT environment variable is required');
  }
  const secret = process.env.OAUTH_STATE_SECRET || await getSecret(process.env.GOOGLE_CLOUD_PROJECT!, 'oauth-state-secret');
  const timestamp = Date.now();
  const expiresAt = timestamp + 10 * 60 * 1000; // 10 minutes

  const payload = JSON.stringify({ userId, expiresAt });
  const hmac = crypto.createHmac('sha256', secret);
  hmac.update(payload);
  const signature = hmac.digest('hex');

  const state = { payload, signature };
  return Buffer.from(JSON.stringify(state)).toString('base64url');
}

/**
 * Validate and extract user ID from OAuth state token
 * @param state - Base64-encoded state token
 * @returns User ID if valid, null otherwise
 */
export async function validateOAuthState(state: string): Promise<string | null> {
  // Fetch secret first - let this throw if configuration is missing (infrastructure error)
  if (!process.env.OAUTH_STATE_SECRET && !process.env.GOOGLE_CLOUD_PROJECT) {
    throw new Error('Missing configuration: OAUTH_STATE_SECRET or GOOGLE_CLOUD_PROJECT environment variable is required');
  }
  const secret = process.env.OAUTH_STATE_SECRET || await getSecret(process.env.GOOGLE_CLOUD_PROJECT!, 'oauth-state-secret');

  try {
    const decoded = JSON.parse(Buffer.from(state, 'base64url').toString());
    const { payload, signature } = decoded;

    // Verify signature
    const hmac = crypto.createHmac('sha256', secret);
    hmac.update(payload);
    const expectedSignature = hmac.digest('hex');

    if (signature !== expectedSignature) {
      console.warn('OAuth state signature mismatch');
      return null;
    }

    // Check expiration
    const { userId, expiresAt } = JSON.parse(payload);
    if (Date.now() > expiresAt) {
      console.warn('OAuth state expired');
      return null;
    }

    return userId;
  } catch (error) {
    console.warn('Error validating OAuth state:', error);
    return null;
  }
}

/**
 * Store OAuth tokens for a user integration
 */
export async function storeOAuthTokens(
  userId: string,
  provider: 'strava' | 'fitbit',
  tokens: {
    accessToken: string;
    refreshToken: string;
    expiresAt: Date;
    externalUserId: string;
  }
): Promise<void> {
  const { getUsersCollection, getIntegrationIdentitiesCollection } = await import('../../storage/firestore');

  const db = admin.firestore(); // Need db for batch
  const batch = db.batch();

  // Update user's integration
  // We use the typed collection to get the reference, but batch.update expects a reference.
  // Our Collections return standard Firestore references which work with batch.


  // Note: batch.update(ref, data) - data must be typed if ref is typed?
  // `getUsersCollection()` returns `CollectionReference<UserRecord>`.
  // `batch.update(userRef, ...)` expects Partial<UserRecord> or map?
  // Actually, standard Firestore batch.update works with flattened Dot-Notation keys regardless of typing if using `Precondition`?
  // But wait, `withConverter` types the `set` and `update` methods of the *DocumentReference*.
  // `batch.update(docRef, data)`: if docRef is typed, data should be typed.
  // HOWEVER, we are updating `integrations.strava.enabled`. This is a nested field.
  // `UserRecord` has `integrations` field.
  // Can we update partial nested fields on a typed reference with `batch`?
  // The type of `data` in `batch.update` is `UpdateData<T>`.
  // Dot notation like `'integrations.strava.enabled': true` is NOT strictly type-safe in standard TS Firestore types for nested objects unless the type definition supports it (e.g. `NestedUpdateFields<T>`).
  // The `firebase-admin` types are permissive for string keys.

  // To be safe and compliant with our goal (Type Safety), we should construct a partial object?
  // No, Firestore MERGE of nested fields requires dot notation or keys.
  // If we pass `{ integrations: { strava: { ... } } }` with `merge: true` (in Set), it merges.
  // But `update` is different.

  // Given we want to update specific fields without overwriting the whole integrations object:
  // We use the `userConverter` which maps camelCase -> snake_case.
  // BUT `batch.update` bypasses the converter?
  // NO. `withConverter` applies to `batch.set` and `batch.create`.
  // DOES IT apply to `batch.update`?
  // Documentation says: "If you use a custom converter ... the data argument ... must be consistent with the converter."
  // AND "The converter's toFirestore method is called ..."
  // OUR converter `toFirestore` maps `UserRecord` object to `snake_case` object.
  // It effectively expects a full `UserRecord` (or Partial).
  // It does NOT automatically map dot-notated keys like `'integrations.strava.enabled'`.
  // `toFirestore` implementation:
  // `return { user_id: model.userId, ... }`

  // If we pass `{'integrations.strava.enabled': true}` to `toFirestore`, it won't match `UserRecord` interface!
  // THIS IS A PROBLEM with `FirestoreDataConverter` and `update` with dot notation.
  // Typically `toFirestore` needs to handle partials or the SDK handles it?
  // SDK says `toFirestore(modelObject)`.
  // For `update`, SDK often supports `update(field, value, ...)` or `update(dataMap)`.
  // If `dataMap` is passed, SDK calls `toFirestore`?
  // Actually, for `update`, `toFirestore` signature is `toFirestore(modelObject): DocumentData` OR `toFirestore(modelObject, options): DocumentData`.
  // If we use dot notation strings, we essentially bypass the typed model structure.

  // DECISION: For specific field updates like this, we might need to cast to `any` or use untyped reference if we want dot notation, OR update our converter to handle it (hard).
  // OR: Use `set({ integrations: { strava: ... } }, { merge: true })`.
  // `set` with merge works with the converter if the converter can handle Partial<T>.
  // Our converter expects full `UserRecord`.
  // `{ integrations: ... }` is not a full UserRecord.

  // Current Implementation of `toFirestore` assumes `model` properties exist.
  // `model.userId` might be undefined in a Partial.

  // WORKAROUND: For this specific function `storeOAuthTokens`, we are writing legacy snake_case keys anyway in the existing code.
  // `['integrations.strava.enabled']: true`.
  // If we use the new `getUsersCollection()`, it expects CamelCase keys if we were using it "properly" but since we use dot notation...
  // I will UNWRAP the converter for this specific `batch` operation to ensure we write the keys we WANT (snake_case) without fighting the converter.
  // `getUsersCollection().withConverter(null)` returns untyped ref?
  // Yes.

  const untypedUserRef = getUsersCollection().doc(userId).withConverter(null);

  // Write using snake_case (Source of Truth: Proto/Go)
  batch.update(untypedUserRef, {
    [`integrations.${provider}.enabled`]: true,
    [`integrations.${provider}.access_token`]: tokens.accessToken,
    [`integrations.${provider}.refresh_token`]: tokens.refreshToken,
    [`integrations.${provider}.expires_at`]: tokens.expiresAt, // Timestamp conversion handles Date? Yes mostly.
    [`integrations.${provider}.${provider === 'strava' ? 'athlete_id' : 'fitbit_user_id'}`]: provider === 'strava' ? Number(tokens.externalUserId) : tokens.externalUserId,
  });

  // Create identity mapping
  // This one IS a full record (IntegrationIdentity), so we can use the typed collection.
  const identityRef = getIntegrationIdentitiesCollection(provider).doc(tokens.externalUserId);
  batch.set(identityRef, {
    userId,
    createdAt: new Date(),
  });

  await batch.commit();
}

/**
 * Refresh tokens with the provider using the refresh token
 */
export async function refreshOAuthToken(
  provider: 'strava' | 'fitbit',
  refreshToken: string
): Promise<{ accessToken: string; refreshToken: string; expiresAt: Date }> {
  const projectId = process.env.GOOGLE_CLOUD_PROJECT || '';
  // Note: getSecret falls back to env vars if project not set or local
  const clientId = await getSecret(projectId, `${provider}-client-id`);
  const clientSecret = await getSecret(projectId, `${provider}-client-secret`);

  let url = '';
  const body = new URLSearchParams();

  if (provider === 'strava') {
    url = 'https://www.strava.com/oauth/token';
    body.append('client_id', clientId);
    body.append('client_secret', clientSecret);
    body.append('grant_type', 'refresh_token');
    body.append('refresh_token', refreshToken);
  } else if (provider === 'fitbit') {
    url = 'https://api.fitbit.com/oauth2/token';
    body.append('grant_type', 'refresh_token');
    body.append('refresh_token', refreshToken);
  }

  const headers: Record<string, string> = {
    'Content-Type': 'application/x-www-form-urlencoded',
  };

  if (provider === 'fitbit') {
    const credentials = Buffer.from(`${clientId}:${clientSecret}`).toString('base64');
    headers['Authorization'] = `Basic ${credentials}`;
  }

  const response = await fetch(url, {
    method: 'POST',
    headers,
    body,
  });

  if (!response.ok) {
    const errorText = await response.text();
    throw new Error(`Token refresh failed for ${provider}: ${response.status} ${errorText}`);
  }

  const data = await response.json() as any;

  // Normalize response
  let accessToken = '';
  let newRefreshToken = '';
  let expiresAt = new Date();

  if (provider === 'strava') {
    accessToken = data.access_token;
    newRefreshToken = data.refresh_token;
    // Strava usually returns expires_at (seconds since epoch) and expires_in (seconds from now)
    if (data.expires_at) {
      expiresAt = new Date(data.expires_at * 1000);
    } else {
      expiresAt = new Date(Date.now() + data.expires_in * 1000);
    }
  } else if (provider === 'fitbit') {
    accessToken = data.access_token;
    newRefreshToken = data.refresh_token;
    expiresAt = new Date(Date.now() + data.expires_in * 1000);
  }

  return { accessToken, refreshToken: newRefreshToken, expiresAt };
}
