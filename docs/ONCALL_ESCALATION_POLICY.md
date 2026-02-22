# AegisRun On-Call Escalation Policy

**Owner**: Platform / SRE  
**Last Reviewed**: 2026-02-22  
**Review Cadence**: Quarterly

---

## Severity Levels

- **SEV-1**: Customer-impacting outage, data-loss risk, or runaway error-budget burn.
- **SEV-2**: Partial degradation with workaround available.
- **SEV-3**: Non-critical degradation or warning trend.

---

## Burn-Rate Alert Escalation

The Prometheus alert `AegisRunErrorBudgetBurn` is treated as **SEV-1**.

### Escalation Path

1. **0-5 min**: Primary on-call acknowledges and starts triage.
2. **5-10 min**: Escalate to secondary on-call if no acknowledgement.
3. **10-15 min**: Escalate to platform lead and incident commander.
4. **15+ min**: Engage engineering management and invoke rollback playbook if budget burn persists.

### Required Actions for `AegisRunErrorBudgetBurn`

- Open incident channel and assign incident commander.
- Validate current release window and canary status.
- Check SLO dashboard (`error rate`, `p95/p99 latency`, `throughput`).
- Trigger rollback per `docs/ROLLBACK_PLAYBOOK.md` when mitigation is not effective within SLO window.

---

## Notification Channels

- **Primary paging receiver**: `aegisrun-oncall-primary`
- **Secondary paging receiver**: `aegisrun-oncall-secondary`
- **Management escalation receiver**: `aegisrun-escalation`

These receivers are referenced from Alertmanager routing.

Receiver endpoints are configured via environment variables in `ops/prometheus/alertmanager.yml`:

- `ALERTMANAGER_WEBHOOK_PRIMARY_URL`
- `ALERTMANAGER_WEBHOOK_SECONDARY_URL`
- `ALERTMANAGER_WEBHOOK_ESCALATION_URL`

Use `ops/prometheus/alertmanager.env.example` as the template for environment-specific values.

---

## Verification Requirements

For release-readiness evidence, attach:

- Alertmanager route configuration proving `AegisRunErrorBudgetBurn` routes to on-call receivers.
- At least one staged drill record showing acknowledgement + escalation + rollback decision path.
