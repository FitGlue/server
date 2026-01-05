import { handler } from './index';
import { CloudEventPublisher } from '@fitglue/shared';

// Mock shared dependencies
jest.mock('@fitglue/shared', () => {
  const original = jest.requireActual('@fitglue/shared');
  return {
    ...original,
    CloudEventPublisher: jest.fn(),
  };
});

describe('activities-handler', () => {
  let req: any;
  let res: any;
  let ctx: any;
  let mockPublish: any;

  beforeEach(() => {
    mockPublish = jest.fn();
    (CloudEventPublisher as any).mockImplementation(() => ({
      publish: mockPublish
    }));

    req = {
      method: 'GET',
      body: {},
      query: {},
    };
    res = {
      status: jest.fn().mockReturnThis(),
      json: jest.fn(),
      send: jest.fn(),
    };
    ctx = {
      userId: 'user-1',
      logger: {
        info: jest.fn(),
        error: jest.fn(),
      },
      pubsub: {}, // Mock pubsub object (CloudEventPublisher uses it, but we mocked the class)
      services: {},
      stores: {
        activities: {
          countSynchronized: jest.fn(),
          listSynchronized: jest.fn(),
          getSynchronized: jest.fn(),
        },
      }
    };
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  describe('GET /', () => {
    it('returns 401 if no user', async () => {
      ctx.userId = undefined;
      await handler(req, res, ctx);
      expect(res.status).toHaveBeenCalledWith(401);
    });

    it('returns list of synchronized activities', async () => {
      ctx.stores.activities.listSynchronized.mockResolvedValue({
        activityId: 'a1',
        title: 'Activity 1',
        description: 'Description 1',
      });

      await handler(req, res, ctx);

      expect(res.status).toHaveBeenCalledWith(200);
      expect(res.json).toHaveBeenCalledWith({ activities: { activityId: "a1", description: "Description 1", title: "Activity 1" } });
    });

    it('handles errors', async () => {
      ctx.stores.activities.listSynchronized.mockRejectedValue(new Error('db error'));
      await handler(req, res, ctx);
      expect(res.status).toHaveBeenCalledWith(500);
    });
  });
});
