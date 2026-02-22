import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { formatDistanceToNow } from 'date-fns';
import { AlertTriangle, CheckCircle, XCircle, Shield } from 'lucide-react';
import client from '../api/client';
import { Approval, Policy } from '../types';
import ApprovalModal from '../components/ApprovalModal';

export default function Approvals() {
  const [selectedPolicy, setSelectedPolicy] = useState<Policy | null>(null);
  const queryClient = useQueryClient();

  // Fetch policies in "review" status (awaiting approval)
  const { data: reviewPolicies, isLoading: loadingPolicies } = useQuery({
    queryKey: ['policies', { status: 'review' }],
    queryFn: async () => {
      const response = await client.get('/policies/?status=review');
      return response.data.policies as Policy[];
    },
  });

  // Fetch all approval records
  const { data: approvals, isLoading: loadingApprovals } = useQuery({
    queryKey: ['approvals'],
    queryFn: async () => {
      const response = await client.get('/approvals/');
      return response.data.approvals as Approval[];
    },
  });

  const approveMutation = useMutation({
    mutationFn: async ({ policyId, version, comment }: { policyId: string; version: string; comment?: string }) => {
      await client.post(
        `/approvals/policies/${policyId}/approve?version=${encodeURIComponent(version)}`,
        { comment }
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['approvals'] });
      queryClient.invalidateQueries({ queryKey: ['policies'] });
    },
  });

  const rejectMutation = useMutation({
    mutationFn: async ({ policyId, version, comment }: { policyId: string; version: string; comment: string }) => {
      await client.post(
        `/approvals/policies/${policyId}/reject?version=${encodeURIComponent(version)}`,
        { comment }
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['approvals'] });
      queryClient.invalidateQueries({ queryKey: ['policies'] });
    },
  });

  if (loadingPolicies || loadingApprovals) {
    return <div className="text-center py-12">Loading approvals...</div>;
  }

  return (
    <div>
      <div className="sm:flex sm:items-center">
        <div className="sm:flex-auto">
          <h1 className="text-2xl font-semibold text-gray-900">Approval Queue</h1>
          <p className="mt-2 text-sm text-gray-700">
            Review and approve policy changes
          </p>
        </div>
      </div>

      {/* Policies awaiting review */}
      <div className="mt-8">
        <h2 className="text-lg font-medium text-gray-900 mb-4">Pending Review</h2>
        {(!reviewPolicies || reviewPolicies.length === 0) ? (
          <div className="text-center py-12">
            <CheckCircle className="mx-auto h-12 w-12 text-green-400" />
            <h3 className="mt-2 text-sm font-medium text-gray-900">All caught up!</h3>
            <p className="mt-1 text-sm text-gray-500">
              No policies awaiting approval.
            </p>
          </div>
        ) : (
          <div className="space-y-4">
            {reviewPolicies.map((policy) => (
              <div
                key={policy.policy_id}
                className="bg-white rounded-lg shadow p-4 hover:shadow-md transition-shadow"
              >
                <div className="flex items-start justify-between">
                  <div className="flex-1">
                    <div className="flex items-center">
                      <Shield className="h-5 w-5 text-blue-600 mr-2" />
                      <span className="font-medium text-gray-900">
                        {policy.name}
                      </span>
                      <span className="ml-2 inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-yellow-100 text-yellow-800">
                        <AlertTriangle className="h-3 w-3 mr-1" />
                        review
                      </span>
                    </div>
                    <p className="mt-1 text-sm text-gray-500">
                      Policy: <span className="font-mono">{policy.policy_id}</span>
                      {' · '}Version: {policy.version}
                    </p>
                    <p className="text-xs text-gray-400 mt-1">
                      Created {formatDistanceToNow(new Date(policy.created_at), { addSuffix: true })}
                    </p>
                  </div>

                  <button
                    onClick={() => setSelectedPolicy(policy)}
                    className="inline-flex items-center px-3 py-1.5 text-sm font-medium rounded-md text-white bg-blue-600 hover:bg-blue-700"
                  >
                    Review
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Past approval decisions */}
      {approvals && approvals.length > 0 && (
        <div className="mt-12">
          <h2 className="text-lg font-medium text-gray-900 mb-4">Approval History</h2>
          <div className="space-y-3">
            {approvals.map((approval) => (
              <div
                key={approval.approval_id}
                className="bg-white rounded-lg shadow p-4"
              >
                <div className="flex items-center">
                  {approval.decision === 'approved' ? (
                    <CheckCircle className="h-5 w-5 text-green-500 mr-2" />
                  ) : (
                    <XCircle className="h-5 w-5 text-red-500 mr-2" />
                  )}
                  <span className="font-medium text-gray-900 capitalize">
                    {approval.decision}
                  </span>
                  <span className="ml-2 text-sm text-gray-500">
                    Policy {approval.policy_id} v{approval.version}
                  </span>
                  <span className="ml-auto text-xs text-gray-400">
                    {formatDistanceToNow(new Date(approval.created_at), { addSuffix: true })}
                  </span>
                </div>
                {approval.comment && (
                  <p className="mt-2 ml-7 text-sm text-gray-600">{approval.comment}</p>
                )}
                <p className="mt-1 ml-7 text-xs text-gray-400">by {approval.approver_id}</p>
              </div>
            ))}
          </div>
        </div>
      )}

      {selectedPolicy && (
        <ApprovalModal
          policy={selectedPolicy}
          isOpen={!!selectedPolicy}
          onClose={() => setSelectedPolicy(null)}
          onApprove={async (comment) => {
            await approveMutation.mutateAsync({
              policyId: selectedPolicy.policy_id,
              version: selectedPolicy.version,
              comment,
            });
            setSelectedPolicy(null);
          }}
          onReject={async (comment) => {
            await rejectMutation.mutateAsync({
              policyId: selectedPolicy.policy_id,
              version: selectedPolicy.version,
              comment,
            });
            setSelectedPolicy(null);
          }}
        />
      )}
    </div>
  );
}
