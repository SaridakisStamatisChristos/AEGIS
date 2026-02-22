package server

// Routes are defined in server.go via registerRoutes
// This file is a placeholder for route documentation

/*
Route Summary:

Public Routes:
  GET  /health                          - Health check
  GET  /auth/login                      - Initiate OIDC login
  GET  /auth/callback                   - OIDC callback

Authenticated API Routes (all under /api/v1):

  Runs:
    GET    /runs                         - List runs
    POST   /runs                         - Create run
    GET    /runs/{run_id}                - Get run
    GET    /runs/{run_id}/timeline       - Get run timeline
    POST   /runs/{run_id}/cancel         - Cancel run

  Policies:
    GET    /policies                     - List policies
    POST   /policies                     - Create policy
    GET    /policies/{policy_id}/versions/{version}  - Get policy version
    GET    /policies/{policy_id}/versions            - List versions
    PUT    /policies/{policy_id}/versions/{version}/status - Update status

  Approvals:
    GET    /approvals                    - List approvals (requires policy_id/version)
    POST   /approvals                    - Create approval
    GET    /approvals/{approval_id}      - Get approval

  Evidence:
    GET    /evidence/{run_id}/bundle     - Export evidence bundle
    GET    /evidence/{run_id}/verify     - Verify integrity

  Gateway:
    POST   /gateway/tool-call            - Execute tool call

  Metrics:
    GET    /metrics                      - Prometheus metrics
*/
