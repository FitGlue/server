
import { createHevyClient } from './client';

// Mock dependencies
jest.mock('openapi-fetch', () => jest.fn(() => ({
  use: jest.fn()
})));

describe('createHevyClient', () => {
  it('should create a client with API key', () => {
    const client = createHevyClient({ apiKey: 'test-key' });
    expect(client).toBeDefined();
  });
});
