import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { formatDistanceToNow } from 'date-fns';
import client from '../api/client';
import { Run } from '../types';
import { Activity, Clock, CheckCircle, XCircle, Ban } from 'lucide-react';

export default function Runs() {
  const { data, isLoading } = useQuery({
    queryKey: ['runs'],
    queryFn: async () => {
      const response = await client.get('/runs');
      return response.data.runs as Run[];
    },
  });

  if (isLoading) {
    return <div className="text-center py-12">Loading runs...</div>;
  }

  return (
    <div>
      <div className="sm:flex sm:items-center">
        <div className="sm:flex-auto">
          <h1 className="text-2xl font-semibold text-gray-900">Runs</h1>
          <p className="mt-2 text-sm text-gray-700">
            View and manage all agent execution runs
          </p>
        </div>
      </div>

      <div className="mt-8 flow-root">
        <div className="-mx-4 -my-2 overflow-x-auto sm:-mx-6 lg:-mx-8">
          <div className="inline-block min-w-full py-2 align-middle sm:px-6 lg:px-8">
            <table className="min-w-full divide-y divide-gray-300">
              <thead>
                <tr>
                  <th className="py-3.5 pl-4 pr-3 text-left text-sm font-semibold text-gray-900">
                    Run ID
                  </th>
                  <th className="px-3 py-3.5 text-left text-sm font-semibold text-gray-900">
                    Status
                  </th>
                  <th className="px-3 py-3.5 text-left text-sm font-semibold text-gray-900">
                    Policy
                  </th>
                  <th className="px-3 py-3.5 text-left text-sm font-semibold text-gray-900">
                    Steps
                  </th>
                  <th className="px-3 py-3.5 text-left text-sm font-semibold text-gray-900">
                    Tool Calls
                  </th>
                  <th className="px-3 py-3.5 text-left text-sm font-semibold text-gray-900">
                    Created
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200 bg-white">
                {data?.map((run) => (
                  <tr key={run.run_id} className="hover:bg-gray-50">
                    <td className="whitespace-nowrap py-4 pl-4 pr-3 text-sm">
                      <Link
                        to={`/runs/${run.run_id}`}
                        className="font-medium text-blue-600 hover:text-blue-500"
                      >
                        {run.run_id.slice(0, 8)}...
                      </Link>
                    </td>
                    <td className="whitespace-nowrap px-3 py-4 text-sm">
                      <StatusBadge status={run.status} />
                    </td>
                    <td className="whitespace-nowrap px-3 py-4 text-sm text-gray-500">
                      {run.policy_ref.policy_id} ({run.policy_ref.version})
                    </td>
                    <td className="whitespace-nowrap px-3 py-4 text-sm text-gray-500">
                      {run.counters.steps}
                    </td>
                    <td className="whitespace-nowrap px-3 py-4 text-sm text-gray-500">
                      {run.counters.tool_calls}
                    </td>
                    <td className="whitespace-nowrap px-3 py-4 text-sm text-gray-500">
                      {formatDistanceToNow(new Date(run.created_at), { addSuffix: true })}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const config = {
    running: { icon: Activity, color: 'bg-blue-100 text-blue-800' },
    completed: { icon: CheckCircle, color: 'bg-green-100 text-green-800' },
    failed: { icon: XCircle, color: 'bg-red-100 text-red-800' },
    blocked: { icon: Ban, color: 'bg-orange-100 text-orange-800' },
    cancelled: { icon: Clock, color: 'bg-gray-100 text-gray-800' },
  };

  const { icon: Icon, color } = config[status as keyof typeof config] || config.running;

  return (
    <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${color}`}>
      <Icon className="h-3 w-3 mr-1" />
      {status}
    </span>
  );
}
