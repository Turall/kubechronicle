import axios from 'axios';

const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080';
const API_PREFIX = '/kubechronicle';

export const apiClient = axios.create({
  baseURL: API_URL + API_PREFIX,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Create a client without auth interceptor for auth checks
export const apiClientNoAuth = axios.create({
  baseURL: API_URL + API_PREFIX,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Add token to requests if available
apiClient.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('auth_token');
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => {
    return Promise.reject(error);
  }
);

apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    // Handle 401 Unauthorized - token expired or invalid
    if (error.response?.status === 401) {
      // Clear auth data
      localStorage.removeItem('auth_token');
      localStorage.removeItem('auth_user');
      // Redirect to login if not already there
      if (window.location.pathname !== '/login') {
        window.location.href = '/login';
      }
    }
    // Standardized error handling can go here
    console.error('API Error:', error.response?.data || error.message);
    return Promise.reject(error);
  }
);
