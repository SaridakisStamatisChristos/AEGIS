# AegisRun API Reference

**Version**: 1.0.0  
**Base URL**: `https://api.aegisrun.example.com/api/v1`  
**Last Updated**: 2026-02-03

---

## 1. Authentication

### 1.1 Bearer Token

All API requests require a valid JWT token in the Authorization header:

```
Authorization: Bearer <token>
```

### 1.2 OIDC Flow

```
1. Redirect to OIDC provider
2. User authenticates
3. Receive authorization code
4. Exchange for tokens at /auth/callback
5. Use access_token for API requests
```

---

## 2. Common Response Formats

### 2.1 Success Response

```json
{
  "data": { /* resource object */ },
  "meta": {
    "request_id": "req_01JQZX3K2FGH9VWBCD45EFGHIJ"
  }
}
```

### 2.2 Error Response

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid policy spec",
    "details": [
      {
        "field": "tools[0].name",
        "message": "Tool name is required"
      }
    ]
  },
  "meta": {
    "request_id": "req_01JQZX3K2FGH9VWBCD45EFGHIJ"
  }
}
```

### 2.3 Pagination

```json
{
  "data": [ /* array of resources */ ],
  "pagination": {
    "total": 150,
    "page": 1,
    "per_page": 20,
    "total_pages": 8
  }
}
```

---

## 3. Runs

### 3.1 Create Run

Start a new agent run.

```
POST /runs
```

**Request Body:**
```json
{
  "policy_id": "01JQZX3K2FGH9VWBCDPOLICYID",
  "metadata": {
    "agent_name": "customer-support",
    "user_id": "user_123",
    "environment": "production"
  }
}
```

**Response:**
```json
{
  "data": {
    "id": "01JQZX3K2FGH9VWBCD45EFGHIJ",
    "org_id": "01JPKDEF456OrgExample123",
    "policy_id": "01JQZX3K2FGH9VWBCDPOLICYID",
    "status": "active",
    "started_at": "2026-02-03T12:00:00.000Z",
    "metadata": {
      "agent_name": "customer-support",
      "user_id": "user_123",
      "environment": "production"
    }
  }
}
```

### 3.2 Get Run

Retrieve run details.

```
GET /runs/{run_id}
```

**Response:**
```json
{
  "data": {
    "id": "01JQZX3K2FGH9VWBCD45EFGHIJ",
    "org_id": "01JPKDEF456OrgExample123",
    "policy_id": "01JQZX3K2FGH9VWBCDPOLICYID",
    "status": "completed",
    "started_at": "2026-02-03T12:00:00.000Z",
    "finished_at": "2026-02-03T12:34:56.789Z",
    "metadata": {},
    "counters": {
      "steps": 15,
      "tool_calls": 42,
      "blocked": 3
    }
  }
}
```

### 3.3 List Runs

List runs with optional filters.

```
GET /runs?status=active&page=1&per_page=20
```

**Query Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| `status` | string | Filter by status: active, completed, failed |
| `policy_id` | string | Filter by policy ID |
| `page` | integer | Page number (default: 1) |
| `per_page` | integer | Items per page (default: 20, max: 100) |

### 3.4 Complete Run

Mark a run as completed.

```
POST /runs/{run_id}/complete
```

**Request Body:**
```json
{
  "status": "completed",
  "final_state": {
    "output": "Task completed successfully"
  }
}
```

---

## 4. Steps

### 4.1 Create Step

Add a new step to a run.

```
POST /runs/{run_id}/steps
```

**Request Body:**
```json
{
  "type": "agent_action",
  "input": {
    "prompt": "Search for customer information"
  },
  "state": {
    "context_length": 4096
  }
}
```

**Response:**
```json
{
  "data": {
    "id": "01JQZX3K2FGH9VWBCDSTEP0001",
    "run_id": "01JQZX3K2FGH9VWBCD45EFGHIJ",
    "sequence": 0,
    "type": "agent_action",
    "status": "started",
    "input_hash": "sha256:abc123...",
    "started_at": "2026-02-03T12:00:00.000Z"
  }
}
```

### 4.2 Get Step

Retrieve step details.

```
GET /runs/{run_id}/steps/{step_id}
```

### 4.3 List Steps

List steps for a run.

```
GET /runs/{run_id}/steps?page=1&per_page=50
```

### 4.4 Complete Step

Mark a step as completed.

```
POST /runs/{run_id}/steps/{step_id}/complete
```

**Request Body:**
```json
{
  "output": {
    "result": "Found 3 matching customers"
  }
}
```

---

## 5. Tool Calls

### 5.1 Submit Tool Call

Submit a tool call for policy evaluation and execution.

```
POST /runs/{run_id}/steps/{step_id}/tool-calls
```

**Request Body:**
```json
{
  "tool_name": "http_request",
  "arguments": {
    "url": "https://api.example.com/customers",
    "method": "GET",
    "headers": {
      "Authorization": "Bearer token123"
    }
  }
}
```

**Response (Allowed):**
```json
{
  "data": {
    "id": "01JQZX3K2FGH9VWBCDTOOLCALL",
    "tool_name": "http_request",
    "decision": {
      "action": "allow",
      "policy_rule_id": "tool.http_request",
      "evaluated_at": "2026-02-03T12:00:01.234Z"
    },
    "status": "pending_execution"
  }
}
```

**Response (Blocked):**
```json
{
  "data": {
    "id": "01JQZX3K2FGH9VWBCDTOOLCALL",
    "tool_name": "shell_exec",
    "decision": {
      "action": "block",
      "policy_rule_id": "tool.shell_exec",
      "reason": "Tool 'shell_exec' is blocked by policy"
    },
    "status": "blocked",
    "error": "Tool call blocked by policy"
  }
}
```

**Response (Requires Approval):**
```json
{
  "data": {
    "id": "01JQZX3K2FGH9VWBCDTOOLCALL",
    "tool_name": "database_write",
    "decision": {
      "action": "require_approval",
      "policy_rule_id": "tool.database_write",
      "approval_id": "01JQZX3K2FGH9VWBCDAPPROVAL"
    },
    "status": "pending_approval"
  }
}
```

### 5.2 Submit Tool Response

Submit the result of tool execution.

```
POST /runs/{run_id}/steps/{step_id}/tool-calls/{tool_call_id}/response
```

**Request Body:**
```json
{
  "response": {
    "status_code": 200,
    "body": {
      "customers": [
        {"id": "cust_1", "name": "John Doe"}
      ]
    }
  },
  "error": null
}
```

### 5.3 Get Tool Call

Retrieve tool call details.

```
GET /runs/{run_id}/steps/{step_id}/tool-calls/{tool_call_id}
```

---

## 6. Policies

### 6.1 Create Policy

Create a new policy.

```
POST /policies
```

**Request Body:**
```json
{
  "name": "production-policy",
  "version": "v1",
  "description": "Production policy with strict controls",
  "spec": {
    "tools": [
      {
        "name": "http_request",
        "action": "allow",
        "arg_schema": {
          "type": "object",
          "properties": {
            "url": {"type": "string", "format": "uri"},
            "method": {"type": "string", "enum": ["GET", "POST"]}
          }
        }
      }
    ],
    "budgets": {
      "max_tool_calls": 100
    }
  }
}
```

**Response:**
```json
{
  "data": {
    "id": "01JQZX3K2FGH9VWBCDPOLICYID",
    "name": "production-policy",
    "version": "v1",
    "status": "draft",
    "spec_hash": "sha256:policy123...",
    "created_at": "2026-02-03T12:00:00.000Z"
  }
}
```

### 6.2 Get Policy

Retrieve policy details.

```
GET /policies/{policy_id}
```

### 6.3 List Policies

List policies with optional filters.

```
GET /policies?status=deployed&page=1&per_page=20
```

**Query Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| `status` | string | Filter by status: draft, review, approved, deployed, deprecated |
| `name` | string | Filter by policy name (exact match) |

### 6.4 Update Policy Status

Change policy lifecycle status.

```
POST /policies/{policy_id}/status
```

**Request Body:**
```json
{
  "status": "approved",
  "comment": "Reviewed and approved for production use"
}
```

### 6.5 Validate Policy

Validate a policy spec without creating it.

```
POST /policies/validate
```

**Request Body:**
```json
{
  "spec": {
    "tools": [...]
  }
}
```

**Response:**
```json
{
  "data": {
    "valid": true,
    "warnings": [
      "Tool 'legacy_api' not used in any conditions"
    ]
  }
}
```

---

## 7. Approvals

### 7.1 List Pending Approvals

List approvals awaiting decision.

```
GET /approvals?status=pending
```

### 7.2 Get Approval

Retrieve approval details.

```
GET /approvals/{approval_id}
```

**Response:**
```json
{
  "data": {
    "id": "01JQZX3K2FGH9VWBCDAPPROVAL",
    "run_id": "01JQZX3K2FGH9VWBCD45EFGHIJ",
    "tool_call_id": "01JQZX3K2FGH9VWBCDTOOLCALL",
    "tool_name": "database_write",
    "arguments": {
      "table": "users",
      "operation": "UPDATE"
    },
    "status": "pending",
    "requested_at": "2026-02-03T12:00:00.000Z",
    "policy_rule_id": "tool.database_write"
  }
}
```

### 7.3 Approve Tool Call

Grant approval for a pending tool call.

```
POST /approvals/{approval_id}/approve
```

**Request Body:**
```json
{
  "comment": "Approved after verifying data integrity"
}
```

### 7.4 Deny Tool Call

Deny a pending tool call.

```
POST /approvals/{approval_id}/deny
```

**Request Body:**
```json
{
  "reason": "Operation not necessary for this task"
}
```

---

## 8. Events

### 8.1 List Events

List events for a run.

```
GET /runs/{run_id}/events?page=1&per_page=100
```

**Response:**
```json
{
  "data": [
    {
      "id": "01JQZX3K2FGH9VWBCDEVENT001",
      "run_id": "01JQZX3K2FGH9VWBCD45EFGHIJ",
      "type": "run.started",
      "timestamp": "2026-02-03T12:00:00.000Z",
      "sequence": 0,
      "payload": {}
    },
    {
      "id": "01JQZX3K2FGH9VWBCDEVENT002",
      "run_id": "01JQZX3K2FGH9VWBCD45EFGHIJ",
      "type": "tool_call.submitted",
      "timestamp": "2026-02-03T12:00:01.000Z",
      "sequence": 1,
      "payload": {
        "tool_call_id": "01JQZX3K2FGH9VWBCDTOOLCALL",
        "tool_name": "http_request"
      }
    }
  ]
}
```

### 8.2 Stream Events (SSE)

Subscribe to real-time events.

```
GET /runs/{run_id}/events/stream
Accept: text/event-stream
```

**Event Format:**
```
event: tool_call.completed
data: {"tool_call_id":"01JQZX3K2FGH...","tool_name":"http_request"}

