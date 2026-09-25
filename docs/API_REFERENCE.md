# AegisRun API Reference

**Application baseline:** v1.0.1  
**API prefix:** `/api/v1`  
**Reviewed against:** current Go router/handlers on 2026-09-25

This document describes the routes that are actually registered by `api/internal/server/server.go`. Older route shapes that are not present in the router are intentionally omitted.

## 1. Authentication

All routes under `/api/v1` require:

```http
Authorization: Bearer <OIDC access token>
```

AegisRun validates the token through its configured OIDC provider and places the authenticated user/organization in request context.

The current server does **not** expose its own `/auth/login` or `/auth/callback` endpoints. Obtain a token from the configured identity provider and present it as a bearer token.

## 2. Public endpoints

### GET /health

Liveness/status endpoint.

Example:

```json
{
  "status": "ok",
  "version": "1.0.1"
}
```

The version is injected into the API binary at build time.

### GET /ready

Readiness endpoint. It checks database connectivity.

Healthy response:

```json
{
  "status": "ready",
  "checks": {
    "database": "healthy"
  }
}
```

When a required dependency is unavailable, the endpoint returns HTTP 503 with `status: "not ready"`.

### GET /metrics

Prometheus metrics endpoint.

## 3. Route summary

| Method | Route | Authorization |
|---|---|---|
| GET | `/api/v1/runs/` | run:view |
| POST | `/api/v1/runs/` | run:create |
| GET | `/api/v1/runs/{runID}` | run:view |
| GET | `/api/v1/runs/{runID}/steps` | run:view |
| GET | `/api/v1/runs/{runID}/events` | run:view |
| POST | `/api/v1/runs/{runID}/events` | run:create |
| GET | `/api/v1/policies/` | policy:view |
| POST | `/api/v1/policies/` | policy:create |
| GET | `/api/v1/policies/{policyID}` | policy:view |
| PUT | `/api/v1/policies/{policyID}` | policy:edit |
| DELETE | `/api/v1/policies/{policyID}` | policy:edit |
| POST | `/api/v1/policies/{policyID}/activate` | policy:deploy |
| POST | `/api/v1/policies/{policyID}/deactivate` | policy:deploy |
| GET | `/api/v1/approvals/` | policy:view |
| GET | `/api/v1/approvals/{approvalID}` | policy:view |
| POST | `/api/v1/approvals/policies/{policyID}/approve?version=<version>` | policy:approve |
| POST | `/api/v1/approvals/policies/{policyID}/reject?version=<version>` | policy:approve |
| GET | `/api/v1/evidence/runs/{runID}/bundle` | evidence:export |
| POST | `/api/v1/evidence/verify` | evidence:view |
| POST | `/api/v1/gateway/execute` | authenticated |
| GET | `/api/v1/stats` | run:view |

Tenant-scoped handlers also verify that resources belong to the authenticated organization.

## 4. Runs

### POST /api/v1/runs/

Create a run.

Request:

```json
{
  "policy_ref": {
    "policy_id": "01JQZX3K2FGH9VWBCDPOLICYID",
    "version": "v1"
  },
  "parent_run_id": null,
  "state_schema_ref": {
    "schema_id": "optional-schema",
    "version": "v1"
  },
  "metadata": {
    "environment": "staging",
    "agent": "example"
  }
}
```

`policy_ref.policy_id` is required.

Successful creation returns HTTP 201 and a direct run object containing:

- `run_id`;
- `org_id`;
- `policy_ref`;
- optional parent/schema references;
- metadata;
- timestamps;
- status/outcome;
- counters;
- evidence/signature fields when available.

### GET /api/v1/runs/

List runs for the authenticated organization.

Supported query parameters:

- `limit`
- `offset`
- `status`
- `policy_id`
- `start_time`
- `end_time`

Response shape:

```json
{
  "runs": []
}
```

`limit` defaults to 50 and is capped at 100; invalid/non-positive values fall back to 50. `offset` defaults to 0. The `status` filter accepts a comma-separated list.

### GET /api/v1/runs/{runID}

Returns a single run when it belongs to the authenticated organization.

### GET /api/v1/runs/{runID}/steps

Returns:

```json
{
  "steps": []
}
```

A step includes `step_id`, `run_id`, sequence number, name, state vector, timestamps, status and optional error.

### GET /api/v1/runs/{runID}/events

Returns:

```json
{
  "events": []
}
```

