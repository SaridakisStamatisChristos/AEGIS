# Release Checklist Filler Instructions

Use this guide to gather and paste evidence into `docs/RELEASE_CHECKLIST.md`.

---

## 1) Run local verification evidence (Windows)

From repository root:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\verify-all.ps1
```

Record:
- Evidence directory path (example: `artifacts/verification/20260222T090503Z/`)
- Any relevant log links/files for API, verifier, UI, TypeScript SDK

---

## 2) Trigger release-branch gate workflow

Create and push a release branch:

```powershell
git checkout -b release/test-gate
git push -u origin release/test-gate
```

This triggers `.github/workflows/release-gate.yml` automatically.

In GitHub Actions, open the run and record:
- Run URL
- Run ID

Required jobs to mark as passed:
- `security-sbom`
- `security-provenance`
- `load-test`
- `canary-health-gate`
- `ops-drill-cadence`
- `slo-escalation-gate`
- `release-gate-verdict`

Also record artifact evidence:
- `sbom-release-gate` artifact link

---

## 3) Trigger release-tag workflow

Create and push a release tag from the validated commit:

```powershell
git tag v0.0.0-rc1
git push origin v0.0.0-rc1
```

This triggers `.github/workflows/release.yml`.

In GitHub Actions, open the run and record:
- Run URL
- Run ID

Required jobs to mark as passed:
- `release-security-gate`
- `release-sbom`
- `build-and-push`
- `publish-python-sdk`
- `publish-typescript-sdk`
- `create-release`

Also record artifact evidence:
- `release-sbom` artifact link

---

## 4) Attestation and on-call config evidence

Record evidence links/screenshots for:
- Image attestations: API, UI, verifier
- SDK artifact attestations: Python + TypeScript package
- Alertmanager environment values configured (without exposing secrets):
  - `ALERTMANAGER_WEBHOOK_PRIMARY_URL`
  - `ALERTMANAGER_WEBHOOK_SECONDARY_URL`
  - `ALERTMANAGER_WEBHOOK_ESCALATION_URL`

Template file for endpoint values:
- `ops/prometheus/alertmanager.env.example`

---

## 5) Paste into `docs/RELEASE_CHECKLIST.md`

Fill these sections:
- **2. CI Pipeline Results** (run URL + run ID + job statuses)
- **2. Security Checks**
- **Operations Gate Checks** (`2.17`, `2.18`, `2.19`)
- **4. Canary Deployment** (if run in staging)
- Any notes in **Final Sign-Off**

---

## Quick Copy Block (for PR description)

```markdown
### Release Evidence
- Local verify-all evidence dir: <path>
- Release-gate run URL: <url> (Run ID: <id>)
- Release run URL: <url> (Run ID: <id>)
- SBOM artifact links: <release-gate sbom>, <release sbom>
- Attestation links: <api/ui/verifier/python/ts>
- Alertmanager env config evidence (masked): <link/screenshot>
```
