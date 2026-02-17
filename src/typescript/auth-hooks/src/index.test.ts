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
  Timestamp: {
    now: jest.fn(() => ({ seconds: 1700000000, nanoseconds: 0 })),
  },
}));

describe('Auth Hooks: authOnCreate', () => {
  let mockCreateUser: jest.Mock;
  let mockShowcaseSet: jest.Mock;
  let authOnCreate: (event: AuthUserRecord) => Promise<void>;

  beforeEach(() => {
    jest.resetModules(); // Critical for re-evaluating top-level const userService = new UserService(db)

    mockCreateUser = jest.fn().mockResolvedValue(undefined);
    mockShowcaseSet = jest.fn().mockResolvedValue(undefined);

    jest.doMock('@fitglue/shared/dist/domain/services/user', () => ({
      UserService: jest.fn().mockImplementation(() => ({
        createUser: mockCreateUser
      }))
    }));

    jest.doMock('@fitglue/shared/dist/storage/firestore/showcase-profile-store', () => ({
      ShowcaseProfileStore: jest.fn().mockImplementation(() => ({
        set: mockShowcaseSet
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

  it('should create a base showcase profile on registration', async () => {
    const mockEvent: AuthUserRecord = {
      uid: 'AbCdEf12345678',
      email: 'test@example.com'
    };

    await authOnCreate(mockEvent);

    expect(mockShowcaseSet).toHaveBeenCalledWith(
      'abcdef12', // first 8 chars, lowercased
      expect.objectContaining({
        slug: 'abcdef12',
        user_id: 'AbCdEf12345678',
        display_name: '',
        entries: [],
        total_activities: 0,
        visible: false,
      })
    );
  });

  it('should not fail if showcase profile creation fails', async () => {
    mockShowcaseSet.mockRejectedValue(new Error('Firestore write failed'));

    const mockEvent: AuthUserRecord = {
      uid: 'test-uid-456',
      email: 'test@example.com'
    };

    // Should not throw — showcase creation is non-fatal
    await expect(authOnCreate(mockEvent)).resolves.not.toThrow();

    // User creation should still have succeeded
    expect(mockCreateUser).toHaveBeenCalledWith('test-uid-456');
  });
});
