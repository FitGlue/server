// Module-level imports for smart pruning
import { AuthStrategy, AuthResult, FrameworkContext } from '@fitglue/shared/framework';
import * as crypto from 'crypto';

/**
 * GitHubWebhookAuthStrategy verifies incoming GitHub webhook requests
 * using HMAC-SHA256 signature validation (X-Hub-Signature-256 header).
 *
 * This strategy authenticates the webhook payload using a shared secret
 * stored in Secret Manager. The connector resolves the user separately
 * via PayloadUserStrategy.
 */
export class GitHubWebhookAuthStrategy implements AuthStrategy {
    readonly name = 'github_webhook_auth';

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    async authenticate(req: any, ctx: FrameworkContext): Promise<AuthResult | null> {
        const { logger } = ctx;

        // Only handle POST requests (webhook deliveries)
        if (req.method !== 'POST') {
            return null;
        }

        const signature = req.headers['x-hub-signature-256'];
        if (!signature) {
            logger.warn('GitHub webhook: missing X-Hub-Signature-256 header');
            return null;
        }

        // Load the shared webhook secret from Secret Manager
        const webhookSecret = process.env.GITHUB_WEBHOOK_SECRET;
        if (!webhookSecret) {
            logger.error('GitHub webhook: GITHUB_WEBHOOK_SECRET not configured');
            return null;
        }

        // Compute expected HMAC
        const rawBody = typeof req.rawBody === 'string'
            ? req.rawBody
            : (req.rawBody ? req.rawBody.toString('utf8') : JSON.stringify(req.body));

        const hmac = crypto.createHmac('sha256', webhookSecret);
        hmac.update(rawBody, 'utf8');
        const expectedSignature = `sha256=${hmac.digest('hex')}`;

        // Constant-time comparison to prevent timing attacks
        if (!crypto.timingSafeEqual(
            Buffer.from(signature as string),
            Buffer.from(expectedSignature)
        )) {
            logger.warn('GitHub webhook: HMAC signature mismatch');
            return null;
        }

        logger.info('GitHub webhook: HMAC signature verified');

        // Return system auth — the actual user resolution happens via PayloadUserStrategy
        return { userId: 'system', scopes: [] };
    }
}
