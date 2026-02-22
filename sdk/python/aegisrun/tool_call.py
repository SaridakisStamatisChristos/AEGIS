"""Tool call execution"""

from typing import Dict, Any, Optional
from .client import AegisRunClient
from .types import Decision, PolicyAction


class ToolCall:
    """Represents a single tool invocation"""

    def __init__(
        self,
        client: AegisRunClient,
        run_id: str,
        step_id: str,
        seq_no: int,
        tool_name: str,
    ):
        self.client = client
        self.run_id = run_id
        self.step_id = step_id
        self.seq_no = seq_no
        self.tool_name = tool_name

        self.tool_call_id: Optional[str] = None
        self.decision: Optional[Decision] = None
        self.result: Optional[Any] = None
        self.error: Optional[str] = None

    def execute(
        self,
        args: Dict[str, Any],
        state_vector: Dict[str, Any],
        executor: str = "builtin",
        metadata: Optional[Dict[str, Any]] = None,
    ) -> Any:
        """Execute the tool call"""
        response = self.client.execute_tool_call(
            run_id=self.run_id,
            step_id=self.step_id,
            tool_name=self.tool_name,
            args=args,
            state_vector=state_vector,
            metadata=metadata,
            executor=executor,
        )

        self.tool_call_id = response["tool_call_id"]

        # Parse decision as a struct: {action, policy_rule_id, reason, approval_id}
        raw_decision = response.get("decision", {})
        if isinstance(raw_decision, dict):
            self.decision = Decision(**raw_decision)
        else:
            self.decision = Decision(action=PolicyAction(raw_decision))

        self.result = response.get("result")
        self.error = response.get("error")

        # Handle blocked calls
        if self.decision and self.decision.action == PolicyAction.BLOCK:
            raise ToolCallBlockedError(
                f"Tool call blocked by policy: {self.decision.reason}",
                decision=self.decision,
            )

        if self.error:
            raise ToolCallExecutionError(self.error)

        return self.result


class ToolCallBlockedError(Exception):
    """Raised when a tool call is blocked by policy"""

    def __init__(self, message: str, decision: Decision):
        super().__init__(message)
        self.decision = decision


class ToolCallExecutionError(Exception):
    """Raised when tool execution fails"""
    pass
