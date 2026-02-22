# AegisRun Release Checklist

**Version**: _______________  
**Release Branch**: `release/___`  
**Freeze SHA**: _______________  
**Target Release Date**: _______________  
**Release Manager**: _______________

---

## Instructions

1. Copy this file for each release (e.g., `RELEASE_CHECKLIST_v1.3.0.md`).
2. Fill in each section as the release progresses.
3. Each owner must sign off by adding their name, date, and ✅.
4. All P0 items must be ✅ before the release tag is cut.
5. P1 items should be completed but can be deferred with documented justification.

---

## 1. Pre-Freeze Verification

| # | Check | Status | Owner | Date |
|---|-------|--------|-------|------|
| 1.1 | All P0 production readiness items closed | ☐ | Engineering Lead | |
| 1.2 | All P1 production readiness items closed or deferred with justification | ☐ | Engineering Lead | |
| 1.3 | CHANGELOG.md updated with all user-facing changes | ☐ | Release Manager | |
| 1.4 | Version numbers updated in package.json, pyproject.toml, go sources | ☐ | Release Manager | |
| 1.5 | API documentation (docs/API_REFERENCE.md) matches current endpoints | ☐ | API Owner | |
| 1.6 | No known regressions from previous release | ☐ | QA Lead | |

---

## 2. CI Pipeline Results

> Attach or link the release-gate workflow run URL.

**Release-Gate Run**: [Link](___)  
**Run ID**: _______________

### Build Checks

| # | Check | Status | Notes |
|---|-------|--------|-------|
| 2.1 | API binary builds | ☐ | |
| 2.2 | Verifier binary builds | ☐ | |
| 2.3 | UI builds | ☐ | |
| 2.4 | Docker images build | ☐ | |

### Test Checks

| # | Check | Status | Coverage | Notes |
|---|-------|--------|----------|-------|
| 2.5 | API unit tests pass | ☐ | ___% | |
| 2.6 | Verifier tests pass | ☐ | | |
| 2.7 | Python SDK tests pass | ☐ | ___% | |
| 2.8 | TypeScript SDK tests pass | ☐ | | |
| 2.9 | UI tests pass | ☐ | | |
| 2.10 | E2E full flow tests pass | ☐ | | |
| 2.11 | Migration roundtrip passes | ☐ | | |
| 2.12 | Load test passes (error rate ≤ 5%, p95 ≤ 500ms, p99 ≤ 2000ms, throughput floor met) | ☐ | | |

### Security Checks

| # | Check | Status | Critical/High Findings | Notes |
|---|-------|--------|----------------------|-------|
| 2.13 | Gosec + govulncheck clean | ☐ | 0 | |
| 2.14 | Trivy dependency scan clean | ☐ | 0 | |
| 2.15 | Gitleaks secret scan clean | ☐ | 0 | |
| 2.16 | Container image scan clean | ☐ | 0 | |

### Operations Gate Checks

| # | Check | Status | Notes |
|---|-------|--------|-------|
| 2.17 | Canary health scoring gate passes (`canary-health-gate`) | ☐ | |
| 2.18 | Drill cadence evidence gate passes (`ops-drill-cadence`) | ☐ | |
| 2.19 | SLO burn-rate escalation wiring gate passes (`slo-escalation-gate`) | ☐ | |

**Security sign-off**: Any findings above 0 must be documented and accepted:

| Finding | Severity | Justification for Release | Accepted By |
|---------|----------|--------------------------|-------------|
| | | | |

---

## 3. Owner Approvals

Each component owner must review and sign off. Sign-off means:
- You have reviewed the changes in this release that affect your component.
- You confirm no known critical issues.
- You approve releasing this version to production.

| # | Component | Owner | Approved | Signature | Date |
|---|-----------|-------|----------|-----------|------|
| 3.1 | **API** | | ☐ | _______________ | |
| 3.2 | **UI** | | ☐ | _______________ | |
| 3.3 | **Python SDK** | | ☐ | _______________ | |
| 3.4 | **TypeScript SDK** | | ☐ | _______________ | |
| 3.5 | **Ops / Infrastructure** | | ☐ | _______________ | |
| 3.6 | **Security** | | ☐ | _______________ | |

---

## 4. Canary Deployment

> Two consecutive successful canary cycles are required before full rollout.

### Canary Abort Thresholds

| Metric | Threshold | Source |
|--------|-----------|--------|
| Error rate (5xx) | > 5% | Prometheus: `aegisrun:http_error_ratio_5m` |
| P99 latency | > 2,000ms | Prometheus: `aegisrun:http_p99_latency_5m` |
| Pod restarts | > 2 in canary pod | `kubectl get pods` |
| Readiness failure | > 60s not ready | k8s readiness probe |

### Canary Cycle 1

