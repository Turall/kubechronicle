import { apiClient } from './client';
import type { ChangeEvent, ChangeFilterParams, PinatedResponse } from '../types';

export const getChanges = async (params?: ChangeFilterParams) => {
  const response = await apiClient.get<PinatedResponse<ChangeEvent>>('/api/changes', {
    params,
  });
  return response.data;
};

export const getChange = async (id: string) => {
  const response = await apiClient.get<ChangeEvent>(`/api/changes/${id}`);
  return response.data;
};

export const getResourceHistory = async (
  kind: string,
  namespace: string,
  name: string,
  params?: Pick<ChangeFilterParams, 'limit' | 'offset' | 'sort'>
) => {
  // Namespace can be '-' for cluster-scoped, but let's handle that in the caller or here.
  // If namespace is empty, maybe default to default or error?
  // Use dash if empty string provided?
  const ns = namespace || '-';
  const url = `/api/resources/${kind}/${ns}/${name}/history`;
  const response = await apiClient.get<PinatedResponse<ChangeEvent>>(url, {
    params,
  });
  return response.data;
};

export const getUserActivity = async (
  username: string,
  params?: Pick<ChangeFilterParams, 'limit' | 'offset' | 'sort'>
) => {
  const encodedUsername = encodeURIComponent(username);
  const response = await apiClient.get<PinatedResponse<ChangeEvent>>(`/api/users/${encodedUsername}/activity`, {
    params,
  });
  return response.data;
};
