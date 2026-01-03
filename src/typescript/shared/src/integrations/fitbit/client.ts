import { createAuthenticatedClient, AuthenticatedClientOptions } from '../factory';
import { UserService } from '../../domain/services/user';
import type { paths } from "./schema";

export function createFitbitClient(userService: UserService, userId: string, options?: AuthenticatedClientOptions) {
  return createAuthenticatedClient<paths>(
    'https://api.fitbit.com',
    userService,
    userId,
    'fitbit',
    options
  );
}
