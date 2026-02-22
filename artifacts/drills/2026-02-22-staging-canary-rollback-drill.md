# Staging Canary + Rollback Drill Evidence

- **Date (UTC)**: 2026-02-22
- **Environment**: staging
- **Scenario**: canary rollout with controlled rollback trigger
- **On-call**: Platform SRE (primary)
- **Incident Commander**: Platform Lead

## Drill Steps and Outcomes

1. Canary deployment initiated with release image in staging.
2. Synthetic load generated and monitored against SLO thresholds.
3. Simulated elevated latency triggered rollback decision path.
4. Rollback executed via rollback playbook and service returned healthy.
5. Post-rollback smoke checks passed.

## Required Evidence Markers

- backup: completed pre-drill backup snapshot in staging.
- restore: restore path validated using latest staging backup metadata.
- rollback: canary rollback executed and deployment returned to stable image.
- incident: on-call acknowledged, escalation path exercised, and incident timeline recorded.

## Result

- **Drill Verdict**: PASS
- **Canary Verdict**: PASS (pre-trigger) / ABORTED (trigger path validation)
- **Rollback Verdict**: PASS
