"""Run management"""

from datetime import datetime, timezone
from typing import Any, Callable, Dict, Optional

from ulid import ULID

from .client import AegisRunClient
from .events import EventEmitter
from .offline_buffer import OfflineBuffer
from .step import Step
from .types import RunCounters, RunStatus


class Run:
    """Represents an agent run with policy enforcement"""

    def __init__(
        self,
        client: AegisRunClient,
        policy_id: str,
        policy_version: str,
        metadata: Optional[Dict[str, Any]] = None,
        parent_run_id: Optional[str] = None,
        offline_mode: bool = False,
    ):
        self.client = client
        self.policy_id = policy_id
        self.policy_version = policy_version
        self.metadata = metadata or {}
        self.parent_run_id = parent_run_id
        self.offline_mode = offline_mode

        self.run_id: Optional[str] = None
        self.status = RunStatus.RUNNING
        self.counters = RunCounters()
        self.created_at: Optional[datetime] = None

        self.offline_buffer = OfflineBuffer() if offline_mode else None
        self.emitter = EventEmitter(client) if not offline_mode else None

        self._step_seq = 0

    def start(self) -> "Run":
        """Start the run (creates it on the server)"""
        try:
            response = self.client.create_run(
                policy_id=self.policy_id,
                policy_version=self.policy_version,
                metadata=self.metadata,
                parent_run_id=self.parent_run_id,
            )

            self.run_id = response["run_id"]
            created = response.get("created_at")
            if created:
                self.created_at = datetime.fromisoformat(
                    created.replace("Z", "+00:00")
                ).replace(tzinfo=timezone.utc)

            # Emit run.started telemetry event
            if self.emitter and self.run_id:
                self.emitter.emit_run_started(self.run_id, self.metadata)

            return self
        except Exception:
            if self.offline_mode:
                # Generate local run_id
                self.run_id = str(ULID())
                self.created_at = datetime.now(timezone.utc)
                if self.offline_buffer:
                    # Include policy ref so flush() can replay create_run.
                    run_meta = dict(self.metadata)
                    run_meta.setdefault("policy_id", self.policy_id)
                    run_meta.setdefault("policy_version", self.policy_version)
                    self.offline_buffer.queue_run_start(self.run_id, run_meta)
                return self
            raise

    def step(
        self,
        name: str,
        state_vector: Dict[str, Any],
        fn: Callable[[Step], Any],
    ) -> Any:
        """Execute a step with automatic event tracking"""
        if not self.run_id:
            raise RuntimeError("Run not started. Call run.start() first.")

        step = Step(
            client=self.client,
            run_id=self.run_id,
            run=self,
            seq_no=self._step_seq,
            name=name,
            state_vector=state_vector,
        )

        self._step_seq += 1

        if self.offline_mode and self.offline_buffer:
            self.offline_buffer.queue_step_started(self.run_id, step.step_id, name)

        # Execute step
        try:
            step.start()
            # Emit step.started telemetry event
            if self.emitter:
                self.emitter.emit_step_started(self.run_id, step.step_id, name)
            result = fn(step)
            step.complete()
            # Emit step.ended telemetry event
            if self.emitter:
                self.emitter.emit_step_ended(self.run_id, step.step_id, "completed")
            self.counters.steps += 1
            return result
        except Exception as e:
            step.fail(str(e))
            if self.emitter:
                self.emitter.emit_step_ended(
                    self.run_id, step.step_id, "failed", error=str(e)
                )
            raise

    def end(self, outcome: Optional[Dict[str, Any]] = None):
        """End the run."""
        if self.offline_mode and self.offline_buffer and self.run_id:
            self.offline_buffer.queue_run_end(self.run_id, outcome)

        # Emit run.ended telemetry event
        if self.emitter and self.run_id:
            self.emitter.emit_run_ended(self.run_id, outcome)

        self.status = RunStatus.COMPLETED

    def flush_offline_events(self):
        """Flush buffered events to server (offline mode)"""
        if self.offline_buffer:
            self.offline_buffer.flush(self.client)
