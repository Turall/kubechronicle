import React, { useEffect, useState, useCallback } from 'react';
import { useParams, Link } from 'react-router-dom';
import { getUserActivity } from '../api/changes';
import type { ChangeEvent } from '../types';
import { Timeline, Pagination } from '../components';
import { ArrowLeft, User } from 'lucide-react';

export const ActorPage: React.FC = () => {
  const { username } = useParams<{ username: string }>();
  // Decode double encoded username if needed, but react router usually decodes once.
  // API client re-encodes it.
  
  const [data, setData] = useState<{ events: ChangeEvent[]; total: number }>({ events: [], total: 0 });
  const [loading, setLoading] = useState(true);
  const [offset, setOffset] = useState(0);
  const limit = 20;

  const fetchData = useCallback(async () => {
    if (!username) return;
    setLoading(true);
    try {
      const result = await getUserActivity(username, { limit, offset });
      setData(result);
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  }, [username, offset]);

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
           Actor Activity
        </h1>
        <div className="flex items-center gap-2 mt-2">
           <div className="p-2 bg-indigo-100 dark:bg-indigo-900/30 rounded-full">
             <User className="w-6 h-6 text-indigo-700 dark:text-indigo-400" />
           </div>
           <span className="font-mono text-lg font-medium">{username}</span>
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
