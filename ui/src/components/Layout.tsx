import React from 'react';
import { Link, Outlet } from 'react-router-dom';
import { History } from 'lucide-react';

export const Layout: React.FC = () => {
  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-950 text-gray-900 dark:text-gray-100 font-sans">
      <header className="bg-white dark:bg-gray-900 border-b border-gray-200 dark:border-gray-800 sticky top-0 z-10">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex h-16 items-center justify-between">
            <div className="flex items-center">
              <Link to="/" className="flex items-center gap-2 font-bold text-xl text-indigo-600 dark:text-indigo-400">
                <History className="w-6 h-6" />
                <span>KubeChronicle</span>
              </Link>
              <nav className="ml-10 flex gap-4">
                <Link to="/" className="text-sm font-medium text-gray-700 hover:text-indigo-600 dark:text-gray-300 dark:hover:text-white">
                  Timeline
                </Link>
                {/* Add more nav links if needed */}
              </nav>
            </div>
            <div>
              {/* User profile or Theme toggle can go here */}
            </div>
          </div>
        </div>
      </header>

      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <Outlet />
      </main>
    </div>
  );
};
