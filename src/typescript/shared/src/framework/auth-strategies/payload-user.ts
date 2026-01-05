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
    private resolveUser: (payload: unknown, ctx: FrameworkContext) => Promise<string | null>
  ) { }

  async authenticate(req: unknown, ctx: FrameworkContext): Promise<AuthResult | null> {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const userId = await this.resolveUser((req as any).body, ctx);
    if (!userId) {
      return null;
    }
    return { userId, scopes: [] };
  }
}
