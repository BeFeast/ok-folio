import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { fetchTodayPhotos } from '../api';
import ImageThumbnail from '../components/ImageThumbnail';
import ImageModal from '../components/ImageModal';
import { formatBytes, formatDate } from '../utils';
import { Link } from 'react-router-dom';

export default function TodayImages() {
  const [selectedPhotoId, setSelectedPhotoId] = useState<number | null>(null);
  const [limit] = useState(100);
  const [offset] = useState(0);

  const { data, isLoading, error } = useQuery({
    queryKey: ['today-photos', limit, offset],
    queryFn: () => fetchTodayPhotos(limit, offset),
    refetchInterval: 30000, // Refetch every 30 seconds for real-time updates
  });

  if (isLoading) {
    return (
      <div className="flex justify-center items-center h-64">
        <div className="text-gray-600">Loading today's photos...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-red-50 border border-red-200 rounded-lg p-4">
        <p className="text-red-800">Failed to load today's photos</p>
      </div>
    );
  }

  const photos = data?.photos || [];
  const total = data?.total || 0;

  return (
    <div className="space-y-6">
      <div className="bg-white rounded-lg shadow p-6">
        <div className="flex items-center justify-between mb-4">
          <div>
            <h2 className="text-2xl font-bold text-gray-900">Today's Downloads</h2>
            <p className="text-sm text-gray-600 mt-1">
              {data?.date && `Photos downloaded on ${formatDate(data.date)}`}
            </p>
          </div>
          <div className="text-right">
            <div className="text-3xl font-bold text-blue-600">{total}</div>
            <div className="text-sm text-gray-600">
              {total === 1 ? 'photo' : 'photos'} today
            </div>
          </div>
        </div>

        {total === 0 ? (
          <div className="text-center py-12 text-gray-500">
            <svg
              className="mx-auto h-12 w-12 text-gray-400 mb-4"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z"
              />
            </svg>
            <p>No photos downloaded today yet</p>
            <p className="text-sm mt-2">Check back later or trigger a manual extraction</p>
          </div>
        ) : (
          <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 xl:grid-cols-8 gap-4">
            {photos.map((photo) => (
              <div key={photo.ID} className="group">
                <ImageThumbnail
                  photoId={photo.ID}
                  title={photo.Title}
                  onClick={() => setSelectedPhotoId(photo.ID)}
                />
                <div className="mt-2 text-xs">
                  <div className="font-medium text-gray-900 truncate" title={photo.Title}>
                    {photo.Title}
                  </div>
                  <Link
                    to={`/artists/${encodeURIComponent(photo.Artist)}`}
                    className="text-blue-600 hover:text-blue-800 truncate block"
                    title={photo.Artist}
                  >
                    {photo.Artist}
                  </Link>
                  <div className="text-gray-500">{formatBytes(photo.FileSize)}</div>
                </div>
              </div>
            ))}
          </div>
        )}

        {total > limit && (
          <div className="mt-6 text-center text-sm text-gray-600">
            Showing {photos.length} of {total} photos
          </div>
        )}
      </div>

      {selectedPhotoId && (
        <ImageModal
          photoId={selectedPhotoId}
          photoIds={photos.map(p => p.ID)}
          onClose={() => setSelectedPhotoId(null)}
          onNavigate={(id) => setSelectedPhotoId(id)}
        />
      )}
    </div>
  );
}
