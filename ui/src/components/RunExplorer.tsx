/**
 * RunExplorer Component
 * 
 * Displays a filterable, sortable list of agent runs with
 * real-time status updates and quick actions.
 */

import { useState, useMemo } from 'react';
import { Link } from 'react-router-dom';
import { formatDistanceToNow, format } from 'date-fns';
import {
  Activity,
  CheckCircle,
  XCircle,
  Ban,
  Clock,
  Search,
  Filter,
  RefreshCw,
  Download,
  ChevronDown,
  ChevronUp,
} from 'lucide-react';
import { useRuns, useExportEvidence } from '../hooks/useApi';
import type { Run, RunStatus } from '../types';

interface RunExplorerProps {
  // Optional filter presets
  policyId?: string;
  initialStatus?: RunStatus;
}

type SortField = 'created_at' | 'status' | 'steps' | 'tool_calls';
type SortDirection = 'asc' | 'desc';

const STATUS_CONFIG: Record<
  RunStatus,
  { icon: typeof Activity; color: string; bgColor: string }
> = {
  running: {
    icon: Activity,
    color: 'text-blue-700',
    bgColor: 'bg-blue-100',
  },
  completed: {
    icon: CheckCircle,
    color: 'text-green-700',
    bgColor: 'bg-green-100',
  },
  failed: {
    icon: XCircle,
    color: 'text-red-700',
    bgColor: 'bg-red-100',
  },
  blocked: {
    icon: Ban,
    color: 'text-orange-700',
    bgColor: 'bg-orange-100',
  },
  cancelled: {
    icon: Clock,
    color: 'text-gray-700',
    bgColor: 'bg-gray-100',
  },
};

