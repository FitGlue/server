// Module-level imports for smart pruning
import { AuthStrategy, AuthResult, FrameworkContext } from '@fitglue/shared/framework';


export class StravaVerificationStrategy implements AuthStrategy {
  readonly name = 'strava_verification_auth';

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  async authenticate(req: any, ctx: FrameworkContext): Promise<AuthResult | null> {
    const { logger } = ctx;

    // Handle Verification Requests (GET with 'hub.challenge' param)
    // https://developers.strava.com/docs/webhooks/
    if (req.method === 'GET' && req.query && req.query['hub.challenge']) {
      logger.info('Detected Strava verification request - bypassing auth');
      return { userId: 'system', scopes: [] };
    }

    return null;
  }
}
