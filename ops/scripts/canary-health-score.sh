#!/usr/bin/env bash
set -euo pipefail

ERROR_RATE="${ERROR_RATE:-0}"
P99_LATENCY_MS="${P99_LATENCY_MS:-0}"
POD_RESTARTS="${POD_RESTARTS:-0}"
READINESS_OK="${READINESS_OK:-true}"

MAX_ERROR_RATE="${MAX_ERROR_RATE:-5}"
MAX_P99_LATENCY_MS="${MAX_P99_LATENCY_MS:-2000}"
MAX_POD_RESTARTS="${MAX_POD_RESTARTS:-2}"

SCORE=100

to_int() {
  awk "BEGIN { printf \"%d\", ($1) }"
}

is_gt() {
  awk "BEGIN { exit !($1 > $2) }"
}

if is_gt "${ERROR_RATE}" "${MAX_ERROR_RATE}"; then
  SCORE=$((SCORE - 40))
fi

if is_gt "${P99_LATENCY_MS}" "${MAX_P99_LATENCY_MS}"; then
  SCORE=$((SCORE - 30))
fi

if is_gt "${POD_RESTARTS}" "${MAX_POD_RESTARTS}"; then
  SCORE=$((SCORE - 20))
fi

if [[ "${READINESS_OK}" != "true" ]]; then
  SCORE=$((SCORE - 25))
fi

if (( SCORE < 0 )); then
  SCORE=0
fi

ROLLBACK_REQUIRED=false
if (( SCORE < 70 )); then
  ROLLBACK_REQUIRED=true
fi

cat <<EOF
{
  "inputs": {
    "error_rate": ${ERROR_RATE},
    "p99_latency_ms": ${P99_LATENCY_MS},
    "pod_restarts": ${POD_RESTARTS},
    "readiness_ok": ${READINESS_OK}
  },
  "thresholds": {
    "max_error_rate": ${MAX_ERROR_RATE},
    "max_p99_latency_ms": ${MAX_P99_LATENCY_MS},
    "max_pod_restarts": ${MAX_POD_RESTARTS}
  },
  "score": $(to_int "${SCORE}"),
  "rollback_required": ${ROLLBACK_REQUIRED}
}
EOF

if [[ "${ROLLBACK_REQUIRED}" == "true" ]]; then
  exit 1
fi
