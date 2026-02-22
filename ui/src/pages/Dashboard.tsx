import { useQuery } from '@tanstack/react-query';
import { 
  Activity, 
  XCircle, 
  AlertTriangle,
  Clock
} from 'lucide-react';
import client from '../api/client';

interface DashboardStats {
  total_runs: number;
  active_runs: number;
  completed_runs: number;
  failed_runs: number;
  total_tool_calls: number;
  total_blocks: number;
  status_counts: Record<string, number>;
}

export default function Dashboard() {
  // Fetch server-computed aggregate stats (avoids fetching 100 run rows).
  const { data: stats, isLoading } = useQuery<DashboardStats>({
    queryKey: ['dashboard-stats'],
    queryFn: async () => {
      const response = await client.get('/stats');
      return response.data as DashboardStats;
    },
    refetchInterval: 15_000, // refresh every 15 s
  });

  if (isLoading) {
    return <div className="text-center py-12">Loading dashboard...</div>;
  }

  const statCards = [
    {
      name: 'Total Runs',
      value: stats?.total_runs || 0,
      icon: Activity,
      color: 'text-blue-600',
      bgColor: 'bg-blue-100',
    },
    {
      name: 'Active Runs',
      value: stats?.active_runs || 0,
      icon: Clock,
      color: 'text-green-600',
      bgColor: 'bg-green-100',
    },
    {
      name: 'Blocked',
      value: stats?.total_blocks || 0,
      icon: XCircle,
      color: 'text-red-600',
      bgColor: 'bg-red-100',
    },
    {
      name: 'Failed Runs',
      value: stats?.failed_runs || 0,
      icon: AlertTriangle,
      color: 'text-yellow-600',
      bgColor: 'bg-yellow-100',
    },
  ];

  return (
    <div>
      <div className="sm:flex sm:items-center">
        <div className="sm:flex-auto">
          <h1 className="text-2xl font-semibold text-gray-900">Dashboard</h1>
          <p className="mt-2 text-sm text-gray-700">
            Overview of your AegisRun agent control plane
          </p>
        </div>
      </div>

      {/* Stats Grid */}
      <div className="mt-8 grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
        {statCards.map((stat) => (
          <div
            key={stat.name}
            className="bg-white overflow-hidden shadow rounded-lg"
          >
            <div className="p-5">
              <div className="flex items-center">
                <div className={`flex-shrink-0 ${stat.bgColor} rounded-md p-3`}>
                  <stat.icon className={`h-6 w-6 ${stat.color}`} />
                </div>
                <div className="ml-5 w-0 flex-1">
                  <dl>
                    <dt className="text-sm font-medium text-gray-500 truncate">
                      {stat.name}
                    </dt>
                    <dd className="text-lg font-semibold text-gray-900">
                      {stat.value}
                    </dd>
                  </dl>
                </div>
              </div>
            </div>
          </div>
        ))}
      </div>

      {/* Tool Call Stats */}
      <div className="mt-8 grid grid-cols-1 gap-5 lg:grid-cols-2">
        <div className="bg-white shadow rounded-lg p-6">
          <h3 className="text-lg font-medium text-gray-900 mb-4">
            Tool Call Summary
          </h3>
          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <span className="text-gray-600">Total Tool Calls</span>
              <span className="font-semibold">{stats?.total_tool_calls || 0}</span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-gray-600">Blocked Calls</span>
              <span className="font-semibold text-red-600">
                {stats?.total_blocks || 0}
              </span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-gray-600">Block Rate</span>
              <span className="font-semibold">
                {stats?.total_tool_calls
                  ? ((stats.total_blocks / stats.total_tool_calls) * 100).toFixed(1)
                  : 0}%
              </span>
            </div>
          </div>
        </div>

        <div className="bg-white shadow rounded-lg p-6">
          <h3 className="text-lg font-medium text-gray-900 mb-4">
            Run Status Distribution
          </h3>
          <div className="space-y-4">
            <div className="flex items-center">
              <div className="flex-1">
                <div className="flex items-center justify-between mb-1">
                  <span className="text-sm text-gray-600">Completed</span>
                  <span className="text-sm font-medium">{stats?.completed_runs || 0}</span>
                </div>
                <div className="w-full bg-gray-200 rounded-full h-2">
                  <div
                    className="bg-green-500 h-2 rounded-full"
                    style={{
                      width: `${
                        stats?.total_runs
                          ? (stats.completed_runs / stats.total_runs) * 100
                          : 0
                      }%`,
                    }}
                  />
                </div>
              </div>
            </div>
            <div className="flex items-center">
              <div className="flex-1">
                <div className="flex items-center justify-between mb-1">
                  <span className="text-sm text-gray-600">Failed</span>
                  <span className="text-sm font-medium">{stats?.failed_runs || 0}</span>
                </div>
                <div className="w-full bg-gray-200 rounded-full h-2">
                  <div
                    className="bg-red-500 h-2 rounded-full"
                    style={{
                      width: `${
                        stats?.total_runs
                          ? (stats.failed_runs / stats.total_runs) * 100
                          : 0
                      }%`,
                    }}
                  />
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
