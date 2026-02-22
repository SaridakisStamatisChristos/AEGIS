#!/usr/bin/env bash
#
# AegisRun Release Branch Freeze Script
#
# Creates a release branch from main, runs local pre-flight checks,
# and prepares the branch for the release-gate CI pipeline.
#
# Usage:
#   ./release-freeze.sh <version>
#
# Example:
#   ./release-freeze.sh 1.3.0
#
# What it does:
#   1. Validates the version string
#   2. Creates release/<version> branch from main
#   3. Runs local build + test + vet checks
#   4. Generates a release manifest (JSON) capturing the freeze point
#   5. Commits the manifest and pushes (triggers release-gate.yml)
#
set -euo pipefail

# ── Configuration ──────────────────────────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
MANIFEST_FILE="${REPO_ROOT}/RELEASE_MANIFEST.json"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log()  { echo -e "${BLUE}[release]${NC} $*"; }
ok()   { echo -e "${GREEN}  ✓${NC} $*"; }
warn() { echo -e "${YELLOW}  ⚠${NC} $*"; }
fail() { echo -e "${RED}  ✗${NC} $*"; exit 1; }

# ── Input Validation ──────────────────────────────────────────────────────────

VERSION="${1:-}"
if [[ -z "${VERSION}" ]]; then
    echo "Usage: $0 <version>"
    echo "  e.g., $0 1.3.0"
    exit 1
fi

# Validate semver format
if ! [[ "${VERSION}" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?$ ]]; then
    fail "Invalid version format: ${VERSION} (expected semver, e.g. 1.3.0 or 1.3.0-rc.1)"
fi

BRANCH="release/${VERSION}"
TAG="v${VERSION}"

log "AegisRun Release Freeze — v${VERSION}"
log "=================================="

# ── Pre-Flight Checks ────────────────────────────────────────────────────────

log "Pre-flight checks..."

# Must be in repo root
cd "${REPO_ROOT}"

# Must be on main
CURRENT_BRANCH="$(git branch --show-current)"
if [[ "${CURRENT_BRANCH}" != "main" ]]; then
    fail "Must be on 'main' branch (currently on '${CURRENT_BRANCH}')"
fi

# Must be clean
if [[ -n "$(git status --porcelain)" ]]; then
    fail "Working tree is dirty. Commit or stash changes first."
fi

# Must be up to date
git fetch origin main --quiet
LOCAL="$(git rev-parse HEAD)"
REMOTE="$(git rev-parse origin/main)"
if [[ "${LOCAL}" != "${REMOTE}" ]]; then
    fail "Local main (${LOCAL:0:8}) is behind origin/main (${REMOTE:0:8}). Run 'git pull' first."
fi

# Branch must not already exist
if git show-ref --verify --quiet "refs/heads/${BRANCH}" 2>/dev/null || \
   git show-ref --verify --quiet "refs/remotes/origin/${BRANCH}" 2>/dev/null; then
    fail "Branch '${BRANCH}' already exists"
fi

# Tag must not already exist
if git show-ref --verify --quiet "refs/tags/${TAG}" 2>/dev/null; then
    fail "Tag '${TAG}' already exists"
fi

ok "Pre-flight checks passed"

# ── Local Build & Test ────────────────────────────────────────────────────────

log "Running local build & test validation..."

CHECKS_PASSED=0
CHECKS_TOTAL=0

run_check() {
    local name="$1"
    shift
    CHECKS_TOTAL=$((CHECKS_TOTAL + 1))
    if "$@" > /dev/null 2>&1; then
        ok "${name}"
        CHECKS_PASSED=$((CHECKS_PASSED + 1))
    else
        warn "${name} — FAILED (non-blocking at freeze time)"
    fi
}

# Go API
run_check "API: go build"    bash -c "cd api && go build ./..."
run_check "API: go vet"      bash -c "cd api && go vet ./..."
run_check "API: go test"     bash -c "cd api && go test -count=1 -short ./..."

# Verifier
run_check "Verifier: build"  bash -c "cd verifier && go build ./..."
run_check "Verifier: test"   bash -c "cd verifier && go test -count=1 -short ./..."

# UI (if node_modules present)
if [[ -d "ui/node_modules" ]]; then
    run_check "UI: build"    bash -c "cd ui && npm run build"
else
    warn "UI: skipped (no node_modules — run 'npm ci' in ui/ first)"
    CHECKS_TOTAL=$((CHECKS_TOTAL + 1))
fi

