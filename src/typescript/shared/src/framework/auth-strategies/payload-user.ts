import { FrameworkContext } from '../index';
import { AuthStrategy, AuthResult } from '../auth';

/**
 * PayloadUserStrategy resolves the user from the webhook payload itself.
 * Used for webhooks like Fitbit that don't include per-user authentication,
 * but instead include a vendor-specific user ID in the payload.
 *
 * Requires a resolver function that maps vendor ID to our userId.
 */
export class PayloadUserStrategy implements AuthStrategy {
  name = 'payload_user';

  constructor(
    private resolveUser: (payload: any, ctx: FrameworkContext) => Promise<string | null>
  ) { }

  async authenticate(req: any, ctx: FrameworkContext): Promise<AuthResult | null> {
    const userId = await this.resolveUser(req.body, ctx);
    if (!userId) {
      return null;
    }
    return { userId, scopes: [] };
  }
}
