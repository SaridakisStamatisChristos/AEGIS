"""Offline event buffering"""

import json
import os
from typing import Dict, Any, List, Optional
from pathlib import Path
from .client import AegisRunClient

class OfflineBuffer:
    """Buffers events when server is unavailable"""

    def __init__(self, buffer_dir: str = ".aegisrun_buffer"):
        self.buffer_dir = Path(buffer_dir)
        self.buffer_dir.mkdir(exist_ok=True)
        self.events: List[Dict[str, Any]] = []

    def queue_run_start(self, run_id: str, metadata: Dict[str, Any]):
        """Queue run start event"""
        self.events.append(
            {
                "type": "run.started",
                "run_id": run_id,
                "metadata": metadata,
                "timestamp": self._now(),
            }
        )
        self._persist()

    def queue_run_end(self, run_id: str, outcome: Optional[Dict[str, Any]]):
        """Queue run end event"""
        self.events.append(
            {
                "type": "run.ended",
                "run_id": run_id,
                "outcome": outcome,
                "payload": {"outcome": outcome} if outcome else {},
                "timestamp": self._now(),
            }
        )
        self._persist()

    def queue_step_started(self, run_id: str, step_id: str, name: str):
        """Queue step start event"""
        self.events.append(
            {
                "type": "step.started",
                "run_id": run_id,
                "step_id": step_id,
                "name": name,
                "payload": {
                    "step_id": step_id,
                    "name": name,
                },
                "timestamp": self._now(),
            }
        )
        self._persist()

    def flush(self, client: AegisRunClient):
        """Flush buffered events to the server.

        For ``run.started`` events the method calls :pymeth:`client.create_run`
        (which creates the run on the server).  All other buffered event types
        are submitted via :pymeth:`client.submit_event`.

        Events that fail to send are retained in the buffer so they can be
        retried on the next call.
        """
        if not self.events:
            return

        remaining: List[Dict[str, Any]] = []

        for event in self.events:
            try:
                etype = event.get("type", "")
                run_id = event.get("run_id", "")

                if etype == "run.started":
                    # The run.started event maps to creating the run itself.
                    # Metadata stored in the buffer carries policy_id and
                    # policy_version that were used when the Run was created
                    # offline.
                    meta = event.get("metadata", {})
                    client.create_run(
                        policy_id=meta.get("policy_id", ""),
                        policy_version=meta.get("policy_version", ""),
                        metadata=meta,
                    )
                else:
                    # All other types go through the generic event endpoint.
                    client.submit_event(
                        run_id=run_id,
                        event_type=etype,
                        payload=event.get("payload") or event.get("metadata") or {},
                        timestamp=event.get("timestamp"),
                    )
            except Exception:
                # Keep failed events so the caller can retry later.
                remaining.append(event)

        self.events = remaining
        self._persist()

    def _persist(self):
        """Persist buffer to disk"""
        buffer_file = self.buffer_dir / "events.json"
        try:
            with open(buffer_file, "w") as f:
                json.dump(self.events, f, indent=2)
        except OSError as exc:
            import warnings
            warnings.warn(
                f"OfflineBuffer._persist() failed to write {buffer_file}: {exc}",
                stacklevel=2,
            )

    def _now(self) -> str:
        """Get current timestamp in ISO format"""
        from datetime import datetime, timezone

        return datetime.now(timezone.utc).isoformat()
