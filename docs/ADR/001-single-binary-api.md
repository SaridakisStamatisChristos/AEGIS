# ADR-001: Single Binary API (Monolith)

**Status**: Accepted  
**Date**: 2026-02-03  
**Authors**: AegisRun Team

---

## Context

When designing the AegisRun backend, we needed to decide between:

1. **Microservices**: Separate services for gateway, policy engine, ledger, auth
2. **Monolith**: Single binary containing all components

The system needs to handle:
- Policy evaluation (sub-millisecond decisions)
- Event logging (high throughput)
- Evidence signing (cryptographic operations)
- Approval workflows (stateful coordination)

## Decision

We chose a **single binary monolith** for the API server.

## Rationale

### Advantages of Monolith

1. **Operational Simplicity**
   - Single deployment unit
   - Single log stream
   - No service mesh required
   - Easier debugging and tracing

2. **Performance**
   - No network hops between components
   - In-process function calls for policy evaluation
   - Reduced latency for critical path (tool call → decision)

3. **Transactional Integrity**
   - Single database connection pool
   - Local transactions without distributed coordination
   - Simpler rollback on failures

4. **Development Velocity**
   - Faster iteration on cross-cutting concerns
   - No API versioning between internal services
   - Refactoring is straightforward

### Why Not Microservices

1. **Premature Optimization**
   - No proven need for independent scaling
   - Similar resource requirements across all components

2. **Operational Overhead**
   - Kubernetes adds complexity
   - Service discovery, retries, circuit breakers
   - Distributed tracing is harder

3. **Consistency Challenges**
   - Policy decision + event logging should be atomic
   - Two-phase commit adds latency and failure modes

## Consequences

### Positive

- Simple deployment: `docker run aegisrun/api`
- All components share connection pools and caches
- Easy to reason about the entire request lifecycle
- Lower infrastructure costs (no service mesh, fewer nodes)

### Negative

- Cannot scale components independently
- Must deploy entire system for any change
- Potential for tighter coupling between modules

### Mitigations

- Use clean internal interfaces (packages with clear APIs)
- Keep modules loosely coupled via dependency injection
- Design for future extraction if needed
- Use feature flags for gradual rollouts

## Alternatives Considered

### Microservices with gRPC

Rejected because:
- Added 10-20ms latency per inter-service call
- Required distributed transactions for policy + logging
- Operational complexity not justified at current scale

### Serverless Functions

Rejected because:
- Cold start latency incompatible with sub-100ms SLA
- State management complexity for approval workflows
- Vendor lock-in concerns

## Related Decisions

- [ADR-002: PostgreSQL as Queue](002-postgres-as-queue.md)
- [ADR-003: Ed25519 Signing](003-ed25519-signing.md)

---

**End of ADR-001**
