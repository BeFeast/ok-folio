import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useParams, useNavigate } from "react-router-dom";
import { fetchRunPhotos, fetchRuns } from "../api";
import { formatBytes, formatDate, formatDuration } from "../utils";
import ImageThumbnail from "../components/ImageThumbnail";
import ImageModal from "../components/ImageModal";
import { Link } from "react-router-dom";

export default function RunDetail() {
  const { runId } = useParams<{ runId: string }>();
  const navigate = useNavigate();
  const [selectedPhotoId, setSelectedPhotoId] = useState<number | null>(null);
  const [limit] = useState(100);
  const [offset] = useState(0);

  const runIdNum = runId ? parseInt(runId, 10) : 0;

  // Fetch run details
  const { data: runsData } = useQuery({
    queryKey: ["runs"],
    queryFn: () => fetchRuns(100),
  });

  const run = runsData?.runs.find((r) => r.ID === runIdNum);

  // Fetch photos for this run
  const { data, isLoading, error } = useQuery({
    queryKey: ["run-photos", runIdNum, limit, offset],
    queryFn: () => fetchRunPhotos(runIdNum, limit, offset),
    enabled: runIdNum > 0,
  });

  if (isLoading) {
    return (
      <div className="flex justify-center items-center h-64">
        <div className="text-gray-600">Loading run photos...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-red-50 border border-red-200 rounded-lg p-4">
        <p className="text-red-800">Failed to load run photos</p>
      </div>
    );
  }

  const photos = data?.photos || [];
  const total = data?.total || 0;

  return (
    <div className="space-y-6">
      <div className="rounded-lg border border-gray-200 bg-white p-5 shadow-sm">
        <div className="flex items-center justify-between mb-6">
          <div className="flex items-center space-x-4">
            <button
              onClick={() => navigate("/")}
              className="p-2 hover:bg-gray-100 rounded-full transition-colors"
              title="Back to overview"
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
              <p className="text-xs font-semibold uppercase tracking-wide text-gray-500">
                Run verification
              </p>
              <h2 className="mt-2 text-2xl font-semibold text-gray-950">
                Extraction run #{runId}
              </h2>
              {run && (
                <p className="text-sm text-gray-600 mt-1">
                  {formatDate(run.StartTime)} • Duration: {formatDuration(run.StartTime, run.EndTime)} • Status:{" "}
                  <span
                    className={`font-medium ${
                      run.Status === "completed"
                        ? "text-green-600"
                        : run.Status === "running"
                        ? "text-blue-600"
                        : "text-red-600"
                    }`}
                  >
                    {run.Status}
                  </span>
                </p>
              )}
            </div>
          </div>
          <div className="text-right">
            <div className="text-3xl font-semibold text-gray-950">{total}</div>
            <div className="text-sm text-gray-600">
              {total === 1 ? "photo" : "photos"} downloaded
            </div>
          </div>
        </div>

        {/* Run Stats */}
        {run && (
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6 rounded-md border border-gray-200 bg-gray-50 p-4">
            <div>
              <div className="text-sm text-gray-600">Pages Processed</div>
              <div className="text-xl font-semibold">{run.PagesProcessed}</div>
            </div>
            <div>
              <div className="text-sm text-gray-600">Photos Found</div>
              <div className="text-xl font-semibold">{run.PhotosFound}</div>
            </div>
            <div>
              <div className="text-sm text-gray-600">Downloaded</div>
              <div className="text-xl font-semibold text-emerald-700">
                {run.PhotosDownloaded}
              </div>
            </div>
            <div>
              <div className="text-sm text-gray-600">Skipped</div>
              <div className="text-xl font-semibold text-gray-600">
                {run.PhotosSkipped}
              </div>
            </div>
          </div>
        )}

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
            <p>No photos were downloaded during this run</p>
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
          onClose={() => setSelectedPhotoId(null)}
        />
      )}
    </div>
  );
}
