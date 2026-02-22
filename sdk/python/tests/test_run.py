"""Comprehensive tests for Run, Step, EventEmitter, and OfflineBuffer.

All HTTP calls are mocked via unittest.mock.
"""

import json
import os
import shutil
import pytest
from datetime import datetime, timezone
from unittest.mock import MagicMock, patch, call

from aegisrun.client import AegisRunClient
from aegisrun.run import Run
from aegisrun.step import Step
from aegisrun.types import (
    RunStatus, StepStatus, PolicyAction, Decision, RunCounters
)
from aegisrun.events import EventEmitter
from aegisrun.offline_buffer import OfflineBuffer
from aegisrun.tool_call import ToolCall, ToolCallBlockedError, ToolCallExecutionError


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _make_client() -> MagicMock:
    """Return a mock client with stubbed methods."""
    client = MagicMock(spec=AegisRunClient)
    client.base_url = "http://localhost:8080"
    client.create_run.return_value = {
        "run_id": "run-001",
        "status": "running",
        "created_at": "2024-06-01T12:00:00Z",
    }
    client.submit_event.return_value = {
        "event_id": "evt-1",
        "event_hash": "abc123",
    }
    client.execute_tool_call.return_value = {
        "tool_call_id": "tc-1",
        "decision": {"action": "allow", "policy_rule_id": "", "reason": ""},
        "result": {"data": "ok"},
    }
    return client


# ===================================================================
# Run
# ===================================================================

class TestRunConstruction:

    def test_initial_state(self):
        client = _make_client()
        run = Run(client, "pol-1", "v1")

        assert run.policy_id == "pol-1"
        assert run.policy_version == "v1"
        assert run.run_id is None
        assert run.status == RunStatus.RUNNING
        assert run.counters.steps == 0
        assert run.offline_buffer is None
        assert run.emitter is not None

    def test_offline_mode_creates_buffer(self):
        client = _make_client()
        run = Run(client, "pol-1", "v1", offline_mode=True)

        assert run.offline_buffer is not None
        assert run.emitter is None

    def test_metadata_defaults_to_empty(self):
        client = _make_client()
        run = Run(client, "pol-1", "v1")
        assert run.metadata == {}

    def test_custom_metadata(self):
        client = _make_client()
        run = Run(client, "pol-1", "v1", metadata={"env": "test"})
        assert run.metadata == {"env": "test"}


class TestRunStart:

    def test_start_sets_run_id(self):
        client = _make_client()
        run = Run(client, "pol-1", "v1")
        result = run.start()

        assert result is run  # returns self for chaining
        assert run.run_id == "run-001"
        assert run.created_at is not None
        client.create_run.assert_called_once_with(
            policy_id="pol-1",
            policy_version="v1",
            metadata={},
            parent_run_id=None,
        )

    def test_start_emits_run_started_event(self):
        client = _make_client()
        run = Run(client, "pol-1", "v1")
        run.start()

        client.submit_event.assert_called_once()
        call_args = client.submit_event.call_args
        assert call_args.kwargs["event_type"] == "run.started"
        assert call_args.kwargs["run_id"] == "run-001"

    def test_start_with_parent_run_id(self):
        client = _make_client()
        run = Run(client, "pol-1", "v1", parent_run_id="parent-1")
        run.start()

        client.create_run.assert_called_once_with(
            policy_id="pol-1",
            policy_version="v1",
            metadata={},
            parent_run_id="parent-1",
        )

    def test_start_offline_fallback_on_error(self):
        client = _make_client()
        client.create_run.side_effect = ConnectionError("no server")

        run = Run(client, "pol-1", "v1", offline_mode=True)
        result = run.start()

        assert result is run
        assert run.run_id is not None  # should have generated local ULID
        assert run.created_at is not None

    def test_start_online_raises_on_error(self):
        client = _make_client()
        client.create_run.side_effect = ConnectionError("no server")

        run = Run(client, "pol-1", "v1")
        with pytest.raises(ConnectionError):
            run.start()


