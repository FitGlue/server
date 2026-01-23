import { createAuthenticatedClient, AuthenticatedClientOptions } from '../factory';
import { UserService } from '../../domain/services/user';
import type { paths } from './schema';

export function createWahooClient(userService: UserService, userId: string, options?: AuthenticatedClientOptions) {
  return createAuthenticatedClient<paths>(
    'https://api.wahooligan.com',
    userService,
    userId,
    'wahoo',
    options
  );
}
