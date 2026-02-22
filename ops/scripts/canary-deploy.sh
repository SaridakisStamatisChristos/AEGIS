#!/usr/bin/env bash
#
# AegisRun Canary Deployment & Validation Script
#
# Deploys a canary replica with the release image, routes partial traffic,
# monitors abort thresholds, and records cycle results.
#
# Usage:
#   ./canary-deploy.sh <command> [options]
#
# Commands:
#   deploy <image-tag>    Deploy canary with the given image tag
#   promote               Promote canary image to full deployment
#   abort                 Remove canary and revert to stable
#   status                Show canary status and metrics
#   validate              Run a full validation cycle (deploy + monitor + record)
#
# Environment:
#   NAMESPACE             Kubernetes namespace (default: aegisrun)
#   PROMETHEUS_URL        Prometheus query URL (default: http://prometheus:9090)
#   MONITOR_DURATION      Monitoring duration in seconds (default: 900 = 15 min)
#   MONITOR_INTERVAL      Check interval in seconds (default: 30)
#   TRAFFIC_WEIGHT        Percentage of traffic to route to canary (default: 10)
#
set -euo pipefail

# ── Configuration ──────────────────────────────────────────────────────────────

NAMESPACE="${NAMESPACE:-aegisrun}"
PROMETHEUS_URL="${PROMETHEUS_URL:-http://prometheus:9090}"
MONITOR_DURATION="${MONITOR_DURATION:-900}"
MONITOR_INTERVAL="${MONITOR_INTERVAL:-30}"
TRAFFIC_WEIGHT="${TRAFFIC_WEIGHT:-10}"
CANARY_DEPLOY="aegisrun-api-canary"
STABLE_DEPLOY="aegisrun-api"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
MANIFEST_FILE="${REPO_ROOT}/RELEASE_MANIFEST.json"
CANARY_SCORE_SCRIPT="${SCRIPT_DIR}/canary-health-score.sh"

# Abort thresholds
ABORT_ERROR_RATE=5        # percent
ABORT_P99_LATENCY=2000    # milliseconds
ABORT_POD_RESTARTS=2
ABORT_READINESS_FAIL=60   # seconds

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log()  { echo -e "${BLUE}[canary]${NC} $(date -u +%H:%M:%SZ) $*"; }
ok()   { echo -e "${GREEN}  ✓${NC} $*"; }
warn() { echo -e "${YELLOW}  ⚠${NC} $*"; }
fail() { echo -e "${RED}  ✗${NC} $*"; }
die()  { fail "$@"; exit 1; }

# ── Prometheus Query Helper ──────────────────────────────────────────────────

