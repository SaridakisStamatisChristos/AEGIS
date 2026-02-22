"""LangGraph-style wrapper for AegisRun"""

from typing import Callable, Dict, Any, TypedDict
from aegisrun import Run, Step

class State(TypedDict):
    """Agent state"""

    messages: list
    current_step: str
    data: Dict[str, Any]

class AegisGraph:
    """LangGraph-like wrapper with AegisRun enforcement"""

    def __init__(self, run: Run):
        self.run = run
        self.nodes: Dict[str, Callable] = {}
        self.edges: Dict[str, str] = {}

    def add_node(self, name: str, fn: Callable[[State], State]):
        """Add a node to the graph"""
        self.nodes[name] = fn
        return self

    def add_edge(self, from_node: str, to_node: str):
        """Add an edge between nodes"""
        self.edges[from_node] = to_node
        return self

    def compile(self) -> "CompiledGraph":
        """Compile the graph"""
        return CompiledGraph(self.run, self.nodes, self.edges)

class CompiledGraph:
    """Compiled graph ready for execution"""

    def __init__(
        self, run: Run, nodes: Dict[str, Callable], edges: Dict[str, str]
    ):
        self.run = run
        self.nodes = nodes
        self.edges = edges

    def invoke(self, initial_state: State) -> State:
        """Execute the graph"""
        state = initial_state
        current_node = "start"

        while current_node in self.nodes:
            node_fn = self.nodes[current_node]

            # Execute node as a step
            def execute_node(step: Step):
                return node_fn(state)

            state = self.run.step(
                name=current_node,
                state_vector={"node": current_node, "state": state},
                fn=execute_node,
            )

            # Get next node
            current_node = self.edges.get(current_node)
            if not current_node:
                break

        return state

def example_usage():
    """Example LangGraph-style usage"""
    from aegisrun import AegisRunClient

    client = AegisRunClient(base_url="http://localhost:8080")

    run = Run(
        client=client,
        agent_id="langgraph-agent",
        policy_ref="langgraph-demo",
    ).start()

    # Define nodes
    def plan(state: State) -> State:
        print("Planning...")
        state["messages"].append({"role": "plan", "content": "Create a plan"})
        return state

    def execute(state: State) -> State:
        print("Executing...")
        state["messages"].append({"role": "execute", "content": "Execute plan"})
        return state

    def reflect(state: State) -> State:
        print("Reflecting...")
        state["messages"].append({"role": "reflect", "content": "Reflect on results"})
        return state

    # Build graph
    graph = (
        AegisGraph(run)
        .add_node("start", plan)
        .add_node("execute", execute)
        .add_node("reflect", reflect)
        .add_edge("start", "execute")
        .add_edge("execute", "reflect")
    )

    # Compile and run
    compiled = graph.compile()
    final_state = compiled.invoke(
        State(messages=[], current_step="start", data={})
    )

    print(f"Final state: {final_state}")

    run.end(outcome={"state": final_state})

if __name__ == "__main__":
    example_usage()
