import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { searchPhotos } from "../api";
import { formatBytes } from "../utils";
import { Link } from "react-router-dom";
import ImageThumbnail from "./ImageThumbnail";
import ImageModal from "./ImageModal";
import type { Photo } from "../types";

export default function Search() {
  const [query, setQuery] = useState("");
  const [debouncedQuery, setDebouncedQuery] = useState("");
  const [page, setPage] = useState(0);
  const [selectedPhotoId, setSelectedPhotoId] = useState<number | null>(null);
  const limit = 50;

  // Debounce search input
  const handleSearch = (value: string) => {
    setQuery(value);
    const timer = setTimeout(() => {
      setDebouncedQuery(value);
      setPage(0);
    }, 500);
    return () => clearTimeout(timer);
  };

  const { data, isLoading, error } = useQuery({
    queryKey: ["search", debouncedQuery, page],
    queryFn: () => searchPhotos(debouncedQuery, limit, page * limit),
    enabled: debouncedQuery.length > 0,
  });

  const totalPages = data ? Math.ceil(data.total / limit) : 0;

  return (
    <div className="space-y-6">
      <div className="bg-white rounded-lg shadow p-6">
        <h2 className="text-2xl font-bold mb-4">Search Photos</h2>

        {/* Search Input */}
        <div className="mb-6">
          <input
            type="text"
            value={query}
            onChange={(e) => handleSearch(e.target.value)}
            placeholder="Search by title, artist, or filename..."
            className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent"
          />
        </div>

        {/* Results */}
        {query.length === 0 && (
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
                d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
              />
            </svg>
            <p className="mt-2">Enter a search term to find photos</p>
          </div>
        )}

        {query.length > 0 && isLoading && (
          <div className="text-center py-12">
            <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-500 mx-auto"></div>
            <p className="mt-4 text-gray-500">Searching...</p>
          </div>
        )}

        {error && (
          <div className="bg-red-50 border border-red-200 rounded p-4">
            <p className="text-red-800">Error searching photos</p>
          </div>
        )}

        {data && data.photos.length === 0 && (
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
                d="M9.172 16.172a4 4 0 015.656 0M9 10h.01M15 10h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
            <p className="mt-2">No results found for "{debouncedQuery}"</p>
          </div>
        )}

        {data && data.photos.length > 0 && (
          <>
            <div className="mb-4 text-sm text-gray-600">
              Found {data.total} {data.total === 1 ? "result" : "results"} for "
              {debouncedQuery}"
            </div>

            {/* Results Grid */}
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 xl:grid-cols-8 gap-4">
              {data.photos.map((photo: Photo) => (
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
                    <Link
                      to={`/artists/${encodeURIComponent(photo.Artist)}`}
                      className="text-blue-600 hover:text-blue-800 truncate block"
                      title={photo.Artist}
                    >
                      {photo.Artist}
                    </Link>
                    <div className="text-gray-500">
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
