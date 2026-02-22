# AegisRun Python SDK

Official Python SDK for AegisRun Agent Control Plane.

## Installation
```bash
pip install aegisrun
```

## Quick Start
```python
from aegisrun import AegisRunClient, Run

# Initialize client
client = AegisRunClient(
    base_url="http://localhost:8080",
    api_token="your-api-token"
)

# Create a run
run = Run(
    client=client,
    policy_ref={"policy_id": "pol_abc123", "version": "v1"},
    metadata={"environment": "production"}
).start()

# Execute steps with tool calls
def my_agent_logic(step):
    # Tool calls are automatically enforced by policy
    result = step.tool_call(
        tool_name="http_request",
        args={"url": "https://api.example.com/data", "method": "GET"}
    )
    return result

result = run.step(
    name="fetch_data",
    state_vector={"current_task": "data_collection"},
    fn=my_agent_logic
)

# End run
run.end(outcome={"status": "success"})
```

## Features

- **Policy Enforcement**: All tool calls go through the gateway with hard enforcement
- **Automatic Event Tracking**: Steps and tool calls are automatically logged
- **Offline Mode**: Buffer events when server is unavailable
- **Type Safety**: Full type hints for better IDE support
- **Evidence Export**: Download tamper-evident evidence bundles

## API Reference

See [docs.aegisrun.io](https://docs.aegisrun.io) for full API documentation.

## Examples

See the `examples/` directory for complete examples including:
- Basic agent workflow
- LangGraph integration
- HTTP tool usage
- Error handling