# Python SDK (if installed)
if command -v pytest &>/dev/null && [[ -d "sdk/python" ]]; then
    run_check "Python SDK: test" bash -c "cd sdk/python && pytest -x --timeout=30"
else
    warn "Python SDK: skipped (pytest not available)"
    CHECKS_TOTAL=$((CHECKS_TOTAL + 1))
fi

log "Local checks: ${CHECKS_PASSED}/${CHECKS_TOTAL} passed"

if (( CHECKS_PASSED < CHECKS_TOTAL )); then
    warn "Some local checks failed — CI release-gate will provide the authoritative result."
fi

# ── Create Release Branch ────────────────────────────────────────────────────

log "Creating branch '${BRANCH}'..."
git checkout -b "${BRANCH}"
ok "Branch created"

# ── Generate Release Manifest ────────────────────────────────────────────────

log "Generating release manifest..."

FREEZE_SHA="$(git rev-parse HEAD)"
FREEZE_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
FREEZE_BY="$(git config user.name) <$(git config user.email)>"

# Count components
API_TEST_COUNT="$(cd api && go test -list '.*' ./... 2>/dev/null | grep -c '^Test' || echo '?')"
MIGRATION_COUNT="$(ls api/migrations/*.up.sql 2>/dev/null | wc -l || echo '?')"

cat > "${MANIFEST_FILE}" << EOF
{
  "version": "${VERSION}",
  "tag": "${TAG}",
  "branch": "${BRANCH}",
  "freeze": {
    "sha": "${FREEZE_SHA}",
    "timestamp": "${FREEZE_TIME}",
    "frozen_by": "${FREEZE_BY}"
  },
  "components": {
    "api": {
      "go_version": "1.23",
      "test_count": "${API_TEST_COUNT}",
      "migration_count": "${MIGRATION_COUNT}"
    },
    "ui": {
      "node_version": "20",
      "framework": "react+vite"
    },
    "verifier": {
      "go_version": "1.23"
    },
    "sdk_python": {
      "python_version": "3.11"
    },
    "sdk_typescript": {
      "node_version": "20"
    }
  },
  "release_gate": {
    "workflow": ".github/workflows/release-gate.yml",
    "required_checks": [
      "build-api",
      "build-verifier",
      "build-ui",
      "build-docker",
      "api-unit-tests",
      "verifier-tests",
      "python-sdk-tests",
      "typescript-sdk-tests",
      "ui-tests",
      "security-go",
      "security-deps",
      "security-secrets",
      "security-containers",
      "e2e-tests",
      "migration-roundtrip",
      "load-test"
    ],
    "status": "pending"
  },
  "canary": {
    "cycles_required": 2,
    "cycles_completed": 0,
    "abort_thresholds": {
      "error_rate_percent": 5,
      "p99_latency_ms": 2000,
      "pod_restart_count": 2,
      "readiness_failure_seconds": 60
    }
  },
  "checklist": {
    "status": "not_started",
    "file": "docs/RELEASE_CHECKLIST.md"
  }
}
EOF

ok "Release manifest written to RELEASE_MANIFEST.json"

# ── Commit & Push ────────────────────────────────────────────────────────────

log "Committing release manifest..."
git add "${MANIFEST_FILE}"
git commit -m "release: freeze v${VERSION}

Branch: ${BRANCH}
SHA: ${FREEZE_SHA}
Frozen by: ${FREEZE_BY}

This commit triggers the release-gate CI pipeline.
All required checks must pass before tagging."

log "Pushing '${BRANCH}' to origin..."
git push -u origin "${BRANCH}"

ok "Branch pushed — release-gate CI should start automatically"

# ── Summary ──────────────────────────────────────────────────────────────────

echo ""
log "╔═══════════════════════════════════════════════════════════╗"
log "║  Release v${VERSION} frozen successfully                  "
log "║                                                           "
log "║  Branch:  ${BRANCH}                                       "
log "║  SHA:     ${FREEZE_SHA:0:8}                               "
log "║  Time:    ${FREEZE_TIME}                                  "
log "║                                                           "
log "║  Next steps:                                              "
log "║  1. Wait for release-gate CI to pass                      "
log "║  2. Fill out docs/RELEASE_CHECKLIST.md                    "
log "║  3. Run canary deployment (ops/scripts/canary-deploy.sh)  "
log "║  4. After 2 successful canary cycles, tag: git tag ${TAG} "
log "╚═══════════════════════════════════════════════════════════╝"
