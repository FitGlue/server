import { AuthStrategy, AuthResult, FrameworkContext } from '@fitglue/shared';

/**
 * Wahoo webhook verification strategy.
 *
 * Wahoo Cloud API may send verification requests (GET) to validate the webhook endpoint.
 * This strategy bypasses auth for verification requests.
 */
export class WahooVerificationStrategy implements AuthStrategy {
  readonly name = 'wahoo_verification_auth';

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  async authenticate(req: any, ctx: FrameworkContext): Promise<AuthResult | null> {
    const { logger } = ctx;

    // Handle Verification Requests (GET method)
    // Wahoo may send GET requests to verify the webhook endpoint is accessible
    if (req.method === 'GET') {
      logger.info('Detected Wahoo verification request - bypassing auth');
      return { userId: 'system', scopes: [] };
    }

    return null;
  }
}
