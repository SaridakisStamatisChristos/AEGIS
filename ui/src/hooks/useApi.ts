/**
 * Custom hooks for API interactions.
 * Aligned to actual Go backend endpoints and response shapes.
 */

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import client from '../api/client';
import type {
  Run,
  Step,
  Event,
  Policy,
  Approval,
  PolicyStatus,
  VerifyResponse,
} from '../types';

// ============================================
// Run Hooks
// ============================================

export function useRuns(filters?: {
  status?: string;
  policyId?: string;
  limit?: number;
}) {
  return useQuery({
    queryKey: ['runs', filters],
    queryFn: async () => {
      const params = new URLSearchParams();
      if (filters?.status) params.set('status', filters.status);
      if (filters?.policyId) params.set('policy_id', filters.policyId);
      if (filters?.limit) params.set('limit', filters.limit.toString());

      const response = await client.get(`/runs/?${params.toString()}`);
      return response.data.runs as Run[];
    },
    staleTime: 10000,
  });
}

export function useRun(runId: string | undefined) {
  return useQuery({
    queryKey: ['run', runId],
    queryFn: async () => {
      if (!runId) throw new Error('Run ID required');
      const response = await client.get(`/runs/${runId}/`);
      return response.data as Run;
    },
    enabled: !!runId,
  });
}

/** Fetches steps for a run via GET /runs/{runID}/steps */
export function useRunSteps(runId: string | undefined) {
  return useQuery({
    queryKey: ['run-steps', runId],
    queryFn: async () => {
      if (!runId) throw new Error('Run ID required');
      const response = await client.get(`/runs/${runId}/steps`);
      return response.data.steps as Step[];
    },
    enabled: !!runId,
  });
}

/** Fetches events for a run via GET /runs/{runID}/events */
export function useRunEvents(runId: string | undefined) {
  return useQuery({
    queryKey: ['run-events', runId],
    queryFn: async () => {
      if (!runId) throw new Error('Run ID required');
      const response = await client.get(`/runs/${runId}/events`);
      return response.data.events as Event[];
    },
    enabled: !!runId,
    refetchInterval: () => {
      // Could auto-refresh if run is still active — caller can override
      return false;
    },
  });
}

// ============================================
// Policy Hooks
// ============================================

export function usePolicies(filters?: {
  status?: PolicyStatus;
  limit?: number;
}) {
  return useQuery({
    queryKey: ['policies', filters],
    queryFn: async () => {
      const params = new URLSearchParams();
      if (filters?.status) params.set('status', filters.status);
      if (filters?.limit) params.set('limit', filters.limit.toString());

      const response = await client.get(`/policies/?${params.toString()}`);
      return response.data.policies as Policy[];
    },
  });
}

export function usePolicy(policyId: string | undefined, version?: string) {
  return useQuery({
    queryKey: ['policy', policyId, version],
    queryFn: async () => {
      if (!policyId) throw new Error('Policy ID required');
      const params = version ? `?version=${encodeURIComponent(version)}` : '';
      const response = await client.get(`/policies/${policyId}/${params}`);
      return response.data as Policy;
    },
    enabled: !!policyId,
  });
}

/** Create a new policy (draft). Body: { name, spec } */
export function useCreatePolicy() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (policy: {
      name: string;
      spec: Record<string, any>;
    }) => {
      const response = await client.post('/policies/', policy);
      return response.data as Policy;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['policies'] });
    },
  });
}

/** Update a policy (creates new version). Body: { spec } */
export function useUpdatePolicy() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({
      policyId,
      spec,
    }: {
      policyId: string;
      spec: Record<string, any>;
    }) => {
      const response = await client.put(`/policies/${policyId}/`, { spec });
      return response.data as Policy;
    },
    onSuccess: (_, { policyId }) => {
      queryClient.invalidateQueries({ queryKey: ['policy', policyId] });
      queryClient.invalidateQueries({ queryKey: ['policies'] });
    },
  });
}

/** DELETE /policies/{policyID}/ — soft delete (204) */
export function useDeletePolicy() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (policyId: string) => {
      await client.delete(`/policies/${policyId}/`);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['policies'] });
    },
  });
}

/** POST /policies/{policyID}/activate — deploy */
export function useActivatePolicy() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (policyId: string) => {
      const response = await client.post(`/policies/${policyId}/activate`);
      return response.data;
    },
    onSuccess: (_, policyId) => {
      queryClient.invalidateQueries({ queryKey: ['policy', policyId] });
      queryClient.invalidateQueries({ queryKey: ['policies'] });
    },
  });
}

