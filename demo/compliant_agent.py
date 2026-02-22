#!/usr/bin/env python3
"""
AegisRun Demo: Compliant Agent

This agent demonstrates proper usage of AegisRun SDK following
policy constraints. It performs legitimate operations that should
all be allowed by a permissive or production policy.

Usage:
    export AEGISRUN_API_URL=http://localhost:8080
    export AEGISRUN_API_TOKEN=your-token
    python compliant_agent.py
"""

import os
import sys
import json
import time
from typing import Dict, Any, Optional

# Add SDK to path if running from demo directory
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "sdk", "python"))

from aegisrun import AegisRunClient, Run, PolicyRef


def get_config() -> Dict[str, str]:
    """Load configuration from environment"""
    return {
        "api_url": os.getenv("AEGISRUN_API_URL", "http://localhost:8080"),
        "api_token": os.getenv("AEGISRUN_API_TOKEN"),
        "policy_id": os.getenv("AEGISRUN_POLICY_ID", "production-standard"),
        "policy_version": os.getenv("AEGISRUN_POLICY_VERSION", "v1"),
    }


def create_aegis_client(config: Dict[str, str]) -> AegisRunClient:
    """Create and configure AegisRun client"""
    return AegisRunClient(
        base_url=config["api_url"],
        api_token=config["api_token"],
    )


def compliant_http_operations(step) -> Dict[str, Any]:
    """
    Demonstrate compliant HTTP operations.
    Uses GET requests to public APIs without sensitive data.
    """
    print("  [STEP] Performing compliant HTTP operations...")
    results = []
    
    # 1. Simple GET request to public API
    result = step.tool_call(
        tool_name="http_request",
        args={
            "url": "https://api.github.com/zen",
            "method": "GET",
            "headers": {"Accept": "application/json"},
            "timeout_sec": 10,
        },
    )
    results.append({"operation": "get_zen", "result": result})
    print(f"    ✓ GitHub Zen API: {result.get('result', 'Success')[:50]}...")
    
    # 2. GET with path parameter
    result = step.tool_call(
        tool_name="http_request",
        args={
            "url": "https://api.github.com/users/octocat",
            "method": "GET",
            "headers": {"Accept": "application/vnd.github+json"},
            "timeout_sec": 10,
        },
    )
    results.append({"operation": "get_user", "result": result})
    print(f"    ✓ User lookup: Retrieved user info")
    
    return {"http_results": results}


def compliant_file_operations(step) -> Dict[str, Any]:
    """
    Demonstrate compliant file operations.
    Reads from allowed paths, writes to /tmp only.
    """
    print("  [STEP] Performing compliant file operations...")
    results = []
    
    # 1. Read from allowed directory
    result = step.tool_call(
        tool_name="file_read",
        args={
            "path": "./output/sample_data.json",
        },
    )
    results.append({"operation": "read_data", "result": "success"})
    print(f"    ✓ File read: Read from allowed directory")
    
    # 2. Write to /tmp (always allowed)
    result = step.tool_call(
        tool_name="file_write",
        args={
            "path": "/tmp/aegis_compliant_output.json",
            "content": json.dumps({
                "agent": "compliant_agent",
                "timestamp": time.time(),
                "status": "completed",
            }, indent=2),
        },
    )
    results.append({"operation": "write_temp", "result": "success"})
    print(f"    ✓ File write: Wrote to /tmp directory")
    
    return {"file_results": results}


def compliant_database_operations(step) -> Dict[str, Any]:
    """
    Demonstrate compliant database operations.
    Uses SELECT queries only, with parameterized inputs.
    """
    print("  [STEP] Performing compliant database operations...")
    results = []
    
    # 1. Simple SELECT query
    result = step.tool_call(
        tool_name="database_query",
        args={
            "query": "SELECT id, name, created_at FROM users WHERE active = $1 LIMIT 10",
            "params": [True],
            "timeout_ms": 5000,
        },
    )
    results.append({"operation": "select_users", "result": "success"})
    print(f"    ✓ Database query: SELECT executed successfully")
    
    # 2. Aggregation query
    result = step.tool_call(
        tool_name="database_query",
        args={
            "query": "SELECT COUNT(*) as total, status FROM orders GROUP BY status",
            "params": [],
            "timeout_ms": 5000,
        },
    )
    results.append({"operation": "aggregate_orders", "result": "success"})
    print(f"    ✓ Database query: Aggregation completed")
    
    return {"database_results": results}