event: step.completed
data: {"step_id":"01JQZX3K2FGH...","sequence":5}
```

---

## 9. Evidence Export

### 9.1 Export Bundle

Export a complete evidence bundle.

```
GET /runs/{run_id}/export?format=bundle
```

**Query Parameters:**
| Parameter | Values | Description |
|-----------|--------|-------------|
| `format` | bundle, manifest, attestation | Export format |

**Response Headers:**
```
Content-Type: application/zip
Content-Disposition: attachment; filename="evidence-01JQZX3K2FGH9VWBCD45EFGHIJ.zip"
```

---

## 10. Health & Info

### 10.1 Health Check

```
GET /health
```

**Response:**
```json
{
  "status": "healthy",
  "version": "1.0.0"
}
```

### 10.2 Readiness Check

```
GET /ready
```

**Response:**
```json
{
  "status": "ready",
  "checks": {
    "database": "ok",
    "signing_key": "ok"
  }
}
```

---

## 11. Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `VALIDATION_ERROR` | 400 | Invalid request payload |
| `UNAUTHORIZED` | 401 | Missing or invalid token |
| `FORBIDDEN` | 403 | Insufficient permissions |
| `NOT_FOUND` | 404 | Resource not found |
| `CONFLICT` | 409 | Resource conflict (e.g., duplicate) |
| `POLICY_VIOLATION` | 422 | Tool call blocked by policy |
| `BUDGET_EXCEEDED` | 422 | Run budget limit reached |
| `RATE_LIMITED` | 429 | Too many requests |
| `INTERNAL_ERROR` | 500 | Server error |

---

## 12. Rate Limits

| Endpoint Pattern | Limit |
|------------------|-------|
| `/runs/*` | 100/min |
| `/policies/*` | 50/min |
| `/approvals/*` | 200/min |
| `/*/*/tool-calls` | 1000/min |

Rate limit headers:
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1706961600
```

---

**End of API_REFERENCE.md**
