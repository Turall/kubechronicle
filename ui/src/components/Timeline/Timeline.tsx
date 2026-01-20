import React from 'react';
import type { ChangeEvent } from '../../types';
import { TimelineItem } from './TimelineItem';
import { Loader2 } from 'lucide-react';

interface TimelineProps {
  events: ChangeEvent[];
  isLoading?: boolean;
}

export const Timeline: React.FC<TimelineProps> = ({ events, isLoading }) => {
  if (isLoading) {
    return (
      <div className="flex flex-col gap-4 py-8 items-center justify-center text-gray-500">
        <Loader2 className="w-8 h-8 animate-spin" />
        <p>Loading changes...</p>
      </div>
    );
  }

  if (!events || events.length === 0) {
    return (
      <div className="flex flex-col gap-4 py-12 items-center justify-center text-gray-500 bg-gray-50 dark:bg-gray-800/50 rounded-lg border border-dashed border-gray-300 dark:border-gray-700">
        <p className="font-medium">No changes found</p>
        <p className="text-sm">Try adjusting your filters</p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {events.map((event) => (
        <TimelineItem key={event.id} event={event} />
      ))}
    </div>
  );
};
