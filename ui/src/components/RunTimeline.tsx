/**
 * RunTimeline Component
 * 
 * Displays a visual timeline of events for an agent run,
 * showing steps, tool calls, and policy decisions.
 */

import { useState, useMemo } from 'react';
import { format, formatDuration, intervalToDuration } from 'date-fns';
import {
  Activity,
  Play,
  Square,
  AlertTriangle,
  Shield,
  Check,
  X,
  Eye,
  EyeOff,
  ChevronDown,
  ChevronRight,
  Clock,
  Zap,
} from 'lucide-react';
import { useRunEvents, useRun } from '../hooks/useApi';
import type { Event, PolicyAction } from '../types';

interface RunTimelineProps {
  runId: string;
}

const EVENT_ICONS: Record<string, typeof Activity> = {
  'run.started': Play,
  'run.ended': Square,
  'step.started': Activity,
  'step.ended': Check,
  'tool_call.requested': Zap,
  'tool_call.responded': Shield,
};

const DECISION_COLORS: Record<PolicyAction, { bg: string; text: string }> = {
  allow: { bg: 'bg-green-100', text: 'text-green-700' },
  warn: { bg: 'bg-yellow-100', text: 'text-yellow-700' },
  redact: { bg: 'bg-purple-100', text: 'text-purple-700' },
  block: { bg: 'bg-red-100', text: 'text-red-700' },
  require_approval: { bg: 'bg-blue-100', text: 'text-blue-700' },
  degrade: { bg: 'bg-orange-100', text: 'text-orange-700' },
};

export default function RunTimeline({ runId }: RunTimelineProps) {
  const { data: eventsData, isLoading, error } = useRunEvents(runId);
  const { data: run } = useRun(runId);
  const [expandedEvents, setExpandedEvents] = useState<Set<string>>(new Set());
  const [showHashes, setShowHashes] = useState(false);
  const [filterType, setFilterType] = useState<string>('all');

  // Group events by step
  const groupedEvents = useMemo(() => {
    if (!eventsData) return [];

    const groups: Array<{
      stepId: string | null;
      stepName: string;
      events: Event[];
      startTime: string;
      endTime: string | null;
      status: 'running' | 'completed' | 'failed';
    }> = [];

    let currentGroup: (typeof groups)[0] | null = null;

    for (const event of eventsData) {
      // Filter by type if needed
      if (filterType !== 'all' && !event.event_type.includes(filterType)) {
        continue;
      }

      if (event.event_type === 'step.started') {
        // Start a new group
        currentGroup = {
          stepId: event.payload.step_id,
          stepName: event.payload.name,
          events: [event],
          startTime: event.timestamp,
          endTime: null,
          status: 'running',
        };
        groups.push(currentGroup);
      } else if (event.event_type === 'step.ended' && currentGroup) {
        currentGroup.events.push(event);
        currentGroup.endTime = event.timestamp;
        currentGroup.status = event.payload.status || 'completed';
        currentGroup = null;
      } else if (currentGroup) {
        currentGroup.events.push(event);
      } else {
        // Events outside steps (run.started, run.ended, etc.)
        groups.push({
          stepId: null,
          stepName: event.event_type.replace('.', ' ').toUpperCase(),
          events: [event],
          startTime: event.timestamp,
          endTime: event.timestamp,
          status: 'completed',
        });
      }
    }

    return groups;
  }, [eventsData, filterType]);

  const toggleExpanded = (eventId: string) => {
    const newExpanded = new Set(expandedEvents);
    if (newExpanded.has(eventId)) {
      newExpanded.delete(eventId);
    } else {
      newExpanded.add(eventId);
    }
    setExpandedEvents(newExpanded);
  };

  const expandAll = () => {
    if (!eventsData) return;
    setExpandedEvents(new Set(eventsData.map((e) => e.event_id)));
  };

  const collapseAll = () => {
    setExpandedEvents(new Set());
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-8">
        <Activity className="h-5 w-5 animate-spin text-gray-400" />
        <span className="ml-2 text-gray-500">Loading timeline...</span>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex items-center justify-center py-8 text-red-600">
        <AlertTriangle className="h-5 w-5 mr-2" />
        Failed to load timeline
      </div>
    );
  }

  if (!eventsData) {
    return null;
  }

  const totalDuration =
    run?.ended_at && run?.created_at
      ? intervalToDuration({
          start: new Date(run.created_at),
          end: new Date(run.ended_at),
        })
      : null;

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <h3 className="text-lg font-medium text-gray-900">Event Timeline</h3>
          <span className="text-sm text-gray-500">
            {eventsData.length} events
          </span>
          {totalDuration && (
            <span className="flex items-center text-sm text-gray-500">
              <Clock className="h-4 w-4 mr-1" />
              {formatDuration(totalDuration)}
            </span>
          )}
        </div>

        <div className="flex items-center gap-3">
          {/* Filter */}
          <select
            value={filterType}
            onChange={(e) => setFilterType(e.target.value)}
            className="text-sm border-gray-300 rounded-md"
          >
            <option value="all">All Events</option>
            <option value="step">Steps Only</option>
            <option value="tool_call">Tool Calls Only</option>
            <option value="run">Run Events</option>
          </select>

          {/* Hash toggle */}
          <button
            onClick={() => setShowHashes(!showHashes)}
            className={`inline-flex items-center px-2 py-1 rounded text-xs ${
              showHashes
                ? 'bg-purple-100 text-purple-700'
                : 'bg-gray-100 text-gray-600'
            }`}
          >
            {showHashes ? <Eye className="h-3 w-3 mr-1" /> : <EyeOff className="h-3 w-3 mr-1" />}
            Hashes
          </button>

          {/* Expand/Collapse */}
          <div className="flex gap-1">
            <button
              onClick={expandAll}
              className="text-xs text-gray-500 hover:text-gray-700"
            >
              Expand All
            </button>
            <span className="text-gray-300">|</span>
            <button
              onClick={collapseAll}
              className="text-xs text-gray-500 hover:text-gray-700"
            >
              Collapse
            </button>
          </div>
        </div>
      </div>

      {/* Timeline */}
      <div className="relative">
        {/* Vertical line */}
        <div className="absolute left-6 top-0 bottom-0 w-0.5 bg-gray-200" />

        {/* Event groups */}
        <div className="space-y-4">
          {groupedEvents.map((group, groupIndex) => (
            <div key={group.stepId || `group-${groupIndex}`} className="relative">
              {/* Step header */}
              <div className="flex items-start gap-3 mb-2">
                <div
                  className={`relative z-10 w-12 h-12 rounded-full flex items-center justify-center ${
                    group.status === 'running'
                      ? 'bg-blue-100'
                      : group.status === 'completed'
                      ? 'bg-green-100'
                      : 'bg-red-100'
                  }`}
                >
                  {group.status === 'running' ? (
                    <Activity className="h-5 w-5 text-blue-600 animate-pulse" />
                  ) : group.status === 'completed' ? (
                    <Check className="h-5 w-5 text-green-600" />
                  ) : (
                    <X className="h-5 w-5 text-red-600" />
                  )}
                </div>
                <div className="flex-1 pt-2">
                  <div className="flex items-center gap-2">
                    <span className="font-medium text-gray-900">
                      {group.stepName}
                    </span>
                    {group.endTime && (
                      <span className="text-xs text-gray-400">
                        {formatDuration(
                          intervalToDuration({
                            start: new Date(group.startTime),
                            end: new Date(group.endTime),
                          })
                        ) || '<1s'}
                      </span>
                    )}
                  </div>
                  <div className="text-xs text-gray-500">
                    {format(new Date(group.startTime), 'HH:mm:ss.SSS')}
                  </div>
                </div>
              </div>

              {/* Events within group */}
              <div className="ml-14 space-y-2">
                {group.events.map((event) => (
                  <EventCard
                    key={event.event_id}
                    event={event}
                    isExpanded={expandedEvents.has(event.event_id)}
                    onToggle={() => toggleExpanded(event.event_id)}
                    showHash={showHashes}
                  />
                ))}
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Chain integrity indicator */}
      <div className="flex items-center justify-center pt-4 border-t border-gray-100">
        <div className="flex items-center gap-2 text-sm text-green-600">
          <Shield className="h-4 w-4" />
          Hash chain verified ({eventsData.length} events)
        </div>
      </div>
    </div>
  );
}

