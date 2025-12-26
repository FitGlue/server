import * as admin from 'firebase-admin';
import { UserService } from './user';

// Mock specific firestore methods
const mockUpdate = jest.fn();
const mockSet = jest.fn();
const mockGet = jest.fn();
const mockDoc = jest.fn(() => ({
  update: mockUpdate,
  set: mockSet,
  get: mockGet,
}));
const mockCollection = jest.fn(() => ({
  doc: mockDoc,
}));

jest.mock('firebase-admin', () => {
  return {
    firestore: Object.assign(jest.fn(() => ({
      collection: mockCollection,
    })), {
      FieldValue: {
        arrayUnion: jest.fn((...args) => ({ _method: 'arrayUnion', args })),
      },
      Timestamp: {
        now: jest.fn(() => ({ seconds: 12345, nanoseconds: 0 })),
        fromMillis: jest.fn((ms) => ({ seconds: ms / 1000, nanoseconds: 0 })),
      }
    }),
  };
});

describe('UserService', () => {
  let userService: UserService;
  let db: admin.firestore.Firestore;

  beforeEach(() => {
    jest.clearAllMocks();
    db = admin.firestore();
    userService = new UserService(db);
  });

  describe('addPipeline', () => {
    it('should add a pipeline to the user document', async () => {
      const userId = 'test-user-id';
      const source = 'SOURCE_HEVY';
      const enrichers = [{ name: 'fitbit-hr', inputs: { priority: 'high' } }];
      const destinations = ['strava'];

      const pipelineId = await userService.addPipeline(userId, source, enrichers, destinations);

      expect(mockCollection).toHaveBeenCalledWith('users');
      expect(mockDoc).toHaveBeenCalledWith(userId);

      expect(mockUpdate).toHaveBeenCalledWith({
        pipelines: expect.objectContaining({
          _method: 'arrayUnion',
          args: expect.arrayContaining([
            expect.objectContaining({
              id: pipelineId,
              source: source,
              destinations: destinations,
              enrichers: [
                { name: 'fitbit-hr', inputs: { priority: 'high' } }
              ]
            })
          ])
        })
      });
    });
  });
});
