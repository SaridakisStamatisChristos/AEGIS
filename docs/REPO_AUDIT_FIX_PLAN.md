# AegisRun Repo Audit Fix Plan

Version: 1.0  
Date: 2026-02-22  
Status: Draft for execution

---

## 1) Goal

Close the production-readiness gaps found in the deep repository audit and move readiness from 88/100 to 90+/100 with verifiable evidence.

---

## 2) Current State Summary

Audit outcome:
- Grade: B+ (88/100)
- Decision: Conditional No-Go for production GA until P0 and evidence tasks are complete

Validated strengths:
- Core test suites pass (API, verifier, UI, TypeScript SDK)
- Strong release-gate workflow coverage
- Good runtime hardening in Kubernetes and auth configuration checks

Confirmed gaps to fix:
1. Broken verifier command path in root automation
2. Broken documentation links in README
3. Windows readiness scoring and operator workflow portability gap
4. Missing first green release-branch and release-tag evidence pack for final sign-off
5. No automated markdown link-drift prevention in CI

---

## 3) Prioritized Remediation Plan

## P0 (Same day)

### P0-1: Fix verify-bundle target path

Problem:
- Root task points to verifier/main.go, but verifier entrypoint is cmd/verify/main.go.

Actions:
- Update Makefile verify-bundle target to run the correct verifier entrypoint.

Acceptance criteria:
- Running the root verify-bundle command invokes the verifier CLI successfully.
- Command prints CLI usage or verifies a provided bundle without file-not-found errors.

Verification:
- From verifier: go run ./cmd/verify/main.go
- From root: run verify-bundle with a valid bundle path

Owner:
- Platform Engineering

Effort:
- 0.5 hour

---

### P0-2: Fix broken README documentation links

Problem:
- README references docs that are missing in the repository.

Actions:
- Update README documentation section to only include existing docs, or add the missing docs immediately if they are required.

Acceptance criteria:
- Every README documentation link resolves to an existing file.
- No dead links remain in the top-level docs section.

Verification:
- Manual link check in editor
- Optional: run a markdown link checker locally

Owner:
- API/UI Maintainers + Docs Owner

Effort:
- 0.5 to 1 hour

---

## P1 (1 to 2 days)

### P1-1: Add Windows-native readiness scoring path

Problem:
- Primary readiness scoring workflow assumes Bash and make availability, which is inconsistent in Windows-only environments.

Actions:
- Implement a PowerShell readiness scoring script equivalent to readiness-score.sh.
- Add usage instructions to README and operations docs.
- Keep output compatibility with JSON mode for CI and local evidence.

Acceptance criteria:
- Readiness scoring runs on Windows PowerShell without WSL or make.
- Output includes total score, dimension scores, and pass/fail threshold.

Verification:
- PowerShell execution from repo root succeeds and returns score output.

Owner:
- Platform Engineering

Effort:
- 3 to 5 hours

---

### P1-2: Produce first full release evidence cycle

Problem:
- Gates are wired, but final sign-off needs real successful run evidence for release branch and release tag.

Actions:
- Execute one green release-gate run on release/* branch.
- Execute one green release.yml run on signed tag.
- Archive and link artifacts (SBOM, provenance, load-threshold summary, canary gate, SLO escalation gate).
- Attach evidence links in release checklist.

Acceptance criteria:
- Release gate and release workflows both green.
- Required artifacts present and discoverable.
- Sign-off table filled with run URLs and reviewer approvals.

Verification:
- Workflow run pages + artifact download checks
- Release checklist completed

Owner:
- Release Manager + Security + SRE

Effort:
- 4 to 8 hours (depends on environment and review latency)

---

## P2 (Next sprint)

### P2-1: Add CI guard for markdown link integrity

Problem:
- Documentation link drift is currently undetected until manual review.

Actions:
- Add a markdown link checker job to CI for docs and README.
- Fail CI on broken internal links.

Acceptance criteria:
- Pull requests fail when markdown links are broken.
- Existing docs baseline passes.

Verification:
- Introduce a known-bad link in a test branch and confirm CI failure.

Owner:
- DevEx / Platform

Effort:
- 2 to 4 hours

---

## 4) Execution Order and Timeline

Day 0:
- Complete P0-1 and P0-2
- Re-run core verification and confirm no regressions

Day 1 to 2:
- Complete P1-1
- Execute P1-2 evidence cycle and record artifacts

Next sprint:
- Complete P2-1 and keep as permanent gate

---

## 5) Risk and Rollback Notes

- P0 changes are low-risk and mostly operational/docs corrections.
- P1 evidence run may fail due to environment or secrets configuration; if so, resolve in release branch before tag.
- P2 CI tightening may initially fail due to legacy doc drift; fix links before enforcing required status.

---

## 6) Definition of Done

This plan is complete when all of the following are true:
- P0 and P1 items are complete and verified
- Release evidence is attached and review-signed
- Readiness score reaches 90+/100
- Production go/no-go status can be switched to Go with recorded evidence

---

## 7) Tracking Table

| Work Item | Priority | Owner | Status | Target Date | Evidence Link |
|---|---|---|---|---|---|
| P0-1 Verify-bundle path fix | P0 | Platform Eng | Completed (2026-02-22) | 2026-02-22 | Makefile |
| P0-2 README link fixes | P0 | Docs + Maintainers | Completed (2026-02-22) | 2026-02-22 | README.md |
| P1-1 Windows readiness scoring | P1 | Platform Eng | Completed (2026-02-22) | 2026-02-24 | ops/scripts/readiness-score.ps1 |
| P1-2 Release evidence cycle | P1 | Release/SRE/Security | In progress (evidence pack scaffolded 2026-02-22) | 2026-02-24 | docs/RELEASE_EVIDENCE_RUNBOOK.md; ops/scripts/prepare-release-evidence.ps1 |
| P2-1 CI markdown link checker | P2 | DevEx | Not started | Next sprint | |
