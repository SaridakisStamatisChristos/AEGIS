/**
 * Express middleware example for AegisRun SDK
 * 
 * This middleware wraps API requests in AegisRun runs for policy enforcement.
 */

import { Request, Response, NextFunction } from 'express';
import { AegisRunClient, Run } from '../src';

// Extend Express Request to include aegis run
declare global {
  namespace Express {
    interface Request {
      aegisRun?: Run;
    }
  }
}

export interface AegisMiddlewareConfig {
  client: AegisRunClient;
  agentId: string;
  policyRef: string;
  extractMetadata?: (req: Request) => Record<string, any>;
}

/**
 * Creates an Express middleware that wraps requests in AegisRun runs
 */
export function aegisMiddleware(config: AegisMiddlewareConfig) {
  return async (req: Request, res: Response, next: NextFunction) => {
    try {
      // Extract metadata from request
      const metadata = config.extractMetadata
        ? config.extractMetadata(req)
        : {
            method: req.method,
            path: req.path,
            ip: req.ip,
            userAgent: req.get('user-agent'),
          };

      // Create and start run
      const run = await new Run({
        client: config.client,
        agentId: config.agentId,
        policyRef: config.policyRef,
        metadata,
      }).start();

      // Attach run to request
      req.aegisRun = run;

      // End run on response finish
      res.on('finish', () => {
        run.end({
          statusCode: res.statusCode,
          completed: true,
        });
      });

      next();
    } catch (error) {
      console.error('AegisRun middleware error:', error);
      // Continue without run if AegisRun is unavailable
      next();
    }
  };
}

/**
 * Example usage with Express
 */
/*
import express from 'express';
import { AegisRunClient } from '../src';
import { aegisMiddleware } from './express-middleware';

const app = express();
const aegisClient = new AegisRunClient({
  baseUrl: process.env.AEGIS_URL || 'http://localhost:8080',
  apiToken: process.env.AEGIS_TOKEN,
});

// Apply middleware globally
app.use(aegisMiddleware({
  client: aegisClient,
  agentId: 'api-agent',
  policyRef: 'api-policy:v1',
  extractMetadata: (req) => ({
    method: req.method,
    path: req.path,
    userId: req.headers['x-user-id'],
  }),
}));

// Use in route handlers
app.post('/api/process', async (req, res) => {
  const { aegisRun } = req;
  
  if (aegisRun) {
    await aegisRun.step('process_request', { action: 'process' }, async (step) => {
      // Make tool calls through AegisRun
      const result = await step.toolCall('http_request', {
        url: 'https://api.example.com/process',
        method: 'POST',
        body: req.body,
      });
      return result;
    });
  }
  
  res.json({ success: true });
});

app.listen(3000);
*/
