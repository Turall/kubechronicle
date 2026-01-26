import React, { useState, useEffect } from 'react';
import { getIgnorePatterns, updateIgnorePatterns, getBlockPatterns, updateBlockPatterns, type IgnorePatterns, type BlockPatterns } from '../api/patterns';
import { Save, Plus, X, AlertTriangle } from 'lucide-react';

export const PatternsPage: React.FC = () => {
  const [ignorePatterns, setIgnorePatterns] = useState<IgnorePatterns>({});
  const [blockPatterns, setBlockPatterns] = useState<BlockPatterns>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<'ignore' | 'block'>('ignore');

  useEffect(() => {
    loadPatterns();
  }, []);

  const loadPatterns = async () => {
    try {
      setLoading(true);
      setError(null);
      const [ignore, block] = await Promise.all([
        getIgnorePatterns(),
        getBlockPatterns(),
      ]);
      setIgnorePatterns(ignore);
      setBlockPatterns(block);
    } catch (err: any) {
      console.error('Failed to load patterns', err);
      setError(err.response?.data?.error || 'Failed to load patterns');
    } finally {
      setLoading(false);
    }
  };

  const handleSaveIgnore = async () => {
    try {
      setSaving(true);
      setError(null);
      setSuccess(null);
      const updated = await updateIgnorePatterns(ignorePatterns);
      setIgnorePatterns(updated);
      setSuccess('Ignore patterns updated successfully');
      setTimeout(() => setSuccess(null), 3000);
    } catch (err: any) {
      console.error('Failed to update ignore patterns', err);
      setError(err.response?.data?.error || 'Failed to update ignore patterns');
    } finally {
      setSaving(false);
    }
  };

  const handleSaveBlock = async () => {
    try {
      setSaving(true);
      setError(null);
      setSuccess(null);
      const updated = await updateBlockPatterns(blockPatterns);
      setBlockPatterns(updated);
      setSuccess('Block patterns updated successfully');
      setTimeout(() => setSuccess(null), 3000);
    } catch (err: any) {
      console.error('Failed to update block patterns', err);
      setError(err.response?.data?.error || 'Failed to update block patterns');
    } finally {
      setSaving(false);
    }
  };

  const addPattern = (type: 'namespace' | 'name' | 'resource_kind', isBlock: boolean) => {
    const key = `${type}_patterns`;
    if (isBlock) {
      setBlockPatterns({
        ...blockPatterns,
        [key]: [...(blockPatterns[key as keyof BlockPatterns] as string[] || []), ''],
      });
    } else {
      setIgnorePatterns({
        ...ignorePatterns,
        [key]: [...(ignorePatterns[key as keyof IgnorePatterns] as string[] || []), ''],
      });
    }
  };

  const removePattern = (type: 'namespace' | 'name' | 'resource_kind', index: number, isBlock: boolean) => {
    const key = `${type}_patterns`;
    if (isBlock) {
      const patterns = [...(blockPatterns[key as keyof BlockPatterns] as string[] || [])];
      patterns.splice(index, 1);
      setBlockPatterns({
        ...blockPatterns,
        [key]: patterns,
      });
    } else {
      const patterns = [...(ignorePatterns[key as keyof IgnorePatterns] as string[] || [])];
      patterns.splice(index, 1);
      setIgnorePatterns({
        ...ignorePatterns,
        [key]: patterns,
      });
    }
  };

  const updatePattern = (type: 'namespace' | 'name' | 'resource_kind', index: number, value: string, isBlock: boolean) => {
    const key = `${type}_patterns`;
    if (isBlock) {
      const patterns = [...(blockPatterns[key as keyof BlockPatterns] as string[] || [])];
      patterns[index] = value;
      setBlockPatterns({
        ...blockPatterns,
        [key]: patterns,
      });
    } else {
      const patterns = [...(ignorePatterns[key as keyof IgnorePatterns] as string[] || [])];
      patterns[index] = value;
      setIgnorePatterns({
        ...ignorePatterns,
        [key]: patterns,
      });
    }
  };

  const PatternList = ({ patterns, type, isBlock }: { patterns: string[] | undefined; type: 'namespace' | 'name' | 'resource_kind'; isBlock: boolean }) => (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <label className="text-sm font-medium capitalize">
          {type.replace('_', ' ')} Patterns
        </label>
        <button
          onClick={() => addPattern(type, isBlock)}
          className="text-sm text-blue-600 hover:text-blue-800 flex items-center gap-1"
        >
          <Plus size={16} />
          Add Pattern
        </button>
      </div>
      <div className="space-y-2">
        {(patterns || []).map((pattern, index) => (
          <div key={index} className="flex gap-2">
            <input
              type="text"
              value={pattern}
              onChange={(e) => updatePattern(type, index, e.target.value, isBlock)}
              placeholder={`e.g., ${type === 'namespace' ? 'kube-*' : type === 'name' ? '*-controller' : 'ConfigMap'}`}
              className="flex-1 px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
            <button
              onClick={() => removePattern(type, index, isBlock)}
              className="px-3 py-2 text-red-600 hover:text-red-800"
            >
              <X size={20} />
            </button>
          </div>
        ))}
        {(!patterns || patterns.length === 0) && (
          <p className="text-sm text-gray-500 italic">No patterns configured</p>
        )}
      </div>
    </div>
  );

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-gray-500">Loading patterns...</div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Pattern Management</h1>
      </div>

      {error && (
        <div className="bg-red-50 border border-red-200 text-red-800 px-4 py-3 rounded-md flex items-center gap-2">
          <AlertTriangle size={20} />
          {error}
        </div>
      )}

      {success && (
        <div className="bg-green-50 border border-green-200 text-green-800 px-4 py-3 rounded-md">
          {success}
        </div>
      )}

      <div className="border-b border-gray-200">
        <nav className="flex space-x-8">
          <button
            onClick={() => setActiveTab('ignore')}
            className={`py-4 px-1 border-b-2 font-medium text-sm ${
              activeTab === 'ignore'
                ? 'border-blue-500 text-blue-600'
                : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
            }`}
          >
            Ignore Patterns
          </button>
          <button
            onClick={() => setActiveTab('block')}
            className={`py-4 px-1 border-b-2 font-medium text-sm ${
              activeTab === 'block'
                ? 'border-blue-500 text-blue-600'
                : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
            }`}
          >
            Block Patterns
          </button>
        </nav>
      </div>

      {activeTab === 'ignore' && (
        <div className="bg-white rounded-lg shadow p-6 space-y-6">
          <div>
            <h2 className="text-lg font-semibold mb-4">Ignore Patterns</h2>
            <p className="text-sm text-gray-600 mb-4">
              Resources matching these patterns will be ignored (not tracked) by kubechronicle.
              Patterns support wildcards: <code className="bg-gray-100 px-1 rounded">*</code> matches any sequence.
            </p>
          </div>

          <PatternList
            patterns={ignorePatterns.namespace_patterns}
            type="namespace"
            isBlock={false}
          />
          <PatternList
            patterns={ignorePatterns.name_patterns}
            type="name"
            isBlock={false}
          />
          <PatternList
            patterns={ignorePatterns.resource_kind_patterns}
            type="resource_kind"
            isBlock={false}
          />

          <div className="pt-4 border-t">
            <button
              onClick={handleSaveIgnore}
              disabled={saving}
              className="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 flex items-center gap-2"
            >
              <Save size={16} />
              {saving ? 'Saving...' : 'Save Ignore Patterns'}
            </button>
          </div>
        </div>
      )}

      {activeTab === 'block' && (
        <div className="bg-white rounded-lg shadow p-6 space-y-6">
          <div>
            <h2 className="text-lg font-semibold mb-4">Block Patterns</h2>
            <p className="text-sm text-gray-600 mb-4">
              <strong className="text-red-600">Warning:</strong> Resources matching these patterns will be blocked (denied) by the webhook.
              This changes kubechronicle from observe-only to enforcement mode. Use with caution.
            </p>
          </div>

          <PatternList
            patterns={blockPatterns.namespace_patterns}
            type="namespace"
            isBlock={true}
          />
          <PatternList
            patterns={blockPatterns.name_patterns}
            type="name"
            isBlock={true}
          />
          <PatternList
            patterns={blockPatterns.resource_kind_patterns}
            type="resource_kind"
            isBlock={true}
          />

          <div className="space-y-2">
            <label className="text-sm font-medium">Operation Patterns</label>
            <div className="space-y-2">
              {(blockPatterns.operation_patterns || []).map((pattern, index) => (
                <div key={index} className="flex gap-2">
                  <select
                    value={pattern}
                    onChange={(e) => {
                      const patterns = [...(blockPatterns.operation_patterns || [])];
                      patterns[index] = e.target.value;
                      setBlockPatterns({ ...blockPatterns, operation_patterns: patterns });
                    }}
                    className="flex-1 px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                  >
                    <option value="CREATE">CREATE</option>
                    <option value="UPDATE">UPDATE</option>
                    <option value="DELETE">DELETE</option>
                  </select>
                  <button
                    onClick={() => {
                      const patterns = [...(blockPatterns.operation_patterns || [])];
                      patterns.splice(index, 1);
                      setBlockPatterns({ ...blockPatterns, operation_patterns: patterns });
                    }}
                    className="px-3 py-2 text-red-600 hover:text-red-800"
                  >
                    <X size={20} />
                  </button>
                </div>
              ))}
              <button
                onClick={() => {
                  setBlockPatterns({
                    ...blockPatterns,
                    operation_patterns: [...(blockPatterns.operation_patterns || []), 'CREATE'],
                  });
                }}
                className="text-sm text-blue-600 hover:text-blue-800 flex items-center gap-1"
              >
                <Plus size={16} />
                Add Operation
              </button>
            </div>
          </div>

          <div className="space-y-2">
            <label className="text-sm font-medium">Block Message</label>
            <input
              type="text"
              value={blockPatterns.message || ''}
              onChange={(e) => setBlockPatterns({ ...blockPatterns, message: e.target.value })}
              placeholder="Resource blocked by kubechronicle policy"
              className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>

          <div className="pt-4 border-t">
            <button
              onClick={handleSaveBlock}
              disabled={saving}
              className="px-4 py-2 bg-red-600 text-white rounded-md hover:bg-red-700 disabled:opacity-50 flex items-center gap-2"
            >
              <Save size={16} />
              {saving ? 'Saving...' : 'Save Block Patterns'}
            </button>
          </div>
        </div>
      )}
    </div>
  );
};
