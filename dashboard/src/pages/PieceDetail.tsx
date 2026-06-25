import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useParams } from "react-router-dom";
import {
  addToFavorites,
  fetchPhotoDetail,
  getPhotoImageUrl,
  removeFromFavorites,
} from "../api";
import { formatBytes, formatDate } from "../utils";

function displayValue(value: string | null | undefined, fallback: string) {
  return value && value.trim() !== "" ? value : fallback;
}

export default function PieceDetail() {
  const { photoId } = useParams();
  const queryClient = useQueryClient();
  const id = Number(photoId);

  const {
    data: piece,
    isLoading,
    error,
  } = useQuery({
    queryKey: ["photo", id],
    queryFn: () => fetchPhotoDetail(id),
    enabled: Number.isInteger(id) && id > 0,
  });

  const favoriteMutation = useMutation({
    mutationFn: (favorite: boolean) =>
      favorite ? addToFavorites(id) : removeFromFavorites(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["photo", id] });
      queryClient.invalidateQueries({ queryKey: ["favorite", id] });
      queryClient.invalidateQueries({ queryKey: ["gallery-catalog"] });
    },
  });

  if (!Number.isInteger(id) || id <= 0) {
    return (
      <div className="rounded border border-red-200 bg-red-50 p-4">
        <p className="text-sm font-medium text-red-900">Invalid piece route.</p>
      </div>
    );
  }

  if (isLoading) {
    return (
      <div className="flex min-h-64 items-center justify-center text-sm text-gray-600" role="status">
        Loading piece...
      </div>
    );
  }

  if (error || !piece) {
    return (
      <div className="rounded border border-red-200 bg-red-50 p-4">
        <p className="text-sm font-medium text-red-900">Failed to load piece.</p>
        <p className="mt-1 text-sm text-red-800">
          The detail route reads from the local OK Folio API. Check the API service, then retry.
        </p>
      </div>
    );
  }

  const title = displayValue(piece.title, piece.file_name || `Piece ${piece.id}`);
  const artist = displayValue(piece.artist, "Unknown artist");
  const source = displayValue(piece.source, "Unknown source");
  const category = displayValue(piece.category, "Unknown category");
  const isUpdatingFavorite = favoriteMutation.isPending;

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 border-b border-gray-200 pb-5 sm:flex-row sm:items-center sm:justify-between">
        <div className="min-w-0">
          <Link to="/" className="text-sm font-medium text-gray-600 hover:text-gray-950">
            Back to gallery
          </Link>
          <h2 className="mt-2 truncate text-2xl font-semibold text-gray-950">
            {title}
          </h2>
        </div>
        <button
          type="button"
          className={`w-fit rounded border px-4 py-2 text-sm font-medium disabled:cursor-not-allowed disabled:opacity-50 ${
            piece.favorite
              ? "border-gray-950 bg-gray-950 text-white hover:bg-gray-800"
              : "border-gray-300 bg-white text-gray-900 hover:bg-gray-50"
          }`}
          disabled={isUpdatingFavorite}
          onClick={() => favoriteMutation.mutate(!piece.favorite)}
        >
          {isUpdatingFavorite ? "Saving..." : piece.favorite ? "Favorited" : "Favorite"}
        </button>
      </div>

      <section className="bg-black">
        <img
          src={getPhotoImageUrl(piece.id)}
          alt={title}
          className="mx-auto max-h-[76vh] w-full object-contain"
        />
      </section>

      <section className="grid gap-6 border-t border-gray-200 pt-6 lg:grid-cols-[2fr_1fr]">
        <div className="min-w-0">
          <h3 className="text-base font-semibold text-gray-950">Provenance</h3>
          <dl className="mt-4 grid gap-4 sm:grid-cols-2">
            <div>
              <dt className="text-xs font-medium uppercase text-gray-500">Source</dt>
              <dd className="mt-1 break-words text-sm text-gray-950">{source}</dd>
            </div>
            <div>
              <dt className="text-xs font-medium uppercase text-gray-500">Artist</dt>
              <dd className="mt-1 text-sm text-gray-950">
                {piece.artist ? (
                  <Link
                    to={`/artists/${encodeURIComponent(piece.artist)}`}
                    className="hover:text-gray-600"
                  >
                    {artist}
                  </Link>
                ) : (
                  artist
                )}
              </dd>
            </div>
            <div>
              <dt className="text-xs font-medium uppercase text-gray-500">Date</dt>
              <dd className="mt-1 text-sm text-gray-950">{formatDate(piece.upload_date)}</dd>
            </div>
            <div>
              <dt className="text-xs font-medium uppercase text-gray-500">Category</dt>
              <dd className="mt-1 text-sm text-gray-950">{category}</dd>
            </div>
          </dl>
        </div>

        <div>
          <h3 className="text-base font-semibold text-gray-950">File</h3>
          <dl className="mt-4 space-y-4">
            <div>
              <dt className="text-xs font-medium uppercase text-gray-500">Downloaded</dt>
              <dd className="mt-1 text-sm text-gray-950">{formatDate(piece.downloaded_at)}</dd>
            </div>
            <div>
              <dt className="text-xs font-medium uppercase text-gray-500">Size</dt>
              <dd className="mt-1 text-sm text-gray-950">{formatBytes(piece.file_size)}</dd>
            </div>
            <div>
              <dt className="text-xs font-medium uppercase text-gray-500">Filename</dt>
              <dd className="mt-1 break-all font-mono text-xs text-gray-950">{piece.file_name}</dd>
            </div>
            {piece.source_page && (
              <div>
                <dt className="text-xs font-medium uppercase text-gray-500">Original page</dt>
                <dd className="mt-1 break-all text-sm text-gray-950">
                  <a href={piece.source_page} target="_blank" rel="noreferrer" className="hover:text-gray-600">
                    {piece.source_page}
                  </a>
                </dd>
              </div>
            )}
          </dl>
        </div>
      </section>

      {favoriteMutation.error && (
        <div className="rounded border border-red-200 bg-red-50 p-3 text-sm text-red-800">
          Failed to save favorite.
        </div>
      )}
    </div>
  );
}
