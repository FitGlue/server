import { createAuthenticatedClient } from '../factory';
import { UserService } from '../../domain/services/user';
import type { paths } from './schema';

export function createStravaClient(userService: UserService, userId: string) {
  return createAuthenticatedClient<paths>(
    'https://www.strava.com/api/v3',
    userService,
    userId,
    'strava'
  );
}
