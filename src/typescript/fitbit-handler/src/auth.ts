// Module-level imports for smart pruning
import { AuthStrategy, AuthResult, FrameworkContext } from '@fitglue/shared/framework';


export class FitbitVerificationStrategy implements AuthStrategy {
  readonly name = 'fitbit_verification_auth';

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  async authenticate(req: any, ctx: FrameworkContext): Promise<AuthResult | null> {
    const { logger } = ctx;

    // Handle Verification Requests (GET with 'verify' param)
    if (req.method === 'GET' && req.query && req.query.verify) {
      logger.info('Detected Fitbit verification request - bypassing auth');
      return { userId: 'system', scopes: [] };
    }

    return null;
  }
}
