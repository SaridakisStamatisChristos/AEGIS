import { useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { formatDistanceToNow } from 'date-fns';
import { 
  ArrowLeft, 
  Activity, 
  Clock, 
  CheckCircle, 
  XCircle, 
  Ban,
  Download,
  RefreshCw
} from 'lucide-react';
import client from '../api/client';
import { Run, Step, Event } from '../types';
import StepTimeline from '../components/StepTimeline';

export default function RunDetail() {
  const { runId } = useParams<{ runId: string }>();
  const [selectedStepId, setSelectedStepId] = useState<string | undefined>();

  const { data: run, isLoading: runLoading } = useQuery({
    queryKey: ['run', runId],
    queryFn: async () => {
      const response = await client.get(`/runs/${runId}`);
      return response.data as Run;
    },
  });

  const { data: steps } = useQuery({
    queryKey: ['run-steps', runId],
    queryFn: async () => {
      const response = await client.get(`/runs/${runId}/steps`);
      return response.data.steps as Step[];
    },
  });

  const { data: events } = useQuery({
    queryKey: ['run-events', runId],
    queryFn: async () => {
      const response = await client.get(`/runs/${runId}/events`);
      return response.data.events as Event[];
    },
  });

  if (runLoading) {
    return <div className="text-center py-12">Loading run details...</div>;
  }

  if (!run) {
    return (
      <div className="text-center py-12">
        <p className="text-gray-500">Run not found</p>
        <Link to="/runs" className="text-blue-600 hover:text-blue-500 mt-2 inline-block">
          Back to runs
        </Link>
      </div>
    );
  }

  const statusConfig = {
    running: { icon: Activity, color: 'text-blue-600', bgColor: 'bg-blue-100' },
    completed: { icon: CheckCircle, color: 'text-green-600', bgColor: 'bg-green-100' },
    failed: { icon: XCircle, color: 'text-red-600', bgColor: 'bg-red-100' },
    blocked: { icon: Ban, color: 'text-orange-600', bgColor: 'bg-orange-100' },
    cancelled: { icon: Clock, color: 'text-gray-600', bgColor: 'bg-gray-100' },
  };

  const { icon: StatusIcon, color, bgColor } = 
    statusConfig[run.status] || statusConfig.running;

  return (
    <div>
      {/* Header */}
      <div className="mb-6">
        <Link
          to="/runs"
          className="inline-flex items-center text-sm text-gray-500 hover:text-gray-700 mb-4"
        >
          <ArrowLeft className="h-4 w-4 mr-1" />
          Back to runs
        </Link>

        <div className="flex items-start justify-between">
          <div>
            <div className="flex items-center">
              <h1 className="text-2xl font-semibold text-gray-900 font-mono">
                {run.run_id.slice(0, 16)}...
              </h1>
              <span className={`ml-3 inline-flex items-center px-3 py-1 rounded-full text-sm font-medium ${bgColor} ${color}`}>
                <StatusIcon className="h-4 w-4 mr-1" />
                {run.status}
              </span>
            </div>
            <p className="mt-1 text-sm text-gray-500">
              Policy: {run.policy_ref.policy_id} ({run.policy_ref.version})
            </p>
          </div>

          <div className="flex items-center space-x-2">
            <button className="inline-flex items-center px-3 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50">
              <RefreshCw className="h-4 w-4 mr-1" />
              Replay
            </button>
            <button className="inline-flex items-center px-3 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50">
              <Download className="h-4 w-4 mr-1" />
              Export Evidence
            </button>
          </div>
        </div>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-5 gap-4 mb-6">
        <div className="bg-white rounded-lg shadow p-4">
          <p className="text-sm text-gray-500">Steps</p>
          <p className="text-2xl font-semibold">{run.counters.steps}</p>
        </div>
        <div className="bg-white rounded-lg shadow p-4">
          <p className="text-sm text-gray-500">Tool Calls</p>
          <p className="text-2xl font-semibold">{run.counters.tool_calls}</p>
        </div>
        <div className="bg-white rounded-lg shadow p-4">
          <p className="text-sm text-gray-500">Blocks</p>
          <p className="text-2xl font-semibold text-red-600">{run.counters.blocks}</p>
        </div>
        <div className="bg-white rounded-lg shadow p-4">
          <p className="text-sm text-gray-500">Retries</p>
          <p className="text-2xl font-semibold">{run.counters.retries}</p>
        </div>
        <div className="bg-white rounded-lg shadow p-4">
          <p className="text-sm text-gray-500">Created</p>
          <p className="text-sm font-medium">
            {formatDistanceToNow(new Date(run.created_at), { addSuffix: true })}
          </p>
        </div>
      </div>

      {/* Main Content */}
      <div className="grid grid-cols-2 gap-6">
        {/* Steps Timeline */}
        <div className="bg-white rounded-lg shadow p-4">
          <h2 className="text-lg font-medium text-gray-900 mb-4">Steps</h2>
          {steps && steps.length > 0 ? (
            <StepTimeline
              steps={steps}
              selectedStepId={selectedStepId}
              onSelectStep={(step) => setSelectedStepId(step.step_id)}
            />
          ) : (
            <p className="text-gray-500 text-center py-8">No steps recorded</p>
          )}
        </div>

        {/* Events */}
        <div className="bg-white rounded-lg shadow p-4">
          <h2 className="text-lg font-medium text-gray-900 mb-4">
            Events ({events?.length ?? 0})
          </h2>
          {events && events.length > 0 ? (
            <div className="space-y-2 max-h-96 overflow-y-auto">
              {events.map((evt) => (
                <div key={evt.event_id} className="border border-gray-200 rounded p-3 text-sm">
                  <div className="flex items-center justify-between">
                    <span className="font-medium text-gray-900">{evt.event_type}</span>
                    <span className="text-xs text-gray-500">
                      {new Date(evt.timestamp).toLocaleTimeString()}
                    </span>
                  </div>
                  <pre className="mt-1 text-xs text-gray-600 bg-gray-50 rounded p-2 overflow-x-auto">
                    {JSON.stringify(evt.payload, null, 2)}
                  </pre>
                  <div className="mt-1 text-xs text-gray-400 font-mono truncate">
                    hash: {evt.event_hash.slice(0, 24)}...
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <p className="text-gray-500 text-center py-8">
              No events recorded
            </p>
          )}
        </div>
      </div>

      {/* Metadata */}
      {run.metadata && Object.keys(run.metadata).length > 0 && (
        <div className="mt-6 bg-white rounded-lg shadow p-4">
          <h2 className="text-lg font-medium text-gray-900 mb-4">Metadata</h2>
          <pre className="text-sm text-gray-700 bg-gray-50 rounded p-3 overflow-x-auto">
            {JSON.stringify(run.metadata, null, 2)}
          </pre>
        </div>
      )}
    </div>
  );
}
