import React from 'react';
import type { ChangeFilterParams } from '../../types';
import { Search, X } from 'lucide-react';

interface FiltersProps {
  filters: ChangeFilterParams;
  onFilterChange: (filters: ChangeFilterParams) => void;
}

export const Filters: React.FC<FiltersProps> = ({ filters, onFilterChange }) => {
  // Local state to manage inputs before applying (debounce could be added, but simple submit for now)
  // Or just controlled inputs that trigger update? 
  // For text inputs better to have local state and apply on Enter or blur or debounce.
  // Let's use local state and an "Apply" button or auto-apply with debounce.
  // For simplicity: controlled inputs, apply on change (maybe with small debounce if needed, but let's do direct first)
  
  const handleChange = (key: keyof ChangeFilterParams, value: string | undefined) => {
    const newFilters = { ...filters, [key]: value };
    if (!value) delete newFilters[key]; // Clean up empty values
    // Reset offset when filtering
    newFilters.offset = 0;
    onFilterChange(newFilters);
  };

  const handleClear = () => {
    onFilterChange({ limit: filters.limit });
  };

  return (
    <div className="bg-white dark:bg-gray-900 p-4 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-semibold text-gray-900 dark:text-gray-100 flex items-center gap-2">
          <Search className="w-4 h-4" />
          Filters
        </h2>
        {(Object.keys(filters).length > 1 || (filters.limit && Object.keys(filters).length > 2)) && ( // considering limit/offset usually present
          <button 
            onClick={handleClear}
            className="text-xs text-gray-500 hover:text-gray-700 dark:hover:text-gray-300 flex items-center gap-1"
          >
            <X className="w-3 h-3" />
            Clear all
          </button>
        )}
      </div>
      
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <div>
          <label className="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">Namespace</label>
          <input
            type="text"
            className="w-full rounded-md border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
            placeholder="default"
            value={filters.namespace || ''}
            onChange={(e) => handleChange('namespace', e.target.value)}
          />
        </div>
        
        <div>
          <label className="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">Resource Kind</label>
          <input
            type="text"
            className="w-full rounded-md border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
            placeholder="Deployment"
            value={filters.resource_kind || ''}
            onChange={(e) => handleChange('resource_kind', e.target.value)}
          />
        </div>

        <div>
           <label className="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">Resource Name</label>
           <input
             type="text"
             className="w-full rounded-md border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
             placeholder="my-app"
             value={filters.name || ''}
             onChange={(e) => handleChange('name', e.target.value)}
           />
        </div>

        <div>
          <label className="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">Operation</label>
          <select
            className="w-full rounded-md border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
            value={filters.operation || ''}
            onChange={(e) => handleChange('operation', e.target.value || undefined)}
          >
            <option value="">All Operations</option>
            <option value="CREATE">CREATE</option>
            <option value="UPDATE">UPDATE</option>
            <option value="DELETE">DELETE</option>
          </select>
        </div>

        <div>
           <label className="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">User</label>
           <input
             type="text"
             className="w-full rounded-md border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
             placeholder="user@example.com"
             value={filters.user || ''}
             onChange={(e) => handleChange('user', e.target.value)}
           />
        </div>
      </div>
    </div>
  );
};
