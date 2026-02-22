import { useState } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, Shield } from 'lucide-react';
import client from '../api/client';
import PolicyEditor from '../components/PolicyEditor';

export default function PolicyCreate() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [name, setName] = useState('');

  const createMutation = useMutation({
    mutationFn: async (spec: string) => {
      const response = await client.post('/policies', {
        name: name.trim(),
        spec: JSON.parse(spec),
      });
      return response.data;
    },
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['policies'] });
      navigate(`/policies/${data.policy_id ?? ''}`);
    },
  });

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

        <div className="flex items-center">
          <Shield className="h-8 w-8 text-blue-600 mr-3" />
          <div>
            <h1 className="text-2xl font-semibold text-gray-900">Create Policy</h1>
            <p className="text-sm text-gray-500">Define a new agent policy</p>
          </div>
        </div>

        {/* Policy Name */}
        <div className="mt-4">
          <label htmlFor="policy-name" className="block text-sm font-medium text-gray-700">
            Policy Name
          </label>
          <input
            id="policy-name"
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="e.g. default-agent-policy"
            className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
          />
        </div>
      </div>

      {/* Policy Editor */}
      <div className="flex-1">
        <PolicyEditor
          onSave={async (content) => {
            await createMutation.mutateAsync(content);
          }}
        />
      </div>
    </div>
  );
}
