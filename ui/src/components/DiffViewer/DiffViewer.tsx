import React from 'react';
import type { JsonPatch } from '../../types';
import { clsx } from 'clsx';
import { Check, Trash2, Edit2, Plus, Move, Copy } from 'lucide-react';

interface DiffViewerProps {
  diff: JsonPatch[];
}

const getOpIcon = (op: string) => {
  switch (op) {
    case 'add': return <Plus className="w-4 h-4 text-green-500" />;
    case 'remove': return <Trash2 className="w-4 h-4 text-red-500" />;
    case 'replace': return <Edit2 className="w-4 h-4 text-blue-500" />;
    case 'move': return <Move className="w-4 h-4 text-yellow-500" />;
    case 'copy': return <Copy className="w-4 h-4 text-purple-500" />;
    case 'test': return <Check className="w-4 h-4 text-gray-500" />;
    default: return null;
  }
};

// getOpLabel removed

const ValueDisplay: React.FC<{ value: unknown }> = ({ value }) => {
  if (value === undefined) return <span className="text-gray-400 italic">none</span>;
  if (value === null) return <span className="text-gray-500 font-mono">null</span>;
  
  if (typeof value === 'object') {
    return (
      <pre className="text-xs bg-gray-100 dark:bg-gray-800 p-2 rounded overflow-auto max-h-40">
        {JSON.stringify(value, null, 2)}
      </pre>
    );
  }
  
  return <span className="font-mono text-sm break-all">{String(value)}</span>;
};

export const DiffViewer: React.FC<DiffViewerProps> = ({ diff }) => {
  if (!diff || diff.length === 0) {
    return <div className="text-gray-500 italic">No changes detected in diff.</div>;
  }

  return (
    <div className="border rounded-lg overflow-hidden border-gray-200 dark:border-gray-700">
      <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
        <thead className="bg-gray-50 dark:bg-gray-800">
          <tr>
            <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider w-24">
              Op
            </th>
            <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
              Path
            </th>
            <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
              Value
            </th>
          </tr>
        </thead>
        <tbody className="bg-white dark:bg-gray-900 divide-y divide-gray-200 dark:divide-gray-700">
          {diff.map((patch, idx) => (
            <tr key={idx} className="hover:bg-gray-50 dark:hover:bg-gray-800/50 transition-colors">
              <td className="px-6 py-4 whitespace-nowrap text-sm">
                <div className="flex items-center gap-2">
                  {getOpIcon(patch.op)}
                  <span className={clsx("font-medium capitalize", {
                    'text-green-600': patch.op === 'add',
                    'text-red-600': patch.op === 'remove',
                    'text-blue-600': patch.op === 'replace',
                  })}>
                    {patch.op}
                  </span>
                </div>
              </td>
              <td className="px-6 py-4 text-sm font-mono text-gray-600 dark:text-gray-300">
                {patch.path}
                {patch.from && (
                  <div className="text-xs text-gray-400 mt-1">
                    from: {patch.from}
                  </div>
                )}
              </td>
              <td className="px-6 py-4 text-sm text-gray-900 dark:text-gray-100">
                <ValueDisplay value={patch.value} />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
};
