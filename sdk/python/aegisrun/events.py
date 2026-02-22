"""Event emission for telemetry"""

import logging
from datetime import datetime, timezone
from typing import Any, Dict, Optional

from .client import AegisRunClient

logger = logging.getLogger(__name__)


class EventEmitter:
    """Handles event emission to the AegisRun API.

    Each method submits a hash-chained event to the server via
    :pymeth:`AegisRunClient.submit_event`.  Errors are logged but
    never propagated so that event emission cannot break the agent's
    primary workflow.
    """

    def __init__(self, client: AegisRunClient):
        self.client = client

    # ------------------------------------------------------------------
    # Helpers
    # ------------------------------------------------------------------

    @staticmethod
    def _now_iso() -> str:
        return datetime.now(timezone.utc).isoformat()

    def _emit(
        self,
        run_id: str,
        event_type: str,
        payload: Dict[str, Any],
    ) -> None:
        """Best-effort event submission."""
        try:
            self.client.submit_event(
                run_id=run_id,
                event_type=event_type,
                payload=payload,
                timestamp=self._now_iso(),
            )
        except Exception:
            logger.warning(
                "Failed to emit %s event for run %s",
                event_type,
                run_id,
                exc_info=True,
            )

    # ------------------------------------------------------------------
    # Public API
    # ------------------------------------------------------------------

    def emit_run_started(self, run_id: str, metadata: Dict[str, Any]) -> None:
        """Emit run.started event"""
        self._emit(run_id, "run.started", {"metadata": metadata})

    def emit_run_ended(self, run_id: str, outcome: Optional[Dict[str, Any]]) -> None:
        """Emit run.ended event"""
        payload: Dict[str, Any] = {}
        if outcome is not None:
            payload["outcome"] = outcome
        self._emit(run_id, "run.ended", payload)

    def emit_step_started(self, run_id: str, step_id: str, name: str) -> None:
        """Emit step.started event"""
        self._emit(
            run_id,
            "step.started",
            {"step_id": step_id, "name": name},
        )

    def emit_step_ended(
        self,
        run_id: str,
        step_id: str,
        status: str,
        error: Optional[str] = None,
    ) -> None:
        """Emit step.ended event"""
        payload: Dict[str, Any] = {"step_id": step_id, "status": status}
        if error is not None:
            payload["error"] = error
        self._emit(run_id, "step.ended", payload)
