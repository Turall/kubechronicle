import { apiClient } from './client';

export interface LoginRequest {
  username: string;
  password: string;
}

export interface LoginResponse {
  token: string;
  user: {
    username: string;
    roles: string[];
    email?: string;
  };
}

export const login = async (username: string, password: string): Promise<LoginResponse> => {
  const response = await apiClient.post<LoginResponse>('/api/auth/login', {
    username,
    password,
  });
  return response.data;
};