Each event includes `event_id`, `run_id`, `seq_no`, `event_type`, timestamp, payload, previous hash and event hash.

### POST /api/v1/runs/{runID}/events

Submit an SDK/lifecycle event.

Request:

```json
{
  "event_type": "step.started",
  "payload": {
    "step_id": "01EXAMPLE"
  },
  "timestamp": "2026-09-25T06:00:00Z"
}
```

`event_type` is required and must be in the server's allowed event-type set. Successful creation returns HTTP 201 with the persisted event.

## 5. Policies

### POST /api/v1/policies/

Create a draft policy.

```json
{
  "name": "production-policy",
  "spec": {
    "tools": [
      {
        "name": "http_request",
        "action": "allow"
      }
    ],
    "budgets": {
      "max_tool_calls": 100
    }
  }
}
```

The policy compiler validates the spec before persistence.

### GET /api/v1/policies/

List policies. Optional query parameter:

- `status`

Response:

```json
{
  "policies": []
}
```

### GET /api/v1/policies/{policyID}

Returns the latest policy version by default. An exact version may be requested with `?version=v1`.

### PUT /api/v1/policies/{policyID}

Creates an updated policy version from a validated `spec`.

### DELETE /api/v1/policies/{policyID}

Deprecates the current policy rather than physically deleting historical data. Success returns HTTP 204.

### POST /api/v1/policies/{policyID}/activate

Activates/deploys the current eligible policy version according to lifecycle rules.

### POST /api/v1/policies/{policyID}/deactivate

Moves the current active policy back to draft state.

## 6. Approvals

### GET /api/v1/approvals/?policy_id=<id>&version=<version>

Both query parameters are required.

### GET /api/v1/approvals/{approvalID}

Returns one approval decision.

### POST /api/v1/approvals/policies/{policyID}/approve?version=<version>

The policy must be in review status. Body is optional:

```json
{ "comment": "Reviewed" }
```

### POST /api/v1/approvals/policies/{policyID}/reject?version=<version>

A rejection requires a comment:

```json
{ "comment": "Explain the required changes" }
```

## 7. Gateway

### POST /api/v1/gateway/execute

Central policy-enforcement endpoint.

```json
{
  "run_id": "01RUN",
  "step_id": "01STEP",
  "tool_name": "http_request",
  "args": {
    "url": "https://api.example.com"
  },
  "state_vector": {},
  "metadata": {},
  "executor": "builtin"
}
```

Required: `run_id`, `step_id`, `tool_name`.

Response:

```json
{
  "tool_call_id": "01TOOLCALL",
  "decision": {
    "action": "allow",
    "policy_rule_id": "http_request",
    "reason": "allowed"
  },
  "result": {}
}
```

| Decision | HTTP status |
|---|---:|
| allow | 200 |
| warn | 200 |
| redact | 200 |
| block | 403 |
| require_approval | 202 |
| unexpected/internal failure | 500 |

The current gateway returns a `require_approval` decision, but pending tool-call approval/resume execution is **not implemented in this iteration**. The `/approvals` endpoints documented above are policy-version approvals, not pending tool-call approvals.

## 8. Evidence

### GET /api/v1/evidence/runs/{runID}/bundle

Streams the evidence ZIP. Current bundle format: **1.0.0**.

### POST /api/v1/evidence/verify

Server-side chain verification:

```json
{ "run_id": "01RUN" }
```

Response:

```json
{
  "run_id": "01RUN",
  "chain_valid": true,
  "message": ""
}
```

For independent signature verification, use `aegis-verify` against the exported ZIP.

## 9. Statistics

### GET /api/v1/stats

Returns database-computed dashboard aggregates for the authenticated organization, including run counts and status aggregates.

## 10. Error behavior

Common status codes:

- 400 — invalid/missing input;
- 401 — missing/invalid authentication;
- 403 — permission denied or gateway policy block;
- 404 — tenant-scoped resource not found;
- 409 — lifecycle/duplicate approval conflict;
- 500 — internal failure;
- 503 — readiness dependency unavailable.

Some handler errors are plain-text `http.Error` responses while gateway errors are JSON. Clients should rely primarily on status codes unless an endpoint explicitly documents a JSON error body.

## 11. Source of truth

If documentation and code disagree, these are authoritative:

- `api/internal/server/server.go`
- `api/internal/server/handlers/`
- `api/internal/gateway/`
- `api/internal/contracts/`
