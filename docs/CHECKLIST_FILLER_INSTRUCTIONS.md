# Release Checklist Filler Instructions

**Reviewed:** 2026-09-25

Use this guide to fill [RELEASE_CHECKLIST.md](RELEASE_CHECKLIST.md).

## 1. Record candidate identity

Capture version, candidate ref, exact SHA, intended tag and changelog entry. Verify Python SDK, TypeScript SDK and UI versions equal the intended version.

## 2. Local verification

Linux/macOS:

```bash
make verify-all
```

Windows:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\verify-all.ps1
```

Record the generated `artifacts/verification/<timestamp>/` directory.

## 3. Release Gate

Record run URL/ID, head SHA, final verdict, security/SBOM, load threshold, canary, drill-cadence and SLO escalation results.

Use the final run for the exact candidate SHA; ignore superseded runs as final evidence.

## 4. Tag

```bash
git tag -a v<VERSION> -m "AegisRun v<VERSION>"
git push origin v<VERSION>
```

The tag target must already be contained in `main`.

If a GitHub Actions job created the tag using `GITHUB_TOKEN` and Release did not trigger, explicitly dispatch the `Release` workflow on the tag.

## 5. Release workflow

Record each stage:

- Release Preflight;
- Release Security Gate;
- Release SBOM;
- Build and Push Images;
- Publish Python SDK;
- Publish TypeScript SDK;
- Create Release.

Do not compress a one-channel failure into a misleading whole-release pass/fail.

## 6. Published artifacts

GitHub Release:

- URL/tag/SHA;
- verifier binaries;
- checksums;
- SBOMs.

Containers:

- names/digests;
- provenance/attestations.

Registries:

- PyPI status + install verification or exact pending reason;
- npm status + install verification or exact pending reason.

## 7. Deployment sections

Fill canary/production sections only for a real deployment. Capture DB backup, migration, image digests, health/readiness, monitoring window and rollback target.

## 8. v1.0.1 example

The core release/security/SBOM/image stages succeeded. PyPI/npm publishing authentication was absent, so those registry jobs failed. The original GitHub Release job was skipped by dependency, and a recovery workflow successfully published the GitHub Release/assets. Record such mixed outcomes explicitly.
