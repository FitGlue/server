import createClient from 'openapi-fetch';
import { UserService } from '../domain/services/user';

// Define a generic type for the client since we might not have the generated schema types imported here universally
// But actually createAuthenticatedClient needs to be generic or strict.
// Ideally usage: createAuthenticatedClient<paths>(...)

export interface AuthenticatedClientOptions {
  usageTracking?: boolean;
}

export function createAuthenticatedClient<Paths extends object>(
  baseUrl: string,
  userService: UserService,
  userId: string,
  provider: 'strava' | 'fitbit',
  options?: AuthenticatedClientOptions
) {
  const retryFetch: typeof fetch = async (input, init) => {
    const token = await userService.getValidToken(userId, provider);

    // Inject Authorization Header
    const headers = new Headers(init?.headers);
    headers.set('Authorization', `Bearer ${token}`);

    const newInit = { ...init, headers };

    const response = await fetch(input, newInit);

    // Track usage if enabled and request was successful
    if (options?.usageTracking && response.ok) {
      userService.updateLastUsed(userId, provider).catch(err => {
        console.warn(`[${provider}] Failed to update last_used_at for user ${userId}`, err);
      });
    }

    if (response.status === 401) {
      console.log(`[${provider}] 401 Unauthorized for user ${userId}. Retrying with force refresh.`);
      // Force Refresh
      const newToken = await userService.getValidToken(userId, provider, true);

      headers.set('Authorization', `Bearer ${newToken}`);
      const retryInit = { ...init, headers };

      const retryResponse = await fetch(input, retryInit);

      // Track usage on retry success too
      if (options?.usageTracking && retryResponse.ok) {
        userService.updateLastUsed(userId, provider).catch(err => {
          console.warn(`[${provider}] Failed to update last_used_at for user ${userId} (on retry)`, err);
        });
      }

      return retryResponse;
    }

    return response;
  };

  return createClient<Paths>({
    baseUrl,
    fetch: retryFetch, // Inject our wrapper
  });
}