class TestRunStep:

    def test_step_increments_counter(self):
        client = _make_client()
        run = Run(client, "pol-1", "v1")
        run.start()

        result = run.step("step-1", {"k": "v"}, lambda s: "done")

        assert result == "done"
        assert run.counters.steps == 1

    def test_step_raises_without_start(self):
        client = _make_client()
        run = Run(client, "pol-1", "v1")

        with pytest.raises(RuntimeError, match="Run not started"):
            run.step("s1", {}, lambda s: None)

    def test_step_emits_started_and_ended_events(self):
        client = _make_client()
        run = Run(client, "pol-1", "v1")
        run.start()
        client.submit_event.reset_mock()

        run.step("step-1", {}, lambda s: "ok")

        # Should have emitted step.started and step.ended
        event_types = [
            c.kwargs["event_type"] for c in client.submit_event.call_args_list
        ]
        assert "step.started" in event_types
        assert "step.ended" in event_types

    def test_step_propagates_exception(self):
        client = _make_client()
        run = Run(client, "pol-1", "v1")
        run.start()

        with pytest.raises(ValueError, match="boom"):
            run.step("bad-step", {}, lambda s: (_ for _ in ()).throw(ValueError("boom")))

    def test_step_increments_seq_no(self):
        client = _make_client()
        run = Run(client, "pol-1", "v1")
        run.start()

        seq_nos = []

        def capture_seq(step):
            seq_nos.append(step.seq_no)

        run.step("s1", {}, capture_seq)
        run.step("s2", {}, capture_seq)
        run.step("s3", {}, capture_seq)

        assert seq_nos == [0, 1, 2]


class TestRunEnd:

    def test_end_sets_status_completed(self):
        client = _make_client()
        run = Run(client, "pol-1", "v1")
        run.start()
        run.end()

        assert run.status == RunStatus.COMPLETED

    def test_end_emits_run_ended_event(self):
        client = _make_client()
        run = Run(client, "pol-1", "v1")
        run.start()
        client.submit_event.reset_mock()

        run.end(outcome={"result": "success"})

        client.submit_event.assert_called_once()
        call_args = client.submit_event.call_args
        assert call_args.kwargs["event_type"] == "run.ended"

    def test_end_offline_queues_event(self):
        client = _make_client()
        client.create_run.side_effect = ConnectionError("no server")

        run = Run(client, "pol-1", "v1", offline_mode=True)
        run.start()
        run.end(outcome={"status": "ok"})

        assert len(run.offline_buffer.events) >= 1
        end_events = [e for e in run.offline_buffer.events if e["type"] == "run.ended"]
        assert len(end_events) == 1


class TestRunFlushOffline:

    def test_flush_calls_buffer_flush(self):
        client = _make_client()
        client.create_run.side_effect = ConnectionError("no server")

        run = Run(client, "pol-1", "v1", offline_mode=True)
        run.start()

        # Now make client work for flush
        client.create_run.side_effect = None
        client.create_run.return_value = {"run_id": run.run_id}
        client.submit_event.return_value = {"event_id": "e1", "event_hash": "abc"}

        run.flush_offline_events()
        # After flush, buffer should be empty (all events sent)
        assert run.offline_buffer is not None
        assert len(run.offline_buffer.events) == 0


# ===================================================================
# Step
# ===================================================================

