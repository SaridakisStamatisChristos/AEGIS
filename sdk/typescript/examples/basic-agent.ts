/**
 * Basic agent example using AegisRun SDK
 */

import { AegisRunClient, Run } from '../src';

async function main() {
  // Initialize client
  const client = new AegisRunClient({
    baseUrl: 'http://localhost:8080',
  });

  // Create run
  const run = await new Run({
    client,
    policyId: 'demo_policy',
    policyVersion: 'v1',
    metadata: {
      agent_name: 'demo_agent',
      agentId: 'demo_agent',
      environment: 'development',
    },
  }).start();

  console.log(`Run created: ${run.runId}`);

  // Step 1: Gather information
  const info = await run.step(
    'gather_information',
    { phase: 'research' },
    async (step) => {
      console.log('Step 1: Gathering information...');
      const result = await step.toolCall('http_request', {
        url: 'https://api.github.com/repos/anthropics/anthropic-sdk-typescript',
        method: 'GET',
      });
      return result;
    }
  );

  console.log(`Gathered info: ${info.status_code}`);

  // Step 2: Process data
  const processed = await run.step(
    'process_data',
    { phase: 'processing', data: info },
    async (step) => {
      console.log('Step 2: Processing data...');
      // Simulate processing
      return { processed: true, records: 42 };
    }
  );

  console.log(`Processed: ${JSON.stringify(processed)}`);

  // End run
  run.end({ status: 'success', result: processed });

  console.log(`Run completed successfully: ${run.runId}`);
  console.log(`Steps: ${run.counters.steps}`);
  console.log(`Tool calls: ${run.counters.tool_calls}`);
}

main().catch(console.error);
