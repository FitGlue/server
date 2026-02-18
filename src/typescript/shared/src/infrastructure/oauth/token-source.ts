import { UserStore } from '../../storage/firestore';
import { UserIntegrations } from '../../types/pb/user';

export interface Token {
  accessToken: string;
  refreshToken: string;
  expiresAt: Date;
}

export interface TokenSource {
  getToken(forceRefresh?: boolean): Promise<Token>;
}

/** All supported OAuth providers */
export type OAuthProvider = 'strava' | 'fitbit' | 'oura' | 'polar' | 'spotify' | 'wahoo' | 'trainingpeaks' | 'github';

/**
 * Providers whose OAuth tokens don't expire and don't use refresh tokens.
 * For these providers we skip the refresh-token requirement and never
 * attempt token refresh.
 */
const NON_REFRESHABLE_PROVIDERS: ReadonlySet<OAuthProvider> = new Set(['github']);

/** Provider-specific configuration for token refresh */
interface ProviderConfig {
  tokenUrl: string;
  /** If true, use Basic Auth header instead of body params */
  useBasicAuth?: boolean;
  /** For Polar, we need different content types */
  contentType?: string;
}

const PROVIDER_CONFIGS: Record<OAuthProvider, ProviderConfig> = {
  strava: {
    tokenUrl: 'https://www.strava.com/oauth/token',
  },
  fitbit: {
    tokenUrl: 'https://api.fitbit.com/oauth2/token',
    useBasicAuth: true,
  },
  oura: {
    tokenUrl: 'https://api.ouraring.com/oauth/token',
  },
  polar: {
    tokenUrl: 'https://polarremote.com/v2/oauth2/token',
    useBasicAuth: true,
    contentType: 'application/x-www-form-urlencoded',
  },
  spotify: {
    tokenUrl: 'https://accounts.spotify.com/api/token',
    useBasicAuth: true,
  },
  wahoo: {
    tokenUrl: 'https://api.wahooligan.com/oauth/token',
  },
  trainingpeaks: {
    tokenUrl: 'https://oauth.trainingpeaks.com/token',
  },
  github: {
    tokenUrl: 'https://github.com/login/oauth/access_token',
  },
};

export class FirestoreTokenSource implements TokenSource {
  constructor(
    private userStore: UserStore,
    private userId: string,
    private provider: OAuthProvider
  ) { }

  private getIntegration(integrations: UserIntegrations): {
    accessToken?: string;
    refreshToken?: string;
    expiresAt?: Date;
    enabled?: boolean;
  } | undefined {
    // Access integration dynamically by provider key
    return (integrations as Record<string, unknown>)[this.provider] as {
      accessToken?: string;
      refreshToken?: string;
      expiresAt?: Date;
      enabled?: boolean;
    } | undefined;
  }

  async getToken(forceRefresh = false): Promise<Token> {
    // 1. Fetch current user from Firestore
    const user = await this.userStore.get(this.userId);
    if (!user) {
      throw new Error(`User ${this.userId} not found`);
    }

    if (!user.integrations) {
      throw new Error(`User ${this.userId} has no integrations`);
    }

    const integration = this.getIntegration(user.integrations);

    if (!integration || !integration.enabled) {
      throw new Error(`${this.provider} integration not enabled for user ${this.userId}`);
    }

    const accessToken = integration.accessToken;
    const refreshToken = integration.refreshToken;
    const expiresAtRaw = integration.expiresAt;

    if (!accessToken) {
      throw new Error(`Missing access token for ${this.provider}`);
    }

    // Non-refreshable providers (e.g. GitHub): return the token directly,
    // no refresh token or expiry check needed.
    if (NON_REFRESHABLE_PROVIDERS.has(this.provider)) {
      return {
        accessToken,
        refreshToken: '',
        expiresAt: expiresAtRaw || new Date('2099-12-31'),
      };
    }

    if (!refreshToken) {
      throw new Error(`Missing refresh token for ${this.provider}`);
    }

    // Check Expiry
    // converters.ts guarantees this is a Date or undefined
    const expiresAt = expiresAtRaw || new Date(0);

    const now = new Date();
    const isExpired = expiresAt <= now;

    // Proactive refresh window (1 minute) to match Go implementation
    const isExpiringSoon = expiresAt.getTime() - now.getTime() < 60 * 1000;

    if (forceRefresh || isExpired || isExpiringSoon) {
      return this.refreshTokenFlow(refreshToken);
    }

    return {
      accessToken,
      refreshToken,
      expiresAt
    };
  }

  private async refreshTokenFlow(refreshToken: string): Promise<Token> {
    try {
      const envVarPrefix = this.provider.toUpperCase().replace(/-/g, '_');
      const clientId = process.env[`${envVarPrefix}_CLIENT_ID`];
      const clientSecret = process.env[`${envVarPrefix}_CLIENT_SECRET`];

      if (!clientId || !clientSecret) {
        throw new Error(`Missing OAuth credentials for ${this.provider}`);
      }

      const config = PROVIDER_CONFIGS[this.provider];
      const headers: HeadersInit = {
        'Content-Type': config.contentType || 'application/x-www-form-urlencoded'
      };

      let body: URLSearchParams;

      if (config.useBasicAuth) {
        // Use Basic Auth header
        const basicAuth = Buffer.from(`${clientId}:${clientSecret}`).toString('base64');
        headers['Authorization'] = `Basic ${basicAuth}`;

        body = new URLSearchParams({
          grant_type: 'refresh_token',
          refresh_token: refreshToken
        });
      } else {
        // Include credentials in body
        body = new URLSearchParams({
          client_id: clientId,
          client_secret: clientSecret,
          grant_type: 'refresh_token',
          refresh_token: refreshToken
        });
      }

      const response = await fetch(config.tokenUrl, {
        method: 'POST',
        headers: headers,
        body: body
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`Refresh failed with status ${response.status}: ${errorText}`);
      }

      const data = await response.json();

      // Normalize response
      const newAccessToken = data.access_token;
      const newRefreshToken = data.refresh_token || refreshToken; // Some providers don't rotate refresh tokens
      const expiresIn = data.expires_in; // Seconds

      if (!newAccessToken) {
        throw new Error(`Invalid refresh response from ${this.provider}`);
      }

      const newExpiresAt = new Date(Date.now() + expiresIn * 1000);

      // Fetch current state to merge
      const user = await this.userStore.get(this.userId);
      if (!user || !user.integrations) throw new Error('User lost during refresh');

      // Update Firestore
      const integrationData = (user.integrations as Record<string, unknown>)[this.provider];
      if (!integrationData) {
        throw new Error(`Integration ${this.provider} not found for user ${this.userId} while attempting to update`);
      }
      await this.userStore.setIntegration(this.userId, this.provider as keyof UserIntegrations, {
        ...(integrationData as Record<string, unknown>),
        accessToken: newAccessToken,
        refreshToken: newRefreshToken,
        expiresAt: newExpiresAt
      } as UserIntegrations[keyof UserIntegrations]);

      return {
        accessToken: newAccessToken,
        refreshToken: newRefreshToken,
        expiresAt: newExpiresAt
      };

    } catch (error) {
      console.error(`[${this.provider}] Token refresh failed for user ${this.userId}`, error);
      throw error;
    }
  }
}

