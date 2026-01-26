import React, { useEffect, useState } from 'react';
import { Navigate, useLocation } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';

interface ProtectedRouteProps {
  children: React.ReactElement;
  requireAdmin?: boolean;
}

export const ProtectedRoute: React.FC<ProtectedRouteProps> = ({ children, requireAdmin = false }) => {
  const { isAuthenticated, isAdmin, loading } = useAuth();
  const location = useLocation();
  const [authRequired, setAuthRequired] = useState<boolean | null>(null);

  // Check if authentication is required by trying to access API without token
  useEffect(() => {
    const checkAuthRequired = async () => {
      const apiUrl = import.meta.env.VITE_API_URL || 'http://localhost:8080';
      
      try {
        // Try to access API endpoint without Authorization header
        const response = await fetch(`${apiUrl}/api/changes?limit=1`, {
          method: 'GET',
          headers: {
            'Content-Type': 'application/json',
          },
          // Explicitly don't include Authorization header
        });
        
        console.log('Auth check response status:', response.status);
        
        if (response.status === 401) {
          // 401 means auth is required
          console.log('Authentication is required');
          setAuthRequired(true);
        } else if (response.ok) {
          // 200 OK means auth is not required
          console.log('Authentication is not required');
          setAuthRequired(false);
        } else {
          // Other status - check if login endpoint exists
          console.log('Checking login endpoint...');
          try {
            const loginResponse = await fetch(`${apiUrl}/api/auth/login`, {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ username: '', password: '' }),
            });
            console.log('Login endpoint status:', loginResponse.status);
            // If login endpoint exists and returns 400/401 (not 404), auth is enabled
            setAuthRequired(loginResponse.status !== 404);
          } catch (loginErr) {
            console.warn('Login endpoint check failed:', loginErr);
            // Can't determine, assume auth is required for safety
            setAuthRequired(true);
          }
        }
      } catch (err) {
        // Network error - assume auth might be required
        console.warn('Failed to check auth requirement:', err);
        // If user is not authenticated, require auth to be safe
        if (!isAuthenticated) {
          setAuthRequired(true);
        } else {
          // If user is authenticated, allow access (might be network issue)
          setAuthRequired(false);
        }
      }
    };

    if (!loading) {
      checkAuthRequired();
    }
  }, [loading, isAuthenticated]);

  // Show loading while checking
  if (loading || authRequired === null) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="text-gray-500">Loading...</div>
      </div>
    );
  }

  // If auth is not required, allow access
  if (!authRequired) {
    return children;
  }

  // Auth is required - check if user is authenticated
  if (!isAuthenticated) {
    // Redirect to login page
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  // Check admin requirement
  if (requireAdmin && !isAdmin) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="text-center">
          <h2 className="text-2xl font-bold text-gray-900 dark:text-gray-100 mb-2">Access Denied</h2>
          <p className="text-gray-600 dark:text-gray-400">You need admin privileges to access this page.</p>
        </div>
      </div>
    );
  }

  return children;
};
