import { useState } from 'react';
import { 
  Download, 
  FileText, 
  Hash, 
  CheckCircle, 
  XCircle,
  Search,
  Activity,
} from 'lucide-react';
import type { Run } from '../types';

interface EvidenceExplorerProps {
  runs: Run[];
  onDownload: (runId: string) => Promise<void>;
  onVerify: (runId: string) => Promise<{ valid: boolean; message?: string }>;
}

export default function EvidenceExplorer({
  runs,
  onDownload,
  onVerify,
}: EvidenceExplorerProps) {
  const [searchTerm, setSearchTerm] = useState('');
  const [verificationStatus, setVerificationStatus] = useState<
    Record<string, { checking: boolean; result?: { valid: boolean; message?: string } }>
  >({});

  const filteredRuns = runs.filter(
    (run) =>
      run.run_id.toLowerCase().includes(searchTerm.toLowerCase()) ||
      run.policy_ref.policy_id.toLowerCase().includes(searchTerm.toLowerCase())
  );

  const handleVerify = async (runId: string) => {
    setVerificationStatus((prev) => ({
      ...prev,
      [runId]: { checking: true },
    }));

    try {
      const result = await onVerify(runId);
      setVerificationStatus((prev) => ({
        ...prev,
        [runId]: { checking: false, result },
      }));
    } catch (error) {
      setVerificationStatus((prev) => ({
        ...prev,
        [runId]: { 
          checking: false, 
          result: { valid: false, message: String(error) } 
        },
      }));
    }
  };

  return (
    <div className="bg-white rounded-lg shadow">
      <div className="p-4 border-b border-gray-200">
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-medium text-gray-900">Evidence Explorer</h2>
          <div className="flex items-center space-x-2">
            <div className="relative">
              <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-gray-400" />
              <input
                type="text"
                placeholder="Search by run ID or policy..."
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                className="pl-9 pr-4 py-2 border border-gray-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
          </div>
        </div>
      </div>

      <div className="divide-y divide-gray-200">
        {filteredRuns.length === 0 ? (
          <div className="p-8 text-center text-gray-500">
            <FileText className="h-12 w-12 mx-auto mb-3 text-gray-300" />
            <p>No runs found</p>
          </div>
        ) : (
          filteredRuns.map((run) => {
            const status = verificationStatus[run.run_id];

            return (
              <div key={run.run_id} className="p-4 hover:bg-gray-50">
                <div className="flex items-start justify-between">
                  <div className="flex-1">
                    <div className="flex items-center">
                      <Activity className="h-5 w-5 text-gray-400 mr-2" />
                      <span className="font-mono text-sm text-gray-900">
                        {run.run_id.slice(0, 16)}...
                      </span>
                      <span className={`ml-2 px-2 py-0.5 rounded-full text-xs font-medium ${
                        run.status === 'completed' ? 'bg-green-100 text-green-700' :
                        run.status === 'failed' ? 'bg-red-100 text-red-700' :
                        'bg-gray-100 text-gray-700'
                      }`}>
                        {run.status}
                      </span>
                      {status?.result && (
                        <span className={`ml-2 flex items-center text-sm ${
                          status.result.valid ? 'text-green-600' : 'text-red-600'
                        }`}>
                          {status.result.valid ? (
                            <>
                              <CheckCircle className="h-4 w-4 mr-1" />
                              Verified
                            </>
                          ) : (
                            <>
                              <XCircle className="h-4 w-4 mr-1" />
                              Invalid
                            </>
                          )}
                        </span>
                      )}
                    </div>
                    
                    <div className="mt-2 space-y-1">
                      <p className="text-sm text-gray-500">
                        Policy: <span className="font-mono">{run.policy_ref.policy_id}</span>
                        <span className="text-gray-400 ml-1">v{run.policy_ref.version}</span>
                      </p>
                      {run.evidence_hash && (
                        <div className="flex items-center text-sm text-gray-500">
                          <Hash className="h-3 w-3 mr-1" />
                          <span className="font-mono text-xs">{run.evidence_hash.slice(0, 32)}...</span>
                        </div>
                      )}
                      <p className="text-sm text-gray-500">
                        {run.counters.steps} steps, {run.counters.tool_calls} tool calls
                        {' '}&bull;{' '}{new Date(run.created_at).toLocaleString()}
                      </p>
                    </div>
                  </div>

                  <div className="flex items-center space-x-2">
                    <button
                      onClick={() => handleVerify(run.run_id)}
                      disabled={status?.checking}
                      className="inline-flex items-center px-3 py-1.5 text-sm font-medium rounded-md text-gray-700 bg-gray-100 hover:bg-gray-200 disabled:opacity-50"
                    >
                      {status?.checking ? (
                        'Verifying...'
                      ) : (
                        <>
                          <CheckCircle className="h-4 w-4 mr-1" />
                          Verify
                        </>
                      )}
                    </button>
                    <button
                      onClick={() => onDownload(run.run_id)}
                      className="inline-flex items-center px-3 py-1.5 text-sm font-medium rounded-md text-white bg-blue-600 hover:bg-blue-700"
                    >
                      <Download className="h-4 w-4 mr-1" />
                      Download
                    </button>
                  </div>
                </div>

                {status?.result && !status.result.valid && status.result.message && (
                  <div className="mt-2 p-2 bg-red-50 rounded text-sm text-red-700">
                    {status.result.message}
                  </div>
                )}
              </div>
            );
          })
        )}
      </div>
    </div>
  );
}
