import { apiClient } from './client';

export interface IgnorePatterns {
  namespace_patterns?: string[];
  name_patterns?: string[];
  resource_kind_patterns?: string[];
}

export interface BlockPatterns {
  namespace_patterns?: string[];
  name_patterns?: string[];
  resource_kind_patterns?: string[];
  operation_patterns?: string[];
  message?: string;
}

export const getIgnorePatterns = async (): Promise<IgnorePatterns> => {
  const response = await apiClient.get<IgnorePatterns>('/api/admin/patterns/ignore');
  return response.data;
};

export const updateIgnorePatterns = async (patterns: IgnorePatterns): Promise<IgnorePatterns> => {
  const response = await apiClient.put<IgnorePatterns>('/api/admin/patterns/ignore', patterns);
  return response.data;
};

export const getBlockPatterns = async (): Promise<BlockPatterns> => {
  const response = await apiClient.get<BlockPatterns>('/api/admin/patterns/block');
  return response.data;
};

export const updateBlockPatterns = async (patterns: BlockPatterns): Promise<BlockPatterns> => {
  const response = await apiClient.put<BlockPatterns>('/api/admin/patterns/block', patterns);
  return response.data;
};
