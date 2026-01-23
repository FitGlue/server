import * as crypto from 'crypto';
import { FrameworkContext } from '../index';
import { AuthStrategy, AuthResult } from '../auth';

export class ApiKeyStrategy implements AuthStrategy {
  name = 'api_key';

  private hashApiKey(token: string): string {
    return crypto.createHash('sha256').update(token).digest('hex');
  }

  // eslint-disable-next-line complexity
  async authenticate(req: { headers: Record<string, string | string[] | undefined>; query?: Record<string, string | string[] | undefined> }, ctx: FrameworkContext): Promise<AuthResult | null> {
    let token: string | undefined;

    // 1. Check Authorization Header (Bearer or Raw)
    const authHeaderRaw = req.headers['authorization'];
    const authHeader = Array.isArray(authHeaderRaw) ? authHeaderRaw[0] : authHeaderRaw;
    if (authHeader) {
      if (authHeader.startsWith('Bearer ')) {
        token = authHeader.split(' ')[1];
      } else {
        // Support raw key in Authorization header (e.g. Hevy webhook)
        token = authHeader;
      }
    }

    // 1b. Check X-Api-Key Header
    if (!token && req.headers['x-api-key']) {
      token = req.headers['x-api-key'] as string;
    }

    // 2. Check Query Parameter (key or api_key)
    if (!token && req.query) {
      token = (req.query.key as string) || (req.query.api_key as string);
    }

    if (token) {
      // Log partial key for debugging (first 4 chars)
      const maskedKey = token.substring(0, 4) + '...';
      ctx.logger.debug(`[ApiKeyStrategy] Attempting auth with token starting with ${maskedKey}`);
    }

    if (!token) {
      return null;
    }

    const hash = this.hashApiKey(token);

    try {
      const result = await ctx.services.apiKey.validate(hash);

      if (!result.valid) {
        ctx.logger.warn('API key not found or disabled');
        return null;
      }

      return {
        userId: result.userId ?? '',
        scopes: result.scopes || []
      };
    } catch (err: unknown) {
      const errorMessage = err instanceof Error ? err.message : String(err);
      ctx.logger.error('Error fetching API key', { error: errorMessage });
      return null;
    }
  }
}
