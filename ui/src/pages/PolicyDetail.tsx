import { useParams, Link } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, Shield } from 'lucide-react';
import client from '../api/client';
import { Policy } from '../types';
import PolicyEditor from '../components/PolicyEditor';

export default function PolicyDetail() {
  const { policyId } = useParams<{ policyId: string }>();
  const queryClient = useQueryClient();

  const { data: policy, isLoading } = useQuery({
    queryKey: ['policy', policyId],
    queryFn: async () => {
      const response = await client.get(`/policies/${policyId}`);
      return response.data as Policy;
    },
  });

  const updateMutation = useMutation({
    mutationFn: async (spec: string) => {
      await client.put(`/policies/${policyId}/`, { spec: JSON.parse(spec) });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['policy', policyId] });
      queryClient.invalidateQueries({ queryKey: ['policies'] });
    },
  });

  if (isLoading) {
    return <div className="text-center py-12">Loading policy...</div>;
  }

  if (!policy) {
    return (
      <div className="text-center py-12">
        <p className="text-gray-500">Policy not found</p>
        <Link to="/policies" className="text-blue-600 hover:text-blue-500 mt-2 inline-block">
          Back to policies
        </Link>
      </div>
    );
  }

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="mb-6">
        <Link
          to="/policies"
          className="inline-flex items-center text-sm text-gray-500 hover:text-gray-700 mb-4"
        >
          <ArrowLeft className="h-4 w-4 mr-1" />
          Back to policies
        </Link>

        <div className="flex items-start justify-between">
          <div className="flex items-center">
            <Shield className="h-8 w-8 text-blue-600 mr-3" />
            <div>
              <h1 className="text-2xl font-semibold text-gray-900">{policy.name}</h1>
              <p className="text-sm text-gray-500">
                {policy.policy_id} • Version {policy.version}
              </p>
            </div>
          </div>

          <div className="flex items-center space-x-2">
            <span className={`px-3 py-1 rounded-full text-sm font-medium ${
              policy.status === 'deployed' 
                ? 'bg-green-100 text-green-800'
                : policy.status === 'draft'
                ? 'bg-gray-100 text-gray-800'
                : 'bg-yellow-100 text-yellow-800'
            }`}>
              {policy.status}
            </span>
          </div>
        </div>
      </div>

      {/* Policy Editor */}
      <div className="flex-1">
        <PolicyEditor
          initialValue={JSON.stringify(policy.spec, null, 2)}
          onSave={async (content) => {
            await updateMutation.mutateAsync(content);
          }}
          readOnly={policy.status === 'deployed'}
        />
      </div>
    </div>
  );
}
