import { formatDistanceToNow, format } from 'date-fns';

export const formatTimeAgo = (dateStr: string) => {
  try {
    return formatDistanceToNow(new Date(dateStr), { addSuffix: true });
  } catch {
    return dateStr;
  }
};

export const formatDateTime = (dateStr: string) => {
  try {
    return format(new Date(dateStr), 'PPpp');
  } catch {
    return dateStr;
  }
};
