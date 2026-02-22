import React from 'react';
import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { formatDistanceToNow } from 'date-fns';
import { Plus, Shield, FileText, CheckCircle, Clock, XCircle } from 'lucide-react';
import client from '../api/client';
import { Policy, PolicyStatus } from '../types';

export default function Policies() {
  const { data, isLoading } = useQuery({
    queryKey: ['policies'],
    queryFn: async () => {
      const response = await client.get('/policies');
      return response.data.policies as Policy[];
    },
  });

  if (isLoading) {
    return <div className="text-center py-12">Loading policies...</div>;
  }

  return (
    <div>
      <div className="sm:flex sm:items-center">
        <div className="sm:flex-auto">
          <h1 className="text-2xl font-semibold text-gray-900">Policies</h1>
          <p className="mt-2 text-sm text-gray-700">
            Manage agent policies and configurations
          </p>
        </div>
        <div className="mt-4 sm:mt-0 sm:ml-16 sm:flex-none">
          <Link
            to="/policies/new"
            className="inline-flex items-center rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white shadow-sm hover:bg-blue-500"
          >
            <Plus className="h-4 w-4 mr-1" />
            New Policy
          </Link>
        </div>
      </div>

      <div className="mt-8 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {data?.map((policy) => (
          <PolicyCard key={policy.policy_id} policy={policy} />
        ))}
      </div>

      {data?.length === 0 && (
        <div className="text-center py-12">
          <Shield className="mx-auto h-12 w-12 text-gray-400" />
          <h3 className="mt-2 text-sm font-medium text-gray-900">No policies</h3>
          <p className="mt-1 text-sm text-gray-500">
            Get started by creating a new policy.
          </p>
          <div className="mt-6">
            <Link
              to="/policies/new"
              className="inline-flex items-center rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white shadow-sm hover:bg-blue-500"
            >
              <Plus className="h-4 w-4 mr-1" />
              New Policy
            </Link>
          </div>
        </div>
      )}
    </div>
  );
}

function PolicyCard({ policy }: { policy: Policy }) {
  const statusConfig: Record<PolicyStatus, { icon: React.ComponentType<any>; color: string }> = {
    draft: { icon: FileText, color: 'bg-gray-100 text-gray-800' },
    review: { icon: Clock, color: 'bg-yellow-100 text-yellow-800' },
    approved: { icon: CheckCircle, color: 'bg-blue-100 text-blue-800' },
    deployed: { icon: Shield, color: 'bg-green-100 text-green-800' },
    deprecated: { icon: XCircle, color: 'bg-red-100 text-red-800' },
  };

  const { icon: StatusIcon, color } = statusConfig[policy.status] || statusConfig.draft;

  return (
    <Link
      to={`/policies/${policy.policy_id}`}
      className="block bg-white rounded-lg shadow hover:shadow-md transition-shadow p-4"
    >
      <div className="flex items-start justify-between">
        <div className="flex items-center">
          <Shield className="h-5 w-5 text-blue-600 mr-2" />
          <div>
            <h3 className="text-sm font-medium text-gray-900">{policy.name}</h3>
            <p className="text-xs text-gray-500">{policy.policy_id}</p>
          </div>
        </div>
        <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${color}`}>
          <StatusIcon className="h-3 w-3 mr-1" />
          {policy.status}
        </span>
      </div>

      <div className="mt-4 text-sm text-gray-600">
        <p>Version: {policy.version}</p>
        <p className="text-xs text-gray-400 mt-1">
          Created {formatDistanceToNow(new Date(policy.created_at), { addSuffix: true })}
        </p>
      </div>

      {policy.spec.tools && (
        <div className="mt-3">
          <p className="text-xs text-gray-500">
            {policy.spec.tools.length} tool rule(s)
          </p>
        </div>
      )}
    </Link>
  );
}
