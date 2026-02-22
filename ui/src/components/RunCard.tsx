import React from 'react';
import { Link } from 'react-router-dom';
import { formatDistanceToNow } from 'date-fns';
import { Activity, Clock, CheckCircle, XCircle, Ban } from 'lucide-react';
import { Run, RunStatus } from '../types';

interface RunCardProps {
  run: Run;
}

const statusConfig: Record<RunStatus, { icon: React.ComponentType<any>; color: string }> = {
  running: { icon: Activity, color: 'bg-blue-100 text-blue-800' },
  completed: { icon: CheckCircle, color: 'bg-green-100 text-green-800' },
  failed: { icon: XCircle, color: 'bg-red-100 text-red-800' },
  blocked: { icon: Ban, color: 'bg-orange-100 text-orange-800' },
  cancelled: { icon: Clock, color: 'bg-gray-100 text-gray-800' },
};

export default function RunCard({ run }: RunCardProps) {
  const { icon: StatusIcon, color } = statusConfig[run.status] || statusConfig.running;

  return (
    <Link
      to={`/runs/${run.run_id}`}
      className="block bg-white rounded-lg shadow hover:shadow-md transition-shadow p-4"
    >
      <div className="flex items-start justify-between">
        <div className="flex-1">
          <div className="flex items-center">
            <span className="font-mono text-sm text-gray-900">
              {run.run_id.slice(0, 8)}...
            </span>
            <span className={`ml-2 inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${color}`}>
              <StatusIcon className="h-3 w-3 mr-1" />
              {run.status}
            </span>
          </div>
          <p className="mt-1 text-sm text-gray-500">
            {run.policy_ref.policy_id} ({run.policy_ref.version})
          </p>
        </div>
        <div className="text-right text-sm text-gray-500">
          {formatDistanceToNow(new Date(run.created_at), { addSuffix: true })}
        </div>
      </div>
      
      <div className="mt-3 flex items-center space-x-4 text-sm text-gray-600">
        <div>
          <span className="font-medium">{run.counters.steps}</span> steps
        </div>
        <div>
          <span className="font-medium">{run.counters.tool_calls}</span> tool calls
        </div>
        {run.counters.blocks > 0 && (
          <div className="text-orange-600">
            <span className="font-medium">{run.counters.blocks}</span> blocked
          </div>
        )}
      </div>
    </Link>
  );
}
