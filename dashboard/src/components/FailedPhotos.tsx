import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { fetchFailedPhotos, retryPhoto } from "../api";
import { formatBytes, formatDate } from "../utils";

interface FailedPhotosProps {
  compact?: boolean;
}

export default function FailedPhotos({ compact = false }: FailedPhotosProps) {
  const [selectedPhotos, setSelectedPhotos] = useState<Set<number>>(new Set());
  const queryClient = useQueryClient();

  const { data, isLoading, error } = useQuery({
    queryKey: ["failed-photos"],
    queryFn: () => fetchFailedPhotos(50),
    refetchInterval: 30000,
  });

  const retryMutation = useMutation({
    mutationFn: retryPhoto,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["failed-photos"] });
      queryClient.invalidateQueries({ queryKey: ["stats"] });
    },
  });

  const handleRetry = (id: number) => {
    retryMutation.mutate(id);
    setSelectedPhotos((prev) => {
      const next = new Set(prev);
      next.delete(id);
      return next;
    });
  };

  const handleRetrySelected = () => {
    selectedPhotos.forEach((id) => {
      retryMutation.mutate(id);
    });
    setSelectedPhotos(new Set());
  };

  const togglePhoto = (id: number) => {
    setSelectedPhotos((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  };

  if (isLoading) {
    return (
      <section className="rounded-lg border border-gray-200 bg-white p-5 shadow-xs">
        <h2 className="text-xl font-semibold text-gray-950">Failed downloads</h2>
        <div className="animate-pulse space-y-3">
          {[...Array(3)].map((_, i) => (
            <div key={i} className="h-16 bg-gray-200 rounded-sm"></div>
          ))}
        </div>
      </section>
    );
  }

  if (error) {
    return (
      <section className="rounded-lg border border-gray-200 bg-white p-5 shadow-xs">
        <h2 className="text-xl font-semibold text-gray-950">Failed downloads</h2>
        <div className="bg-red-50 border border-red-200 rounded-sm p-4">
          <p className="text-red-800">
            Error loading failed photos: {error.message}
          </p>
        </div>
      </section>
    );
  }

  const photos = data?.photos || [];

  if (photos.length === 0) {
    return (
      <section className="rounded-lg border border-gray-200 bg-white p-5 shadow-xs">
        <h2 className="text-xl font-semibold text-gray-950">Failed downloads</h2>
        <div className="mt-4 rounded-md border border-dashed border-gray-300 p-6 text-center text-gray-600">
          No failed downloads are reported by the API.
        </div>
      </section>
    );
  }

  const visiblePhotos = compact ? photos.slice(0, 6) : photos;

  return (
    <section className="rounded-lg border border-gray-200 bg-white shadow-xs">
      <div className="flex flex-col gap-4 border-b border-gray-200 p-5 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <p className="text-xs font-semibold uppercase tracking-wide text-gray-500">
            Failure triage
          </p>
          <h2 className="mt-2 text-xl font-semibold text-gray-950">
            Failed downloads
          </h2>
          <p className="text-sm text-gray-600 mt-1">
            {photos.length} photo{photos.length !== 1 ? "s" : ""} failed to
            download
          </p>
        </div>
        {selectedPhotos.size > 0 && (
          <button
            onClick={handleRetrySelected}
            disabled={retryMutation.isPending}
            className="rounded-md bg-gray-950 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 disabled:bg-gray-300"
          >
            Retry Selected ({selectedPhotos.size})
          </button>
        )}
      </div>

      <div className="overflow-x-auto">
        <table className="w-full">
          <thead className="bg-gray-50">
            <tr>
              <th className="px-6 py-3 text-left">
                <input
                  type="checkbox"
                  checked={selectedPhotos.size === photos.length}
                  onChange={(e) => {
                    if (e.target.checked) {
                      setSelectedPhotos(new Set(photos.map((p) => p.ID)));
                    } else {
                      setSelectedPhotos(new Set());
                    }
                  }}
                  className="rounded-sm"
                />
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Artist
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Title
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Upload Date
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Size
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Failed At
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Actions
              </th>
            </tr>
          </thead>
          <tbody className="bg-white divide-y divide-gray-200">
            {visiblePhotos.map((photo) => (
              <tr key={photo.ID} className="hover:bg-gray-50">
                <td className="px-6 py-4 whitespace-nowrap">
                  <input
                    type="checkbox"
                    checked={selectedPhotos.has(photo.ID)}
                    onChange={() => togglePhoto(photo.ID)}
                    className="rounded-sm"
                  />
                </td>
                <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">
                  {photo.Artist}
                </td>
                <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                  {photo.Title || "Untitled"}
                </td>
                <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                  {formatDate(photo.UploadDate)}
                </td>
                <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                  {formatBytes(photo.FileSize)}
                </td>
                <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                  {formatDate(photo.DownloadedAt)}
                </td>
                <td className="px-6 py-4 whitespace-nowrap text-sm">
                  <button
                    onClick={() => handleRetry(photo.ID)}
                    disabled={retryMutation.isPending}
                    className="font-medium text-gray-950 hover:text-gray-700 disabled:text-gray-400"
                  >
                    Retry
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {compact && photos.length > visiblePhotos.length && (
        <div className="border-t border-gray-200 p-4 text-sm text-gray-600">
          Showing {visiblePhotos.length} of {photos.length} failed downloads.
        </div>
      )}
    </section>
  );
}
