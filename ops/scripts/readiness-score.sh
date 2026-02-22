#!/usr/bin/env bash
#
# AegisRun Production Readiness Scoring Script
#
# Evaluates the current state of the repository against the 5-dimension
# readiness framework and produces a score out of 100.
#
# Usage:
#   ./readiness-score.sh [--json] [--ci]
#
# Flags:
#   --json    Output result as JSON (for CI consumption)
#   --ci      Exit non-zero if score < 90
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

JSON_OUTPUT=false
CI_MODE=false
for arg in "$@"; do
    case "$arg" in
        --json) JSON_OUTPUT=true ;;
        --ci)   CI_MODE=true ;;
    esac
done

# Colors (disabled for JSON output)
if [[ "${JSON_OUTPUT}" == "true" ]]; then
    RED=''; GREEN=''; YELLOW=''; BLUE=''; NC=''
else
    RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; NC='\033[0m'
fi

# ── Scoring State ────────────────────────────────────────────────────────────

declare -A SCORES
declare -A MAX_SCORES
declare -A DETAILS

DIMENSIONS=("Security" "Reliability" "Operability" "Quality" "Governance")
for d in "${DIMENSIONS[@]}"; do
    SCORES[$d]=0
    MAX_SCORES[$d]=20
    DETAILS[$d]=""
done

award() {
    local dim="$1"
    local points="$2"
    local reason="$3"
    SCORES[$dim]=$(( ${SCORES[$dim]} + points ))
    DETAILS[$dim]+="  +${points}: ${reason}\n"
}

deduct() {
    local dim="$1"
    local reason="$2"
    DETAILS[$dim]+="  -0: ${reason} (gap)\n"
}

# ── Security (20 points) ─────────────────────────────────────────────────────

cd "${REPO_ROOT}"

# S1: Security scanning workflow exists (4 pts)
if [[ -f ".github/workflows/security-scan.yml" ]]; then
    award "Security" 4 "Security scan CI workflow present"
else
    deduct "Security" "No security-scan.yml workflow"
fi

# S2: Mock OIDC production guard (4 pts)
if grep -q "ErrMockProviderInProduction" api/internal/auth/oidc.go 2>/dev/null; then
    award "Security" 4 "Mock OIDC production guard implemented"
else
    deduct "Security" "Mock OIDC production guard missing"
fi

# S3: Issuer/audience/token-age validation (4 pts)
if grep -q "MaxTokenAge" api/internal/auth/oidc.go 2>/dev/null && \
   grep -q "Audience" api/internal/auth/oidc.go 2>/dev/null; then
    award "Security" 4 "Token validation (issuer/audience/age) implemented"
else
    deduct "Security" "Token validation incomplete"
fi

# S4: No hardcoded credentials in compose (4 pts)
if grep -q '${' docker-compose.yml 2>/dev/null && \
   ! grep -qP 'password:\s+["\x27]?[a-zA-Z]' docker-compose.yml 2>/dev/null; then
    award "Security" 4 "No hardcoded credentials in docker-compose.yml"
else
    deduct "Security" "Hardcoded credentials found in docker-compose.yml"
fi

# S5: Network policies in k8s (4 pts)
if [[ -f "ops/k8s/network-policy.yaml" ]]; then
    award "Security" 4 "Kubernetes NetworkPolicy defined"
else
    deduct "Security" "No NetworkPolicy manifest"
fi

# ── Reliability (20 points) ──────────────────────────────────────────────────

# R1: Backup/restore runbook (4 pts)
if [[ -f "docs/BACKUP_RESTORE_RUNBOOK.md" ]]; then
    award "Reliability" 4 "Backup/restore runbook present"
else
    deduct "Reliability" "No backup/restore runbook"
fi

# R2: Prometheus alert rules (4 pts)
if [[ -f "ops/prometheus/rules/aegisrun-alerts.yml" ]]; then
    award "Reliability" 4 "Prometheus alert rules defined"
else
    deduct "Reliability" "No Prometheus alert rules"
