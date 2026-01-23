import { handler } from './index';
import * as admin from 'firebase-admin';
import { FrameworkContext } from '@fitglue/shared';

// Mock firebase-admin
jest.mock('firebase-admin', () => {
  const mockTransaction = {
    get: jest.fn(),
    update: jest.fn(),
    create: jest.fn(),
  };

  const mockFirestore = {
    collection: jest.fn().mockReturnThis(),
    doc: jest.fn().mockReturnThis(),
    get: jest.fn(),
    runTransaction: jest.fn((fn) => fn(mockTransaction)),
    Timestamp: {
      now: jest.fn().mockReturnValue({ seconds: 1234567890, nanoseconds: 0 }),
    },
    FieldValue: {
      increment: jest.fn((n) => ({ _increment: n })),
      arrayUnion: jest.fn((...args) => ({ _arrayUnion: args })),
    },
  };

  return {
    apps: [],
    initializeApp: jest.fn(),
    firestore: Object.assign(jest.fn(() => mockFirestore), {
      Timestamp: mockFirestore.Timestamp,
      FieldValue: mockFirestore.FieldValue,
    }),
  };
});

// Helper to create request objects
const createRequest = (overrides: Record<string, unknown> = {}): Parameters<typeof handler>[0] => ({
  method: 'POST',
  path: '/api/integration-request',
  body: {},
  headers: {},
  query: {},
  ...overrides,
} as Parameters<typeof handler>[0]);

// Helper to create mock context
const createContext = (): FrameworkContext => ({
  userId: undefined,
  logger: {
    info: jest.fn(),
    warn: jest.fn(),
    error: jest.fn(),
    debug: jest.fn(),
  },
  services: {
    user: {} as any,
    apiKey: {} as any,
    execution: {} as any,
    authorization: {} as any,
  },
  stores: {} as any,
  pubsub: {} as any,
  secrets: {} as any,
  executionId: 'test-123',
} as unknown as FrameworkContext);

describe('integration-request-handler', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  describe('POST /integration-request', () => {
    it('returns 400 if integration is missing', async () => {
      const req = createRequest({ body: {} });
      const ctx = createContext();

      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 400 }));
    });

    it('returns 400 if integration is too short', async () => {
      const req = createRequest({ body: { integration: 'x' } });
      const ctx = createContext();

      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 400 }));
    });

    it('silently accepts spam (honeypot filled)', async () => {
      const req = createRequest({ body: { integration: 'garmin', website_url: 'spam.com' } });
      const ctx = createContext();

      const result = await handler(req, ctx);

      expect(result).toEqual({ success: true, message: 'Thanks for your feedback!' });
    });

    it('normalizes integration name and stores request', async () => {
      const req = createRequest({ body: { integration: 'Garmin Connect' } });
      const ctx = createContext();

      const mockDoc = { exists: false };
      const mockFirestore = admin.firestore() as any;
      mockFirestore.runTransaction.mockImplementation((fn: any) => {
        return fn({
          get: jest.fn().mockResolvedValue(mockDoc),
          create: jest.fn(),
        });
      });

      const result = await handler(req, ctx);

      expect(result).toEqual(expect.objectContaining({
        success: true,
        canonicalName: 'garmin',
      }));
    });

    it('returns 500 on Firestore error', async () => {
      const req = createRequest({ body: { integration: 'garmin' } });
      const ctx = createContext();

      const mockFirestore = admin.firestore() as any;
      mockFirestore.runTransaction.mockRejectedValue(new Error('Firestore error'));

      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 500 }));
    });
  });

  describe('GET /integration-request (stats)', () => {
    it('returns sorted stats', async () => {
      const req = createRequest({ method: 'GET' });
      const ctx = createContext();

      const mockDocs = [
        { id: 'garmin', data: () => ({ count: 10, rawInputs: ['Garmin', 'Garmin Connect'] }) },
        { id: 'whoop', data: () => ({ count: 5, rawInputs: ['Whoop'] }) },
      ];
      const mockFirestore = admin.firestore() as any;
      mockFirestore.get.mockResolvedValue({
        forEach: (fn: (doc: any) => void) => mockDocs.forEach(fn),
      });

      const result = await handler(req, ctx);

      expect(result).toEqual({
        requests: [
          { name: 'garmin', count: 10, rawInputs: ['Garmin', 'Garmin Connect'] },
          { name: 'whoop', count: 5, rawInputs: ['Whoop'] },
        ],
      });
    });

    it('returns 500 on error', async () => {
      const req = createRequest({ method: 'GET' });
      const ctx = createContext();

      const mockFirestore = admin.firestore() as any;
      mockFirestore.get.mockRejectedValue(new Error('Firestore error'));

      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 500 }));
    });
  });

  describe('method validation', () => {
    it('returns 405 for unsupported methods', async () => {
      const req = createRequest({ method: 'PUT' });
      const ctx = createContext();

      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 405 }));
    });
  });
});
