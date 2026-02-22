# AegisRun TypeScript SDK

Official TypeScript/JavaScript SDK for AegisRun Agent Control Plane.

## Installation
```bash
npm install @aegisrun/sdk
```

## Quick Start
```typescript
import { AegisRunClient, Run } from '@aegisrun/sdk';

// Initialize client
const client = new AegisRunClient({
  baseUrl: 'http://localhost:8080',
  apiToken: 'your-api-token'
});

// Create a run
const run = await new Run({
  client,
  agentId: 'my-agent',
  policyRef: 'pol_abc123:v1',
  metadata: { environment: 'production' }
}).start();

// Execute steps with tool calls
await run.step('fetch_data', { current_task: 'data_collection' }, async (step) => {
  const result = await step.toolCall('http_request', {
    url: 'https://api.example.com/data',
    method: 'GET'
  });
  return result;
});

// End run
run.end({ status: 'success' });
```

## Features

- **Policy Enforcement**: All tool calls enforced by policy gateway
- **Type Safety**: Full TypeScript support with type definitions
- **Async/Await**: Modern promise-based API
- **Offline Mode**: Buffer events when server unavailable
- **Evidence Export**: Download tamper-evident evidence bundles

## Examples

See the `examples/` directory for complete examples.

## API Reference

See [docs.aegisrun.io](https://docs.aegisrun.io) for full documentation.