fi

# R3: SLO dashboard (4 pts)
if [[ -f "ops/grafana/dashboards/slo-reliability-dashboard.json" ]]; then
    award "Reliability" 4 "Grafana SLO dashboard present"
else
    deduct "Reliability" "No SLO dashboard"
fi

# R4: Rollback playbook (4 pts)
if [[ -f "docs/ROLLBACK_PLAYBOOK.md" ]]; then
    award "Reliability" 4 "Deploy rollback playbook present"
else
    deduct "Reliability" "No rollback playbook"
fi

# R5: Backup script (4 pts)
if [[ -f "ops/scripts/backup-postgres.sh" ]]; then
    award "Reliability" 4 "Backup/restore script present"
else
    deduct "Reliability" "No backup script"
fi

# ── Operability (20 points) ─────────────────────────────────────────────────

# O1: Metrics endpoint + telemetry (4 pts)
if [[ -f "api/internal/telemetry/metrics.go" ]]; then
    award "Operability" 4 "Prometheus metrics instrumentation present"
else
    deduct "Operability" "No metrics instrumentation"
fi

# O2: Health & readiness probes (4 pts)
if grep -q "readinessProbe" ops/k8s/api-deployment.yaml 2>/dev/null && \
   grep -q "livenessProbe" ops/k8s/api-deployment.yaml 2>/dev/null; then
    award "Operability" 4 "Liveness and readiness probes configured"
else
    deduct "Operability" "Probes missing in deployment"
fi

# O3: HPA (4 pts)
if [[ -f "ops/k8s/api-hpa.yaml" ]]; then
    award "Operability" 4 "HorizontalPodAutoscaler configured"
else
    deduct "Operability" "No HPA manifest"
fi

# O4: Game-day drill template (4 pts)
if [[ -f "docs/GAMEDAY_DRILL_TEMPLATE.md" ]]; then
    award "Operability" 4 "Game-day drill template present"
else
    deduct "Operability" "No game-day drill template"
fi

# O5: CI deploy workflow (4 pts)
if [[ -f ".github/workflows/deploy.yml" ]]; then
    award "Operability" 4 "Automated deploy workflow present"
else
    deduct "Operability" "No deploy workflow"
fi

# ── Quality & Testing (20 points) ───────────────────────────────────────────