| # | Step | Status | Timestamp | Notes |
|---|------|--------|-----------|-------|
| 4.1 | Deploy canary (1 replica with release image) | ☐ | | |
| 4.2 | Route 10% traffic to canary | ☐ | | |
| 4.3 | Monitor for 15 minutes | ☐ | | |
| 4.4 | Error rate within threshold | ☐ | Observed: ___% | |
| 4.5 | P99 latency within threshold | ☐ | Observed: ___ms | |
| 4.6 | No pod restarts | ☐ | Restarts: ___ | |
| 4.7 | Readiness probe passing | ☐ | | |
| 4.8 | Smoke tests against canary pass | ☐ | | |

**Canary Cycle 1 Result**: ☐ PASS / ☐ FAIL / ☐ ABORTED  
**Observed by**: _______________  
**Time window**: ___ to ___

### Canary Cycle 2

| # | Step | Status | Timestamp | Notes |
|---|------|--------|-----------|-------|
| 4.9 | Route 33% traffic to canary | ☐ | | |
| 4.10 | Monitor for 15 minutes | ☐ | | |
| 4.11 | Error rate within threshold | ☐ | Observed: ___% | |
| 4.12 | P99 latency within threshold | ☐ | Observed: ___ms | |
| 4.13 | No pod restarts | ☐ | Restarts: ___ | |
| 4.14 | Readiness probe passing | ☐ | | |
| 4.15 | Smoke tests against canary pass | ☐ | | |

**Canary Cycle 2 Result**: ☐ PASS / ☐ FAIL / ☐ ABORTED  
**Observed by**: _______________  
**Time window**: ___ to ___

---

## 5. Production Rollout

| # | Step | Status | Timestamp | Notes |
|---|------|--------|-----------|-------|
| 5.1 | Backup production database | ☐ | | |
| 5.2 | Record current deployment state (image tags, migration version) | ☐ | | |
| 5.3 | Run database migrations (if any) | ☐ | | |
| 5.4 | Full rollout (all replicas on new image) | ☐ | | |
| 5.5 | Rollout status shows all pods ready | ☐ | | |
| 5.6 | Production smoke tests pass (/health, /ready, /metrics) | ☐ | | |
| 5.7 | Grafana SLO dashboard shows normal | ☐ | | |
| 5.8 | Monitor for 30 minutes post-rollout | ☐ | | |
| 5.9 | Slack announcement posted | ☐ | | |

---

## 6. Post-Release Verification

| # | Check | Status | Notes |
|---|-------|--------|-------|
| 6.1 | Error rate stable (< 1%) for 1 hour | ☐ | |
| 6.2 | P99 latency stable (< 500ms) for 1 hour | ☐ | |
| 6.3 | No new alerts fired | ☐ | |
| 6.4 | SDK compatibility verified (Python + TypeScript) | ☐ | |
| 6.5 | GitHub Release created with binaries + CHANGELOG | ☐ | |
| 6.6 | Python SDK published to PyPI | ☐ | |
| 6.7 | TypeScript SDK published to npm | ☐ | |
| 6.8 | CHANGELOG.md committed to main | ☐ | |

---

## 7. Readiness Score

> Fill in based on current state at release time.

| Dimension | Score | Max | Notes |
|-----------|-------|-----|-------|
| Security | /20 | 20 | |
| Reliability | /20 | 20 | |
| Operability | /20 | 20 | |
| Quality & Testing | /20 | 20 | |
| Release & Governance | /20 | 20 | |
| **Total** | **/100** | **100** | |

**Target**: ≥ 90/100  
**Actual**: ___/100  
**Gate**: ☐ PASS (≥ 90) / ☐ FAIL (< 90)

---

## 8. Final Sign-Off

> All approvals in Section 3 must be complete, both canary cycles must pass,
> and the readiness score must be ≥ 90 before this section is signed.

| Role | Name | Approved | Date |
|------|------|----------|------|
| Release Manager | | ☐ | |
| Engineering Lead | | ☐ | |
| Security Lead | | ☐ | |

**Release Tag Command** (execute only after all sign-offs):
```bash
git tag -s v<VERSION> -m "Release v<VERSION> — signed off by <names>"
git push origin v<VERSION>
```

---

## Appendix: Rollback Plan

If issues are discovered post-release:

1. **Image rollback**: See [docs/ROLLBACK_PLAYBOOK.md](ROLLBACK_PLAYBOOK.md)
2. **Migration rollback**: See [docs/ROLLBACK_PLAYBOOK.md](ROLLBACK_PLAYBOOK.md#4-migration-rollback-db-schema-change)
3. **Emergency contact**: See escalation matrix in rollback playbook

**Recovery Time Objectives**:
- Image rollback: < 5 minutes
- Migration rollback: < 15 minutes
- Full restore from backup: < 30 minutes (see [docs/BACKUP_RESTORE_RUNBOOK.md](BACKUP_RESTORE_RUNBOOK.md))
