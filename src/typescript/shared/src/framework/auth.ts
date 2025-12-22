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
        const authHeader = req.headers['authorization'];
        if (!authHeader || !authHeader.startsWith('Bearer ')) {
            return null; // Not this strategy or missing
        }

        const token = authHeader.split(' ')[1];
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
        }, { merge: true }).catch(err => ctx.logger.error('Failed to update lastUsed', { error: err}));

        return {
            userId: record.userId,
            scopes: record.scopes || []
        };
    }
}
