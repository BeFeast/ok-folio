import { useQuery } from '@tanstack/react-query';
import { fetchHealth } from '../api';

export default function HealthIndicator() {
  const { data: health, isLoading } = useQuery({
    queryKey: ['health'],
    queryFn: fetchHealth,
    refetchInterval: 10000, // Check every 10 seconds
  });

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 text-gray-500">
        <div className="w-2 h-2 bg-gray-400 rounded-full animate-pulse"></div>
        <span className="text-sm">Checking...</span>
      </div>
    );
  }

  const isHealthy = health?.status === 'healthy' && health?.database === 'connected';

  return (
    <div className="flex items-center gap-2">
      <div
        className={`w-2 h-2 rounded-full ${
          isHealthy ? 'bg-green-500' : 'bg-red-500'
        } ${isHealthy ? 'animate-pulse' : ''}`}
      ></div>
      <span className="text-sm text-gray-600">
        {isHealthy ? 'System Healthy' : 'System Error'}
      </span>
    </div>
  );
}
