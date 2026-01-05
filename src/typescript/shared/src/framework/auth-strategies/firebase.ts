import { AuthStrategy, AuthResult } from '../auth';
import { FrameworkContext } from '../index';
import * as admin from 'firebase-admin';

export class FirebaseAuthStrategy implements AuthStrategy {
  name = 'firebase';

  async authenticate(req: { headers?: Record<string, string | string[] | undefined> }, ctx: FrameworkContext): Promise<AuthResult | null> {
    const authHeaderRaw = req.headers?.authorization;
    const authHeader = Array.isArray(authHeaderRaw) ? authHeaderRaw[0] : authHeaderRaw;
    if (!authHeader || !authHeader.startsWith('Bearer ')) {
      return null;
    }

    const token = authHeader.split('Bearer ')[1];
    try {
      if (!admin.apps.length) {
        admin.initializeApp();
      }
      const decoded = await admin.auth().verifyIdToken(token);
      return {
        userId: decoded.uid,
        scopes: [], // We could map custom claims to scopes if needed
      };
    } catch (e) {
      ctx.logger.warn('FirebaseAuthStrategy: Verification failed', { error: e });
      return null;
    }
  }
}
