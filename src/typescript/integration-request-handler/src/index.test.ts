import { integrationRequestHandler } from './index';
import { Request, Response } from '@google-cloud/functions-framework';
import * as admin from 'firebase-admin';

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

// Helper to create mock request
const mockRequest = (overrides: Partial<Request> = {}): Request => ({
  method: 'POST',
  body: {},
  ...overrides,
} as Request);

// Helper to create mock response
const mockResponse = (): Response => {
  const res: Partial<Response> = {
    set: jest.fn().mockReturnThis(),
    status: jest.fn().mockReturnThis(),
    send: jest.fn().mockReturnThis(),
    json: jest.fn().mockReturnThis(),
  };
  return res as Response;
};

describe('integration-request-handler', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  describe('CORS handling', () => {
    it('sets CORS headers on all requests', async () => {
      const req = mockRequest({ method: 'POST', body: { integration: 'garmin' } });
      const res = mockResponse();

      const mockDoc = { exists: false };
      const mockFirestore = admin.firestore() as any;
      mockFirestore.runTransaction.mockImplementation((fn: any) => {
        return fn({
          get: jest.fn().mockResolvedValue(mockDoc),
          create: jest.fn(),
        });
      });

      await integrationRequestHandler(req, res);

      expect(res.set).toHaveBeenCalledWith('Access-Control-Allow-Origin', '*');
      expect(res.set).toHaveBeenCalledWith('Access-Control-Allow-Methods', 'POST, GET, OPTIONS');
    });

    it('responds to OPTIONS with 204', async () => {
      const req = mockRequest({ method: 'OPTIONS' });
      const res = mockResponse();

      await integrationRequestHandler(req, res);

      expect(res.status).toHaveBeenCalledWith(204);
      expect(res.send).toHaveBeenCalledWith('');
    });
  });

  describe('POST /integration-request', () => {
    it('returns 400 if integration is missing', async () => {
      const req = mockRequest({ body: {} });
      const res = mockResponse();

      await integrationRequestHandler(req, res);

      expect(res.status).toHaveBeenCalledWith(400);
      expect(res.json).toHaveBeenCalledWith({ error: 'Please provide a valid integration name.' });
    });

    it('returns 400 if integration is too short', async () => {
      const req = mockRequest({ body: { integration: 'x' } });
      const res = mockResponse();

      await integrationRequestHandler(req, res);

      expect(res.status).toHaveBeenCalledWith(400);
    });

    it('silently accepts spam (honeypot filled)', async () => {
      const req = mockRequest({ body: { integration: 'garmin', website_url: 'spam.com' } });
      const res = mockResponse();

      await integrationRequestHandler(req, res);

      expect(res.status).toHaveBeenCalledWith(200);
      expect(res.json).toHaveBeenCalledWith({ success: true, message: 'Thanks for your feedback!' });
    });

    it('normalizes integration name and stores request', async () => {
      const req = mockRequest({ body: { integration: 'Garmin Connect' } });
      const res = mockResponse();

      const mockDoc = { exists: false };
      const mockFirestore = admin.firestore() as any;
      mockFirestore.runTransaction.mockImplementation((fn: any) => {
        return fn({
          get: jest.fn().mockResolvedValue(mockDoc),
          create: jest.fn(),
        });
      });

      await integrationRequestHandler(req, res);

      expect(res.status).toHaveBeenCalledWith(200);
      expect(res.json).toHaveBeenCalledWith(
        expect.objectContaining({
          success: true,
          canonicalName: 'garmin',
        })
      );
    });

    it('returns 500 on Firestore error', async () => {
      const req = mockRequest({ body: { integration: 'garmin' } });
      const res = mockResponse();

      const mockFirestore = admin.firestore() as any;
      mockFirestore.runTransaction.mockRejectedValue(new Error('Firestore error'));

      await integrationRequestHandler(req, res);

      expect(res.status).toHaveBeenCalledWith(500);
      expect(res.json).toHaveBeenCalledWith({ error: 'Internal Server Error' });
    });
  });

  describe('GET /integration-request (stats)', () => {
    it('returns sorted stats', async () => {
      const req = mockRequest({ method: 'GET' });
      const res = mockResponse();

      const mockDocs = [
        { id: 'garmin', data: () => ({ count: 10, rawInputs: ['Garmin', 'Garmin Connect'] }) },
        { id: 'whoop', data: () => ({ count: 5, rawInputs: ['Whoop'] }) },
      ];
      const mockFirestore = admin.firestore() as any;
      mockFirestore.get.mockResolvedValue({
        forEach: (fn: (doc: any) => void) => mockDocs.forEach(fn),
      });

      await integrationRequestHandler(req, res);

      expect(res.status).toHaveBeenCalledWith(200);
      expect(res.json).toHaveBeenCalledWith({
        requests: [
          { name: 'garmin', count: 10, rawInputs: ['Garmin', 'Garmin Connect'] },
          { name: 'whoop', count: 5, rawInputs: ['Whoop'] },
        ],
      });
    });

    it('returns 500 on error', async () => {
      const req = mockRequest({ method: 'GET' });
      const res = mockResponse();

      const mockFirestore = admin.firestore() as any;
      mockFirestore.get.mockRejectedValue(new Error('Firestore error'));

      await integrationRequestHandler(req, res);

      expect(res.status).toHaveBeenCalledWith(500);
    });
  });

  describe('method validation', () => {
    it('returns 405 for unsupported methods', async () => {
      const req = mockRequest({ method: 'PUT' });
      const res = mockResponse();

      await integrationRequestHandler(req, res);

      expect(res.status).toHaveBeenCalledWith(405);
      expect(res.send).toHaveBeenCalledWith('Method Not Allowed');
    });
  });
});