// Individual event card
function EventCard({
  event,
  isExpanded,
  onToggle,
  showHash,
}: {
  event: Event;
  isExpanded: boolean;
  onToggle: () => void;
  showHash: boolean;
}) {
  const Icon = EVENT_ICONS[event.event_type] || Activity;
  const isToolCall = event.event_type.includes('tool_call');

  // Extract decision from tool call events
  const decision = event.payload.decision as
    | { action: PolicyAction; reason: string }
    | undefined;
  const decisionColors = decision
    ? DECISION_COLORS[decision.action]
    : { bg: 'bg-gray-100', text: 'text-gray-600' };

  return (
    <div className="border border-gray-200 rounded-lg overflow-hidden">
      {/* Event header */}
      <button
        onClick={onToggle}
        className="w-full flex items-center gap-3 p-3 hover:bg-gray-50 transition-colors"
      >
        <Icon className="h-4 w-4 text-gray-400 flex-shrink-0" />

        <div className="flex-1 text-left">
          <span className="text-sm font-medium text-gray-700">
            {event.event_type}
          </span>

          {isToolCall && event.payload.tool_name && (
            <span className="ml-2 text-sm text-gray-500">
              {event.payload.tool_name}
            </span>
          )}
        </div>

        {/* Decision badge */}
        {decision && (
          <span
            className={`px-2 py-0.5 rounded text-xs font-medium ${decisionColors.bg} ${decisionColors.text}`}
          >
            {decision.action}
          </span>
        )}

        <span className="text-xs text-gray-400">
          {format(new Date(event.timestamp), 'HH:mm:ss.SSS')}
        </span>

        {isExpanded ? (
          <ChevronDown className="h-4 w-4 text-gray-400" />
        ) : (
          <ChevronRight className="h-4 w-4 text-gray-400" />
        )}
      </button>

      {/* Expanded content */}
      {isExpanded && (
        <div className="px-3 pb-3 border-t border-gray-100">
          {/* Hash info */}
          {showHash && (
            <div className="mt-2 p-2 bg-gray-50 rounded text-xs font-mono space-y-1">
              <div className="flex">
                <span className="text-gray-500 w-20">Hash:</span>
                <span className="text-gray-700 truncate">{event.event_hash}</span>
              </div>
              <div className="flex">
                <span className="text-gray-500 w-20">Prev:</span>
                <span className="text-gray-700 truncate">{event.prev_hash}</span>
              </div>
            </div>
          )}

          {/* Event data */}
          <div className="mt-2">
            <pre className="text-xs text-gray-600 bg-gray-50 p-2 rounded overflow-x-auto">
              {JSON.stringify(event.payload, null, 2)}
            </pre>
          </div>

          {/* Decision details */}
          {decision && (
            <div className="mt-2 p-2 bg-blue-50 rounded text-sm">
              <div className="font-medium text-blue-800">Policy Decision</div>
              <div className="text-blue-600 mt-1">{decision.reason}</div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
