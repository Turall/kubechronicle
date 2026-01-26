import React from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { AuthProvider } from '../contexts/AuthContext';
import { Layout } from '../components/Layout';
import { ProtectedRoute } from '../components/ProtectedRoute';
import { TimelinePage, ChangeDetailPage, ResourcePage, ActorPage, PatternsPage, LoginPage } from '../pages';

export const AppRoutes: React.FC = () => {
  return (
    <AuthProvider>
      <BrowserRouter>
        <Routes>
          {/* Login page - always accessible */}
          <Route path="/login" element={<LoginPage />} />
          
          {/* Protected routes */}
          <Route
            path="/"
            element={
              <ProtectedRoute>
                <Layout />
              </ProtectedRoute>
            }
          >
            <Route index element={<TimelinePage />} />
            <Route path="changes/:id" element={<ChangeDetailPage />} />
            <Route path="resources/:kind/:namespace/:name" element={<ResourcePage />} />
            <Route path="users/:username" element={<ActorPage />} />
            <Route
              path="patterns"
              element={
                <ProtectedRoute requireAdmin>
                  <PatternsPage />
                </ProtectedRoute>
              }
            />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  );
};
