import React, { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { getChange } from '../api/changes';
import type { ChangeEvent } from '../types';
import { DiffViewer } from '../components';
import { formatDateTime, formatTimeAgo } from '../utils/date';
import { ArrowLeft, User, Terminal, Clock, ShieldCheck, Database, Tag } from 'lucide-react';
import { clsx } from 'clsx';

export const ChangeDetailPage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const [event, setEvent] = useState<ChangeEvent | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    const loadData = async () => {
      setLoading(true);
      try {
        const data = await getChange(id);
        setEvent(data);
      } catch {
        setError('Failed to load change details.');
      } finally {
        setLoading(false);
      }
    };
    loadData();
  }, [id]);

  if (loading) return <div className="p-8 text-center text-gray-500">Loading details...</div>;
  if (error || !event) return <div className="p-8 text-center text-red-500">{error || 'Change not found'}</div>;

  const { operation, resource_kind, namespace, name, actor, source, timestamp, diff, allowed, block_pattern } = event;

  const opColor = {
    CREATE: 'text-green-700 bg-green-50 ring-green-600/20 dark:bg-green-900/10 dark:text-green-400 dark:ring-green-400/20',
    UPDATE: 'text-blue-700 bg-blue-50 ring-blue-600/20 dark:bg-blue-900/10 dark:text-blue-400 dark:ring-blue-400/20',
    DELETE: 'text-red-700 bg-red-50 ring-red-600/20 dark:bg-red-900/10 dark:text-red-400 dark:ring-red-400/20',
  }[operation] || 'text-gray-600 bg-gray-50 ring-gray-500/10';

  return (
    <div className="space-y-6">
      <div>
        <Link to="/" className="inline-flex items-center text-sm text-gray-500 hover:text-gray-900 dark:hover:text-gray-100 mb-4 transition-colors">
          <ArrowLeft className="w-4 h-4 mr-1" />
          Back to Timeline
        </Link>
        
        <div className="flex flex-col md:flex-row md:items-start md:justify-between gap-4">
          <div>
            <div className="flex items-center gap-3 mb-2">
              <span className={clsx("inline-flex items-center rounded-md px-2 py-1 text-sm font-medium ring-1 ring-inset", opColor)}>
                {operation}
              </span>
              <h1 className="text-2xl font-bold text-gray-900 dark:text-gray-100 break-all">
                {name}
              </h1>
            </div>
            <div className="flex flex-wrap items-center gap-x-6 gap-y-2 text-sm text-gray-600 dark:text-gray-400">
               <div className="flex items-center gap-1.5">
                 <Database className="w-4 h-4" />
                 <span>{resource_kind}</span>
               </div>
               <div className="flex items-center gap-1.5">
                 <Tag className="w-4 h-4" />
                 <span>{namespace}</span>
               </div>
            </div>
          </div>

          <div className="flex flex-col items-end gap-1 text-sm text-gray-500 dark:text-gray-400">
            <div className="flex items-center gap-1.5" title={formatDateTime(timestamp)}>
              <Clock className="w-4 h-4" />
              <span>{formatTimeAgo(timestamp)}</span>
            </div>
            <div className="mono text-xs opacity-70">{id}</div>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className="lg:col-span-2 space-y-6">
          <div className="bg-white dark:bg-gray-900 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 overflow-hidden">
            <div className="px-4 py-3 border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/50 flex justify-between items-center">
              <h3 className="text-sm font-medium text-gray-900 dark:text-gray-100">Changes</h3>
            </div>
            <div className="p-4">
              <DiffViewer diff={diff} />
            </div>
          </div>
        </div>

        <div className="space-y-6">
          <div className="bg-white dark:bg-gray-900 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-4">
             <h3 className="text-sm font-medium text-gray-900 dark:text-gray-100 mb-4 flex items-center gap-2">
               <User className="w-4 h-4" /> Actor Details
             </h3>
             <dl className="space-y-3 text-sm">
               <div>
                 <dt className="text-gray-500 dark:text-gray-400 text-xs uppercase tracking-wider">Username</dt>
                 <dd className="font-medium mt-0.5">{actor.username}</dd>
               </div>
               {actor.groups && actor.groups.length > 0 && (
                 <div>
                   <dt className="text-gray-500 dark:text-gray-400 text-xs uppercase tracking-wider">Groups</dt>
                   <dd className="mt-0.5 flex flex-wrap gap-1">
                     {actor.groups.map(g => (
                       <span key={g} className="inline-flex items-center rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600 dark:bg-gray-800 dark:text-gray-300">
                         {g}
                       </span>
                     ))}
                   </dd>
                 </div>
               )}
               {actor.service_account && (
                 <div>
                   <dt className="text-gray-500 dark:text-gray-400 text-xs uppercase tracking-wider">Service Account</dt>
                   <dd className="font-medium mt-0.5">{actor.service_account}</dd>
                 </div>
               )}
               {actor.source_ip && (
                 <div>
                   <dt className="text-gray-500 dark:text-gray-400 text-xs uppercase tracking-wider">Source IP</dt>
                   <dd className="font-medium mt-0.5 font-mono">{actor.source_ip}</dd>
                 </div>
               )}
             </dl>
          </div>

          <div className="bg-white dark:bg-gray-900 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-4">
             <h3 className="text-sm font-medium text-gray-900 dark:text-gray-100 mb-4 flex items-center gap-2">
               <Terminal className="w-4 h-4" /> Context
             </h3>
             <dl className="space-y-3 text-sm">
               <div>
                 <dt className="text-gray-500 dark:text-gray-400 text-xs uppercase tracking-wider">Tool</dt>
                 <dd className="font-medium mt-0.5">{source?.tool || 'Unknown'}</dd>
               </div>
               <div>
                 <dt className="text-gray-500 dark:text-gray-400 text-xs uppercase tracking-wider">Status</dt>
                 <dd className="mt-0.5">
                   {allowed ? (
                     <span className="inline-flex items-center gap-1 text-green-700 dark:text-green-400">
                       <ShieldCheck className="w-3.5 h-3.5" /> Allowed
                     </span>
                   ) : (
                     <span className="inline-flex items-center gap-1 text-red-700 dark:text-red-400">
                       <ShieldCheck className="w-3.5 h-3.5" /> Blocked {block_pattern && `(${block_pattern})`}
                     </span>
                   )}
                 </dd>
               </div>
             </dl>
          </div>
        </div>
      </div>
    </div>
  );
};