prom_query() {
    local query="$1"
    local result
    result=$(curl -sf --max-time 10 \
        "${PROMETHEUS_URL}/api/v1/query" \
        --data-urlencode "query=${query}" \
        2>/dev/null | python3 -c "
import sys, json
data = json.load(sys.stdin)
if data.get('status') == 'success' and data['data']['result']:
    print(data['data']['result'][0]['value'][1])
else:
    print('0')
" 2>/dev/null || echo "0")
    echo "${result}"
}

# ── Deploy Canary ────────────────────────────────────────────────────────────

cmd_deploy() {
    local image_tag="${1:?Usage: canary-deploy.sh deploy <image-tag>}"
    local image="ghcr.io/aegisrun/aegisrun/api:${image_tag}"

    log "Deploying canary with image: ${image}"

    # Apply canary manifests
    kubectl apply -f "${SCRIPT_DIR}/../k8s/api-canary-deployment.yaml" -n "${NAMESPACE}"
    kubectl apply -f "${SCRIPT_DIR}/../k8s/api-canary-service.yaml" -n "${NAMESPACE}"

    # Set the canary image
    kubectl set image "deployment/${CANARY_DEPLOY}" "api=${image}" -n "${NAMESPACE}"

    # Wait for rollout
    log "Waiting for canary pod to be ready..."
    if kubectl rollout status "deployment/${CANARY_DEPLOY}" -n "${NAMESPACE}" --timeout=120s; then
        ok "Canary pod is ready"
    else
        die "Canary pod failed to become ready"
    fi

    # Verify health
    local canary_pod
    canary_pod=$(kubectl get pods -n "${NAMESPACE}" -l "app.kubernetes.io/variant=canary" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    if [[ -n "${canary_pod}" ]]; then
        log "Canary pod: ${canary_pod}"
        kubectl exec -n "${NAMESPACE}" "${canary_pod}" -- wget -qO- http://localhost:8080/health 2>/dev/null && ok "Health check passed" || warn "Health check via exec failed (may need port-forward)"
    fi

    log "Canary deployed. Run 'canary-deploy.sh validate' to start monitoring."
}

# ── Validate (Monitor) Cycle ────────────────────────────────────────────────

cmd_validate() {
    local cycle_num="${1:-1}"
    local start_time
    start_time="$(date +%s)"
    local end_time=$((start_time + MONITOR_DURATION))
    local checks_passed=0
    local checks_total=0
    local abort=false
    local latest_error_rate="0"
    local latest_p99_latency="0"
    local latest_restarts="0"
    local latest_readiness_ok="true"

    log "═══════════════════════════════════════════════════════════"
    log "  CANARY VALIDATION CYCLE #${cycle_num}"
    log "  Duration: ${MONITOR_DURATION}s ($(( MONITOR_DURATION / 60 )) min)"
    log "  Check interval: ${MONITOR_INTERVAL}s"
    log "  Abort thresholds:"
    log "    Error rate:      > ${ABORT_ERROR_RATE}%"
    log "    P99 latency:     > ${ABORT_P99_LATENCY}ms"
    log "    Pod restarts:    > ${ABORT_POD_RESTARTS}"
    log "    Readiness fail:  > ${ABORT_READINESS_FAIL}s"
    log "═══════════════════════════════════════════════════════════"

    while (( $(date +%s) < end_time )); do
        local elapsed=$(( $(date +%s) - start_time ))
        local remaining=$(( end_time - $(date +%s) ))
        checks_total=$((checks_total + 1))

        log "Check #${checks_total} (${elapsed}s elapsed, ${remaining}s remaining)"

        # ─ Check 1: Error rate ─
        local error_rate
        error_rate=$(prom_query 'sum(rate(http_requests_total{variant="canary",code=~"5.."}[2m])) / sum(rate(http_requests_total{variant="canary"}[2m])) * 100')
        latest_error_rate="${error_rate}"
        if (( $(echo "${error_rate} > ${ABORT_ERROR_RATE}" | bc -l 2>/dev/null || echo 0) )); then
            fail "Error rate ${error_rate}% exceeds threshold ${ABORT_ERROR_RATE}%"
            abort=true
        else
            ok "Error rate: ${error_rate}%"
        fi

        # ─ Check 2: P99 latency ─
        local p99_latency
        p99_latency=$(prom_query 'histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket{variant="canary"}[2m])) by (le)) * 1000')
        latest_p99_latency="${p99_latency}"
        if (( $(echo "${p99_latency} > ${ABORT_P99_LATENCY}" | bc -l 2>/dev/null || echo 0) )); then
            fail "P99 latency ${p99_latency}ms exceeds threshold ${ABORT_P99_LATENCY}ms"
            abort=true
        else
            ok "P99 latency: ${p99_latency}ms"
        fi

        # ─ Check 3: Pod restarts ─
        local restarts
        restarts=$(kubectl get pods -n "${NAMESPACE}" -l "app.kubernetes.io/variant=canary" \
            -o jsonpath='{.items[0].status.containerStatuses[0].restartCount}' 2>/dev/null || echo "0")
        latest_restarts="${restarts}"
        if (( restarts > ABORT_POD_RESTARTS )); then
            fail "Pod restarts (${restarts}) exceeds threshold (${ABORT_POD_RESTARTS})"
            abort=true
        else
            ok "Pod restarts: ${restarts}"
        fi

        # ─ Check 4: Readiness ─
        local ready
        ready=$(kubectl get pods -n "${NAMESPACE}" -l "app.kubernetes.io/variant=canary" \
            -o jsonpath='{.items[0].status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo "Unknown")
        if [[ "${ready}" != "True" ]]; then
            latest_readiness_ok="false"
            warn "Canary pod not ready (status: ${ready})"
            # Only abort if it's been not-ready for too long
            local not_ready_since
            not_ready_since=$(kubectl get pods -n "${NAMESPACE}" -l "app.kubernetes.io/variant=canary" \
                -o jsonpath='{.items[0].status.conditions[?(@.type=="Ready")].lastTransitionTime}' 2>/dev/null || echo "")
            if [[ -n "${not_ready_since}" ]]; then
                local not_ready_epoch
                not_ready_epoch=$(date -d "${not_ready_since}" +%s 2>/dev/null || echo "0")
                local not_ready_duration=$(( $(date +%s) - not_ready_epoch ))
                if (( not_ready_duration > ABORT_READINESS_FAIL )); then
                    fail "Pod not ready for ${not_ready_duration}s (threshold: ${ABORT_READINESS_FAIL}s)"
                    abort=true
                fi
            fi
        else
            latest_readiness_ok="true"
            ok "Pod readiness: True"
        fi

        if [[ -f "${CANARY_SCORE_SCRIPT}" ]]; then
            local score_json
            if ! score_json=$(ERROR_RATE="${latest_error_rate}" \
                P99_LATENCY_MS="${latest_p99_latency}" \
                POD_RESTARTS="${latest_restarts}" \
                READINESS_OK="${latest_readiness_ok}" \
                MAX_ERROR_RATE="${ABORT_ERROR_RATE}" \
                MAX_P99_LATENCY_MS="${ABORT_P99_LATENCY}" \
                MAX_POD_RESTARTS="${ABORT_POD_RESTARTS}" \
            bash "${CANARY_SCORE_SCRIPT}" 2>/dev/null); then
                fail "Canary health score indicates rollback required"
                abort=true
            else
                ok "Canary health score check passed"
            fi

            mkdir -p "${REPO_ROOT}/artifacts/canary"
            printf '%s\n' "${score_json}" > "${REPO_ROOT}/artifacts/canary/latest-score.json"
        fi

        # ─ Abort? ─
        if [[ "${abort}" == "true" ]]; then
            log ""
            fail "╔═══════════════════════════════════════════════════════╗"
            fail "║  CANARY ABORT: Threshold exceeded                    ║"
            fail "║  Cycle #${cycle_num} FAILED at check #${checks_total}     "
            fail "╚═══════════════════════════════════════════════════════╝"

            record_cycle "${cycle_num}" "FAILED" "${checks_total}" "${elapsed}"
            cmd_abort
            return 1
        fi

        checks_passed=$((checks_passed + 1))

        if (( $(date +%s) < end_time )); then
            sleep "${MONITOR_INTERVAL}"
        fi
    done

    local total_duration=$(( $(date +%s) - start_time ))

    log ""
    ok "╔═══════════════════════════════════════════════════════╗"
    ok "║  CANARY CYCLE #${cycle_num}: PASSED                       "
    ok "║  ${checks_passed}/${checks_total} checks passed over ${total_duration}s     "
    ok "╚═══════════════════════════════════════════════════════╝"

    record_cycle "${cycle_num}" "PASSED" "${checks_passed}" "${total_duration}"
    return 0
}

# ── Record Cycle to Manifest ─────────────────────────────────────────────────

record_cycle() {
    local cycle_num="$1"
    local result="$2"
    local checks="$3"
    local duration="$4"

    if [[ ! -f "${MANIFEST_FILE}" ]]; then
        warn "Release manifest not found; skipping cycle record"
        return
    fi

    local timestamp
    timestamp="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

    # Update cycles_completed in the manifest
    if command -v python3 &>/dev/null; then
        python3 << PYEOF
import json, sys

with open("${MANIFEST_FILE}", "r") as f:
    manifest = json.load(f)

canary = manifest.get("canary", {})
if "${result}" == "PASSED":
    canary["cycles_completed"] = canary.get("cycles_completed", 0) + 1

# Record cycle detail
cycles = canary.get("cycle_history", [])
cycles.append({
    "cycle": ${cycle_num},
    "result": "${result}",
    "checks_passed": ${checks},
    "duration_seconds": ${duration},
    "timestamp": "${timestamp}"
})
canary["cycle_history"] = cycles
manifest["canary"] = canary

with open("${MANIFEST_FILE}", "w") as f:
    json.dump(manifest, f, indent=2)

print(f"Manifest updated: cycle {${cycle_num}} = ${result}, total passed = {canary['cycles_completed']}")
PYEOF
    else
        warn "python3 not available; manifest not updated"
    fi
}

# ── Promote ──────────────────────────────────────────────────────────────────

cmd_promote() {
    log "Promoting canary to stable deployment..."

    # Get canary image
    local canary_image
    canary_image=$(kubectl get "deployment/${CANARY_DEPLOY}" -n "${NAMESPACE}" \
        -o jsonpath='{.spec.template.spec.containers[0].image}' 2>/dev/null)

    if [[ -z "${canary_image}" ]]; then
        die "Cannot determine canary image. Is the canary deployed?"
    fi

    log "Canary image: ${canary_image}"
    log "Updating stable deployment..."

    # Update stable deployment with canary image
    kubectl set image "deployment/${STABLE_DEPLOY}" "api=${canary_image}" -n "${NAMESPACE}"

    # Wait for stable rollout
    kubectl rollout status "deployment/${STABLE_DEPLOY}" -n "${NAMESPACE}" --timeout=300s

    ok "Stable deployment updated to ${canary_image}"

    # Remove canary
    log "Removing canary deployment..."
    kubectl delete deployment "${CANARY_DEPLOY}" -n "${NAMESPACE}" --ignore-not-found
    kubectl delete service "${CANARY_DEPLOY}" -n "${NAMESPACE}" --ignore-not-found

    ok "Canary removed. Full rollout complete."

    # Run smoke tests
    log "Running post-promote smoke tests..."
    local api_url
    api_url=$(kubectl get ingress -n "${NAMESPACE}" -o jsonpath='{.items[0].spec.rules[0].host}' 2>/dev/null || echo "")
    if [[ -n "${api_url}" ]]; then
        curl -sf "https://${api_url}/health" > /dev/null && ok "/health passed" || warn "/health check failed"
        curl -sf "https://${api_url}/ready"  > /dev/null && ok "/ready passed"  || warn "/ready check failed"
    else
        warn "Could not determine API URL from ingress; skipping smoke tests"
    fi
}

# ── Abort ────────────────────────────────────────────────────────────────────

cmd_abort() {
    log "Aborting canary deployment..."

    kubectl delete deployment "${CANARY_DEPLOY}" -n "${NAMESPACE}" --ignore-not-found
    kubectl delete service "${CANARY_DEPLOY}" -n "${NAMESPACE}" --ignore-not-found

    ok "Canary removed. Stable deployment unchanged."
}

# ── Status ───────────────────────────────────────────────────────────────────

cmd_status() {
    log "Canary Status"
    echo ""

    # Check if canary exists
    local canary_exists
    canary_exists=$(kubectl get deployment "${CANARY_DEPLOY}" -n "${NAMESPACE}" 2>/dev/null && echo "yes" || echo "no")

    if [[ "${canary_exists}" == "no" ]]; then
        log "No canary deployment found in namespace ${NAMESPACE}"
        return
    fi

    echo "--- Canary Deployment ---"
    kubectl get deployment "${CANARY_DEPLOY}" -n "${NAMESPACE}" -o wide
    echo ""

    echo "--- Canary Pods ---"
    kubectl get pods -n "${NAMESPACE}" -l "app.kubernetes.io/variant=canary" -o wide
    echo ""

    echo "--- Stable Deployment ---"
    kubectl get deployment "${STABLE_DEPLOY}" -n "${NAMESPACE}" -o wide
    echo ""

    # Image comparison
    local canary_img stable_img
    canary_img=$(kubectl get "deployment/${CANARY_DEPLOY}" -n "${NAMESPACE}" \
        -o jsonpath='{.spec.template.spec.containers[0].image}' 2>/dev/null || echo "N/A")
    stable_img=$(kubectl get "deployment/${STABLE_DEPLOY}" -n "${NAMESPACE}" \
        -o jsonpath='{.spec.template.spec.containers[0].image}' 2>/dev/null || echo "N/A")

    echo "Canary image:  ${canary_img}"
    echo "Stable image:  ${stable_img}"
    echo ""

    # Manifest status
    if [[ -f "${MANIFEST_FILE}" ]] && command -v python3 &>/dev/null; then
        python3 << 'PYEOF'
import json
try:
    with open("RELEASE_MANIFEST.json") as f:
        m = json.load(f)
    canary = m.get("canary", {})
    print(f"Cycles required:  {canary.get('cycles_required', '?')}")
    print(f"Cycles completed: {canary.get('cycles_completed', 0)}")
    for c in canary.get("cycle_history", []):
        print(f"  Cycle {c['cycle']}: {c['result']} ({c['duration_seconds']}s, {c['checks_passed']} checks)")
except Exception as e:
    print(f"Could not read manifest: {e}")
PYEOF
    fi
}

# ── Main ─────────────────────────────────────────────────────────────────────

main() {
    local command="${1:-help}"
    shift || true

    case "${command}" in
        deploy)   cmd_deploy "$@" ;;
        validate) cmd_validate "$@" ;;
        promote)  cmd_promote ;;
        abort)    cmd_abort ;;
        status)   cmd_status ;;
        help|--help|-h)
            head -n 18 "$0" | tail -n +3 | sed 's/^# \?//'
            ;;
        *)
            die "Unknown command: ${command}. Run '$0 help' for usage."
            ;;
    esac
}

main "$@"