class TestStep:

    def test_step_initialization(self):
        client = _make_client()
        step = Step(client, "run-1", seq_no=0, name="step-1", state_vector={"k": "v"})

        assert step.run_id == "run-1"
        assert step.seq_no == 0
        assert step.name == "step-1"
        assert step.status == StepStatus.RUNNING
        assert step.step_id is not None  # ULID generated
        assert step.started_at is None

    def test_start_sets_timestamp(self):
        client = _make_client()
        step = Step(client, "run-1", seq_no=0, name="s", state_vector={})
        step.start()

        assert step.started_at is not None
        assert isinstance(step.started_at, datetime)
        client.submit_event.assert_called_once()
        kw = client.submit_event.call_args.kwargs
        assert kw["event_type"] == "step.started"
        assert kw["payload"]["step_id"] == step.step_id
        assert kw["payload"]["name"] == "s"

    def test_complete_sets_status_and_timestamp(self):
        client = _make_client()
        step = Step(client, "run-1", seq_no=0, name="s", state_vector={})
        step.start()
        client.submit_event.reset_mock()
        step.complete()

        assert step.status == StepStatus.COMPLETED
        assert step.ended_at is not None
        client.submit_event.assert_called_once()
        kw = client.submit_event.call_args.kwargs
        assert kw["event_type"] == "step.ended"
        assert kw["payload"]["status"] == "completed"
        assert kw["payload"]["step_id"] == step.step_id

    def test_fail_sets_status(self):
        client = _make_client()
        step = Step(client, "run-1", seq_no=0, name="s", state_vector={})
        step.start()
        client.submit_event.reset_mock()
        step.fail("something went wrong")

        assert step.status == StepStatus.FAILED
        assert step.ended_at is not None
        client.submit_event.assert_called_once()
        kw = client.submit_event.call_args.kwargs
        assert kw["event_type"] == "step.ended"
        assert kw["payload"]["status"] == "failed"
        assert kw["payload"]["error"] == "something went wrong"

    def test_lifecycle_event_emission_is_best_effort(self):
        client = _make_client()
        client.submit_event.side_effect = ConnectionError("network down")
        step = Step(client, "run-1", seq_no=0, name="s", state_vector={})

        # Should not raise from telemetry failures
        step.start()
        step.complete()
        step.fail("err")

    def test_tool_call_increments_seq(self):
        client = _make_client()
        run = MagicMock()
        run.counters = RunCounters()

        step = Step(client, "run-1", seq_no=0, name="s", state_vector={"k": 1}, run=run)

        step.tool_call("http_request", {"url": "https://example.com"})
        step.tool_call("file_write", {"path": "/tmp/x"})

        assert step._tool_call_seq == 2
        assert run.counters.tool_calls == 2

    def test_tool_call_blocked_increments_blocks(self):
        client = _make_client()
        client.execute_tool_call.return_value = {
            "tool_call_id": "tc-1",
            "decision": {
                "action": "block",
                "policy_rule_id": "rule-1",
                "reason": "forbidden",
            },
        }
        run = MagicMock()
        run.counters = RunCounters()

        step = Step(client, "run-1", seq_no=0, name="s", state_vector={}, run=run)

        with pytest.raises(ToolCallBlockedError):
            step.tool_call("shell_exec", {"cmd": "rm -rf /"})

        assert run.counters.blocks == 1


# ===================================================================
# ToolCall
# ===================================================================

class TestToolCall:

    def test_execute_allowed(self):
        client = _make_client()
        tc = ToolCall(client, "run-1", "step-1", seq_no=0, tool_name="http_request")

        result = tc.execute(
            args={"url": "https://example.com"},
            state_vector={"step": 1},
        )

        assert result == {"data": "ok"}
        assert tc.decision is not None
        assert tc.decision.action == PolicyAction.ALLOW

    def test_execute_blocked_raises(self):
        client = _make_client()
        client.execute_tool_call.return_value = {
            "tool_call_id": "tc-1",
            "decision": {"action": "block", "reason": "SSRF", "policy_rule_id": "r1"},
        }
        tc = ToolCall(client, "run-1", "step-1", seq_no=0, tool_name="http_request")

        with pytest.raises(ToolCallBlockedError, match="policy"):
            tc.execute(args={"url": "http://169.254.169.254/"}, state_vector={})

        assert tc.decision is not None
        assert tc.decision.action == PolicyAction.BLOCK

    def test_execute_error_raises(self):
        client = _make_client()
        client.execute_tool_call.return_value = {
            "tool_call_id": "tc-1",
            "decision": {"action": "allow", "policy_rule_id": "", "reason": ""},
            "error": "timeout reaching server",
        }
        tc = ToolCall(client, "run-1", "step-1", seq_no=0, tool_name="http_request")

        with pytest.raises(ToolCallExecutionError, match="timeout"):
            tc.execute(args={}, state_vector={})


