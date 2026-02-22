# ADR-002: PostgreSQL as Queue (No External Message Queue)

**Status**: Accepted  
**Date**: 2026-02-03  
**Authors**: AegisRun Team

---

## Context

AegisRun needs to handle asynchronous workflows:

1. **Approval queue**: Tool calls waiting for human approval
2. **Event stream**: Real-time notifications to subscribers
3. **Export jobs**: Background evidence bundle generation
4. **Retry queue**: Failed operations needing retry

Traditional architectures would use a dedicated message queue (RabbitMQ, Kafka, SQS).

## Decision

We use **PostgreSQL** for all queueing needs, leveraging:

- SKIP LOCKED for work queues
- LISTEN/NOTIFY for real-time notifications
- Transactional outbox pattern for reliable event publishing

## Rationale

### Why PostgreSQL-Based Queues

1. **Transactional Consistency**
   - Event logging and queue insertion in single transaction
   - No "event published but not logged" failure mode
   - No "logged but not published" failure mode

2. **Operational Simplicity**
   - One fewer system to operate, monitor, backup
   - No message broker expertise required
   - Familiar PostgreSQL tooling

3. **Durability Guarantees**
   - Same durability as primary data
   - Point-in-time recovery includes queue state
   - No broker-database sync issues

4. **Sufficient Throughput**
   - PostgreSQL handles 10,000+ TPS for queue operations
   - Current requirements: ~100-1000 tool calls/second
   - SKIP LOCKED scales well with multiple workers

### Implementation Patterns

#### Work Queue with SKIP LOCKED

```sql
-- Fetch next pending approval
SELECT * FROM approvals
WHERE status = 'pending'
ORDER BY created_at
FOR UPDATE SKIP LOCKED
LIMIT 1;
```

#### Real-Time Notifications with LISTEN/NOTIFY

```sql
-- Publisher
NOTIFY run_events, '{"run_id":"01JQZ...","type":"tool_call.completed"}';

-- Subscriber (in Go)
conn.Listen("run_events")
for notification := range conn.Notifications {
    handleEvent(notification.Payload)
}
```

#### Transactional Outbox

```go
// Atomic: write event + queue notification
tx.Begin()
tx.Exec("INSERT INTO events ...")
tx.Exec("INSERT INTO event_outbox ...")
tx.Commit()

// Background worker publishes from outbox
```

## Consequences

### Positive

- Zero additional infrastructure
- Atomic operations (data + queue in one transaction)
- Simpler disaster recovery (single backup)
- Familiar debugging tools (psql, pg_stat_activity)

### Negative

- Limited horizontal scaling (single PostgreSQL primary)
- No built-in message routing or filtering
- Polling-based for multi-consumer scenarios

### Mitigations

- Use connection pooling (PgBouncer) for scalability
- Index queue tables appropriately
- Monitor queue depth and processing latency
- Design for future extraction if throughput exceeds 10K TPS

## Alternatives Considered

### RabbitMQ

Rejected because:
- Additional operational burden
- Requires explicit ACK handling
- Durability guarantees harder to reason about
- "Exactly once" requires careful implementation

### Apache Kafka

Rejected because:
- Massive operational complexity
- Designed for event streaming at much higher scale
- Overkill for approval queue use case
- ZooKeeper dependency (pre-KRaft)

### Redis Streams

Rejected because:
- Additional system to manage
- Persistence model different from PostgreSQL
- Would need Redis Cluster for HA

### AWS SQS

Rejected because:
- Vendor lock-in
- Cannot participate in PostgreSQL transactions
- At-least-once requires idempotency handling

## Performance Characteristics

| Metric | PostgreSQL Queue | Target |
|--------|------------------|--------|
| Enqueue latency | <5ms | <10ms |
| Dequeue latency | <10ms | <50ms |
| Throughput | 10K+ msg/s | 1K msg/s |
| Message visibility | Immediate | Immediate |

## Related Decisions

- [ADR-001: Single Binary API](001-single-binary-api.md)
- [ADR-003: Ed25519 Signing](003-ed25519-signing.md)

---

**End of ADR-002**
