import React, { useState } from 'react';
import { useQuery, useMutation } from '@tanstack/react-query';
import { formatDistanceToNow } from 'date-fns';
import {
  Download,
  CheckCircle,
  XCircle,
  Search,
  FileText,
  Shield,
} from 'lucide-react';
import client from '../api/client';
import { Run, VerifyResponse } from '../types';

export default function Evidence() {
  const [runIdInput, setRunIdInput] = useState('');
  const [verifyResult, setVerifyResult] = useState<VerifyResponse | null>(null);

  // List completed runs that might have evidence
  const { data: completedRuns, isLoading } = useQuery({
    queryKey: ['runs', { status: 'completed' }],
    queryFn: async () => {
      const response = await client.get('/runs/?status=completed&limit=50');
      return response.data.runs as Run[];
    },
  });

  const downloadMutation = useMutation({
    mutationFn: async (runId: string) => {
      const response = await client.get(`/evidence/runs/${runId}/bundle`, {
        responseType: 'blob',
      });
      return { blob: response.data as Blob, runId };
    },
    onSuccess: ({ blob, runId }) => {
      const url = window.URL.createObjectURL(new Blob([blob]));
      const link = document.createElement('a');
      link.href = url;
      link.setAttribute('download', `evidence-${runId}.zip`);
      document.body.appendChild(link);
      link.click();
      link.parentNode?.removeChild(link);
      window.URL.revokeObjectURL(url);
    },
  });

  const verifyMutation = useMutation({
    mutationFn: async (runId: string) => {
      const response = await client.post('/evidence/verify', { run_id: runId });
      return response.data as VerifyResponse;
    },
    onSuccess: (data) => {
      setVerifyResult(data);
    },
  });

  const handleVerifySubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (runIdInput.trim()) {
      verifyMutation.mutate(runIdInput.trim());
    }
  };

  if (isLoading) {
    return <div className="text-center py-12">Loading evidence...</div>;
  }

  return (
    <div>
      <div className="sm:flex sm:items-center mb-8">
        <div className="sm:flex-auto">
          <h1 className="text-2xl font-semibold text-gray-900">Evidence</h1>
          <p className="mt-2 text-sm text-gray-700">
            Download and verify tamper-evident evidence bundles for completed runs
          </p>
        </div>
      </div>

      {/* Verify by Run ID */}
      <div className="bg-white rounded-lg shadow p-6 mb-8">
        <h2 className="text-lg font-medium text-gray-900 mb-4 flex items-center">
          <Shield className="h-5 w-5 mr-2 text-blue-600" />
          Verify Evidence Integrity
        </h2>
        <form onSubmit={handleVerifySubmit} className="flex items-center gap-3">
          <div className="relative flex-1 max-w-md">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
            <input
              type="text"
              placeholder="Enter Run ID to verify..."
              value={runIdInput}
              onChange={(e) => setRunIdInput(e.target.value)}
              className="w-full pl-10 pr-4 py-2 border border-gray-300 rounded-md text-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
            />
          </div>
          <button
            type="submit"
            disabled={!runIdInput.trim() || verifyMutation.isPending}
            className="inline-flex items-center px-4 py-2 bg-blue-600 text-white rounded-md text-sm font-medium hover:bg-blue-700 disabled:opacity-50"
          >
            <CheckCircle className="h-4 w-4 mr-1" />
            {verifyMutation.isPending ? 'Verifying...' : 'Verify'}
          </button>
        </form>

        {verifyResult && (
          <div className={`mt-4 p-4 rounded-md ${
            verifyResult.chain_valid ? 'bg-green-50' : 'bg-red-50'
          }`}>
            <div className="flex items-center">
              {verifyResult.chain_valid ? (
                <CheckCircle className="h-5 w-5 text-green-600 mr-2" />
              ) : (
                <XCircle className="h-5 w-5 text-red-600 mr-2" />
              )}
              <span className={`font-medium ${
                verifyResult.chain_valid ? 'text-green-800' : 'text-red-800'
              }`}>
                {verifyResult.chain_valid ? 'Chain Valid' : 'Chain Invalid'}
              </span>
            </div>
            {verifyResult.message && (
              <p className={`mt-1 text-sm ${
                verifyResult.chain_valid ? 'text-green-700' : 'text-red-700'
              }`}>
                {verifyResult.message}
              </p>
            )}
            <p className="mt-1 text-xs text-gray-500">Run: {verifyResult.run_id}</p>
          </div>
        )}

        {verifyMutation.isError && (
          <div className="mt-4 p-4 bg-red-50 rounded-md text-sm text-red-700">
            Failed to verify: {String(verifyMutation.error)}
          </div>
        )}
      </div>

      {/* Completed Runs with Evidence */}
      <div className="bg-white rounded-lg shadow">
        <div className="p-4 border-b border-gray-200">
          <h2 className="text-lg font-medium text-gray-900">
            Completed Runs ({completedRuns?.length ?? 0})
          </h2>
        </div>
        <div className="divide-y divide-gray-200">
          {(!completedRuns || completedRuns.length === 0) ? (
            <div className="p-8 text-center text-gray-500">
              <FileText className="h-12 w-12 mx-auto mb-3 text-gray-300" />
              <p>No completed runs found</p>
            </div>
          ) : (
            completedRuns.map((run) => (
              <div key={run.run_id} className="p-4 hover:bg-gray-50 flex items-center justify-between">
                <div>
                  <span className="font-mono text-sm text-gray-900">
                    {run.run_id.slice(0, 16)}...
                  </span>
                  <p className="text-sm text-gray-500 mt-1">
                    {run.policy_ref.policy_id} · {run.counters.steps} steps · {run.counters.tool_calls} tool calls
                  </p>
                  <p className="text-xs text-gray-400 mt-0.5">
                    {formatDistanceToNow(new Date(run.created_at), { addSuffix: true })}
                    {run.evidence_hash && (
                      <span className="ml-2 text-green-600">✓ signed</span>
                    )}
                  </p>
                </div>
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => verifyMutation.mutate(run.run_id)}
                    disabled={verifyMutation.isPending}
                    className="inline-flex items-center px-3 py-1.5 text-sm font-medium rounded-md text-gray-700 bg-gray-100 hover:bg-gray-200 disabled:opacity-50"
                  >
                    <CheckCircle className="h-4 w-4 mr-1" />
                    Verify
                  </button>
                  <button
                    onClick={() => downloadMutation.mutate(run.run_id)}
                    disabled={downloadMutation.isPending}
                    className="inline-flex items-center px-3 py-1.5 text-sm font-medium rounded-md text-white bg-blue-600 hover:bg-blue-700 disabled:opacity-50"
                  >
                    <Download className="h-4 w-4 mr-1" />
                    Download
                  </button>
                </div>
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  );
}
