#!/usr/bin/env python3
import argparse
import csv
import json
import sys
from pathlib import Path


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Validate Locust aggregate thresholds")
    parser.add_argument("--csv", required=True, help="Path to Locust results_stats.csv")
    parser.add_argument("--max-error-rate", type=float, default=5.0, help="Max allowed failure ratio in percent")
    parser.add_argument("--max-p95-ms", type=float, default=500.0, help="Max allowed P95 latency in ms")
    parser.add_argument("--max-p99-ms", type=float, default=2000.0, help="Max allowed P99 latency in ms")
    parser.add_argument("--min-rps", type=float, default=1.0, help="Minimum allowed requests/sec throughput")
    parser.add_argument("--out", default="load-threshold-summary.json", help="Summary output path")
    return parser.parse_args()


def num(value: str) -> float:
    if value is None:
        return 0.0
    value = value.strip()
    if value == "":
        return 0.0
    return float(value)


def find_aggregate_row(csv_path: Path) -> dict:
    with csv_path.open("r", encoding="utf-8") as f:
        reader = csv.DictReader(f)
        for row in reader:
            row_type = (row.get("Type") or "").strip().lower()
            row_name = (row.get("Name") or "").strip().lower()
            if row_type == "aggregated" or row_name == "aggregated":
                return row
    raise RuntimeError("Could not find Aggregated row in Locust CSV")


def main() -> int:
    args = parse_args()
    csv_path = Path(args.csv)
    if not csv_path.exists():
        print(f"ERROR: CSV not found: {csv_path}", file=sys.stderr)
        return 2

    row = find_aggregate_row(csv_path)

    request_count = num(row.get("Request Count", "0"))
    failure_count = num(row.get("Failure Count", "0"))
    rps = num(row.get("Requests/s", "0"))
    p95 = num(row.get("95%", "0"))
    p99 = num(row.get("99%", "0"))

    error_rate = (failure_count / request_count * 100.0) if request_count > 0 else 100.0

    checks = {
        "error_rate": {
            "value": error_rate,
            "threshold": args.max_error_rate,
            "operator": "<=",
            "pass": error_rate <= args.max_error_rate,
        },
        "p95_ms": {
            "value": p95,
            "threshold": args.max_p95_ms,
            "operator": "<=",
            "pass": p95 <= args.max_p95_ms,
        },
        "p99_ms": {
            "value": p99,
            "threshold": args.max_p99_ms,
            "operator": "<=",
            "pass": p99 <= args.max_p99_ms,
        },
        "throughput_rps": {
            "value": rps,
            "threshold": args.min_rps,
            "operator": ">=",
            "pass": rps >= args.min_rps,
        },
    }

    passed = all(item["pass"] for item in checks.values())
    summary = {
        "passed": passed,
        "source_csv": str(csv_path),
        "request_count": request_count,
        "failure_count": failure_count,
        "checks": checks,
    }

    out_path = Path(args.out)
    out_path.write_text(json.dumps(summary, indent=2), encoding="utf-8")

    print("Load threshold summary:")
    print(json.dumps(summary, indent=2))

    if passed:
        return 0

    print("ERROR: Load thresholds failed", file=sys.stderr)
    return 1


if __name__ == "__main__":
    sys.exit(main())