def compliant_data_processing(step) -> Dict[str, Any]:
    """
    Demonstrate compliant data processing workflow.
    Simulates a typical agent workflow without policy violations.
    """
    print("  [STEP] Performing compliant data processing...")
    
    # This step demonstrates a multi-stage workflow
    stages = []
    
    # Stage 1: Fetch configuration
    config_result = step.tool_call(
        tool_name="http_request",
        args={
            "url": "https://httpbin.org/json",
            "method": "GET",
            "timeout_sec": 10,
        },
    )
    stages.append({"stage": "fetch_config", "status": "completed"})
    print(f"    ✓ Stage 1: Configuration fetched")
    
    # Stage 2: Process data (simulated with file operations)
    process_result = step.tool_call(
        tool_name="file_write",
        args={
            "path": "/tmp/processing_results.json",
            "content": json.dumps({
                "processing_id": "proc_001",
                "input_size": 1024,
                "output_size": 512,
                "duration_ms": 150,
            }),
        },
    )
    stages.append({"stage": "process_data", "status": "completed"})
    print(f"    ✓ Stage 2: Data processed")
    
    # Stage 3: Generate report
    report_result = step.tool_call(
        tool_name="file_write",
        args={
            "path": "/tmp/agent_report.txt",
            "content": f"Compliant Agent Report\n{'='*40}\nTimestamp: {time.time()}\nStages completed: {len(stages)}\nStatus: SUCCESS\n",
        },
    )
    stages.append({"stage": "generate_report", "status": "completed"})
    print(f"    ✓ Stage 3: Report generated")
    
    return {"processing_stages": stages}


def run_compliant_agent():
    """Main function to run the compliant agent demo"""
    print("\n" + "="*60)
    print(" AegisRun Demo: Compliant Agent")
    print("="*60)
    
    config = get_config()
    print(f"\n[CONFIG] API URL: {config['api_url']}")
    print(f"[CONFIG] Policy: {config['policy_id']}:{config['policy_version']}")
    
    # Create client
    client = create_aegis_client(config)
    
    # Create run with metadata
    print("\n[RUN] Starting compliant agent run...")
    run = Run(
        client=client,
        policy_ref=PolicyRef(
            policy_id=config["policy_id"],
            version=config["policy_version"],
        ),
        metadata={
            "agent_type": "compliant_demo",
            "environment": "demo",
            "version": "1.0.0",
        },
    )
    
    try:
        run.start()
        print(f"[RUN] Created run: {run.run_id}")
        
        # Execute workflow steps
        results = {}
        
        # Step 1: HTTP Operations
        result = run.step(
            name="http_operations",
            state_vector={"phase": "http"},
            fn=compliant_http_operations,
        )
        results["http"] = result
        
        # Step 2: File Operations
        result = run.step(
            name="file_operations",
            state_vector={"phase": "files"},
            fn=compliant_file_operations,
        )
        results["files"] = result
        
        # Step 3: Database Operations
        result = run.step(
            name="database_operations",
            state_vector={"phase": "database"},
            fn=compliant_database_operations,
        )
        results["database"] = result
        
        # Step 4: Data Processing Workflow
        result = run.step(
            name="data_processing",
            state_vector={"phase": "processing"},
            fn=compliant_data_processing,
        )
        results["processing"] = result
        
        # Complete run
        run.end(outcome={"status": "success", "results": results})
        
        print("\n" + "-"*60)
        print("[SUCCESS] Compliant agent run completed successfully!")
        print(f"[RUN ID] {run.run_id}")
        print("\nAll tool calls were approved by policy.")
        print("Evidence bundle available for export.")
        print("-"*60 + "\n")
        
        return 0
        
    except Exception as e:
        print(f"\n[ERROR] Agent failed: {e}")
        run.end(outcome={"status": "error", "error": str(e)})
        return 1


if __name__ == "__main__":
    sys.exit(run_compliant_agent())
