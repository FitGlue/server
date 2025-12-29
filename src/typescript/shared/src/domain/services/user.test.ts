import * as admin from 'firebase-admin';
import { UserService } from './user';
import { EnricherProviderType } from '../../types/pb/user';

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
      const enrichers = [{ providerType: EnricherProviderType.ENRICHER_PROVIDER_FITBIT_HEART_RATE, inputs: { priority: 'high' } }];
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
                { providerType: EnricherProviderType.ENRICHER_PROVIDER_FITBIT_HEART_RATE, inputs: { priority: 'high' } }
              ]
            })
          ])
        })
      });
    });
  });

  describe('removePipeline', () => {
    it('should remove a pipeline if it exists', async () => {
      const userId = 'user-1';
      const pipelineId = 'pipe-1';

      // Mock get returning generic object with data() method
      mockGet.mockResolvedValue({
        exists: true,
        data: () => ({
          pipelines: [{ id: 'pipe-1' }, { id: 'pipe-2' }]
        })
      });

      await userService.removePipeline(userId, pipelineId);

      expect(mockUpdate).toHaveBeenCalledWith({
        pipelines: [{ id: 'pipe-2' }]
      });
    });

    it('should throw if pipeline not found', async () => {
      const userId = 'user-1';
      mockGet.mockResolvedValue({
        exists: true,
        data: () => ({ pipelines: [] })
      });
      await expect(userService.removePipeline(userId, 'pipe-missing'))
        .rejects.toThrow(/not found/);
    });
  });

  describe('replacePipeline', () => {
    it('should replace an existing pipeline', async () => {
      const userId = 'user-1';
      const pipelineId = 'pipe-1';

      mockGet.mockResolvedValue({
        exists: true,
        data: () => ({
          pipelines: [{ id: 'pipe-1', source: 'OLD' }]
        })
      });

      await userService.replacePipeline(userId, pipelineId, 'NEW_SOURCE', [], ['new-dest']);

      expect(mockUpdate).toHaveBeenCalledWith({
        pipelines: [
          expect.objectContaining({
            id: pipelineId,
            source: 'NEW_SOURCE',
            destinations: ['new-dest']
          })
        ]
      });
    });
  });
});
