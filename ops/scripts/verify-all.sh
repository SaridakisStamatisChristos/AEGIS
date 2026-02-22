#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TIMESTAMP="$(date -u +"%Y%m%dT%H%M%SZ")"
OUT_DIR="${ROOT_DIR}/artifacts/verification/${TIMESTAMP}"
SUMMARY_FILE="${OUT_DIR}/summary.txt"

mkdir -p "${OUT_DIR}"

run_step() {
  local name="$1"
  local cmd="$2"
  local log_file="${OUT_DIR}/${name}.log"

  echo "[START] ${name}" | tee -a "${SUMMARY_FILE}"
  echo "[CMD] ${cmd}" | tee -a "${SUMMARY_FILE}"

  if bash -lc "cd '${ROOT_DIR}' && ${cmd}" >"${log_file}" 2>&1; then
    echo "[PASS] ${name}" | tee -a "${SUMMARY_FILE}"
  else
    echo "[FAIL] ${name}" | tee -a "${SUMMARY_FILE}"
    echo "Log: ${log_file}" | tee -a "${SUMMARY_FILE}"
    exit 1
  fi

  echo "Log: ${log_file}" | tee -a "${SUMMARY_FILE}"
  echo "" | tee -a "${SUMMARY_FILE}"
}

{
  echo "AegisRun verification evidence"
  echo "Timestamp (UTC): ${TIMESTAMP}"
  echo "Repository: ${ROOT_DIR}"
  echo ""
} >"${SUMMARY_FILE}"

run_step "api-tests" "cd api && go test ./..."
run_step "verifier-tests" "cd verifier && go test ./..."
run_step "ui-tests" "cd ui && npm run test -- --run"
run_step "typescript-sdk-tests" "cd sdk/typescript && npm test -- --run"

echo "All verification checks passed." | tee -a "${SUMMARY_FILE}"
echo "Evidence directory: ${OUT_DIR}" | tee -a "${SUMMARY_FILE}"
