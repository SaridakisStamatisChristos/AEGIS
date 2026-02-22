/**
 * Tests for the Dashboard page component.
 *
 * Validates rendering of stat cards, loading state, and data display.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import Dashboard from '../pages/Dashboard';

// Mock the API client
vi.mock('../api/client', () => ({
  default: {
    get: vi.fn(),
  },
}));

import client from '../api/client';

function renderWithQuery(ui: React.ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        gcTime: 0,
      },
    },
  });
  return render(
    <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>,
  );
}

const STATS = {
  total_runs: 42,
  active_runs: 5,
  completed_runs: 30,
  failed_runs: 7,
  total_tool_calls: 200,
  total_blocks: 15,
  status_counts: { completed: 30, failed: 7, running: 5 },
};

describe('Dashboard', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('shows a loading indicator while fetching', () => {
    // Hang the request forever so we stay in loading state
    (client.get as ReturnType<typeof vi.fn>).mockReturnValue(new Promise(() => {}));
    renderWithQuery(<Dashboard />);
    expect(screen.getByText('Loading dashboard...')).toBeTruthy();
  });

  it('renders stat cards with correct values after data loads', async () => {
    (client.get as ReturnType<typeof vi.fn>).mockResolvedValue({ data: STATS });
    renderWithQuery(<Dashboard />);

    await waitFor(() => {
      expect(screen.getByText('Total Runs')).toBeTruthy();
    });

    expect(screen.getByText('42')).toBeTruthy(); // total_runs
    expect(screen.getByText('5')).toBeTruthy();  // active_runs
    const failedRunsLabel = screen.getByText('Failed Runs');
    const failedRunsCardValue = failedRunsLabel.parentElement?.querySelector('dd')?.textContent;
    expect(failedRunsCardValue).toBe('7'); // failed_runs
  });

  it('renders four stat cards', async () => {
    (client.get as ReturnType<typeof vi.fn>).mockResolvedValue({ data: STATS });
    renderWithQuery(<Dashboard />);

    await waitFor(() => {
      expect(screen.getByText('Total Runs')).toBeTruthy();
    });

    expect(screen.getByText('Active Runs')).toBeTruthy();
    expect(screen.getByText('Blocked')).toBeTruthy();
    expect(screen.getByText('Failed Runs')).toBeTruthy();
  });

  it('renders tool call summary section', async () => {
    (client.get as ReturnType<typeof vi.fn>).mockResolvedValue({ data: STATS });
    renderWithQuery(<Dashboard />);

    await waitFor(() => {
      expect(screen.getByText('Tool Call Summary')).toBeTruthy();
    });

    expect(screen.getByText('Total Tool Calls')).toBeTruthy();
    expect(screen.getByText('200')).toBeTruthy();
    expect(screen.getByText('Block Rate')).toBeTruthy();
  });

  it('renders run status distribution section', async () => {
    (client.get as ReturnType<typeof vi.fn>).mockResolvedValue({ data: STATS });
    renderWithQuery(<Dashboard />);

    await waitFor(() => {
      expect(screen.getByText('Run Status Distribution')).toBeTruthy();
    });

    expect(screen.getByText('Completed')).toBeTruthy();
    expect(screen.getByText('Failed')).toBeTruthy();
    // Progress bar completed % = 30/42 * 100 ≈ 71.4%
  });

  it('computes block rate correctly', async () => {
    (client.get as ReturnType<typeof vi.fn>).mockResolvedValue({ data: STATS });
    renderWithQuery(<Dashboard />);

    await waitFor(() => {
      expect(screen.getByText('Block Rate')).toBeTruthy();
    });

    // 15 / 200 * 100 = 7.5%
    expect(screen.getByText('7.5%')).toBeTruthy();
  });

  it('shows 0% block rate when there are no tool calls', async () => {
    (client.get as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: { ...STATS, total_tool_calls: 0, total_blocks: 0 },
    });
    renderWithQuery(<Dashboard />);

    await waitFor(() => {
      expect(screen.getByText('Block Rate')).toBeTruthy();
    });

    expect(screen.getByText('0%')).toBeTruthy();
  });

  it('defaults values to 0 when stats are undefined', async () => {
    (client.get as ReturnType<typeof vi.fn>).mockResolvedValue({ data: null });
    renderWithQuery(<Dashboard />);

    await waitFor(() => {
      expect(screen.getByText('Total Runs')).toBeTruthy();
    });

    // All cards default to 0
    const zeros = screen.getAllByText('0');
    expect(zeros.length).toBeGreaterThanOrEqual(4);
  });

  it('calls /stats endpoint', async () => {
    (client.get as ReturnType<typeof vi.fn>).mockResolvedValue({ data: STATS });
    renderWithQuery(<Dashboard />);

    await waitFor(() => {
      expect(client.get).toHaveBeenCalledWith('/stats');
    });
  });
});