# ===================================================================
# EventEmitter
# ===================================================================

class TestEventEmitter:

    def test_emit_run_started(self):
        client = _make_client()
        emitter = EventEmitter(client)
        emitter.emit_run_started("run-1", {"env": "test"})

        client.submit_event.assert_called_once()
        kw = client.submit_event.call_args.kwargs
        assert kw["event_type"] == "run.started"
        assert kw["run_id"] == "run-1"
        assert kw["payload"]["metadata"] == {"env": "test"}

    def test_emit_run_ended(self):
        client = _make_client()
        emitter = EventEmitter(client)
        emitter.emit_run_ended("run-1", {"status": "success"})

        kw = client.submit_event.call_args.kwargs
        assert kw["event_type"] == "run.ended"
        assert kw["payload"]["outcome"] == {"status": "success"}

    def test_emit_run_ended_no_outcome(self):
        client = _make_client()
        emitter = EventEmitter(client)
        emitter.emit_run_ended("run-1", None)

        kw = client.submit_event.call_args.kwargs
        assert kw["payload"] == {}

    def test_emit_step_started(self):
        client = _make_client()
        emitter = EventEmitter(client)
        emitter.emit_step_started("run-1", "step-1", "process_data")

        kw = client.submit_event.call_args.kwargs
        assert kw["event_type"] == "step.started"
        assert kw["payload"]["step_id"] == "step-1"
        assert kw["payload"]["name"] == "process_data"

    def test_emit_step_ended(self):
        client = _make_client()
        emitter = EventEmitter(client)
        emitter.emit_step_ended("run-1", "step-1", "completed")

        kw = client.submit_event.call_args.kwargs
        assert kw["event_type"] == "step.ended"
        assert kw["payload"]["status"] == "completed"
        assert "error" not in kw["payload"]

    def test_emit_step_ended_with_error(self):
        client = _make_client()
        emitter = EventEmitter(client)
        emitter.emit_step_ended("run-1", "step-1", "failed", error="crash!")

        kw = client.submit_event.call_args.kwargs
        assert kw["payload"]["error"] == "crash!"

    def test_emit_does_not_raise_on_failure(self):
        """EventEmitter must be best-effort — never propagate exceptions."""
        client = _make_client()
        client.submit_event.side_effect = ConnectionError("network down")

        emitter = EventEmitter(client)
        # Should NOT raise
        emitter.emit_run_started("run-1", {})


# ===================================================================
# OfflineBuffer
# ===================================================================

