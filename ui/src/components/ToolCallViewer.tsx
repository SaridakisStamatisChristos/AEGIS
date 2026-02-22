import React, { useState } from 'react';
import { formatDistanceToNow } from 'date-fns';
import { 
  CheckCircle, 
  XCircle, 
  AlertTriangle, 
  Shield, 
  ChevronDown, 
  ChevronUp,
  EyeOff
} from 'lucide-react';
import { ToolCall, PolicyAction } from '../types';

interface ToolCallViewerProps {
  toolCalls: ToolCall[];
}

const actionConfig: Record<PolicyAction, { icon: React.ComponentType<any>; color: string; bgColor: string }> = {
  allow: { icon: CheckCircle, color: 'text-green-600', bgColor: 'bg-green-50' },
  warn: { icon: AlertTriangle, color: 'text-yellow-600', bgColor: 'bg-yellow-50' },
  redact: { icon: EyeOff, color: 'text-purple-600', bgColor: 'bg-purple-50' },
  block: { icon: XCircle, color: 'text-red-600', bgColor: 'bg-red-50' },
  require_approval: { icon: Shield, color: 'text-blue-600', bgColor: 'bg-blue-50' },
  degrade: { icon: AlertTriangle, color: 'text-orange-600', bgColor: 'bg-orange-50' },
};

export default function ToolCallViewer({ toolCalls }: ToolCallViewerProps) {
  const [expandedIds, setExpandedIds] = useState<Set<string>>(new Set());

  const toggleExpand = (id: string) => {
    const newExpanded = new Set(expandedIds);
    if (newExpanded.has(id)) {
      newExpanded.delete(id);
    } else {
      newExpanded.add(id);
    }
    setExpandedIds(newExpanded);
  };

  if (toolCalls.length === 0) {
    return (
      <div className="text-center py-8 text-gray-500">
        No tool calls in this step
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {toolCalls.map((call) => {
        const isExpanded = expandedIds.has(call.tool_call_id);
        const config = actionConfig[call.decision.action] || actionConfig.allow;
        const ActionIcon = config.icon;

        return (
          <div
            key={call.tool_call_id}
            className={`rounded-lg border ${config.bgColor} border-opacity-50`}
          >
            <button
              className="w-full px-4 py-3 flex items-center justify-between text-left"
              onClick={() => toggleExpand(call.tool_call_id)}
            >
              <div className="flex items-center space-x-3">
                <ActionIcon className={`h-5 w-5 ${config.color}`} />
                <div>
                  <span className="font-medium text-gray-900">{call.tool_name}</span>
                  <span className={`ml-2 text-sm ${config.color}`}>
                    {call.decision.action}
                  </span>
                </div>
              </div>
              <div className="flex items-center space-x-2">
                <span className="text-xs text-gray-500">
                  {formatDistanceToNow(new Date(call.requested_at), { addSuffix: true })}
                </span>
                {isExpanded ? (
                  <ChevronUp className="h-4 w-4 text-gray-400" />
                ) : (
                  <ChevronDown className="h-4 w-4 text-gray-400" />
                )}
              </div>
            </button>

            {isExpanded && (
              <div className="px-4 pb-4 space-y-3">
                <div>
                  <h4 className="text-xs font-medium text-gray-500 uppercase tracking-wide mb-1">
                    Decision
                  </h4>
                  <p className="text-sm text-gray-700">{call.decision.reason}</p>
                  <p className="text-xs text-gray-500 mt-1">
                    Rule: {call.decision.policy_rule_id}
                  </p>
                </div>

                {call.args_redacted && (
                  <div className="flex items-center text-sm text-purple-600">
                    <EyeOff className="h-4 w-4 mr-1" />
                    Arguments redacted for security
                  </div>
                )}

                <div className="text-xs text-gray-500">
                  <span>ID: {call.tool_call_id.slice(0, 16)}...</span>
                  {call.responded_at && (
                    <span className="ml-3">
                      Duration: {Math.round(
                        (new Date(call.responded_at).getTime() - 
                         new Date(call.requested_at).getTime())
                      )}ms
                    </span>
                  )}
                </div>
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}
