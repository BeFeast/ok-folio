import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useParams, useNavigate } from "react-router-dom";
import { fetchArtistDetail } from "../api";
import { formatBytes, formatDate } from "../utils";
import ImageThumbnail from "./ImageThumbnail";
import ImageModal from "./ImageModal";

export default function ArtistDetail() {
  const { artistName } = useParams<{ artistName: string }>();
  const navigate = useNavigate();
  const artist = artistName ? decodeURIComponent(artistName) : "";
  const [page, setPage] = useState(0);
  const [selectedPhotoId, setSelectedPhotoId] = useState<number | null>(null);
  const limit = 50;

  const { data, isLoading, error } = useQuery({
    queryKey: ["artist-detail", artist, page],
    queryFn: () => fetchArtistDetail(artist, limit, page * limit),
    enabled: !!artist,
  });

  const totalPages = data ? Math.ceil(data.total / limit) : 0;

  if (isLoading) {
    return (
      <div className="bg-white rounded-lg shadow p-6">
        <div className="text-center py-12">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-500 mx-auto"></div>
          <p className="mt-4 text-gray-500">Loading photos...</p>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-white rounded-lg shadow p-6">
        <div className="bg-red-50 border border-red-200 rounded p-4">
          <p className="text-red-800">Error loading photos</p>
        </div>
      </div>
    );
  }

  const photos = data?.photos || [];
  const totalSize = photos.reduce((sum, photo) => sum + photo.FileSize, 0);

  return (
    <div className="space-y-6">
      <div className="bg-white rounded-lg shadow p-6">
        {/* Header with Back Button */}
        <div className="flex items-center justify-between mb-6">
          <div className="flex items-center space-x-4">
            <button
              onClick={() => navigate("/artists")}
              className="p-2 hover:bg-gray-100 rounded-full transition-colors"
              title="Back to artists"
            >
              <svg
                className="h-6 w-6 text-gray-600"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M15 19l-7-7 7-7"
                />
              </svg>
            </button>
            <div>
              <h2 className="text-2xl font-bold">{artist}</h2>
              <p className="text-sm text-gray-600 mt-1">
                {data?.total || 0} photos • {formatBytes(totalSize)}
              </p>
            </div>
          </div>
        </div>

        {photos.length === 0 ? (
          <div className="text-center py-12 text-gray-500">
            <svg
              className="mx-auto h-12 w-12 text-gray-400"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z"
              />
            </svg>
            <p className="mt-2">No photos found for this artist</p>
          </div>
        ) : (
          <>
            {/* Photos Grid */}
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 xl:grid-cols-8 gap-4">
              {photos.map((photo) => (
                <div key={photo.ID} className="group">
                  <ImageThumbnail
                    photoId={photo.ID}
                    title={photo.Title}
                    onClick={() => setSelectedPhotoId(photo.ID)}
                  />
                  <div className="mt-2 text-xs">
                    <div
                      className="font-medium text-gray-900 truncate"
                      title={photo.Title}
                    >
                      {photo.Title}
                    </div>
                    <div className="text-gray-500">
                      {formatDate(photo.UploadDate)} •{" "}
                      {formatBytes(photo.FileSize)}
                    </div>
                  </div>
                </div>
              ))}
            </div>

            {/* Pagination */}
            {totalPages > 1 && (
              <div className="mt-6 flex items-center justify-between">
                <button
                  onClick={() => setPage(Math.max(0, page - 1))}
                  disabled={page === 0}
                  className="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  Previous
                </button>
                <span className="text-sm text-gray-700">
                  Page {page + 1} of {totalPages}
                </span>
                <button
                  onClick={() => setPage(Math.min(totalPages - 1, page + 1))}
                  disabled={page >= totalPages - 1}
                  className="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  Next
                </button>
              </div>
            )}
          </>
        )}
      </div>

      {selectedPhotoId && (
        <ImageModal
          photoId={selectedPhotoId}
          onClose={() => setSelectedPhotoId(null)}
        />
      )}
    </div>
  );
}
