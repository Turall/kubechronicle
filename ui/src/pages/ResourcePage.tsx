import React, { useEffect, useState, useCallback } from 'react';
import { useParams, Link } from 'react-router-dom';
import { getResourceHistory } from '../api/changes';
import type { ChangeEvent } from '../types';
import { Timeline, Pagination } from '../components';
import { ArrowLeft, Database, Tag } from 'lucide-react';

export const ResourcePage: React.FC = () => {
  const { kind, namespace, name } = useParams<{ kind: string; namespace: string; name: string }>();
  const [data, setData] = useState<{ events: ChangeEvent[]; total: number }>({ events: [], total: 0 });
  const [loading, setLoading] = useState(true);
  const [offset, setOffset] = useState(0);
  const limit = 20;

  const fetchData = useCallback(async () => {
    if (!kind || !name) return;
    setLoading(true);
    try {
      const result = await getResourceHistory(kind, namespace || '-', name, { limit, offset });
      setData(result);
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  }, [kind, namespace, name, offset]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  return (
    <div className="space-y-6">
      <div>
        <Link to="/" className="inline-flex items-center text-sm text-gray-500 hover:text-gray-900 dark:hover:text-gray-100 mb-4 transition-colors">
          <ArrowLeft className="w-4 h-4 mr-1" />
          Back to Global Timeline
        </Link>
        <h1 className="text-2xl font-bold text-gray-900 dark:text-gray-100 flex items-center gap-2">
           Resource History
        </h1>
        <div className="flex flex-wrap items-center gap-4 mt-2 text-sm text-gray-600 dark:text-gray-400">
           <div className="flex items-center gap-1.5 px-2 py-1 bg-gray-100 dark:bg-gray-800 rounded">
             <Database className="w-4 h-4" />
             <span className="font-medium">{kind}</span>
           </div>
           <div className="flex items-center gap-1.5 px-2 py-1 bg-gray-100 dark:bg-gray-800 rounded">
             <Tag className="w-4 h-4" />
             <span className="font-medium">{namespace}</span>
           </div>
           <div className="font-bold text-lg text-gray-900 dark:text-gray-100">
             {name}
           </div>
        </div>
      </div>

      <Timeline events={data.events} isLoading={loading} />

      {!loading && data.total > 0 && (
        <Pagination
          total={data.total}
          limit={limit}
          offset={offset}
          onPageChange={setOffset}
        />
      )}
    </div>
  );
};
