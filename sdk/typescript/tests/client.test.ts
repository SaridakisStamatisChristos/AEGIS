/**
 * Tests for AegisRun client
 */

import { AegisRunClient } from '../src/client';

describe('AegisRunClient', () => {
  it('should initialize with default config', () => {
    const client = new AegisRunClient();
    expect(client).toBeDefined();
  });

  it('should initialize with custom config', () => {
    const client = new AegisRunClient({
      baseUrl: 'http://localhost:8080',
      apiToken: 'test-token',
    });
    expect(client).toBeDefined();
  });

  // Additional tests would go here
});
