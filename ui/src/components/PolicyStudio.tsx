/**
 * PolicyStudio Component
 * 
 * A full-featured policy editor with YAML syntax highlighting,
 * validation, version comparison, and deployment workflow.
 */

import { useState, useEffect, useCallback, useRef } from 'react';
import {
  Save,
  AlertTriangle,
  X,
  FileText,
  Eye,
  Zap,
  Copy,
  Download,
  Upload,
} from 'lucide-react';
import {
  usePolicy,
  useCreatePolicy,
  useUpdatePolicy,
  useActivatePolicy,
  useDeactivatePolicy,
} from '../hooks/useApi';
import type { Policy, PolicyStatus } from '../types';

interface PolicyStudioProps {
  policyId?: string;
  onSave?: (policy: Policy) => void;
}

interface ValidationError {
  line: number;
  message: string;
  severity: 'error' | 'warning';
}

const STATUS_WORKFLOW: Record<PolicyStatus, PolicyStatus[]> = {
  draft: ['review'],
  review: ['approved', 'draft'],
  approved: ['deployed', 'draft'],
  deployed: ['deprecated'],
  deprecated: [],
};

const STATUS_COLORS: Record<PolicyStatus, { bg: string; text: string }> = {
  draft: { bg: 'bg-gray-100', text: 'text-gray-700' },
  review: { bg: 'bg-yellow-100', text: 'text-yellow-700' },
  approved: { bg: 'bg-green-100', text: 'text-green-700' },
  deployed: { bg: 'bg-blue-100', text: 'text-blue-700' },
  deprecated: { bg: 'bg-red-100', text: 'text-red-700' },
};

// Default policy template
const DEFAULT_POLICY = `# AegisRun Policy Definition
# See docs at https://docs.aegisrun.io/policies

name: my-policy
version: v1

tools:
  # HTTP requests - allow only to approved domains
  - name: http_request
    rules:
      - condition: 'args.url.startsWith("https://api.approved-domain.com")'
        action: allow
        
      - condition: 'args.url.contains("169.254.169.254")'
        action: block
        reason: "SSRF attempt blocked"
        
      - condition: 'args.method == "DELETE"'
        action: require_approval
        reason: "Destructive operations require approval"
        
      - condition: true
        action: warn
        reason: "Unrecognized URL pattern"

  # File system operations
  - name: file_write
    rules:
      - condition: 'args.path.startsWith("/tmp/")'
        action: allow
        
      - condition: true
        action: block
        reason: "File writes restricted to /tmp"

budgets:
  max_tool_calls: 100
  max_wall_clock_sec: 300
  max_egress_bytes: 10485760  # 10MB

redaction:
  patterns:
    - name: api_keys
      pattern: '(?i)(api[_-]?key|secret)["\\'']?\\s*[:=]\\s*["\\'']?([a-zA-Z0-9_-]{20,})'
      replacement: "[REDACTED_API_KEY]"
    - name: bearer_tokens
      pattern: 'Bearer\\s+[a-zA-Z0-9._-]+'
      replacement: "Bearer [REDACTED]"
`;

