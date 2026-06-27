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
      <div className="border border-red-300 bg-red-50 p-4 dark:bg-red-950/30">
        <p className="text-sm font-medium text-red-900 dark:text-red-100">Invalid piece route.</p>
      </div>
    );
  }

  if (isLoading) {
    return (
      <div className="flex min-h-64 items-center justify-center text-sm text-[color:var(--folio-graphite)]" role="status">
        Loading piece...
      </div>
    );
  }

  if (error || !piece) {
    return (
      <div className="border border-red-300 bg-red-50 p-4 dark:bg-red-950/30">
        <p className="text-sm font-medium text-red-900 dark:text-red-100">Failed to load piece.</p>
        <p className="mt-1 text-sm text-red-800 dark:text-red-200">
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
      <div className="flex flex-col gap-3 border-b border-[color:var(--folio-line)] pb-5 sm:flex-row sm:items-center sm:justify-between">
        <div className="min-w-0">
          <Link to="/" className="text-sm font-medium text-[color:var(--folio-graphite)] hover:text-[color:var(--folio-ink)]">
            Back to gallery
          </Link>
          <h2 className="mt-2 truncate font-serif text-2xl text-[color:var(--folio-ink)]">
            {title}
          </h2>
        </div>
        <button
          type="button"
          className={`w-fit rounded-sm border px-4 py-2 text-sm font-medium disabled:cursor-not-allowed disabled:opacity-50 ${
            piece.favorite
              ? "border-[color:var(--folio-accent)] bg-[color:var(--folio-accent)] text-white hover:bg-[color:var(--folio-accent-strong)]"
              : "border-[color:var(--folio-line)] bg-[color:var(--folio-surface)] text-[color:var(--folio-ink)] hover:bg-[color:var(--folio-surface-muted)]"
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

      <section className="grid gap-6 border-t border-[color:var(--folio-line)] pt-6 lg:grid-cols-[2fr_1fr]">
        <div className="min-w-0">
          <h3 className="text-base font-semibold text-[color:var(--folio-ink)]">Provenance</h3>
          <dl className="mt-4 grid gap-4 sm:grid-cols-2">
            <div>
              <dt className="text-xs font-medium uppercase text-[color:var(--folio-graphite)]">Source</dt>
              <dd className="mt-1 break-words text-sm text-[color:var(--folio-ink)]">{source}</dd>
            </div>
            <div>
              <dt className="text-xs font-medium uppercase text-[color:var(--folio-graphite)]">Artist</dt>
              <dd className="mt-1 text-sm text-[color:var(--folio-ink)]">
                {piece.artist ? (
                  <Link
                    to={`/artists/${encodeURIComponent(piece.artist)}`}
                    className="hover:text-[color:var(--folio-accent)]"
                  >
                    {artist}
                  </Link>
                ) : (
                  artist
                )}
              </dd>
            </div>
            <div>
              <dt className="text-xs font-medium uppercase text-[color:var(--folio-graphite)]">Date</dt>
              <dd className="mt-1 text-sm text-[color:var(--folio-ink)]">{formatDate(piece.upload_date)}</dd>
            </div>
            <div>
              <dt className="text-xs font-medium uppercase text-[color:var(--folio-graphite)]">Category</dt>
              <dd className="mt-1 text-sm text-[color:var(--folio-ink)]">{category}</dd>
            </div>
          </dl>
        </div>

        <div>
          <h3 className="text-base font-semibold text-[color:var(--folio-ink)]">File</h3>
          <dl className="mt-4 space-y-4">
            <div>
              <dt className="text-xs font-medium uppercase text-[color:var(--folio-graphite)]">Downloaded</dt>
              <dd className="mt-1 text-sm text-[color:var(--folio-ink)]">{formatDate(piece.downloaded_at)}</dd>
            </div>
            <div>
              <dt className="text-xs font-medium uppercase text-[color:var(--folio-graphite)]">Size</dt>
              <dd className="mt-1 text-sm text-[color:var(--folio-ink)]">{formatBytes(piece.file_size)}</dd>
            </div>
            <div>
              <dt className="text-xs font-medium uppercase text-[color:var(--folio-graphite)]">Filename</dt>
              <dd className="mt-1 break-all font-mono text-xs text-[color:var(--folio-ink)]">{piece.file_name}</dd>
            </div>
            {piece.source_page && (
              <div>
                <dt className="text-xs font-medium uppercase text-[color:var(--folio-graphite)]">Original page</dt>
                <dd className="mt-1 break-all text-sm text-[color:var(--folio-ink)]">
                  <a href={piece.source_page} target="_blank" rel="noreferrer" className="hover:text-[color:var(--folio-accent)]">
                    {piece.source_page}
                  </a>
                </dd>
              </div>
            )}
          </dl>
        </div>
      </section>

      {favoriteMutation.error && (
        <div className="border border-red-300 bg-red-50 p-3 text-sm text-red-800 dark:bg-red-950/30 dark:text-red-200">
          Failed to save favorite.
        </div>
      )}
    </div>
  );
}
