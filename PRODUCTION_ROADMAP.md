# AegisRun Production Roadmap (2026-02-22 Refresh)

**Baseline**: `PRODUCTION_READINESS.md`  
**Current Readiness**: **89/100 (Pre-Production)**  
**Target Readiness**: **90+/100 (Production-Ready)**

---

## Current Reality

This roadmap supersedes mixed historical snapshots and is aligned to verified test results from this assessment:
- API tests: pass
- Verifier tests: pass
- UI tests: pass (post-fix)
- TypeScript SDK tests: pass (post-fix)
- Deep-analysis fixes applied: verifier release build entrypoints corrected (`./cmd/verify/main.go`) across Docker/workflows/Makefile
- Deep-analysis fixes applied: `ops/scripts/readiness-score.sh` runtime error risk fixed (top-level `local` misuse)
- Remaining evidence gap: Phase 2/3 mandatory jobs still require first green release-branch/tag run sign-off artifacts

---

## Phase Plan

### Phase 0 — Release Gate Stabilization (Week 1) [P0]

**Goal**: Make all quality gates executable and green.

**Deliverables**
- [x] Fix ambiguous UI test assertions in `ui/src/test/Dashboard.test.tsx` (use scoped or semantic queries).
- [x] Configure TypeScript SDK Jest pipeline (`ts-jest` or equivalent transform + ESM/TS compatibility).
- [x] Add module-aware root verification command (`make verify`) that runs:
  - `api`: `go test ./...`
  - `verifier`: `go test ./...`
  - `ui`: `npm run test -- --run`
  - `sdk/typescript`: `npm test -- --run`
- [x] Update CI to fail fast on any module quality gate failure.

**Exit Criteria**
- [x] All module tests green in local + CI gate wiring.
- [x] One-command verification works from repository root.

---

### Phase 1 — Contract & Regression Safety (Weeks 2-3) [P0/P1]

**Goal**: Prevent API/SDK and policy regressions.

**Deliverables**
- [x] Add API↔TypeScript SDK contract tests for run lifecycle and tool-call decision payloads.
- [x] Extend API contract tests to Python SDK parity checks.
- [x] Add golden evidence-bundle verification tests across API + verifier.
- [x] Add stricter schema validation tests for policy DSL edge cases.
- [x] Add deterministic fixtures for load-test critical paths.

**Exit Criteria**
- [x] Contract tests block incompatible API/SDK changes.
- [x] Evidence and policy regressions detected automatically in CI.

---

### Phase 2 — SOTA Supply Chain & Security Gates (Weeks 4-5) [P1]

**Goal**: Reach modern production trust baseline.

**Deliverables**
- [x] Generate SBOM artifacts for API/UI/SDK images and packages.
- [x] Add signed build provenance/attestation to release artifacts.
- [x] Enforce dependency CVE policy (block critical/high by policy threshold).
- [x] Add secret scanning + integrity checks as mandatory release jobs.

**Exit Criteria**
- [x] Release artifacts are traceable, signed, and policy-compliant.
- [x] Security gate is mandatory for release branches/tags.

---

### Phase 3 — Performance & Operations Gate (Weeks 6-7) [P1]

**Goal**: Convert load/ops assets into hard release criteria.

**Deliverables**
- [x] Add release-gate Locust thresholds (error %, p95/p99 latency, throughput floor). *(configured in CI; pending staging run sign-off)*
- [x] Add automated canary health scoring and rollback trigger integration. *(configured in CI/scripts; pending staging run sign-off)*
- [x] Add drill cadence evidence (backup restore + rollback + incident response). *(evidence artifact added; pending review sign-off)*
- [x] Wire SLO burn-rate alerts to on-call escalation policy. *(alert routing + policy docs wired; pending review sign-off)*

**Exit Criteria**
- [ ] Release fails when performance/SLO thresholds are not met. *(gate wiring complete; awaiting first staging gate run evidence)*
- [x] At least one successful staged canary + rollback drill recorded.

---

## Prioritized Backlog

1. [x] [P0] Stabilize UI tests.
2. [x] [P0] Repair TypeScript SDK test execution pipeline.
3. [x] [P0] Add root-level unified quality command.
4. [x] [P1] Introduce API/SDK contract tests.
5. [x] [P1] Enforce SBOM + signed provenance + vulnerability policy.
6. [x] [P1] Convert load/SLO checks into hard release gates.
7. [x] [P1] Implement SDK step lifecycle event emission TODOs (Python + TypeScript).

---

## Success Metrics

- Quality Gate Pass Rate: **100% across API/verifier/UI/SDK**
- SDK Test Executability: **from 0% to 100%**
- UI Test Flake Rate: **< 1% over 30 days**
- Release Security Compliance: **100% signed + SBOM-attested artifacts**
- Performance Gate Compliance: **100% release candidates meet latency/error thresholds**
- Readiness Score: **88 → 90+**

