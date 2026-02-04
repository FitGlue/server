// Don't static import the module under test, strictly require it in dynamic tests
// import { authOnCreate } from './index';

// Interface for Gen 1 Firebase Auth User Record
interface AuthUserRecord {
  uid: string;
  email?: string;
  displayName?: string;
}

// Mocks
jest.mock('firebase-admin/app', () => ({
  initializeApp: jest.fn(),
}));

jest.mock('firebase-admin/firestore', () => ({
  getFirestore: jest.fn(() => ({
    collection: jest.fn(),
  })),
}));

describe('Auth Hooks: authOnCreate', () => {
  let mockCreateUser: jest.Mock;
  let authOnCreate: (event: AuthUserRecord) => Promise<void>;

  beforeEach(() => {
    jest.resetModules(); // Critical for re-evaluating top-level const userService = new UserService(db)

    mockCreateUser = jest.fn().mockResolvedValue(undefined);

    jest.doMock('@fitglue/shared/dist/domain/services/user', () => ({
      UserService: jest.fn().mockImplementation(() => ({
        createUser: mockCreateUser
      }))
    }));

    // Re-require index to trigger top-level execution with new mocks
    const index = require('./index');
    authOnCreate = index.authOnCreate;
  });

  it('should call userService.createUser with the UID from the event', async () => {
    // Gen 1 Firebase Auth triggers pass the user record directly
    const mockEvent: AuthUserRecord = {
      uid: 'test-uid-123',
      email: 'test@example.com'
    };

    await authOnCreate(mockEvent);

    expect(mockCreateUser).toHaveBeenCalledWith('test-uid-123');
  });

  it('should handle events without a valid uid gracefully', async () => {
    // Event with no uid (edge case)
    const mockEvent = {} as AuthUserRecord;

    await authOnCreate(mockEvent);

    expect(mockCreateUser).not.toHaveBeenCalled();
  });
});
