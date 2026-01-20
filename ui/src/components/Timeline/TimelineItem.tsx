import React from 'react';
import type { ChangeEvent } from '../../types';
import { formatTimeAgo, formatDateTime } from '../../utils/date';
import { Link } from 'react-router-dom';
import { clsx } from 'clsx';
import { User, Terminal, Clock } from 'lucide-react';

interface TimelineItemProps {
  event: ChangeEvent;
}

export const TimelineItem: React.FC<TimelineItemProps> = ({ event }) => {
  const { operation, resource_kind, namespace, name, actor, source, timestamp, id } = event;

  const opColor = {
    CREATE: 'bg-green-100 text-green-800 border-green-200 dark:bg-green-900/30 dark:text-green-300 dark:border-green-800',
    UPDATE: 'bg-blue-100 text-blue-800 border-blue-200 dark:bg-blue-900/30 dark:text-blue-300 dark:border-blue-800',
    DELETE: 'bg-red-100 text-red-800 border-red-200 dark:bg-red-900/30 dark:text-red-300 dark:border-red-800',
  }[operation] || 'bg-gray-100 text-gray-800';

  return (
    <div className="group relative flex gap-x-4 hover:bg-gray-50 dark:hover:bg-gray-800/50 p-4 rounded-lg transition-colors">
      <div className="relative flex-none w-16 text-right">
        <div className="text-xs font-semibold text-gray-500 dark:text-gray-400" title={formatDateTime(timestamp)}>
          {formatTimeAgo(timestamp)}
        </div>
      </div>
      
      {/* Vertical line connector (optional, maybe purely visual separation is enough) */}
      <div className="absolute left-[4.5rem] top-0 bottom-0 w-px bg-gray-200 dark:bg-gray-700 hidden sm:block"></div>

      <div className="flex-auto rounded-md p-0 ring-1 ring-inset ring-gray-200 dark:ring-gray-700 bg-white dark:bg-gray-900 shadow-sm">
         <div className="p-4">
            <div className="flex items-center justify-between gap-x-4">
              <div className="min-w-0 flex items-center gap-2">
                <span className={clsx("inline-flex items-center rounded-md px-2 py-1 text-xs font-medium ring-1 ring-inset", opColor)}>
                  {operation}
                </span>
                <h3 className="min-w-0 text-sm font-semibold leading-6 text-gray-900 dark:text-gray-100">
                  <Link to={`/changes/${id}`} className="hover:underline">
                    {resource_kind}
                    <span className="text-gray-400 mx-1">/</span>
                    {namespace !== '-' && namespace ? `${namespace}/` : ''}
                    {name}
                  </Link>
                </h3>
              </div>
              <div className="flex-none text-xs text-gray-500 dark:text-gray-400">
                <Link to={`/changes/${id}`} className="hover:text-gray-700 dark:hover:text-gray-300">
                  View Diff &rarr;
                </Link>
              </div>
            </div>
            
            <div className="mt-2 text-sm text-gray-500 dark:text-gray-400 flex flex-wrap gap-4 items-center">
              <div className="flex items-center gap-1">
                <User className="w-3.5 h-3.5" />
                <span>{actor.username}</span>
              </div>
              {source?.tool && (
                 <div className="flex items-center gap-1">
                   <Terminal className="w-3.5 h-3.5" />
                   <span>{source.tool}</span>
                 </div>
              )}
               <div className="flex items-center gap-1 ml-auto text-xs text-gray-400">
                   <Clock className="w-3.5 h-3.5" />
                   <span>{formatDateTime(timestamp)}</span>
               </div>
            </div>
         </div>
      </div>
    </div>
  );
};
