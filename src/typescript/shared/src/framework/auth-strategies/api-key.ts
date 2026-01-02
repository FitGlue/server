import * as crypto from 'crypto';
import * as admin from 'firebase-admin';
import { FrameworkContext } from '../index';
import { AuthStrategy, AuthResult } from '../auth';

export class ApiKeyStrategy implements AuthStrategy {
  name = 'api_key';

  async authenticate(req: any, ctx: FrameworkContext): Promise<AuthResult | null> {
    let token: string | undefined;

    // 1. Check Authorization Header (Bearer or Raw)
    const authHeader = req.headers['authorization'];
    if (authHeader) {
      if (authHeader.startsWith('Bearer ')) {
        token = authHeader.split(' ')[1];
      } else {
        // Support raw key in Authorization header (e.g. Hevy webhook)
        token = authHeader;
      }
    }

    // 1b. Check X-Api-Key Header
    if (!token && req.headers['x-api-key']) {
      token = req.headers['x-api-key'] as string;
    }

    // 2. Check Query Parameter (key or api_key)
    if (!token && req.query) {
      token = (req.query.key as string) || (req.query.api_key as string);
    }

    if (!token) {
      return null; // Not found in support locations
    }

    // High-entropy token (32 bytes), SHA-256 for fast O(1) lookup
    const hash = crypto.createHash('sha256').update(token).digest('hex');

    const { getIngressApiKeysCollection } = await import('../../storage/firestore');
    const docSnapshot = await getIngressApiKeysCollection().doc(hash).get();

    if (!docSnapshot.exists) {
      ctx.logger.warn(`Auth failed: API Key hash not found`, { hashPrefix: hash.substring(0, 8) });
      return null;
    }

    const record = docSnapshot.data()!;

    // Update lastUsed (fire-and-forget)
    getIngressApiKeysCollection().doc(hash).withConverter(null).update({
      last_used_at: admin.firestore.Timestamp.now()
    }).catch(err => ctx.logger.error('Failed to update lastUsed', { error: err }));

    return {
      userId: record.userId,
      scopes: record.scopes || []
    };
  }
}
