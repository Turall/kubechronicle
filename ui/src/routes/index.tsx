import React from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { Layout } from '../components/Layout';
import { TimelinePage, ChangeDetailPage, ResourcePage, ActorPage } from '../pages';

export const AppRoutes: React.FC = () => {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Layout />}>
          <Route index element={<TimelinePage />} />
          <Route path="changes/:id" element={<ChangeDetailPage />} />
          <Route path="resources/:kind/:namespace/:name" element={<ResourcePage />} />
          <Route path="users/:username" element={<ActorPage />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
};
