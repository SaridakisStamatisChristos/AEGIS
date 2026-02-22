import { useState } from 'react';
import { X, CheckCircle, XCircle, AlertTriangle, Shield } from 'lucide-react';
import { Policy } from '../types';

interface ApprovalModalProps {
  policy: Policy;
  isOpen: boolean;
  onClose: () => void;
  onApprove: (comment?: string) => Promise<void>;
  onReject: (comment: string) => Promise<void>;
}

export default function ApprovalModal({
  policy,
  isOpen,
  onClose,
  onApprove,
  onReject,
}: ApprovalModalProps) {
  const [comment, setComment] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);

  if (!isOpen) return null;

  const handleApprove = async () => {
    setIsSubmitting(true);
    try {
      await onApprove(comment || undefined);
      onClose();
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleReject = async () => {
    if (!comment.trim()) {
      alert('A comment is required when rejecting.');
      return;
    }
    setIsSubmitting(true);
    try {
      await onReject(comment);
      onClose();
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 overflow-y-auto">
      <div className="flex min-h-full items-end justify-center p-4 text-center sm:items-center sm:p-0">
        {/* Backdrop */}
        <div 
          className="fixed inset-0 bg-gray-500 bg-opacity-75 transition-opacity"
          onClick={onClose}
        />

        {/* Modal */}
        <div className="relative transform overflow-hidden rounded-lg bg-white text-left shadow-xl transition-all sm:my-8 sm:w-full sm:max-w-lg">
          <div className="bg-white px-4 pb-4 pt-5 sm:p-6 sm:pb-4">
            <div className="flex items-start justify-between">
              <div className="flex items-center">
                <div className="mx-auto flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-full bg-yellow-100">
                  <AlertTriangle className="h-6 w-6 text-yellow-600" />
                </div>
                <div className="ml-4">
                  <h3 className="text-lg font-semibold text-gray-900">
                    Policy Approval
                  </h3>
                  <p className="text-sm text-gray-500">
                    Review this policy change before deployment
                  </p>
                </div>
              </div>
              <button
                onClick={onClose}
                className="rounded-md bg-white text-gray-400 hover:text-gray-500"
              >
                <X className="h-6 w-6" />
              </button>
            </div>

            <div className="mt-4 space-y-4">
              <div>
                <h4 className="text-sm font-medium text-gray-700">Policy</h4>
                <p className="mt-1 text-sm text-gray-900 font-medium flex items-center">
                  <Shield className="h-4 w-4 text-blue-600 mr-1" />
                  {policy.name}
                </p>
              </div>

              <div>
                <h4 className="text-sm font-medium text-gray-700">Policy ID</h4>
                <p className="mt-1 text-xs text-gray-500 font-mono bg-gray-50 p-2 rounded">
                  {policy.policy_id}
                </p>
              </div>

              <div className="flex gap-4">
                <div>
                  <h4 className="text-sm font-medium text-gray-700">Version</h4>
                  <p className="mt-1 text-sm text-gray-900">{policy.version}</p>
                </div>
                <div>
                  <h4 className="text-sm font-medium text-gray-700">Status</h4>
                  <p className="mt-1 text-sm text-gray-900">{policy.status}</p>
                </div>
              </div>

              <div>
                <h4 className="text-sm font-medium text-gray-700">Spec (preview)</h4>
                <pre className="mt-1 text-xs text-gray-800 bg-gray-50 p-2 rounded overflow-x-auto max-h-40">
                  {JSON.stringify(policy.spec, null, 2)}
                </pre>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700">
                  Comment {' '}
                  <span className="text-gray-400">(required for rejection)</span>
                </label>
                <textarea
                  value={comment}
                  onChange={(e) => setComment(e.target.value)}
                  rows={2}
                  className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
                  placeholder="Provide a reason for your decision..."
                />
              </div>
            </div>
          </div>

          <div className="bg-gray-50 px-4 py-3 sm:flex sm:flex-row-reverse sm:px-6 gap-2">
            <button
              type="button"
              onClick={handleApprove}
              disabled={isSubmitting}
              className="inline-flex w-full justify-center items-center rounded-md bg-green-600 px-3 py-2 text-sm font-semibold text-white shadow-sm hover:bg-green-500 sm:w-auto disabled:opacity-50"
            >
              <CheckCircle className="h-4 w-4 mr-1" />
              Approve
            </button>
            <button
              type="button"
              onClick={handleReject}
              disabled={isSubmitting}
              className="inline-flex w-full justify-center items-center rounded-md bg-red-600 px-3 py-2 text-sm font-semibold text-white shadow-sm hover:bg-red-500 sm:w-auto disabled:opacity-50"
            >
              <XCircle className="h-4 w-4 mr-1" />
              Reject
            </button>
            <button
              type="button"
              onClick={onClose}
              disabled={isSubmitting}
              className="mt-3 inline-flex w-full justify-center rounded-md bg-white px-3 py-2 text-sm font-semibold text-gray-900 shadow-sm ring-1 ring-inset ring-gray-300 hover:bg-gray-50 sm:mt-0 sm:w-auto"
            >
              Cancel
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
