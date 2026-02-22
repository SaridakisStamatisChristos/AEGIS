"""Step execution management"""

from __future__ import annotations

import logging
from datetime import datetime, timezone
from typing import TYPE_CHECKING, Any, Dict, Optional

from ulid import ULID

from .client import AegisRunClient
from .tool_call import ToolCall, ToolCallBlockedError
from .types import StepStatus

if TYPE_CHECKING:
    from .run import Run

logger = logging.getLogger(__name__)


class Step:
    """Represents a single step within a run"""

    def __init__(
        self,
        client: AegisRunClient,
        run_id: str,
        seq_no: int,
        name: str,
        state_vector: Dict[str, Any],
        run: Optional["Run"] = None,
    ):
        self.client = client
        self.run_id = run_id
        self.run = run
        self.seq_no = seq_no
        self.name = name
        self.state_vector = state_vector

        self.step_id = str(ULID())
        self.status = StepStatus.RUNNING
        self.started_at: Optional[datetime] = None
        self.ended_at: Optional[datetime] = None

        self._tool_call_seq = 0

    def start(self):
        """Mark step as started"""
        self.started_at = datetime.now(timezone.utc)
        self._emit_event(
            event_type="step.started",
            payload={
                "step_id": self.step_id,
                "name": self.name,
                "seq_no": self.seq_no,
            },
            timestamp=self.started_at,
        )

    def complete(self):
        """Mark step as completed"""
        self.status = StepStatus.COMPLETED
        self.ended_at = datetime.now(timezone.utc)
        self._emit_event(
            event_type="step.ended",
            payload={
                "step_id": self.step_id,
                "status": "completed",
                "seq_no": self.seq_no,
            },
            timestamp=self.ended_at,
        )

    def fail(self, error: str):
        """Mark step as failed"""
        self.status = StepStatus.FAILED
        self.ended_at = datetime.now(timezone.utc)
        self._emit_event(
            event_type="step.ended",
            payload={
                "step_id": self.step_id,
                "status": "failed",
                "seq_no": self.seq_no,
                "error": error,
            },
            timestamp=self.ended_at,
        )

    def _emit_event(
        self,
        event_type: str,
        payload: Dict[str, Any],
        timestamp: Optional[datetime] = None,
    ) -> None:
        try:
            ts = (timestamp or datetime.now(timezone.utc)).isoformat()
            self.client.submit_event(
                run_id=self.run_id,
                event_type=event_type,
                payload=payload,
                timestamp=ts,
            )
        except Exception:
            logger.warning(
                "Failed to emit %s for run=%s step=%s",
                event_type,
                self.run_id,
                self.step_id,
                exc_info=True,
            )

    def tool_call(
        self,
        tool_name: str,
        args: Dict[str, Any],
        executor: str = "builtin",
    ) -> Any:
        """Execute a tool call through the gateway"""
        tool_call = ToolCall(
            client=self.client,
            run_id=self.run_id,
            step_id=self.step_id,
            seq_no=self._tool_call_seq,
            tool_name=tool_name,
        )

        self._tool_call_seq += 1

        try:
            result = tool_call.execute(
                args=args,
                state_vector=self.state_vector,
                executor=executor,
            )
            if self.run:
                self.run.counters.tool_calls += 1
            return result
        except ToolCallBlockedError:
            if self.run:
                self.run.counters.blocks += 1
            raise
