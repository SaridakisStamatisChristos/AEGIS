# AegisRun Production Roadmap

**Updated:** 2026-09-25  
**Application baseline:** v1.0.1  
**Core hardening status:** **COMPLETED**  
**Current focus:** public distribution and deployment evidence

## Current state

AegisRun has completed the repository-level production hardening phases that previously covered test stabilization, contract/regression safety, supply-chain security, and release/operations gating.

The current core baseline includes:

- green normal CI and standalone Security Scan;
- hardened release preflight and security gates;
- SBOM and provenance generation;
- current dependency/security baseline;
- explicit client-IP trust handling;
- production migration Job flow;
- immutable release-image reuse for deployment;
- verifier release binaries and checksums;
- v1.0.1 GitHub Release;
- source-installable Python and TypeScript SDKs.

The remaining work is concentrated in distribution authentication and environment-specific operational proof.

## Completed phases

### Phase 0 — Quality-gate stabilization — COMPLETED

- API tests integrated into CI.
- Verifier tests integrated into CI.
- UI tests/build integrated into CI.
- TypeScript SDK tests/build integrated into CI.
- Root verification commands available.
- Workflow linting added.

### Phase 1 — Contract and regression safety — COMPLETED

- API ↔ TypeScript SDK contract tests.
- Python SDK contract parity coverage.
- Golden evidence-bundle verification.
- Policy schema/edge-case tests.
- Deterministic load-test fixtures.
- SDK step lifecycle event handling.

### Phase 2 — Supply-chain and security hardening — COMPLETED

- `govulncheck` for API and verifier.
- Trivy high/critical fail-closed behavior.
- CodeQL, secret scanning and license checks.
- SPDX SBOM generation.
- build-provenance attestations.
- current Go/Node/action baselines.
- dependency refresh for Go and npm production dependencies.
- API runtime upgraded from EOL Alpine 3.19 to Alpine 3.24.

### Phase 3 — Release and deployment hardening — COMPLETED

- one release workflow owns tag-release publishing;
- release tags must be valid SemVer and point to `main`;
- package/UI versions must match the tag;
- production deploy reuses immutable release-image digests;
- migration CLI is packaged into the API image;
- production migration executes as a one-shot Kubernetes Job before rollout;
- image rewrites target real Kustomize image names;
- trusted-proxy/client-IP model is explicit and fail-closed;
- v1.0.1 source/GitHub/container release path exercised.

## Active phase

### Phase 4 — Public distribution and release fault isolation — IN PROGRESS

#### P0 — Configure Python Trusted Publishing

- [ ] Configure PyPI Trusted Publishing/OIDC for the repository/workflow.
- [ ] Replace token-dependent Python publication with trusted publishing.
- [ ] Publish and verify the next Python SDK release from PyPI.

#### P0 — Configure npm publication

- [ ] Configure npm trusted publishing/OIDC if available for the package/workflow, otherwise provision the narrowest publish credential.
- [ ] Publish and verify the next TypeScript SDK release from npm.

#### P1 — Decouple release channels

- [ ] Make GitHub Release creation depend on successful core release artifacts rather than successful external package registries.
- [ ] Let PyPI and npm report independent publish status.
- [ ] Provide idempotent recovery jobs for a failed registry channel without rebuilding already-released images.

#### P1 — Distribution metadata

- [ ] On the next version bump, update package metadata to the canonical public repository URL.
- [ ] Verify package registry project pages, README rendering and source links.

## Documentation refresh — COMPLETED

The September 2026 documentation pass:

- updates the canonical repository URL and local-start instructions;
- documents the actual v1.0.1 distribution status;
- rebuilds the API reference from the current router/handlers;
- updates architecture and deployment toolchain versions;
- aligns release/evidence instructions with tag + manual-dispatch behavior;
- corrects evidence-bundle documentation to the implementation;
- refreshes operational review dates;
- archives the February repo-audit plan as historical evidence.

## Environment-specific production work

The repository cannot pre-complete these for every operator. Each staging/production environment must still:

- [ ] provide real OIDC configuration;
- [ ] provide DB secrets and TLS configuration;
- [ ] provide ingress/TLS;
- [ ] set the trusted-proxy topology correctly;
- [ ] configure backup storage and test restore;
- [ ] configure Alertmanager receiver URLs;
- [ ] validate SLO dashboards/alerts;
- [ ] execute canary/rollback drills;
- [ ] capture deployment-specific release evidence.

## Exit criteria for the next release

For the next patch/minor release:

1. normal CI green;
2. Security Scan green;
3. Release Gate green;
4. release preflight/security/SBOM/image build green;
5. GitHub Release created independently of registry availability;
6. Python publication green;
7. TypeScript publication green;
8. install verification from both public registries;
9. release evidence archived.

## Non-goals

The current roadmap does **not** call for a major rewrite of the API, policy engine, evidence model, verifier, or deployment architecture. New core work should be driven by concrete product requirements, measured performance limits, or verified defects rather than additional speculative hardening.
