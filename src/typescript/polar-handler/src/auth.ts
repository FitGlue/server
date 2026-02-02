// Module-level imports for smart pruning
import { AuthStrategy, AuthResult, FrameworkContext } from '@fitglue/shared/framework';


export class PolarVerificationStrategy implements AuthStrategy {
  readonly name = 'polar_verification_auth';

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  async authenticate(req: any, ctx: FrameworkContext): Promise<AuthResult | null> {
    const { logger } = ctx;

    // Handle Verification Requests (GET with signature param for webhook validation)
    // Polar sends a GET request to verify the webhook endpoint during setup
    if (req.method === 'GET' && req.query && req.query.signature) {
      logger.info('Detected Polar verification request - bypassing auth');
      return { userId: 'system', scopes: [] };
    }

    return null;
  }
}
