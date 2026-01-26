import React, { createContext, useContext, useState, useEffect } from 'react';
import type { ReactNode } from 'react';
import { login as apiLogin } from '../api/auth';

export interface User {
  username: string;
  roles: string[];
  email?: string;
}

interface AuthContextType {
  user: User | null;
  token: string | null;
  login: (username: string, password: string) => Promise<void>;
  logout: () => void;
  isAuthenticated: boolean;
  isAdmin: boolean;
  loading: boolean;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export const useAuth = () => {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
};

interface AuthProviderProps {
  children: ReactNode;
}

export const AuthProvider: React.FC<AuthProviderProps> = ({ children }) => {
  const [user, setUser] = useState<User | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  // Load token and user from localStorage on mount
  useEffect(() => {
    const storedToken = localStorage.getItem('auth_token');
    const storedUser = localStorage.getItem('auth_user');
    
    if (storedToken && storedUser) {
      try {
        const parsedUser = JSON.parse(storedUser);
        // Normalize user object to handle both lowercase and capitalized field names
        const normalizedUser: User = {
          username: parsedUser.username || parsedUser.Username || '',
          roles: parsedUser.roles || parsedUser.Roles || [],
          email: parsedUser.email || parsedUser.Email,
        };
        setToken(storedToken);
        setUser(normalizedUser);
      } catch (err) {
        console.error('Failed to parse stored user:', err);
        localStorage.removeItem('auth_token');
        localStorage.removeItem('auth_user');
      }
    }
    setLoading(false);
  }, []);

  const login = async (username: string, password: string) => {
    try {
      const data = await apiLogin(username, password);
      setToken(data.token);
      
      // Normalize user object to handle both lowercase and capitalized field names
      const normalizedUser: User = {
        username: data.user.username || (data.user as any).Username || '',
        roles: data.user.roles || (data.user as any).Roles || [],
        email: data.user.email || (data.user as any).Email,
      };
      
      setUser(normalizedUser);
      
      // Store in localStorage
      localStorage.setItem('auth_token', data.token);
      localStorage.setItem('auth_user', JSON.stringify(normalizedUser));
    } catch (err: any) {
      // If login endpoint doesn't exist (auth disabled), allow access
      if (err.response?.status === 404 || err.message?.includes('404') || err.message?.includes('Not Found')) {
        // Auth is disabled, create a dummy user
        const dummyUser: User = { username: 'anonymous', roles: [] };
        setUser(dummyUser);
        setToken('no-auth');
        return;
      }
      throw err;
    }
  };

  const logout = () => {
    setToken(null);
    setUser(null);
    localStorage.removeItem('auth_token');
    localStorage.removeItem('auth_user');
  };

  const isAuthenticated = !!token && !!user;
  const isAdmin = user?.roles?.includes('admin') || false;

  return (
    <AuthContext.Provider
      value={{
        user,
        token,
        login,
        logout,
        isAuthenticated,
        isAdmin,
        loading,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
};