export default function RunExplorer({
  policyId,
  initialStatus,
}: RunExplorerProps) {
  // State
  const [searchQuery, setSearchQuery] = useState('');
  const [statusFilter, setStatusFilter] = useState<RunStatus | 'all'>(
    initialStatus || 'all'
  );
  const [sortField, setSortField] = useState<SortField>('created_at');
  const [sortDirection, setSortDirection] = useState<SortDirection>('desc');
  const [selectedRuns, setSelectedRuns] = useState<Set<string>>(new Set());

  // Data fetching
  const { data: runs, isLoading, refetch, isRefetching } = useRuns({
    policyId,
    status: statusFilter !== 'all' ? statusFilter : undefined,
  });

  // Filtered and sorted runs
  const filteredRuns = useMemo(() => {
    if (!runs) return [];

    let result = [...runs];

    // Search filter
    if (searchQuery) {
      const query = searchQuery.toLowerCase();
      result = result.filter(
        (run) =>
          run.run_id.toLowerCase().includes(query) ||
          run.policy_ref.policy_id.toLowerCase().includes(query) ||
          JSON.stringify(run.metadata).toLowerCase().includes(query)
      );
    }

    // Sort
    result.sort((a, b) => {
      let comparison = 0;

      switch (sortField) {
        case 'created_at':
          comparison =
            new Date(a.created_at).getTime() - new Date(b.created_at).getTime();
          break;
        case 'status':
          comparison = a.status.localeCompare(b.status);
          break;
        case 'steps':
          comparison = a.counters.steps - b.counters.steps;
          break;
        case 'tool_calls':
          comparison = a.counters.tool_calls - b.counters.tool_calls;
          break;
      }

      return sortDirection === 'asc' ? comparison : -comparison;
    });

    return result;
  }, [runs, searchQuery, sortField, sortDirection]);

  // Handlers
  const handleSort = (field: SortField) => {
    if (sortField === field) {
      setSortDirection(sortDirection === 'asc' ? 'desc' : 'asc');
    } else {
      setSortField(field);
      setSortDirection('desc');
    }
  };

  const handleSelectAll = () => {
    if (selectedRuns.size === filteredRuns.length) {
      setSelectedRuns(new Set());
    } else {
      setSelectedRuns(new Set(filteredRuns.map((r) => r.run_id)));
    }
  };

  const handleSelectRun = (runId: string) => {
    const newSelected = new Set(selectedRuns);
    if (newSelected.has(runId)) {
      newSelected.delete(runId);
    } else {
      newSelected.add(runId);
    }
    setSelectedRuns(newSelected);
  };

  // Render sort indicator
  const SortIndicator = ({ field }: { field: SortField }) => {
    if (sortField !== field) return null;
    return sortDirection === 'asc' ? (
      <ChevronUp className="h-4 w-4" />
    ) : (
      <ChevronDown className="h-4 w-4" />
    );
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <RefreshCw className="h-6 w-6 animate-spin text-gray-400" />
        <span className="ml-2 text-gray-500">Loading runs...</span>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-gray-900">Agent Runs</h2>
        <button
          onClick={() => refetch()}
          disabled={isRefetching}
          className="inline-flex items-center px-3 py-1.5 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 disabled:opacity-50"
        >
          <RefreshCw
            className={`h-4 w-4 mr-1.5 ${isRefetching ? 'animate-spin' : ''}`}
          />
          Refresh
        </button>
      </div>

      {/* Filters */}
      <div className="flex flex-col sm:flex-row gap-3">
        {/* Search */}
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
          <input
            type="text"
            placeholder="Search by ID, policy, or metadata..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="w-full pl-10 pr-4 py-2 border border-gray-300 rounded-md text-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
          />
        </div>

        {/* Status filter */}
        <div className="relative">
          <Filter className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value as RunStatus | 'all')}
            className="pl-10 pr-8 py-2 border border-gray-300 rounded-md text-sm bg-white focus:ring-2 focus:ring-blue-500 focus:border-blue-500 appearance-none"
          >
            <option value="all">All Status</option>
            <option value="running">Running</option>
            <option value="completed">Completed</option>
            <option value="failed">Failed</option>
            <option value="blocked">Blocked</option>
            <option value="cancelled">Cancelled</option>
          </select>
        </div>
      </div>

      {/* Bulk actions */}
      {selectedRuns.size > 0 && (
        <div className="flex items-center gap-3 p-3 bg-blue-50 rounded-md">
          <span className="text-sm text-blue-700">
            {selectedRuns.size} run{selectedRuns.size > 1 ? 's' : ''} selected
          </span>
          <button
            onClick={() => setSelectedRuns(new Set())}
            className="text-sm text-gray-600 hover:text-gray-700"
          >
            Clear Selection
          </button>
        </div>
      )}

      {/* Table */}
      <div className="overflow-hidden border border-gray-200 rounded-lg">
        <table className="min-w-full divide-y divide-gray-200">
          <thead className="bg-gray-50">
            <tr>
              <th className="px-4 py-3 text-left">
                <input
                  type="checkbox"
                  checked={
                    selectedRuns.size === filteredRuns.length &&
                    filteredRuns.length > 0
                  }
                  onChange={handleSelectAll}
                  className="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                />
              </th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Run ID
              </th>
              <th
                className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:text-gray-700"
                onClick={() => handleSort('status')}
              >
                <div className="flex items-center gap-1">
                  Status
                  <SortIndicator field="status" />
                </div>
              </th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Policy
              </th>
              <th
                className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:text-gray-700"
                onClick={() => handleSort('steps')}
              >
                <div className="flex items-center gap-1">
                  Steps
                  <SortIndicator field="steps" />
                </div>
              </th>
              <th
                className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:text-gray-700"
                onClick={() => handleSort('tool_calls')}
              >
                <div className="flex items-center gap-1">
                  Tool Calls
                  <SortIndicator field="tool_calls" />
                </div>
              </th>
              <th
                className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:text-gray-700"
                onClick={() => handleSort('created_at')}
              >
                <div className="flex items-center gap-1">
                  Created
                  <SortIndicator field="created_at" />
                </div>
              </th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Actions
              </th>
            </tr>
          </thead>
          <tbody className="bg-white divide-y divide-gray-200">
            {filteredRuns.length === 0 ? (
              <tr>
                <td
                  colSpan={8}
                  className="px-4 py-8 text-center text-sm text-gray-500"
                >
                  No runs found matching your criteria.
                </td>
              </tr>
            ) : (
              filteredRuns.map((run) => (
                <RunRow
                  key={run.run_id}
                  run={run}
                  isSelected={selectedRuns.has(run.run_id)}
                  onSelect={() => handleSelectRun(run.run_id)}
                />
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Summary */}
      <div className="text-sm text-gray-500">
        Showing {filteredRuns.length} of {runs?.length || 0} runs
      </div>
    </div>
  );
}

// Individual run row component
function RunRow({
  run,
  isSelected,
  onSelect,
}: {
  run: Run;
  isSelected: boolean;
  onSelect: () => void;
}) {
  const statusConfig = STATUS_CONFIG[run.status] || STATUS_CONFIG.running;
  const StatusIcon = statusConfig.icon;

  const exportEvidence = useExportEvidence(run.run_id);

  return (
    <tr className={`hover:bg-gray-50 ${isSelected ? 'bg-blue-50' : ''}`}>
      <td className="px-4 py-3">
        <input
          type="checkbox"
          checked={isSelected}
          onChange={onSelect}
          className="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
        />
      </td>
      <td className="px-4 py-3">
        <Link
          to={`/runs/${run.run_id}`}
          className="text-sm font-medium text-blue-600 hover:text-blue-500"
        >
          {run.run_id.slice(0, 12)}...
        </Link>
      </td>
      <td className="px-4 py-3">
        <span
          className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${statusConfig.bgColor} ${statusConfig.color}`}
        >
          <StatusIcon className="h-3 w-3 mr-1" />
          {run.status}
        </span>
      </td>
      <td className="px-4 py-3 text-sm text-gray-500">
        <div>
          {run.policy_ref.policy_id}
          <span className="text-gray-400 ml-1">
            ({run.policy_ref.version})
          </span>
        </div>
      </td>
      <td className="px-4 py-3 text-sm text-gray-500">{run.counters.steps}</td>
      <td className="px-4 py-3 text-sm text-gray-500">
        <div className="flex items-center gap-2">
          {run.counters.tool_calls}
          {run.counters.blocks > 0 && (
            <span className="text-xs text-red-600">
              ({run.counters.blocks} blocked)
            </span>
          )}
        </div>
      </td>
      <td className="px-4 py-3 text-sm text-gray-500">
        <div title={format(new Date(run.created_at), 'PPpp')}>
          {formatDistanceToNow(new Date(run.created_at), { addSuffix: true })}
        </div>
      </td>
      <td className="px-4 py-3">
        <div className="flex items-center gap-2">
          <button
            onClick={() => exportEvidence.mutate()}
            disabled={exportEvidence.isPending}
            className="text-gray-400 hover:text-gray-600"
            title="Download Evidence"
          >
            <Download className="h-4 w-4" />
          </button>
        </div>
      </td>
    </tr>
  );
}
