#!/usr/bin/env bash
set -euo pipefail

EVIDENCE_DIR="${1:-artifacts/drills}"
MAX_AGE_DAYS="${MAX_AGE_DAYS:-30}"
export EVIDENCE_DIR
export MAX_AGE_DAYS

if [[ ! -d "${EVIDENCE_DIR}" ]]; then
  echo "ERROR: Drill evidence directory not found: ${EVIDENCE_DIR}"
  exit 1
fi

python3 - <<'PYEOF'
import os
import sys
import time
from pathlib import Path

evidence_dir = Path(os.environ.get("EVIDENCE_DIR", "artifacts/drills"))
max_age_days = int(os.environ.get("MAX_AGE_DAYS", "30"))
cutoff = time.time() - max_age_days * 24 * 3600

required = ["backup", "restore", "rollback", "incident"]

latest_file = None
latest_mtime = 0.0
for path in evidence_dir.rglob("*"):
    if path.is_file() and path.suffix.lower() in {".md", ".json", ".txt"}:
        mtime = path.stat().st_mtime
        if mtime > latest_mtime:
            latest_mtime = mtime
            latest_file = path

if latest_file is None:
    print(f"ERROR: No drill evidence files found in {evidence_dir}")
    sys.exit(1)

if latest_mtime < cutoff:
    age_days = int((time.time() - latest_mtime) / 86400)
    print(f"ERROR: Latest drill evidence is too old ({age_days} days): {latest_file}")
    sys.exit(1)

content = latest_file.read_text(encoding="utf-8", errors="ignore").lower()
missing = [key for key in required if key not in content]
if missing:
    print(f"ERROR: Latest drill evidence missing required keywords {missing}: {latest_file}")
    sys.exit(1)

print("Drill cadence verification passed")
print(f"latest_file={latest_file}")
print(f"latest_mtime={int(latest_mtime)}")
PYEOF
