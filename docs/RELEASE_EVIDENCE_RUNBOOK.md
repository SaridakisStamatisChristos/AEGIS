# AegisRun Release Evidence Runbook

Date: 2026-02-22  
Owner: Release Manager / SRE / Security

---

## Purpose

Provide a repeatable process to complete **P1-2** by collecting production release evidence from:

- `release-gate.yml` (release branch)
- `release.yml` (release tag)
- attached artifacts (SBOM, provenance, load threshold summary, canary and SLO gate evidence)

This runbook does **not** replace [docs/RELEASE_CHECKLIST.md](docs/RELEASE_CHECKLIST.md); it operationalizes the evidence collection needed to fill it.

---

## Prerequisites

- Release branch exists (e.g., `release/v1.0.x`)
- Tag/version selected (e.g., `v1.0.0`)
- GitHub Actions permissions to run/view workflows and artifacts
- Required secrets configured for release workflows (PyPI, npm, registry, etc.)
- Local repo synced to the release branch

---

## Step 1 — Create Evidence Pack Folder

Use the helper script to scaffold a timestamped evidence pack:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\prepare-release-evidence.ps1 -Version v1.0.0 -ReleaseBranch release/v1.0.x
```

Output directory:

```text
artifacts/releases/<UTC_TIMESTAMP>-v1.0.0/
```

Generated files:
- `release-evidence.md`
- `release-checklist.md` (copied from `docs/RELEASE_CHECKLIST.md`)

Optional automation runner (trigger workflows + watch + populate run metadata):

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\run-release-evidence.ps1 -Version v1.0.0 -ReleaseBranch release/v1.0.x -EvidenceDir .\artifacts\releases\<UTC_TIMESTAMP>-v1.0.0
```

Notes:
- Requires GitHub CLI (`gh`) installed and authenticated.
- By default this script also creates/pushes the release tag to trigger `release.yml`.
- Use `-SkipTagPush` when you want to run only `release-gate.yml`.

---

## Step 2 — Run Release-Gate Workflow (Release Branch)

1. Trigger `.github/workflows/release-gate.yml` on `release/*` branch.
2. Wait for `release-gate-verdict` to pass.
3. Record in `release-evidence.md`:
   - Workflow run URL
   - Run ID
   - UTC timestamp
   - Final status

Required green jobs (minimum):
- `api-unit-tests`
- `verifier-tests`
- `python-sdk-tests`
- `typescript-sdk-tests`
- `ui-tests`
- `security-sbom`
- `security-provenance`
- `load-test`
- `canary-health-gate`
- `ops-drill-cadence`
- `slo-escalation-gate`
- `release-gate-verdict`

Required artifacts to record:
- `sbom-release-gate`
- `load-test-results`

---

## Step 3 — Run Tag Release Workflow

1. Create/push signed release tag after gate pass.
2. Wait for `.github/workflows/release.yml` to complete.
3. Record in `release-evidence.md`:
   - Workflow run URL
   - Run ID
   - UTC timestamp
   - Final status

Required green jobs (minimum):
- `release-security-gate`
- `release-sbom`
- `build-and-push`
- `publish-python-sdk`
- `publish-typescript-sdk`
- `create-release`

Required artifacts to record:
- `release-sbom`
- verifier binaries attached in release

---

## Step 4 — Collect Provenance Evidence

For each artifact/image/package, record reference links and digest/subject where available:

- API image provenance attestation
- UI image provenance attestation
- Verifier image provenance attestation
- Python SDK artifact attestation
- TypeScript SDK package attestation

Populate the attestation section in `release-evidence.md`.

---

## Step 5 — Complete Sign-Off

1. Fill `release-checklist.md` sections 2, 3, 7, 8 with final evidence.
2. Confirm readiness score is `>= 90`.
3. Store final links and reviewer signatures.
4. Keep evidence pack under `artifacts/releases/...` and attach it to release notes.

---

## Go/No-Go Rules

GO only if all are true:
- release-gate workflow green
- release tag workflow green
- required SBOM/provenance artifacts present
- load/canary/SLO gate evidence present
- checklist approvals complete
- readiness score >= 90

Otherwise: NO-GO and open follow-up remediation issues.

---

## Notes

- This repository environment cannot execute GitHub-hosted workflows directly from local terminal.
- Workflow execution must be completed in GitHub Actions; this runbook defines what to capture for auditable completion.
