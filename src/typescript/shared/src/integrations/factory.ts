import createClient from 'openapi-fetch';
import { UserService } from '../domain/services/user';

// Define a generic type for the client since we might not have the generated schema types imported here universally
// But actually createAuthenticatedClient needs to be generic or strict.
// Ideally usage: createAuthenticatedClient<paths>(...)

export function createAuthenticatedClient<Paths extends object>(
  baseUrl: string,
  userService: UserService,
  userId: string,
  provider: 'strava' | 'fitbit'
) {
  const retryFetch: typeof fetch = async (input, init) => {
    const token = await userService.getValidToken(userId, provider);

    // Inject Authorization Header
    const headers = new Headers(init?.headers);
    headers.set('Authorization', `Bearer ${token}`);

    const newInit = { ...init, headers };

    const response = await fetch(input, newInit);

    if (response.status === 401) {
      console.log(`[${provider}] 401 Unauthorized for user ${userId}. Retrying with force refresh.`);
      // Force Refresh
      const newToken = await userService.getValidToken(userId, provider, true);

      headers.set('Authorization', `Bearer ${newToken}`);
      const retryInit = { ...init, headers };

      return fetch(input, retryInit);
    }

    return response;
  };

  return createClient<Paths>({
    baseUrl,
    fetch: retryFetch, // Inject our wrapper
  });
}