/** POST /policies/{policyID}/deactivate — undeploy */
export function useDeactivatePolicy() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (policyId: string) => {
      const response = await client.post(`/policies/${policyId}/deactivate`);
      return response.data;
    },
    onSuccess: (_, policyId) => {
      queryClient.invalidateQueries({ queryKey: ['policy', policyId] });
      queryClient.invalidateQueries({ queryKey: ['policies'] });
    },
  });
}

// ============================================
// Evidence Hooks
// ============================================

/** GET /evidence/runs/{runID}/bundle — download ZIP */
export function useExportEvidence(runId: string) {
  return useMutation({
    mutationFn: async () => {
      const response = await client.get(`/evidence/runs/${runId}/bundle`, {
        responseType: 'blob',
      });
      return response.data as Blob;
    },
    onSuccess: (data) => {
      const url = window.URL.createObjectURL(data);
      const a = document.createElement('a');
      a.href = url;
      a.download = `evidence-${runId}.zip`;
      document.body.appendChild(a);
      a.click();
      window.URL.revokeObjectURL(url);
      a.remove();
    },
  });
}

/** POST /evidence/verify — body: { run_id } */
export function useVerifyIntegrity() {
  return useMutation({
    mutationFn: async (runId: string) => {
      const response = await client.post('/evidence/verify', {
        run_id: runId,
      });
      return response.data as VerifyResponse;
    },
  });
}

// ============================================
// Approval Hooks
// ============================================

/** GET /approvals/?policy_id=X&version=Y */
export function useApprovals(policyId?: string, version?: string) {
  return useQuery({
    queryKey: ['approvals', policyId, version],
    queryFn: async () => {
      const params = new URLSearchParams();
      if (policyId) params.set('policy_id', policyId);
      if (version) params.set('version', version);
      const response = await client.get(`/approvals/?${params.toString()}`);
      return response.data.approvals as Approval[];
    },
  });
}

/** POST /approvals/policies/{policyID}/approve?version=X */
export function useApprovePolicy() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({
      policyId,
      version,
      comment,
    }: {
      policyId: string;
      version: string;
      comment?: string;
    }) => {
      const response = await client.post(
        `/approvals/policies/${policyId}/approve?version=${encodeURIComponent(version)}`,
        { comment }
      );
      return response.data as Approval;
    },
    onSuccess: (_, { policyId }) => {
      queryClient.invalidateQueries({ queryKey: ['approvals'] });
      queryClient.invalidateQueries({ queryKey: ['policy', policyId] });
      queryClient.invalidateQueries({ queryKey: ['policies'] });
    },
  });
}

/** POST /approvals/policies/{policyID}/reject?version=X */
export function useRejectPolicy() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({
      policyId,
      version,
      comment,
    }: {
      policyId: string;
      version: string;
      comment: string;
    }) => {
      const response = await client.post(
        `/approvals/policies/${policyId}/reject?version=${encodeURIComponent(version)}`,
        { comment }
      );
      return response.data as Approval;
    },
    onSuccess: (_, { policyId }) => {
      queryClient.invalidateQueries({ queryKey: ['approvals'] });
      queryClient.invalidateQueries({ queryKey: ['policy', policyId] });
      queryClient.invalidateQueries({ queryKey: ['policies'] });
    },
  });
}

// ============================================
// Utility Hooks
// ============================================

/**
 * Hook for polling data with configurable interval
 */
export function usePolling<T>(
  queryKey: string[],
  queryFn: () => Promise<T>,
  options?: {
    interval?: number;
    enabled?: boolean;
  }
) {
  return useQuery({
    queryKey,
    queryFn,
    refetchInterval: options?.interval ?? 5000,
    enabled: options?.enabled ?? true,
  });
}

/**
 * Hook for optimistic updates
 */
export function useOptimisticUpdate<T, V>(
  queryKey: string[],
  updateFn: (oldData: T | undefined, variables: V) => T
) {
  const queryClient = useQueryClient();

  return {
    onMutate: async (variables: V) => {
      await queryClient.cancelQueries({ queryKey });
      const previousData = queryClient.getQueryData<T>(queryKey);
      queryClient.setQueryData<T>(queryKey, (old) =>
        updateFn(old, variables)
      );
      return { previousData };
    },
    onError: (
      _err: unknown,
      _variables: V,
      context: { previousData: T | undefined } | undefined
    ) => {
      if (context?.previousData) {
        queryClient.setQueryData(queryKey, context.previousData);
      }
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey });
    },
  };
}
