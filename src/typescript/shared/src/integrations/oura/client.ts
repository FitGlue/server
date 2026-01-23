import { createAuthenticatedClient, AuthenticatedClientOptions } from '../factory';
import { UserService } from '../../domain/services/user';
import type { paths } from './schema';

export function createOuraClient(userService: UserService, userId: string, options?: AuthenticatedClientOptions) {
  return createAuthenticatedClient<paths>(
    'https://api.ouraring.com',
    userService,
    userId,
    'oura',
    options
  );
}
