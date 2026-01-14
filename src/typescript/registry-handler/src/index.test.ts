import { handler } from './index';
import { Request, Response } from 'express';
import { FrameworkContext } from '@fitglue/shared';

// Mock getRegistry
jest.mock('@fitglue/shared', () => ({
  ...jest.requireActual('@fitglue/shared'),
  getRegistry: jest.fn(() => ({
    sources: [
      { id: 'hevy', name: 'Hevy', enabled: true },
      { id: 'mock', name: 'Mock', enabled: false },
    ],
    enrichers: [
      { id: 'ai-description', name: 'AI Description', enabled: true },
    ],
    destinations: [
      { id: 'strava', name: 'Strava', enabled: true },
    ],
    integrations: [
      { id: 'hevy', name: 'Hevy', enabled: true },
      { id: 'fitbit', name: 'Fitbit', enabled: true },
    ],
  })),
  createCloudFunction: jest.fn((handler) => handler),
}));

describe('registry-handler', () => {
  let mockReq: Partial<Request>;
  let mockRes: Partial<Response>;
  let mockCtx: FrameworkContext;

  beforeEach(() => {
    mockReq = {
      method: 'GET',
      query: {},
    };
    mockRes = {
      status: jest.fn().mockReturnThis(),
      json: jest.fn(),
      set: jest.fn(),
    };
    mockCtx = {
      logger: {
        info: jest.fn(),
        error: jest.fn(),
        warn: jest.fn(),
        debug: jest.fn(),
      },
    } as unknown as FrameworkContext;
  });

  it('returns 405 for non-GET requests', async () => {
    mockReq.method = 'POST';
    await handler(mockReq as Request, mockRes as Response, mockCtx);
    expect(mockRes.status).toHaveBeenCalledWith(405);
    expect(mockRes.json).toHaveBeenCalledWith({ error: 'Method Not Allowed' });
  });

  it('returns filtered registry (enabled only)', async () => {
    await handler(mockReq as Request, mockRes as Response, mockCtx);

    expect(mockRes.status).toHaveBeenCalledWith(200);
    expect(mockRes.set).toHaveBeenCalledWith('Cache-Control', 'public, max-age=300');

    const response = (mockRes.json as jest.Mock).mock.calls[0][0];
    expect(response.sources).toHaveLength(1); // Only enabled
    expect(response.sources[0].id).toBe('hevy');
    expect(response.enrichers).toHaveLength(1);
    expect(response.destinations).toHaveLength(1);
    expect(response.integrations).toHaveLength(2);
  });

  it('returns all plugins when showAll=true', async () => {
    mockReq.query = { showAll: 'true' };
    await handler(mockReq as Request, mockRes as Response, mockCtx);

    expect(mockRes.status).toHaveBeenCalledWith(200);

    const response = (mockRes.json as jest.Mock).mock.calls[0][0];
    expect(response.sources).toHaveLength(2); // Includes disabled
    expect(response.sources.find((s: { id: string }) => s.id === 'mock')).toBeDefined();
  });

  it('logs registry response', async () => {
    await handler(mockReq as Request, mockRes as Response, mockCtx);

    expect(mockCtx.logger.info).toHaveBeenCalledWith('Plugin registry returned', {
      sourceCount: 1,
      enricherCount: 1,
      destinationCount: 1,
      integrationCount: 2,
    });
  });
});