# Q1: API tests exist and pass (4 pts)
if [[ -d "api/internal/auth" ]] && ls api/internal/auth/*_test.go &>/dev/null; then
    award "Quality" 4 "API auth tests present"
else
    deduct "Quality" "No API auth tests"
fi

# Q2: E2E tests exist (4 pts)
if [[ -f "tests/e2e/full_flow_test.go" ]]; then
    award "Quality" 4 "E2E test suite present"
else
    deduct "Quality" "No E2E tests"
fi

# Q3: Load tests exist (4 pts)
if [[ -f "tests/load/locustfile.py" ]]; then
    award "Quality" 4 "Load test suite present"
else
    deduct "Quality" "No load tests"
fi

# Q4: SDK tests (4 pts)
SDK_PTS=0
if [[ -d "sdk/python/tests" ]]; then SDK_PTS=$((SDK_PTS + 2)); fi
if [[ -d "sdk/typescript/tests" ]]; then SDK_PTS=$((SDK_PTS + 2)); fi
if (( SDK_PTS > 0 )); then
    award "Quality" ${SDK_PTS} "SDK tests present (${SDK_PTS}/4)"
fi

# Q5: Config tests (4 pts)
if [[ -f "api/cmd/server/config_test.go" ]]; then
    award "Quality" 4 "Config validation tests present"
else
    deduct "Quality" "No config tests"
fi

# ── Release & Governance (20 points) ────────────────────────────────────────

# G1: Release gate workflow (4 pts)
if [[ -f ".github/workflows/release-gate.yml" ]]; then
    award "Governance" 4 "Release gate CI workflow present"
else
    deduct "Governance" "No release-gate workflow"
fi

# G2: Release checklist (4 pts)
if [[ -f "docs/RELEASE_CHECKLIST.md" ]]; then
    award "Governance" 4 "Signed release checklist template present"
else
    deduct "Governance" "No release checklist"
fi

# G3: Canary deployment infrastructure (4 pts)
if [[ -f "ops/k8s/api-canary-deployment.yaml" ]] && [[ -f "ops/scripts/canary-deploy.sh" ]]; then
    award "Governance" 4 "Canary deployment infrastructure present"
else
    deduct "Governance" "No canary deployment setup"
fi

# G4: Release freeze script (4 pts)
if [[ -f "ops/scripts/release-freeze.sh" ]]; then
    award "Governance" 4 "Release freeze script present"
else
    deduct "Governance" "No release freeze script"
fi

# G5: Production roadmap tracked (4 pts)
if [[ -f "PRODUCTION_ROADMAP.md" ]] && grep -q "COMPLETED" PRODUCTION_ROADMAP.md 2>/dev/null; then
    award "Governance" 4 "Production roadmap actively tracked"
else
    deduct "Governance" "Production roadmap not tracked"
fi

# ── Compute Total ────────────────────────────────────────────────────────────

TOTAL=0
for d in "${DIMENSIONS[@]}"; do
    TOTAL=$(( TOTAL + ${SCORES[$d]} ))
done
MAX_TOTAL=100

# ── Output ───────────────────────────────────────────────────────────────────

if [[ "${JSON_OUTPUT}" == "true" ]]; then
    cat << JSONEOF
{
  "total_score": ${TOTAL},
  "max_score": ${MAX_TOTAL},
  "pass": $([ ${TOTAL} -ge 90 ] && echo "true" || echo "false"),
  "dimensions": {
    "security": { "score": ${SCORES[Security]}, "max": ${MAX_SCORES[Security]} },
    "reliability": { "score": ${SCORES[Reliability]}, "max": ${MAX_SCORES[Reliability]} },
    "operability": { "score": ${SCORES[Operability]}, "max": ${MAX_SCORES[Operability]} },
    "quality": { "score": ${SCORES[Quality]}, "max": ${MAX_SCORES[Quality]} },
    "governance": { "score": ${SCORES[Governance]}, "max": ${MAX_SCORES[Governance]} }
  },
  "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}
JSONEOF
else
    echo ""
    echo -e "${BLUE}╔════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║        AegisRun Production Readiness Score                ║${NC}"
    echo -e "${BLUE}╚════════════════════════════════════════════════════════════╝${NC}"
    echo ""

    for d in "${DIMENSIONS[@]}"; do
        score=${SCORES[$d]}
        max=${MAX_SCORES[$d]}
        color="${GREEN}"
        if (( score < max * 80 / 100 )); then color="${YELLOW}"; fi
        if (( score < max * 60 / 100 )); then color="${RED}"; fi

        printf "  ${color}%-20s %2d / %2d${NC}\n" "$d" "$score" "$max"
        echo -e "${DETAILS[$d]}"
    done

    echo "  ────────────────────────────────"

    total_color="${GREEN}"
    if (( TOTAL < 90 )); then total_color="${YELLOW}"; fi
    if (( TOTAL < 70 )); then total_color="${RED}"; fi

    printf "  ${total_color}%-20s %2d / %2d${NC}\n" "TOTAL" "$TOTAL" "$MAX_TOTAL"
    echo ""

    if (( TOTAL >= 90 )); then
        echo -e "  ${GREEN}✅ PRODUCTION READY (score ≥ 90)${NC}"
    else
        echo -e "  ${YELLOW}⚠  NOT YET PRODUCTION READY (score < 90, need $(( 90 - TOTAL )) more points)${NC}"
    fi
    echo ""
fi

# ── CI gate ──────────────────────────────────────────────────────────────────

if [[ "${CI_MODE}" == "true" ]] && (( TOTAL < 90 )); then
    exit 1
fi
