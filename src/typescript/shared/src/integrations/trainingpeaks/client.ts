import { createAuthenticatedClient, AuthenticatedClientOptions } from '../factory';
import { UserService } from '../../domain/services/user';
import type { paths } from './schema';

export function createTrainingPeaksClient(userService: UserService, userId: string, options?: AuthenticatedClientOptions) {
  return createAuthenticatedClient<paths>(
    'https://api.trainingpeaks.com',
    userService,
    userId,
    'trainingpeaks',
    options
  );
}
