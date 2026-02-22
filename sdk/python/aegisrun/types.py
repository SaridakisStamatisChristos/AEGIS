"""Type definitions for AegisRun SDK

Matches the Go backend API /api/v1 response shapes.
"""

from enum import Enum
from typing import Any, Dict, Optional, List
from pydantic import BaseModel
from datetime import datetime


class RunStatus(str, Enum):
    RUNNING = "running"
    COMPLETED = "completed"
    FAILED = "failed"
    BLOCKED = "blocked"
    CANCELLED = "cancelled"


class StepStatus(str, Enum):
    RUNNING = "running"
    COMPLETED = "completed"
    FAILED = "failed"


class PolicyStatus(str, Enum):
    DRAFT = "draft"
    REVIEW = "review"
    APPROVED = "approved"
    DEPLOYED = "deployed"
    DEPRECATED = "deprecated"


class PolicyAction(str, Enum):
    """Policy action values used in a Decision."""
    ALLOW = "allow"
    WARN = "warn"
    REDACT = "redact"
    BLOCK = "block"
    REQUIRE_APPROVAL = "require_approval"
    DEGRADE = "degrade"


class Decision(BaseModel):
    """Gateway decision returned by the API as a nested object."""
    action: PolicyAction = PolicyAction.ALLOW
    policy_rule_id: str = ""
    reason: str = ""
    approval_id: Optional[str] = None


class PolicyRef(BaseModel):
    """Structured policy reference (policy_id + version)."""
    policy_id: str
    version: str


class SchemaRef(BaseModel):
    """Structured schema reference."""
    schema_id: str
    version: str


class RunCounters(BaseModel):
    steps: int = 0
    tool_calls: int = 0
    bytes_egressed: int = 0
    retries: int = 0
    blocks: int = 0


class RunResponse(BaseModel):
    run_id: str
    org_id: str = ""
    parent_run_id: Optional[str] = None
    policy_ref: Optional[PolicyRef] = None
    state_schema_ref: Optional[SchemaRef] = None
    metadata: Dict[str, Any] = {}
    status: RunStatus = RunStatus.RUNNING
    outcome: Optional[Dict[str, Any]] = None
    counters: RunCounters = RunCounters()
    evidence_hash: Optional[str] = None
    signature: Optional[str] = None
    signer_key_id: Optional[str] = None
    created_at: Optional[datetime] = None
    ended_at: Optional[datetime] = None


class StepResponse(BaseModel):
    step_id: str
    run_id: str
    parent_step_id: Optional[str] = None
    seq_no: int = 0
    name: str = ""
    state_vector: Dict[str, Any] = {}
    status: StepStatus = StepStatus.RUNNING
    error: Optional[str] = None
    started_at: Optional[datetime] = None
    ended_at: Optional[datetime] = None


class EventResponse(BaseModel):
    event_id: str
    run_id: str
    seq_no: int = 0
    event_type: str = ""
    timestamp: Optional[datetime] = None
    payload: Optional[Dict[str, Any]] = None
    prev_hash: Optional[str] = None
    event_hash: Optional[str] = None


class PolicyResponse(BaseModel):
    policy_id: str
    org_id: str = ""
    name: str = ""
    version: str = "v1"
    status: str = "draft"
    created_at: Optional[datetime] = None
    approved_at: Optional[datetime] = None
    approved_by: Optional[List[str]] = None
    spec: Dict[str, Any] = {}
    spec_hash: Optional[str] = None


class ToolCallResult(BaseModel):
    tool_call_id: str
    decision: Decision = Decision()
    result: Optional[Any] = None
    error: Optional[str] = None
