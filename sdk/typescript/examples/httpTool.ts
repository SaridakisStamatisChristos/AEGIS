/**
 * HTTP Tool Usage Example with Error Handling
 * 
 * Demonstrates how to use the AegisRun SDK for HTTP tool calls
 * with proper error handling for blocked, redacted, and allowed requests.
 */

import { AegisRunClient } from '../src/client';
import { Run } from '../src/run';
import { ToolCallBlockedError, ToolCallExecutionError } from '../src/toolCall';

async function main() {
  const client = new AegisRunClient({
    baseUrl: 'http://localhost:8080',
    // apiToken: process.env.AEGIS_API_TOKEN,
  });

  const run = await new Run({
    client,
    agentId: 'http-demo-agent',
    policyRef: 'http-demo-policy:v1',
    metadata: {
      demo: 'http_tools',
      purpose: 'demonstrate_http_policy_enforcement',
    },
  }).start();

  console.log(`\n=== HTTP Tool Demo ===`);
  console.log(`Run ID: ${run.runId}\n`);

  // Example 1: Allowed request to public API
  console.log('--- Test 1: Allowed Public Request ---');
  await run.step('public_api_request', { phase: 'testing_allowed' }, async (step) => {
    try {
      const result = await step.toolCall('http_request', {
        url: 'https://api.github.com/zen',
        method: 'GET',
        headers: {
          'Accept': 'application/json',
          'User-Agent': 'AegisRun-Demo/1.0',
        },
      });
      console.log('✓ Request allowed');
      console.log('  Response:', JSON.stringify(result).slice(0, 100), '...');
      return result;
    } catch (error) {
      if (error instanceof ToolCallBlockedError) {
        console.log('✗ Request blocked:', error.decision);
      } else {
        console.log('✗ Request failed:', error);
      }
      return null;
    }
  });

  // Example 2: Blocked request to private IP (SSRF prevention)
  console.log('\n--- Test 2: Blocked Private IP (SSRF) ---');
  await run.step('ssrf_attempt', { phase: 'testing_blocked' }, async (step) => {
    try {
      const result = await step.toolCall('http_request', {
        url: 'http://169.254.169.254/latest/meta-data/',
        method: 'GET',
      });
      console.log('✗ Request should have been blocked!', result);
      return result;
    } catch (error) {
      if (error instanceof ToolCallBlockedError) {
        console.log('✓ Request blocked as expected');
        console.log('  Decision:', error.decision);
        return { blocked: true, decision: error.decision };
      }
      throw error;
    }
  });

  // Example 3: Blocked request to localhost
  console.log('\n--- Test 3: Blocked Localhost Request ---');
  await run.step('localhost_attempt', { phase: 'testing_blocked' }, async (step) => {
    try {
      const result = await step.toolCall('http_request', {
        url: 'http://localhost:8080/admin',
        method: 'GET',
      });
      console.log('✗ Request should have been blocked!', result);
      return result;
    } catch (error) {
      if (error instanceof ToolCallBlockedError) {
        console.log('✓ Request blocked as expected');
        console.log('  Decision:', error.decision);
        return { blocked: true };
      }
      throw error;
    }
  });

  // Example 4: Request with sensitive data (should be redacted in logs)
  console.log('\n--- Test 4: Request with Sensitive Headers ---');
  await run.step('sensitive_request', { phase: 'testing_redaction' }, async (step) => {
    try {
      const result = await step.toolCall('http_request', {
        url: 'https://httpbin.org/post',
        method: 'POST',
        headers: {
          'Authorization': 'Bearer sk-secret-api-key-12345',
          'X-API-Key': 'super-secret-key',
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          user: 'test@example.com',
          // Note: API keys in body should also be redacted
          api_key: 'another-secret-key',
        }),
      });
      console.log('✓ Request completed');
      console.log('  Note: API keys should be redacted in stored logs');
      return result;
    } catch (error) {
      if (error instanceof ToolCallBlockedError) {
        console.log('ℹ Request blocked (may be expected):', error.decision);
      } else {
        console.log('✗ Request failed:', error);
      }
      return null;
    }
  });

  // Example 5: POST request with JSON body
  console.log('\n--- Test 5: POST Request with JSON ---');
  await run.step('post_json', { phase: 'testing_post' }, async (step) => {
    try {
      const result = await step.toolCall('http_request', {
        url: 'https://httpbin.org/post',
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          message: 'Hello from AegisRun!',
          timestamp: new Date().toISOString(),
        }),
      });
      console.log('✓ POST request completed');
      return result;
    } catch (error) {
      console.log('✗ POST failed:', error);
      return null;
    }
  });

  // Example 6: Request with timeout
  console.log('\n--- Test 6: Request with Timeout ---');
  await run.step('timeout_request', { phase: 'testing_timeout' }, async (step) => {
    try {
      const result = await step.toolCall('http_request', {
        url: 'https://httpbin.org/delay/5', // 5 second delay
        method: 'GET',
        timeout_ms: 2000, // 2 second timeout
      });
      console.log('✓ Request completed before timeout');
      return result;
    } catch (error) {
      if (error instanceof ToolCallExecutionError) {
        console.log('ℹ Request timed out (expected behavior)');
        return { timeout: true };
      }
      console.log('✗ Request failed:', error);
      return null;
    }
  });

  // Example 7: Multiple requests in sequence
  console.log('\n--- Test 7: Sequential API Calls ---');
  await run.step('sequential_calls', { phase: 'testing_sequential' }, async (step) => {
    const results: any[] = [];

    // First request: get user info
    try {
      const userInfo = await step.toolCall('http_request', {
        url: 'https://jsonplaceholder.typicode.com/users/1',
        method: 'GET',
      });
      results.push({ step: 'user_info', success: true });
      console.log('✓ Step 1: Got user info');

      // Second request: get user's posts
      const posts = await step.toolCall('http_request', {
        url: 'https://jsonplaceholder.typicode.com/posts?userId=1',
        method: 'GET',
      });
      results.push({ step: 'user_posts', success: true });
      console.log('✓ Step 2: Got user posts');

    } catch (error) {
      if (error instanceof ToolCallBlockedError) {
        results.push({ step: 'blocked', decision: error.decision });
        console.log('✗ Request blocked:', error.decision);
      }
    }

    return results;
  });

  // End the run
  run.end({
    status: 'demo_complete',
    summary: {
      tests_run: 7,
      purpose: 'Demonstrate HTTP tool policy enforcement',
    },
  });

  console.log('\n=== Demo Summary ===');
  console.log(`Run ID: ${run.runId}`);
  console.log(`Total steps: ${run.counters.steps}`);
  console.log(`Total tool calls: ${run.counters.tool_calls}`);
  console.log(`Blocks: ${run.counters.blocks}`);
  console.log('\nDemo completed successfully!');
}

// Helper function for custom HTTP executor
async function customHttpExecutor(args: {
  url: string;
  method: string;
  headers?: Record<string, string>;
  body?: string;
}): Promise<any> {
  // This demonstrates how you might implement a custom executor
  // In practice, this would integrate with your own HTTP client
  console.log(`[Custom Executor] ${args.method} ${args.url}`);
  return { custom: true, url: args.url };
}

// Export for use as a module
export { main as runHttpDemo };

// Run if executed directly
if (require.main === module) {
  main().catch((error) => {
    console.error('Demo failed:', error);
    process.exit(1);
  });
}