Phase 1 progress snapshot:
- Contract tests added in `sdk/typescript/tests/client.contract.test.ts`
- SDK envelope compatibility fix applied in `sdk/typescript/src/client.ts` for `runs/steps/events` list responses
- Python parity contract tests added in `sdk/python/tests/test_client_contract.py`
- Python envelope compatibility fix applied in `sdk/python/aegisrun/client.py` for `runs/steps/events` list responses
- API golden evidence-vector contract test added in `api/test/integration/ledger_test.go` (`TestLedger_GoldenEvidenceVectors`)
- Verifier golden bundle verification test added in `verifier/internal/verifier/golden_bundle_test.go` (`TestGoldenEvidenceBundle_VerifiesAcrossAllChecks`)
- Strict policy DSL schema edge tests added in `api/test/integration/policy_test.go` (`TestPolicy_ArgSchemaStrictEdges`, `TestPolicy_OutputSchemaStrictEdges`)
- Deterministic critical-path load fixtures added in `tests/load/fixtures.yaml`, wired in `tests/load/locustfile.py` via `AEGISRUN_LOAD_DETERMINISTIC` and `AEGISRUN_LOAD_SEED`

Phase 3 implementation snapshot (gates wired; pending staging run sign-off):
- Load threshold gate hardened in `.github/workflows/release-gate.yml` with `ops/scripts/validate-load-thresholds.py` (error %, p95, p99, throughput floor)
- Canary health scoring + rollback trigger integration added via `ops/scripts/canary-health-score.sh` and `ops/scripts/canary-deploy.sh`
- Drill cadence evidence gate added via `ops/scripts/verify-drill-cadence.sh` and staged drill evidence in `artifacts/drills/2026-02-22-staging-canary-rollback-drill.md`
- SLO burn-rate on-call escalation wiring added in `ops/prometheus/alertmanager.yml` and `docs/ONCALL_ESCALATION_POLICY.md`

Deep-analysis remaining gap snapshot:
- SDK step lifecycle event emission TODOs resolved in `sdk/python/aegisrun/step.py` and `sdk/typescript/src/step.ts`
- Alertmanager receiver URLs in `ops/prometheus/alertmanager.yml` are environment-driven and must be set via deployment configuration values for staging/production evidence

---

## Verification Evidence

Phase 0 is considered fully evidenced when these artifacts exist:

- Local command output from repository root:
  - `make verify`
  - `make verify-all` (writes timestamped summary + logs)
  - `powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\verify-all.ps1` (Windows)
- CI run screenshot/log links showing green jobs:
  - `.github/workflows/ci.yml`: `api-test`, `verifier-test`, `typescript-sdk-test`, `ui-build`, `unified-verify`
  - `.github/workflows/release-gate.yml`: `api-unit-tests`, `verifier-tests`, `python-sdk-tests`, `typescript-sdk-tests`, `ui-tests`, `release-gate-verdict`
- Recorded run IDs/URLs attached to release checklist and readiness review notes.
- Local evidence directory archived with release notes:
  - `artifacts/verification/<UTC_TIMESTAMP>/`
  - Current captured evidence: `artifacts/verification/20260222T090503Z/`

Phase 2 is considered fully evidenced when these artifacts exist:

- Release branch gate (`.github/workflows/release-gate.yml`) successful run with:
  - `security-deps`
  - `security-secrets`
  - `security-containers`
  - `security-sbom`
  - `security-provenance`
  - `release-gate-verdict`
- Release tag workflow (`.github/workflows/release.yml`) successful run with:
  - `release-security-gate`
  - `release-sbom`
  - `build-and-push`
- Artifacts attached to workflow runs:
  - `sbom-release-gate` (release branch gate)
  - `release-sbom` (tag release)
- Build provenance attestations present for:
  - API/UI/verifier images
  - Python SDK distribution artifacts
  - TypeScript SDK package artifact (`*.tgz`)
- Recorded run IDs/URLs linked in release checklist and readiness review notes.

Phase 2 sign-off table (fill during release-readiness review):

| Workflow | Run URL | Date (UTC) | Reviewer | Result |
|---|---|---|---|---|
| `release-gate.yml` |  |  |  | ☐ Pass / ☐ Fail |
| `release.yml` |  |  |  | ☐ Pass / ☐ Fail |
| SBOM artifacts (`sbom-release-gate`, `release-sbom`) |  |  |  | ☐ Verified / ☐ Missing |
| Provenance attestations (images + SDK artifacts) |  |  |  | ☐ Verified / ☐ Missing |

Phase 3 sign-off table (fill during staging/release-readiness review):

| Check | Run URL / Evidence | Date (UTC) | Reviewer | Result |
|---|---|---|---|---|
| `load-test` threshold gate (`error`, `p95`, `p99`, `rps`) |  |  |  | ☐ Pass / ☐ Fail |
| `canary-health-gate` rollback-trigger validation |  |  |  | ☐ Pass / ☐ Fail |
| `ops-drill-cadence` evidence recency check |  |  |  | ☐ Pass / ☐ Fail |
| `slo-escalation-gate` burn-rate escalation wiring check |  |  |  | ☐ Pass / ☐ Fail |
| Staged canary + rollback drill record | `artifacts/drills/2026-02-22-staging-canary-rollback-drill.md` | 2026-02-22 |  | ☐ Verified / ☐ Missing |

---

## Go-Live Gate

Production go-live is approved only when:
- [x] Phase 0 complete
- [x] Phase 1 complete (contract protection active)
- [x] Phase 2 security gates enabled for release branches
- [ ] Phase 3 performance/ops gates passing in staging
- [ ] Final readiness score is **90+** with evidence attached