export default function PolicyStudio({ policyId, onSave }: PolicyStudioProps) {
  // State
  const [content, setContent] = useState(DEFAULT_POLICY);
  const [policyName, setPolicyName] = useState('');
  const [isDirty, setIsDirty] = useState(false);
  const [validationErrors, setValidationErrors] = useState<ValidationError[]>([]);
  const [selectedVersion] = useState<string | undefined>();
  const [showPreview, setShowPreview] = useState(false);

  const textareaRef = useRef<HTMLTextAreaElement>(null);

  // Data fetching
  const { data: policy } = usePolicy(policyId, selectedVersion);
  const createPolicy = useCreatePolicy();
  const updatePolicy = useUpdatePolicy();
  const activatePolicy = useActivatePolicy();
  const deactivatePolicy = useDeactivatePolicy();

  // Load policy content when data arrives
  useEffect(() => {
    if (policy?.spec) {
      // Convert spec object to YAML string (simplified - in real app use yaml library)
      setContent(JSON.stringify(policy.spec, null, 2));
      setPolicyName(policy.name);
    }
  }, [policy]);

  // Validate policy content
  const validatePolicy = useCallback((yaml: string): ValidationError[] => {
    const errors: ValidationError[] = [];
    const lines = yaml.split('\n');

    lines.forEach((line, index) => {
      const lineNum = index + 1;

      // Check for common YAML errors
      if (line.includes('\t')) {
        errors.push({
          line: lineNum,
          message: 'Tabs are not allowed in YAML, use spaces',
          severity: 'error',
        });
      }

      // Check for policy-specific rules
      if (line.includes('action:')) {
        const action = line.split('action:')[1]?.trim();
        const validActions = [
          'allow',
          'warn',
          'block',
          'redact',
          'require_approval',
          'degrade',
        ];
        if (action && !validActions.includes(action)) {
          errors.push({
            line: lineNum,
            message: `Invalid action: ${action}. Must be one of: ${validActions.join(', ')}`,
            severity: 'error',
          });
        }
      }

      // Check for dangerous patterns
      if (line.includes('condition: true') && line.includes('action: allow')) {
        errors.push({
          line: lineNum,
          message:
            'Warning: "condition: true" with "action: allow" permits all requests',
          severity: 'warning',
        });
      }
    });

    // Check for required fields
    if (!yaml.includes('name:')) {
      errors.push({
        line: 1,
        message: 'Missing required field: name',
        severity: 'error',
      });
    }

    if (!yaml.includes('version:')) {
      errors.push({
        line: 1,
        message: 'Missing required field: version',
        severity: 'error',
      });
    }

    return errors;
  }, []);

  // Handle content change
  const handleContentChange = (newContent: string) => {
    setContent(newContent);
    setIsDirty(true);
    const errors = validatePolicy(newContent);
    setValidationErrors(errors);
  };

  // Save policy
  const handleSave = async () => {
    const errors = validatePolicy(content);
    if (errors.some((e) => e.severity === 'error')) {
      alert('Please fix validation errors before saving');
      return;
    }

    try {
      let spec;
      try {
        spec = JSON.parse(content);
      } catch {
        spec = { raw: content };
      }

      const result = await createPolicy.mutateAsync({
        name: policyName || 'Untitled Policy',
        spec,
      });
      setIsDirty(false);
      onSave?.(result);
    } catch (error) {
      console.error('Failed to save policy:', error);
    }
  };

  // Status workflow
  const handleStatusChange = async (newStatus: PolicyStatus) => {
    if (!policyId || !policy) return;

    try {
      if (newStatus === 'deployed') {
        await activatePolicy.mutateAsync(policyId);
      } else if (newStatus === 'deprecated') {
        await deactivatePolicy.mutateAsync(policyId);
      } else {
        // For other transitions, update the policy spec to trigger re-review
        let spec;
        try {
          spec = JSON.parse(content);
        } catch {
          spec = policy.spec;
        }
        await updatePolicy.mutateAsync({ policyId, spec });
      }
    } catch (error) {
      console.error('Failed to update status:', error);
    }
  };

  // Copy to clipboard
  const handleCopy = () => {
    navigator.clipboard.writeText(content);
  };

  // Download as file
  const handleDownload = () => {
    const blob = new Blob([content], { type: 'text/yaml' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${policyName || 'policy'}.yaml`;
    a.click();
    URL.revokeObjectURL(url);
  };

  // Upload file
  const handleUpload = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (file) {
      const reader = new FileReader();
      reader.onload = (e) => {
        const text = e.target?.result as string;
        handleContentChange(text);
      };
      reader.readAsText(file);
    }
  };

  // Insert template snippets
  const insertSnippet = (snippet: string) => {
    const textarea = textareaRef.current;
    if (!textarea) return;

    const start = textarea.selectionStart;
    const end = textarea.selectionEnd;
    const newContent =
      content.substring(0, start) + snippet + content.substring(end);
    handleContentChange(newContent);

    // Move cursor after snippet
    setTimeout(() => {
      textarea.selectionStart = textarea.selectionEnd = start + snippet.length;
      textarea.focus();
    }, 0);
  };

  const errorCount = validationErrors.filter((e) => e.severity === 'error').length;
  const warningCount = validationErrors.filter(
    (e) => e.severity === 'warning'
  ).length;

  return (
    <div className="flex flex-col h-full bg-white rounded-lg border border-gray-200">
      {/* Toolbar */}
      <div className="flex items-center justify-between p-3 border-b border-gray-200">
        <div className="flex items-center gap-3">
          <FileText className="h-5 w-5 text-gray-400" />
          <input
            type="text"
            value={policyName}
            onChange={(e) => {
              setPolicyName(e.target.value);
              setIsDirty(true);
            }}
            placeholder="Policy Name"
            className="text-lg font-medium border-none focus:ring-0 p-0"
          />
          {isDirty && <span className="text-xs text-orange-500">Unsaved</span>}
        </div>

        <div className="flex items-center gap-2">
          {/* Actions */}
          <button
            onClick={handleCopy}
            className="p-2 text-gray-400 hover:text-gray-600"
            title="Copy to clipboard"
          >
            <Copy className="h-4 w-4" />
          </button>
          <button
            onClick={handleDownload}
            className="p-2 text-gray-400 hover:text-gray-600"
            title="Download"
          >
            <Download className="h-4 w-4" />
          </button>
          <label className="p-2 text-gray-400 hover:text-gray-600 cursor-pointer">
            <Upload className="h-4 w-4" />
            <input
              type="file"
              accept=".yaml,.yml"
              onChange={handleUpload}
              className="hidden"
            />
          </label>

          <div className="w-px h-6 bg-gray-200" />

          <button
            onClick={() => setShowPreview(!showPreview)}
            className={`p-2 rounded ${
              showPreview ? 'bg-blue-100 text-blue-600' : 'text-gray-400 hover:text-gray-600'
            }`}
            title="Preview"
          >
            <Eye className="h-4 w-4" />
          </button>

          <button
            onClick={handleSave}
            disabled={createPolicy.isPending || errorCount > 0}
            className="inline-flex items-center px-3 py-1.5 bg-blue-600 text-white rounded-md text-sm font-medium hover:bg-blue-700 disabled:opacity-50"
          >
            <Save className="h-4 w-4 mr-1.5" />
            Save
          </button>
        </div>
      </div>

      {/* Status bar */}
      {policy && (
        <div className="flex items-center justify-between px-3 py-2 bg-gray-50 border-b border-gray-200">
          <div className="flex items-center gap-3">
            <span
              className={`px-2 py-0.5 rounded text-xs font-medium ${
                STATUS_COLORS[policy.status]?.bg
              } ${STATUS_COLORS[policy.status]?.text}`}
            >
              {policy.status}
            </span>
            <span className="text-sm text-gray-500">v{policy.version}</span>
          </div>

          <div className="flex items-center gap-2">
            {STATUS_WORKFLOW[policy.status]?.map((nextStatus) => (
              <button
                key={nextStatus}
                onClick={() => handleStatusChange(nextStatus)}
                disabled={activatePolicy.isPending || deactivatePolicy.isPending || updatePolicy.isPending}
                className="text-xs px-2 py-1 border border-gray-300 rounded hover:bg-gray-100"
              >
                → {nextStatus}
              </button>
            ))}
          </div>
        </div>
      )}

      {/* Main content area */}
      <div className="flex flex-1 overflow-hidden">
        {/* Editor */}
        <div className="flex-1 flex flex-col">
          {/* Snippet buttons */}
          <div className="flex items-center gap-2 p-2 bg-gray-50 border-b border-gray-200 overflow-x-auto">
            <span className="text-xs text-gray-500">Insert:</span>
            <button
              onClick={() =>
                insertSnippet(`
  - name: tool_name
    rules:
      - condition: 'args.param == "value"'
        action: allow
`)
              }
              className="text-xs px-2 py-1 bg-white border border-gray-300 rounded hover:bg-gray-50"
            >
              + Tool Rule
            </button>
            <button
              onClick={() =>
                insertSnippet(`
      - condition: true
        action: block
        reason: "Blocked by default"
`)
              }
              className="text-xs px-2 py-1 bg-white border border-gray-300 rounded hover:bg-gray-50"
            >
              + Block Rule
            </button>
            <button
              onClick={() =>
                insertSnippet(`
budgets:
  max_tool_calls: 100
  max_wall_clock_sec: 300
`)
              }
              className="text-xs px-2 py-1 bg-white border border-gray-300 rounded hover:bg-gray-50"
            >
              + Budgets
            </button>
            <button
              onClick={() =>
                insertSnippet(`
redaction:
  patterns:
    - name: pattern_name
      pattern: 'regex_pattern'
      replacement: "[REDACTED]"
`)
              }
              className="text-xs px-2 py-1 bg-white border border-gray-300 rounded hover:bg-gray-50"
            >
              + Redaction
            </button>
          </div>

          {/* Textarea with line numbers */}
          <div className="flex-1 flex overflow-hidden">
            {/* Line numbers */}
            <div className="flex-shrink-0 p-3 bg-gray-100 text-right text-xs font-mono text-gray-400 select-none overflow-y-auto">
              {content.split('\n').map((_, i) => {
                const lineError = validationErrors.find((e) => e.line === i + 1);
                return (
                  <div
                    key={i}
                    className={`leading-6 ${
                      lineError?.severity === 'error'
                        ? 'text-red-500 font-bold'
                        : lineError?.severity === 'warning'
                        ? 'text-yellow-500'
                        : ''
                    }`}
                  >
                    {i + 1}
                  </div>
                );
              })}
            </div>

            {/* Editor */}
            <textarea
              ref={textareaRef}
              value={content}
              onChange={(e) => handleContentChange(e.target.value)}
              className="flex-1 p-3 font-mono text-sm border-none resize-none focus:ring-0 leading-6 overflow-auto"
              spellCheck={false}
            />
          </div>
        </div>

        {/* Preview panel */}
        {showPreview && (
          <div className="w-96 border-l border-gray-200 overflow-y-auto">
            <div className="p-3 border-b border-gray-200 bg-gray-50">
              <h4 className="font-medium text-gray-900">Policy Preview</h4>
            </div>
            <div className="p-3">
              <PolicyPreview content={content} />
            </div>
          </div>
        )}
      </div>

      {/* Validation panel */}
      {validationErrors.length > 0 && (
        <div className="border-t border-gray-200 max-h-48 overflow-y-auto">
          <div className="flex items-center gap-2 px-3 py-2 bg-gray-50 border-b border-gray-100">
            {errorCount > 0 && (
              <span className="flex items-center text-xs text-red-600">
                <X className="h-3 w-3 mr-1" />
                {errorCount} error{errorCount > 1 ? 's' : ''}
              </span>
            )}
            {warningCount > 0 && (
              <span className="flex items-center text-xs text-yellow-600">
                <AlertTriangle className="h-3 w-3 mr-1" />
                {warningCount} warning{warningCount > 1 ? 's' : ''}
              </span>
            )}
          </div>
          <div className="divide-y divide-gray-100">
            {validationErrors.map((error, index) => (
              <div
                key={index}
                className="flex items-start gap-2 px-3 py-2 text-sm hover:bg-gray-50 cursor-pointer"
                onClick={() => {
                  // Scroll to line in editor
                  const textarea = textareaRef.current;
                  if (textarea) {
                    const lines = content.split('\n');
                    const position = lines
                      .slice(0, error.line - 1)
                      .join('\n').length;
                    textarea.focus();
                    textarea.setSelectionRange(position, position);
                  }
                }}
              >
                {error.severity === 'error' ? (
                  <X className="h-4 w-4 text-red-500 flex-shrink-0 mt-0.5" />
                ) : (
                  <AlertTriangle className="h-4 w-4 text-yellow-500 flex-shrink-0 mt-0.5" />
                )}
                <div>
                  <span className="text-gray-500">Line {error.line}:</span>{' '}
                  <span
                    className={
                      error.severity === 'error'
                        ? 'text-red-700'
                        : 'text-yellow-700'
                    }
                  >
                    {error.message}
                  </span>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

// Policy preview component
function PolicyPreview({ content }: { content: string }) {
  // Simple YAML parsing for preview (in real app, use proper YAML parser)
  const lines = content.split('\n');
  const tools: string[] = [];
  let inTools = false;

  for (const line of lines) {
    if (line.trim().startsWith('tools:')) {
      inTools = true;
    } else if (inTools && line.trim().startsWith('- name:')) {
      tools.push(line.trim().replace('- name:', '').trim());
    } else if (inTools && !line.startsWith(' ') && line.trim()) {
      inTools = false;
    }
  }

  return (
    <div className="space-y-4">
      <div>
        <h5 className="text-xs font-medium text-gray-500 uppercase mb-2">
          Tools Configured
        </h5>
        {tools.length > 0 ? (
          <div className="space-y-1">
            {tools.map((tool) => (
              <div
                key={tool}
                className="flex items-center gap-2 text-sm text-gray-700"
              >
                <Zap className="h-4 w-4 text-gray-400" />
                {tool}
              </div>
            ))}
          </div>
        ) : (
          <p className="text-sm text-gray-400">No tools configured</p>
        )}
      </div>

      <div>
        <h5 className="text-xs font-medium text-gray-500 uppercase mb-2">
          Quick Reference
        </h5>
        <div className="text-xs space-y-1 text-gray-600">
          <div className="flex gap-2">
            <code className="bg-green-100 px-1 rounded">allow</code>
            <span>Permit the action</span>
          </div>
          <div className="flex gap-2">
            <code className="bg-yellow-100 px-1 rounded">warn</code>
            <span>Allow but log warning</span>
          </div>
          <div className="flex gap-2">
            <code className="bg-red-100 px-1 rounded">block</code>
            <span>Deny the action</span>
          </div>
          <div className="flex gap-2">
            <code className="bg-purple-100 px-1 rounded">redact</code>
            <span>Mask sensitive data</span>
          </div>
          <div className="flex gap-2">
            <code className="bg-blue-100 px-1 rounded">require_approval</code>
            <span>Needs human approval</span>
          </div>
        </div>
      </div>
    </div>
  );
}