class TestOfflineBuffer:

    @pytest.fixture(autouse=True)
    def _buffer_dir(self, tmp_path):
        """Use a temporary directory for each test to avoid file collisions."""
        self.buf_dir = str(tmp_path / "aegisrun_buffer")
        yield
        # cleanup handled by tmp_path

    def test_queue_run_start(self):
        buf = OfflineBuffer(buffer_dir=self.buf_dir)
        buf.queue_run_start("run-1", {"policy_id": "p1"})

        assert len(buf.events) == 1
        assert buf.events[0]["type"] == "run.started"
        assert buf.events[0]["run_id"] == "run-1"

    def test_queue_run_end(self):
        buf = OfflineBuffer(buffer_dir=self.buf_dir)
        buf.queue_run_end("run-1", {"status": "ok"})

        assert buf.events[0]["type"] == "run.ended"
        assert buf.events[0]["outcome"] == {"status": "ok"}

    def test_queue_step_started(self):
        buf = OfflineBuffer(buffer_dir=self.buf_dir)
        buf.queue_step_started("run-1", "step-1", "process")

        assert buf.events[0]["type"] == "step.started"
        assert buf.events[0]["payload"]["step_id"] == "step-1"

    def test_persist_writes_json_to_disk(self):
        buf = OfflineBuffer(buffer_dir=self.buf_dir)
        buf.queue_run_start("run-1", {"a": 1})

        events_file = os.path.join(self.buf_dir, "events.json")
        assert os.path.exists(events_file)

        with open(events_file) as f:
            data = json.load(f)
        assert len(data) == 1

    def test_flush_sends_run_started_via_create_run(self):
        buf = OfflineBuffer(buffer_dir=self.buf_dir)
        buf.queue_run_start("run-1", {"policy_id": "p1", "policy_version": "v1"})

        client = _make_client()
        buf.flush(client)

        client.create_run.assert_called_once_with(
            policy_id="p1",
            policy_version="v1",
            metadata={"policy_id": "p1", "policy_version": "v1"},
        )
        assert len(buf.events) == 0

    def test_flush_sends_other_events_via_submit_event(self):
        buf = OfflineBuffer(buffer_dir=self.buf_dir)
        buf.queue_step_started("run-1", "step-1", "process")

        client = _make_client()
        buf.flush(client)

        client.submit_event.assert_called_once()
        kw = client.submit_event.call_args.kwargs
        assert kw["event_type"] == "step.started"
        assert kw["run_id"] == "run-1"
        assert len(buf.events) == 0

    def test_flush_retains_failed_events(self):
        buf = OfflineBuffer(buffer_dir=self.buf_dir)
        buf.queue_run_start("run-1", {"policy_id": "p1", "policy_version": "v1"})
        buf.queue_step_started("run-1", "step-1", "process")

        client = _make_client()
        client.create_run.side_effect = ConnectionError("still down")

        buf.flush(client)

        # create_run failed so run.started is retained; step.started should succeed
        assert len(buf.events) == 1
        assert buf.events[0]["type"] == "run.started"

    def test_flush_empty_is_noop(self):
        buf = OfflineBuffer(buffer_dir=self.buf_dir)
        client = _make_client()
        buf.flush(client)

        client.create_run.assert_not_called()
        client.submit_event.assert_not_called()

    def test_timestamps_iso_format(self):
        buf = OfflineBuffer(buffer_dir=self.buf_dir)
        buf.queue_run_start("run-1", {})

        ts = buf.events[0]["timestamp"]
        # Should parse as ISO-8601
        parsed = datetime.fromisoformat(ts.replace("Z", "+00:00"))
        assert parsed.tzinfo is not None


# ===================================================================
# Types
# ===================================================================

class TestTypes:

    def test_run_status_values(self):
        assert RunStatus.RUNNING == "running"
        assert RunStatus.COMPLETED == "completed"
        assert RunStatus.FAILED == "failed"
        assert RunStatus.BLOCKED == "blocked"
        assert RunStatus.CANCELLED == "cancelled"

    def test_step_status_values(self):
        assert StepStatus.RUNNING == "running"
        assert StepStatus.COMPLETED == "completed"
        assert StepStatus.FAILED == "failed"

    def test_policy_action_values(self):
        assert PolicyAction.ALLOW == "allow"
        assert PolicyAction.BLOCK == "block"
        assert PolicyAction.WARN == "warn"
        assert PolicyAction.REDACT == "redact"
        assert PolicyAction.REQUIRE_APPROVAL == "require_approval"

    def test_decision_defaults(self):
        d = Decision()
        assert d.action == PolicyAction.ALLOW
        assert d.policy_rule_id == ""
        assert d.reason == ""
        assert d.approval_id is None

    def test_run_counters_defaults(self):
        c = RunCounters()
        assert c.steps == 0
        assert c.tool_calls == 0
        assert c.bytes_egressed == 0
        assert c.retries == 0
        assert c.blocks == 0
