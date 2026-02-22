/**
 * LangGraph-style wrapper for AegisRun
 * 
 * This provides a graph-based execution model similar to LangGraph,
 * with full policy enforcement via AegisRun.
 */

import { AegisRunClient } from '../src/client';
import { Run } from '../src/run';
import { Step } from '../src/step';

interface State {
  messages: Array<{ role: string; content: string }>;
  currentStep: string;
  data: Record<string, any>;
}

type NodeFunction = (state: State) => Promise<State>;

class AegisGraph {
  private run: Run;
  private nodes: Map<string, NodeFunction> = new Map();
  private edges: Map<string, string> = new Map();
  private conditionalEdges: Map<string, (state: State) => string> = new Map();

  constructor(run: Run) {
    this.run = run;
  }

  addNode(name: string, fn: NodeFunction): this {
    this.nodes.set(name, fn);
    return this;
  }

  addEdge(fromNode: string, toNode: string): this {
    this.edges.set(fromNode, toNode);
    return this;
  }

  addConditionalEdge(
    fromNode: string,
    conditionFn: (state: State) => string
  ): this {
    this.conditionalEdges.set(fromNode, conditionFn);
    return this;
  }

  compile(): CompiledGraph {
    return new CompiledGraph(
      this.run,
      this.nodes,
      this.edges,
      this.conditionalEdges
    );
  }
}

class CompiledGraph {
  constructor(
    private run: Run,
    private nodes: Map<string, NodeFunction>,
    private edges: Map<string, string>,
    private conditionalEdges: Map<string, (state: State) => string>
  ) {}

  async invoke(initialState: State): Promise<State> {
    let state = initialState;
    let currentNode: string | undefined = 'start';
    const visitedNodes: string[] = [];

    while (currentNode && this.nodes.has(currentNode)) {
      const nodeFn = this.nodes.get(currentNode)!;

      // Execute node as a step with AegisRun policy enforcement
      state = await this.run.step(
        currentNode,
        { node: currentNode, state, visited: visitedNodes },
        async (step: Step) => {
          return await nodeFn(state);
        }
      );

      visitedNodes.push(currentNode);

      // Check for conditional edge first
      if (this.conditionalEdges.has(currentNode)) {
        const conditionFn = this.conditionalEdges.get(currentNode)!;
        currentNode = conditionFn(state);
      } else {
        // Otherwise use regular edge
        currentNode = this.edges.get(currentNode);
      }

      // Break if we've reached END or an unknown node
      if (currentNode === 'END' || currentNode === undefined) {
        break;
      }
    }

    return state;
  }
}

// Example usage demonstrating a ReAct-style agent
async function exampleReActAgent() {
  const client = new AegisRunClient({ baseUrl: 'http://localhost:8080' });

  const run = await new Run({
    client,
    agentId: 'react-agent',
    policyRef: 'react-agent-policy:v1',
    metadata: {
      agent_type: 'react',
      task: 'information_gathering',
    },
  }).start();

  console.log(`Run started: ${run.runId}`);

  // Define graph nodes
  const think = async (state: State): Promise<State> => {
    console.log('Thinking about next action...');
    state.messages.push({
      role: 'assistant',
      content: 'Let me analyze the current situation and decide on next steps.',
    });
    state.data.thoughtCount = (state.data.thoughtCount || 0) + 1;
    return state;
  };

  const act = async (state: State): Promise<State> => {
    console.log('Executing action...');
    state.messages.push({
      role: 'action',
      content: 'Executing: search for relevant information',
    });
    state.data.actionTaken = true;
    return state;
  };

  const observe = async (state: State): Promise<State> => {
    console.log('Observing results...');
    state.messages.push({
      role: 'observation',
      content: 'Found relevant information successfully.',
    });
    state.data.observationCount = (state.data.observationCount || 0) + 1;
    return state;
  };

  const respond = async (state: State): Promise<State> => {
    console.log('Generating final response...');
    state.messages.push({
      role: 'assistant',
      content: 'Based on my research, here is the answer...',
    });
    state.data.complete = true;
    return state;
  };

  // Build the graph
  const graph = new AegisGraph(run)
    .addNode('start', think)
    .addNode('act', act)
    .addNode('observe', observe)
    .addNode('respond', respond)
    .addEdge('start', 'act')
    .addEdge('act', 'observe')
    .addConditionalEdge('observe', (state) => {
      // If we've done enough iterations, respond; otherwise think again
      if ((state.data.observationCount || 0) >= 2) {
        return 'respond';
      }
      return 'start'; // Loop back for more thinking
    })
    .addEdge('respond', 'END');

  // Execute the graph
  const compiled = graph.compile();
  const finalState = await compiled.invoke({
    messages: [],
    currentStep: 'start',
    data: {},
  });

  console.log('\n=== Final State ===');
  console.log('Messages:', finalState.messages.length);
  console.log('Thought iterations:', finalState.data.thoughtCount);
  console.log('Observations:', finalState.data.observationCount);
  console.log('Complete:', finalState.data.complete);

  run.end({ state: finalState });
  console.log(`\nRun completed: ${run.runId}`);
}

// Example: Plan-Execute-Reflect pattern
async function examplePlanExecuteReflect() {
  const client = new AegisRunClient({ baseUrl: 'http://localhost:8080' });

  const run = await new Run({
    client,
    agentId: 'plan-execute-agent',
    policyRef: 'plan-execute-policy:v1',
  }).start();

  const plan = async (state: State): Promise<State> => {
    console.log('Planning...');
    state.messages.push({ role: 'plan', content: 'Created 3-step plan' });
    state.data.plan = ['step1', 'step2', 'step3'];
    return state;
  };

  const execute = async (state: State): Promise<State> => {
    console.log('Executing plan...');
    const currentPlanStep = state.data.currentPlanIndex || 0;
    state.messages.push({
      role: 'execute',
      content: `Executing: ${state.data.plan[currentPlanStep]}`,
    });
    state.data.currentPlanIndex = currentPlanStep + 1;
    return state;
  };

  const reflect = async (state: State): Promise<State> => {
    console.log('Reflecting on execution...');
    state.messages.push({
      role: 'reflect',
      content: 'Execution successful, proceeding...',
    });
    return state;
  };

  const graph = new AegisGraph(run)
    .addNode('start', plan)
    .addNode('execute', execute)
    .addNode('reflect', reflect)
    .addEdge('start', 'execute')
    .addEdge('execute', 'reflect')
    .addConditionalEdge('reflect', (state) => {
      // Continue executing until plan is complete
      if (state.data.currentPlanIndex < state.data.plan.length) {
        return 'execute';
      }
      return 'END';
    });

  const compiled = graph.compile();
  const finalState = await compiled.invoke({
    messages: [],
    currentStep: 'start',
    data: {},
  });

  console.log('\nFinal plan execution:', finalState);
  run.end({ outcome: finalState });
}

// Export for use as a module
export { AegisGraph, CompiledGraph, State, NodeFunction };

// Run example if executed directly
if (require.main === module) {
  exampleReActAgent().catch(console.error);
}
