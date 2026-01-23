import { createAuthenticatedClient, AuthenticatedClientOptions } from '../factory';
import { UserService } from '../../domain/services/user';
import type { paths } from './schema';

export function createSpotifyClient(userService: UserService, userId: string, options?: AuthenticatedClientOptions) {
  return createAuthenticatedClient<paths>(
    'https://api.spotify.com/v1',
    userService,
    userId,
    'spotify',
    options
  );
}
