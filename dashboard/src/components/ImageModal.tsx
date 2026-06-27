import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState, useCallback } from "react";
import { fetchPhotoDetail, getPhotoImageUrl, getFavoriteStatus, addToFavorites, removeFromFavorites } from "../api";
import { formatBytes, formatDate } from "../utils";
import { useNavigate } from "react-router-dom";

interface ImageModalProps {
  photoId: number;
  photoIds?: number[];
  onClose: () => void;
  onNavigate?: (photoId: number) => void;
}

export default function ImageModal({ photoId, photoIds = [], onClose, onNavigate }: ImageModalProps) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [favoriteError, setFavoriteError] = useState<string | null>(null);
  const legacyPhotoPrismPort = import.meta.env.VITE_PHOTOPRISM_PORT;

  const {
    data: photo,
    isLoading,
    error,
  } = useQuery({
    queryKey: ["photo", photoId],
    queryFn: () => fetchPhotoDetail(photoId),
  });

  const { data: favoriteStatus } = useQuery({
    queryKey: ["favorite", photoId],
    queryFn: () => getFavoriteStatus(photoId),
    staleTime: 30000,
    enabled: Boolean(legacyPhotoPrismPort),
  });

  const addFavoriteMutation = useMutation({
    mutationFn: () => addToFavorites(photoId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["favorite", photoId] });
      setFavoriteError(null);
    },
    onError: (error: Error) => {
      setFavoriteError(error.message);
      setTimeout(() => setFavoriteError(null), 5000);
    },
  });

  const removeFavoriteMutation = useMutation({
    mutationFn: () => removeFromFavorites(photoId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["favorite", photoId] });
      setFavoriteError(null);
    },
    onError: (error: Error) => {
      setFavoriteError(error.message);
      setTimeout(() => setFavoriteError(null), 5000);
    },
  });

  // Navigation helpers
  const currentIndex = photoIds.indexOf(photoId);
  const hasPrev = currentIndex > 0;
  const hasNext = currentIndex < photoIds.length - 1 && currentIndex !== -1;

  const goToPrev = useCallback(() => {
    if (hasPrev && onNavigate) {
      onNavigate(photoIds[currentIndex - 1]);
    }
  }, [hasPrev, onNavigate, photoIds, currentIndex]);

  const goToNext = useCallback(() => {
    if (hasNext && onNavigate) {
      onNavigate(photoIds[currentIndex + 1]);
    }
  }, [hasNext, onNavigate, photoIds, currentIndex]);

  // Keyboard navigation
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        onClose();
      } else if (e.key === "ArrowLeft") {
        goToPrev();
      } else if (e.key === "ArrowRight") {
        goToNext();
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [onClose, goToPrev, goToNext]);

  // Prevent body scroll when modal is open
  useEffect(() => {
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = "unset";
    };
  }, []);

  const handleArtistClick = () => {
    if (photo) {
      navigate(`/artists/${encodeURIComponent(photo.artist)}`);
      onClose();
    }
  };

  const toggleFavorite = () => {
    if (favoriteStatus?.favorite) {
      removeFavoriteMutation.mutate();
    } else {
      addFavoriteMutation.mutate();
    }
  };

  const isFavorite = favoriteStatus?.favorite ?? false;
  const isPhotoprismAvailable = Boolean(legacyPhotoPrismPort) && (favoriteStatus?.available ?? false);
  const isFavoriteLoading = addFavoriteMutation.isPending || removeFavoriteMutation.isPending;

  return (
    <div
      className="fixed inset-0 bg-black/90 z-50 flex items-center justify-center p-4"
      onClick={onClose}
    >
      <div className="relative max-w-7xl max-h-full w-full h-full flex items-center justify-center">
        {/* Close button */}
        <button
          onClick={onClose}
          className="absolute top-4 right-4 text-white bg-black/50 hover:bg-black/75 rounded-full p-2 z-10"
          title="Close (Esc)"
        >
          <svg
            className="w-6 h-6"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M6 18L18 6M6 6l12 12"
            />
          </svg>
        </button>

        {/* Previous button */}
        {hasPrev && (
          <button
            onClick={(e) => { e.stopPropagation(); goToPrev(); }}
            className="absolute left-4 top-1/2 -translate-y-1/2 text-white bg-black/50 hover:bg-black/75 rounded-full p-3 z-10"
            title="Previous (Left Arrow)"
          >
            <svg className="w-8 h-8" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
            </svg>
          </button>
        )}

        {/* Next button */}
        {hasNext && (
          <button
            onClick={(e) => { e.stopPropagation(); goToNext(); }}
            className="absolute right-4 top-1/2 -translate-y-1/2 text-white bg-black/50 hover:bg-black/75 rounded-full p-3 z-10"
            title="Next (Right Arrow)"
          >
            <svg className="w-8 h-8" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
            </svg>
          </button>
        )}

        {isLoading && <div className="text-white text-xl">Loading...</div>}

        {error && (
          <div className="text-red-400 text-xl">Failed to load image</div>
        )}

        {photo && (
          <div
            className="relative flex items-center justify-center w-full h-full"
            onClick={(e) => e.stopPropagation()}
          >
            <img
              src={getPhotoImageUrl(photoId)}
              alt={photo.title}
              className="max-w-full max-h-full object-contain"
              onClick={onClose}
            />

            {/* Image info overlay */}
            <div className="absolute bottom-0 left-0 right-0 bg-gradient-to-t from-black via-black/80 to-transparent text-white p-6">
              <h2 className="text-2xl font-bold mb-2 drop-shadow-lg">
                {photo.title}
              </h2>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-2 text-sm drop-shadow-lg">
                <div>
                  <span className="font-semibold">Artist:</span>{" "}
                  <button
                    onClick={handleArtistClick}
                    className="text-blue-300 hover:text-blue-200 underline"
                  >
                    {photo.artist}
                  </button>
                </div>
                <div>
                  <span className="font-semibold">Upload Date:</span>{" "}
                  {formatDate(photo.upload_date)}
                </div>
                <div>
                  <span className="font-semibold">Downloaded:</span>{" "}
                  {formatDate(photo.downloaded_at)}
                </div>
                <div>
                  <span className="font-semibold">Size:</span>{" "}
                  {formatBytes(photo.file_size)}
                </div>
                <div className="md:col-span-2">
                  <span className="font-semibold">Filename:</span>{" "}
                  <span className="font-mono text-xs">{photo.file_name}</span>
                </div>
              </div>

              {/* Action buttons */}
              <div className="mt-4 flex flex-wrap gap-3 items-center">
                {legacyPhotoPrismPort && (
                  <a
                    href={`http://${window.location.hostname}:${legacyPhotoPrismPort}/library/browse?q=${encodeURIComponent(`name:"${photo.file_name}"`)}`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded-md text-sm font-medium"
                    onClick={(e) => e.stopPropagation()}
                  >
                    <svg
                      className="w-4 h-4 mr-2"
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
                      />
                    </svg>
                    Open legacy PhotoPrism
                  </a>
                )}

                {/* Favorite button */}
                {isPhotoprismAvailable && (
                  <button
                    onClick={(e) => { e.stopPropagation(); toggleFavorite(); }}
                    disabled={isFavoriteLoading}
                    className={`inline-flex items-center px-4 py-2 rounded-md text-sm font-medium transition-colors ${
                      isFavorite
                        ? "bg-pink-600 hover:bg-pink-700 text-white"
                        : "bg-gray-600 hover:bg-gray-500 text-white"
                    } ${isFavoriteLoading ? "opacity-50 cursor-not-allowed" : ""}`}
                    title={isFavorite ? "Remove from favorites" : "Add to favorites"}
                  >
                    <svg
                      className={`w-4 h-4 mr-2 ${isFavorite ? "fill-current" : ""}`}
                      fill={isFavorite ? "currentColor" : "none"}
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z"
                      />
                    </svg>
                    {isFavoriteLoading ? "..." : isFavorite ? "Favorited" : "Favorite"}
                  </button>
                )}

                {/* Photo counter */}
                {photoIds.length > 0 && currentIndex !== -1 && (
                  <span className="text-gray-400 text-sm ml-auto">
                    {currentIndex + 1} / {photoIds.length}
                  </span>
                )}
              </div>

              {/* Error message */}
              {favoriteError && (
                <div className="mt-2 text-red-400 text-sm">
                  {favoriteError}
                </div>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
