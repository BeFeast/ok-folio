import { useQuery } from '@tanstack/react-query';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer } from 'recharts';
import { fetchTimeline } from '../api';

interface TimelineChartProps {
  days?: number;
}

export default function TimelineChart({ days = 7 }: TimelineChartProps) {
  const { data, isLoading, error } = useQuery({
    queryKey: ['timeline', days],
    queryFn: () => fetchTimeline(days),
    refetchInterval: 60000, // Refetch every minute
  });

  if (isLoading) {
    return (
      <div className="bg-white rounded-lg shadow p-6">
        <h2 className="text-xl font-bold mb-4">Download Activity</h2>
        <div className="h-64 flex items-center justify-center">
          <div className="animate-pulse text-gray-500">Loading chart...</div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-white rounded-lg shadow p-6">
        <h2 className="text-xl font-bold mb-4">Download Activity</h2>
        <div className="bg-red-50 border border-red-200 rounded p-4">
          <p className="text-red-800">Error loading timeline: {error.message}</p>
        </div>
      </div>
    );
  }

  const timeline = data?.timeline || [];

  if (timeline.length === 0) {
    return (
      <div className="bg-white rounded-lg shadow p-6">
        <h2 className="text-xl font-bold mb-4">Download Activity</h2>
        <div className="h-64 flex items-center justify-center text-gray-500">
          No data available for the selected period
        </div>
      </div>
    );
  }

  return (
    <div className="bg-white rounded-lg shadow p-6">
      <h2 className="text-xl font-bold mb-4">Download Activity (Last {days} Days)</h2>
      <ResponsiveContainer width="100%" height={300}>
        <LineChart data={timeline}>
          <CartesianGrid strokeDasharray="3 3" />
          <XAxis dataKey="date" />
          <YAxis />
          <Tooltip />
          <Legend />
          <Line
            type="monotone"
            dataKey="downloaded"
            stroke="#10b981"
            strokeWidth={2}
            name="Downloaded"
          />
          <Line
            type="monotone"
            dataKey="skipped"
            stroke="#6b7280"
            strokeWidth={2}
            name="Skipped"
          />
          <Line
            type="monotone"
            dataKey="failed"
            stroke="#ef4444"
            strokeWidth={2}
            name="Failed"
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}
