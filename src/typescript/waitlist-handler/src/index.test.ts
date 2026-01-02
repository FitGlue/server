import { waitlistHandler } from './index';
import * as admin from 'firebase-admin';

// Jest Mock Factory
jest.mock('firebase-admin', () => {
  // Create stable mocks inside the factory
  const mockCreate = jest.fn().mockResolvedValue({ writeTime: 'mock-timestamp' });

  // mockDoc needs to return an object with .create
  const mockDoc = jest.fn(() => ({ create: mockCreate }));

  // mockCollection needs to return an object with .doc
  const mockCollection = jest.fn(() => ({ doc: mockDoc }));

  // firestoreFn returns the db object with .collection
  const firestoreFn = jest.fn(() => ({ collection: mockCollection }));

  // Attach static properties for FieldValue
  Object.assign(firestoreFn, {
    FieldValue: {
      serverTimestamp: jest.fn().mockReturnValue('mock-timestamp')
    },
    Timestamp: {
      now: jest.fn(() => ({ toDate: () => 'mock-timestamp' }))
    }
  });

  return {
    apps: [{ length: 0 }],
    initializeApp: jest.fn(),
    firestore: firestoreFn
  };
});

jest.mock('@fitglue/shared', () => ({
  storage: {
    getWaitlistCollection: () => require('firebase-admin').firestore().collection('waitlist')
  }
}));

describe('waitlistHandler', () => {
  let req: any;
  let res: any;

  // Accessors for our mocks
  let mockCollection: jest.Mock;
  let mockDoc: jest.Mock;
  let mockCreate: jest.Mock;

  beforeEach(() => {
    jest.clearAllMocks();

    // Silence logs
    jest.spyOn(console, 'log').mockImplementation(() => { });
    jest.spyOn(console, 'warn').mockImplementation(() => { });
    jest.spyOn(console, 'error').mockImplementation(() => { });

    // Traverse the mock graph to get references
    // 1. admin.firestore() -> db
    const db = admin.firestore();
    // 2. db.collection -> mockCollection
    mockCollection = db.collection as jest.Mock;

    // 3. invoke collection to get the doc builder
    const docBuilder = mockCollection('dummy');
    // 4. docBuilder.doc -> mockDoc
    mockDoc = docBuilder.doc as jest.Mock;

    // 5. invoke doc to get the doc ref
    const docRef = mockDoc('dummy');
    // 6. docRef.create -> mockCreate
    mockCreate = docRef.create as jest.Mock;

    // Reset Default Behavior
    mockCreate.mockResolvedValue({ writeTime: 'mock-timestamp' });

    // CLEAR CALL HISTORY from the setup traversal above
    mockCollection.mockClear();
    mockDoc.mockClear();
    mockCreate.mockClear();

    // Setup request/response mocks
    req = {
      method: 'POST',
      body: {},
      get: jest.fn()
    };
    res = {
      set: jest.fn(),
      status: jest.fn().mockReturnThis(),
      json: jest.fn().mockReturnThis(),
      send: jest.fn().mockReturnThis(),
    };
  });

  afterAll(() => {
    jest.restoreAllMocks();
  });

  it('should handle OPTIONS request (CORS)', async () => {
    req.method = 'OPTIONS';
    await waitlistHandler(req, res);

    expect(res.set).toHaveBeenCalledWith('Access-Control-Allow-Origin', '*');
    expect(res.status).toHaveBeenCalledWith(204);
    expect(res.send).toHaveBeenCalledWith('');
  });

  it('should reject non-POST requests', async () => {
    req.method = 'GET';
    await waitlistHandler(req, res);
    expect(res.status).toHaveBeenCalledWith(405);
  });

  it('should detect honeypot (spam) and return fake success without saving', async () => {
    req.body = { email: 'spammer@bot.com', website_url: 'http://spam.com' };

    await waitlistHandler(req, res);

    expect(mockDoc).not.toHaveBeenCalled();
    expect(mockCreate).not.toHaveBeenCalled();
    expect(res.status).toHaveBeenCalledWith(200);
    expect(res.json).toHaveBeenCalledWith(expect.objectContaining({ success: true }));
  });

  it('should reject invalid email', async () => {
    req.body = { email: 'not-an-email' };
    await waitlistHandler(req, res);

    expect(res.status).toHaveBeenCalledWith(400);
    expect(mockCreate).not.toHaveBeenCalled();
  });

  it('should save valid email using email as ID', async () => {
    req.body = { email: 'User@Example.com' }; // Mixed case to test normalization
    await waitlistHandler(req, res);

    // Check it used lowercase ID
    expect(mockCollection).toHaveBeenCalledWith('waitlist');
    expect(mockDoc).toHaveBeenCalledWith('user@example.com');

    expect(mockCreate).toHaveBeenCalledWith({
      email: 'user@example.com',
      source: 'web',
      createdAt: 'mock-timestamp',
      userAgent: expect.any(String),
      ip: expect.any(String)

    });
    expect(res.status).toHaveBeenCalledWith(200);
    expect(res.json).toHaveBeenCalledWith(expect.objectContaining({ success: true }));
  });

  it('should return 409 if create fails with ALREADY_EXISTS', async () => {
    req.body = { email: 'duplicate@example.com' };

    // Mock failure
    const error: any = new Error('ALREADY_EXISTS');
    error.code = 6;
    mockCreate.mockRejectedValue(error);

    await waitlistHandler(req, res);

    expect(mockCreate).toHaveBeenCalled(); // Tried to create

    expect(res.status).toHaveBeenCalledWith(409);
    expect(res.json).toHaveBeenCalledWith(expect.objectContaining({ error: "You're already on the waitlist" }));
  });
});
