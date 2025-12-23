import * as crypto from 'crypto';
import * as admin from 'firebase-admin';
import { FrameworkContext } from './index';
import { ApiKeyRecord } from '../types/pb/auth';

export interface AuthResult {
    userId: string;
    scopes: string[];
}

export interface AuthStrategy {
    name: string;
    authenticate(req: any, ctx: FrameworkContext): Promise<AuthResult | null>;
}

export class ApiKeyStrategy implements AuthStrategy {
    name = 'api_key';

    async authenticate(req: any, ctx: FrameworkContext): Promise<AuthResult | null> {
        let token: string | undefined;

        // 1. Check Authorization Header (Bearer)
        const authHeader = req.headers['authorization'];
        if (authHeader && authHeader.startsWith('Bearer ')) {
            token = authHeader.split(' ')[1];
        }

        // 1b. Check X-Api-Key Header
        if (!token && req.headers['x-api-key']) {
            token = req.headers['x-api-key'] as string;
        }

        // 2. Check Query Parameter (key or api_key)
        // Note: Functions Framework / Express populates req.query
        if (!token && req.query) {
            token = (req.query.key as string) || (req.query.api_key as string);
        }

        if (!token) {
            return null; // Not found in support locations
        }

        // High-entropy token (32 bytes), SHA-256 for fast O(1) lookup
        const hash = crypto.createHash('sha256').update(token).digest('hex');

        const docSnapshot = await ctx.db.collection('ingress_api_keys').doc(hash).get();

        if (!docSnapshot.exists) {
            ctx.logger.warn(`Auth failed: API Key hash not found`, { hashPrefix: hash.substring(0, 8) });
            return null;
        }

        const record = docSnapshot.data() as ApiKeyRecord;

        // Update lastUsed (fire-and-forget to avoid latency)
        // using set/merge because protobuf types might not map 1:1 to firestore update paths easily
        ctx.db.collection('ingress_api_keys').doc(hash).set({
            lastUsedAt: admin.firestore.Timestamp.now()
        }, { merge: true }).catch(err => ctx.logger.error('Failed to update lastUsed', { error: err }));

        return {
            userId: record.userId,
            scopes: record.scopes || []
        };
    }
}
