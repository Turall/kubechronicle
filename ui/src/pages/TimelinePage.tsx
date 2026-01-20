import React, { useEffect, useState, useCallback } from 'react';
import { useSearchParams } from 'react-router-dom';
import { getChanges } from '../api/changes';
import type { ChangeEvent, ChangeFilterParams, Operation } from '../types';
import { Timeline, Filters, Pagination } from '../components';
import { AlertCircle } from 'lucide-react';

export const TimelinePage: React.FC = () => {
  const [searchParams, setSearchParams] = useSearchParams();
  const [data, setData] = useState<{ events: ChangeEvent[]; total: number }>({ events: [], total: 0 });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Initialize filters from URL
  const initialFilters: ChangeFilterParams = {
    limit: Number(searchParams.get('limit')) || 20,
    offset: Number(searchParams.get('offset')) || 0,
    namespace: searchParams.get('namespace') || undefined,
    resource_kind: searchParams.get('resource_kind') || undefined,
    name: searchParams.get('name') || undefined,
    user: searchParams.get('user') || undefined,
    operation: (searchParams.get('operation') as Operation) || undefined,
  };

  const [filters, setFilters] = useState<ChangeFilterParams>(initialFilters);

  const fetchData = useCallback(async (currentFilters: ChangeFilterParams) => {
    setLoading(true);
    setError(null);
    try {
      const result = await getChanges(currentFilters);
      setData(result);
    } catch (err) {
      console.error('Failed to fetch changes', err);
      setError('Failed to load changes. Please try again later.');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData(filters);
    
    // Sync to URL
    const params: Record<string, string> = {};
    if (filters.limit) params.limit = String(filters.limit);
    if (filters.offset) params.offset = String(filters.offset);
    if (filters.namespace) params.namespace = filters.namespace;
    if (filters.resource_kind) params.resource_kind = filters.resource_kind;
    if (filters.name) params.name = filters.name;
    if (filters.user) params.user = filters.user;
    if (filters.operation) params.operation = filters.operation;
    
    setSearchParams(params, { replace: true });
  }, [filters, fetchData, setSearchParams]);

  const handleFilterChange = (newFilters: ChangeFilterParams) => {
    setFilters(newFilters);
  };

  const handlePageChange = (newOffset: number) => {
    setFilters((prev) => ({ ...prev, offset: newOffset }));
    window.scrollTo({ top: 0, behavior: 'smooth' });
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-2">
        <h1 className="text-2xl font-bold tracking-tight text-gray-900 dark:text-gray-100">
          Global Timeline
        </h1>
        <p className="text-gray-500 dark:text-gray-400">
          Chronological history of all Kubernetes resource changes.
        </p>
      </div>

      <Filters filters={filters} onFilterChange={handleFilterChange} />

      {error && (
        <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 p-4 rounded-md flex items-center gap-3 text-red-700 dark:text-red-300">
          <AlertCircle className="w-5 h-5 flex-shrink-0" />
          <p>{error}</p>
          <button 
            onClick={() => fetchData(filters)}
            className="ml-auto text-sm underline hover:no-underline"
          >
            Retry
          </button>
        </div>
      )}

      <Timeline events={data.events} isLoading={loading} />

      {!loading && data.total > 0 && (
        <Pagination
          total={data.total}
          limit={filters.limit || 20}
          offset={filters.offset || 0}
          onPageChange={handlePageChange}
        />
      )}
    </div>
  );
};
