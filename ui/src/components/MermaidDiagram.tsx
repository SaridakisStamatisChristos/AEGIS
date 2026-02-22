import { useEffect, useRef, useState } from 'react';
import { Maximize2, Minimize2, Download, Copy, Check } from 'lucide-react';

interface MermaidDiagramProps {
  code: string;
  title?: string;
}

export default function MermaidDiagram({ code, title }: MermaidDiagramProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [copied, setCopied] = useState(false);
  const [svgContent, setSvgContent] = useState<string>('');
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const renderDiagram = async () => {
      try {
        // Dynamic import of mermaid
        const mermaid = (await import('mermaid')).default;
        
        mermaid.initialize({
          startOnLoad: false,
          theme: 'default',
          securityLevel: 'strict',
        });

        const id = `mermaid-${Date.now()}`;
        const { svg } = await mermaid.render(id, code);

        // C-04: Sanitize SVG — strip <script> tags as defense-in-depth even with strict mode.
        const parser = new DOMParser();
        const doc = parser.parseFromString(svg, 'image/svg+xml');
        doc.querySelectorAll('script, foreignObject').forEach((el) => el.remove());
        const sanitized = new XMLSerializer().serializeToString(doc.documentElement);

        setSvgContent(sanitized);
        setError(null);
      } catch (err) {
        setError(`Failed to render diagram: ${String(err)}`);
        setSvgContent('');
      }
    };

    renderDiagram();
  }, [code]);

  const handleCopy = async () => {
    await navigator.clipboard.writeText(code);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const handleDownload = () => {
    if (!svgContent) return;

    const blob = new Blob([svgContent], { type: 'image/svg+xml' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${title || 'diagram'}.svg`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  };

  return (
    <div className={`bg-white rounded-lg shadow ${
      isFullscreen ? 'fixed inset-4 z-50' : ''
    }`}>
      <div className="flex items-center justify-between px-4 py-2 border-b border-gray-200">
        <h3 className="text-sm font-medium text-gray-700">
          {title || 'Diagram'}
        </h3>
        <div className="flex items-center space-x-2">
          <button
            onClick={handleCopy}
            className="p-1.5 text-gray-500 hover:text-gray-700 rounded"
            title="Copy code"
          >
            {copied ? (
              <Check className="h-4 w-4 text-green-500" />
            ) : (
              <Copy className="h-4 w-4" />
            )}
          </button>
          <button
            onClick={handleDownload}
            disabled={!svgContent}
            className="p-1.5 text-gray-500 hover:text-gray-700 rounded disabled:opacity-50"
            title="Download SVG"
          >
            <Download className="h-4 w-4" />
          </button>
          <button
            onClick={() => setIsFullscreen(!isFullscreen)}
            className="p-1.5 text-gray-500 hover:text-gray-700 rounded"
            title={isFullscreen ? 'Exit fullscreen' : 'Fullscreen'}
          >
            {isFullscreen ? (
              <Minimize2 className="h-4 w-4" />
            ) : (
              <Maximize2 className="h-4 w-4" />
            )}
          </button>
        </div>
      </div>

      <div 
        ref={containerRef}
        className={`p-4 overflow-auto ${isFullscreen ? 'h-[calc(100%-48px)]' : 'max-h-96'}`}
      >
        {error ? (
          <div className="text-red-500 text-sm p-4 bg-red-50 rounded">
            {error}
            <pre className="mt-2 text-xs text-gray-600 overflow-x-auto">
              {code}
            </pre>
          </div>
        ) : svgContent ? (
          <div 
            dangerouslySetInnerHTML={{ __html: svgContent }}
            className="flex justify-center"
          />
        ) : (
          <div className="flex items-center justify-center h-32 text-gray-400">
            Loading diagram...
          </div>
        )}
      </div>

      {isFullscreen && (
        <div 
          className="fixed inset-0 bg-black bg-opacity-50 -z-10"
          onClick={() => setIsFullscreen(false)}
        />
      )}
    </div>
  );
}
