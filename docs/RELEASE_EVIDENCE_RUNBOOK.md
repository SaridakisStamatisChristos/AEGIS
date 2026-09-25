# AegisRun Release Evidence Runbook

**Reviewed:** 2026-09-25  
**Owner:** Release Manager / SRE / Security

## Purpose

This runbook defines evidence to capture for an AegisRun release and complements [RELEASE_CHECKLIST.md](RELEASE_CHECKLIST.md).

Track separately:

1. repository/release-gate validation;
2. core release artifacts;
3. GitHub Release;
4. PyPI/npm;
5. actual environment deployment.

## 1. Prerequisites

Before release:

- choose a SemVer version such as `v1.0.2`;
- ensure the target commit is contained in `main`;
- ensure Python SDK, TypeScript SDK and UI versions equal the tag without `v`;
- update `CHANGELOG.md`;
- verify CI, Security Scan and Release Gate;
- configure publishing identities/credentials where required;
- verify GitHub Actions package/attestation/content permissions.

Use npm for npm version/lockfile changes; do not hand-edit lockfiles.

## 2. Evidence pack

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\prepare-release-evidence.ps1 -Version v<VERSION> -ReleaseBranch release/v<MAJOR>.<MINOR>.x
```

Expected path:

```text
artifacts/releases/<UTC_TIMESTAMP>-v<VERSION>/
```

Historical packs are immutable records; do not rewrite them when process changes.

## 3. Release Gate evidence

Capture:

- run URL/ID;
- exact candidate SHA;
- final verdict;
- tests;
- security/SBOM;
- load/canary/SLO gate results;
- artifact references.

Superseded/cancelled runs caused by later pushes are not final-candidate failures. Record the final run for the exact candidate SHA.

## 4. Tag and Release workflow

Create the tag only after validation:

```bash
git tag -a v<VERSION> -m "AegisRun v<VERSION>"
git push origin v<VERSION>
```

`release.yml` also supports `workflow_dispatch`. If another Actions workflow creates the tag using the default `GITHUB_TOKEN`, GitHub may suppress a recursive tag-triggered workflow; explicitly dispatch Release on the existing tag.

Record every job separately:

- Release Preflight;
- Release Security Gate;
- Release SBOM;
- Build and Push Images;
- Publish Python SDK;
- Publish TypeScript SDK;
- Create Release.

## 5. Core artifacts

Record immutable image digests, SBOM references and provenance attestations. Deployment must reuse published release artifacts rather than rebuild them.

## 6. Registry evidence

For PyPI and npm independently record one of:

- **published**;
- **not configured**;
- **failed**, with sanitized error and recovery plan.

Never include credentials/tokens in evidence.

## 7. GitHub Release evidence

Record:

- release URL/tag/target SHA;
- verifier binaries;
- `checksums.txt`;
- SBOM files;
- release notes/changelog link.

If core artifacts succeeded but the normal GitHub Release job was blocked by a registry failure, use an auditable recovery workflow on the existing tag/artifacts. Do not rewrite history by moving the tag.

## 8. Deployment evidence

For each actual environment record:

- environment;
- image digests;
- migration result;
- DB backup;
- trusted-proxy configuration;
- OIDC/TLS verification;
- canary/smoke outcome;
- SLO/alerts;
- rollback target.

## 9. v1.0.1 historical example

v1.0.1 had:

- preflight: success;
- security gate: success;
- SBOM: success;
- image build/push: success;
- provenance attestations: success;
- Python build/attestation: success, PyPI authentication absent;
- TypeScript build/attestation: success, npm authentication absent;
- normal Create Release: skipped because it depended on both registry jobs;
- recovery GitHub Release: success;
- public release assets: verifier binaries, checksums, four SBOM files.

This demonstrates why release evidence must report distribution channels independently.

## 10. Final checklist

A release record should answer:

- exact commit/tag;
- gates passed;
- image digests;
- SBOM locations;
- attestation locations;
- verifier/checksums;
- PyPI status;
- npm status;
- GitHub Release status;
- deployment status;
- migration/backup;
- rollback target.

## 11. Retention

Keep evidence under `artifacts/releases/` or another durable audit store. Preserve old packs unchanged.
