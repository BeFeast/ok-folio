import { useQuery } from '@tanstack/react-query';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';
import { fetchTopArtists } from '../api';
import { formatBytes } from '../utils';

interface TopArtistsChartProps {
  limit?: number;
}

export default function TopArtistsChart({ limit = 10 }: TopArtistsChartProps) {
  const { data, isLoading, error } = useQuery({
    queryKey: ['top-artists', limit],
    queryFn: () => fetchTopArtists(limit),
    refetchInterval: 60000,
  });

  if (isLoading) {
    return (
      <div className="bg-white rounded-lg shadow p-6">
        <h2 className="text-xl font-bold mb-4">Top Artists</h2>
        <div className="h-64 flex items-center justify-center">
          <div className="animate-pulse text-gray-500">Loading chart...</div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-white rounded-lg shadow p-6">
        <h2 className="text-xl font-bold mb-4">Top Artists</h2>
        <div className="bg-red-50 border border-red-200 rounded p-4">
          <p className="text-red-800">Error loading artists: {error.message}</p>
        </div>
      </div>
    );
  }

  const artists = data?.artists || [];

  if (artists.length === 0) {
    return (
      <div className="bg-white rounded-lg shadow p-6">
        <h2 className="text-xl font-bold mb-4">Top Artists</h2>
        <div className="h-64 flex items-center justify-center text-gray-500">
          No artist data available
        </div>
      </div>
    );
  }

  return (
    <div className="bg-white rounded-lg shadow p-6">
      <h2 className="text-xl font-bold mb-4">Top Artists by Photo Count</h2>
      <ResponsiveContainer width="100%" height={300}>
        <BarChart data={artists} layout="vertical">
          <CartesianGrid strokeDasharray="3 3" />
          <XAxis type="number" />
          <YAxis dataKey="artist" type="category" width={150} />
          <Tooltip
            formatter={(value: number, name: string) => {
              if (name === 'Photo Count') return value;
              if (name === 'Total Size') return formatBytes(value);
              return value;
            }}
          />
          <Bar dataKey="photo_count" fill="#3b82f6" name="Photo Count" />
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}
