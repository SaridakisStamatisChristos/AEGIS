import { useState, useCallback } from 'react';
import { Save, Play, AlertCircle, CheckCircle } from 'lucide-react';

interface PolicyEditorProps {
  initialValue?: string;
  onSave?: (content: string) => Promise<void>;
  onValidate?: (content: string) => Promise<ValidationResult>;
  readOnly?: boolean;
}

interface ValidationResult {
  valid: boolean;
  errors: Array<{
    line: number;
    message: string;
  }>;
}

const defaultPolicy = `# Policy Definition
# Version: 1.0

policy:
  name: "my-policy"
  version: "v1"

tools:
  - name: "http_request"
    action: allow
    conditions:
      - "!contains(args.url, '169.254.')"
      - "!contains(args.url, '10.0.')"
      - "!contains(args.url, '192.168.')"

  - name: "file_write"
    action: block
    reason: "File writes not permitted"

budgets:
  max_tool_calls: 100
  max_wall_clock_sec: 300

redaction:
  patterns:
    - "Bearer [a-zA-Z0-9-_]+"
    - "api[_-]?key[=:][a-zA-Z0-9]+"
`;

export default function PolicyEditor({
  initialValue = defaultPolicy,
  onSave,
  onValidate,
  readOnly = false,
}: PolicyEditorProps) {
  const [content, setContent] = useState(initialValue);
  const [validation, setValidation] = useState<ValidationResult | null>(null);
  const [isSaving, setIsSaving] = useState(false);
  const [isValidating, setIsValidating] = useState(false);

  const handleValidate = useCallback(async () => {
    if (!onValidate) return;
    
    setIsValidating(true);
    try {
      const result = await onValidate(content);
      setValidation(result);
    } catch (error) {
      setValidation({
        valid: false,
        errors: [{ line: 0, message: 'Validation failed: ' + String(error) }],
      });
    } finally {
      setIsValidating(false);
    }
  }, [content, onValidate]);

  const handleSave = useCallback(async () => {
    if (!onSave) return;
    
    setIsSaving(true);
    try {
      await onSave(content);
    } finally {
      setIsSaving(false);
    }
  }, [content, onSave]);

  const lineCount = content.split('\n').length;

  return (
    <div className="flex flex-col h-full bg-white rounded-lg shadow">
      <div className="flex items-center justify-between px-4 py-2 border-b border-gray-200">
        <h3 className="text-sm font-medium text-gray-700">Policy Editor</h3>
        <div className="flex items-center space-x-2">
          {validation && (
            <div className={`flex items-center text-sm ${
              validation.valid ? 'text-green-600' : 'text-red-600'
            }`}>
              {validation.valid ? (
                <>
                  <CheckCircle className="h-4 w-4 mr-1" />
                  Valid
                </>
              ) : (
                <>
                  <AlertCircle className="h-4 w-4 mr-1" />
                  {validation.errors.length} error(s)
                </>
              )}
            </div>
          )}
          
          {onValidate && (
            <button
              onClick={handleValidate}
              disabled={isValidating}
              className="inline-flex items-center px-3 py-1.5 text-sm font-medium rounded-md text-gray-700 bg-gray-100 hover:bg-gray-200 disabled:opacity-50"
            >
              <Play className="h-4 w-4 mr-1" />
              {isValidating ? 'Validating...' : 'Validate'}
            </button>
          )}
          
          {onSave && !readOnly && (
            <button
              onClick={handleSave}
              disabled={isSaving}
              className="inline-flex items-center px-3 py-1.5 text-sm font-medium rounded-md text-white bg-blue-600 hover:bg-blue-700 disabled:opacity-50"
            >
              <Save className="h-4 w-4 mr-1" />
              {isSaving ? 'Saving...' : 'Save'}
            </button>
          )}
        </div>
      </div>

      <div className="flex-1 flex overflow-hidden">
        {/* Line numbers */}
        <div className="w-12 bg-gray-50 border-r border-gray-200 text-right py-3 select-none">
          {Array.from({ length: lineCount }, (_, i) => (
            <div
              key={i}
              className={`px-2 text-xs leading-6 ${
                validation?.errors.some(e => e.line === i + 1)
                  ? 'text-red-500 bg-red-50'
                  : 'text-gray-400'
              }`}
            >
              {i + 1}
            </div>
          ))}
        </div>

        {/* Editor */}
        <textarea
          value={content}
          onChange={(e) => setContent(e.target.value)}
          readOnly={readOnly}
          className="flex-1 p-3 font-mono text-sm leading-6 resize-none focus:outline-none"
          spellCheck={false}
        />
      </div>

      {/* Errors panel */}
      {validation && !validation.valid && (
        <div className="border-t border-gray-200 bg-red-50 p-3 max-h-32 overflow-y-auto">
          {validation.errors.map((error, idx) => (
            <div key={idx} className="flex items-start text-sm text-red-700">
              <AlertCircle className="h-4 w-4 mr-2 mt-0.5 flex-shrink-0" />
              <span>
                {error.line > 0 && <span className="font-medium">Line {error.line}: </span>}
                {error.message}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
