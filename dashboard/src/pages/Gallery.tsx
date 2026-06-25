import { useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { fetchGalleryCatalog } from "../api";
import ImageModal from "../components/ImageModal";
import ImageThumbnail from "../components/ImageThumbnail";
import { formatBytes, formatDate, formatNumber } from "../utils";

const PAGE_SIZE = 100;

export default function Gallery() {
  const [selectedPhotoId, setSelectedPhotoId] = useState<number | null>(null);
  const [offset, setOffset] = useState(0);
  const [providerFilter, setProviderFilter] = useState("");
  const [sourceFilter, setSourceFilter] = useState("");

  const { data, isLoading, error, isFetching } = useQuery({
    queryKey: ["gallery-catalog", PAGE_SIZE, offset, providerFilter, sourceFilter],
    queryFn: () =>
      fetchGalleryCatalog(PAGE_SIZE, offset, {
        provider: providerFilter,
        source: sourceFilter,
      }),
    placeholderData: (previousData) => previousData,
    refetchInterval: 60000,
  });

  const photos = data?.photos ?? [];
  const total = data?.total ?? 0;
  const providers = data?.providers ?? [];
  const selectedProvider = providers.find((provider) => provider.id === providerFilter);
  const sourceOptions = selectedProvider?.sources ?? providers.flatMap((provider) => provider.sources);
  const hasActiveFilter = providerFilter !== "" || sourceFilter !== "";
  const showingStart = total === 0 ? 0 : offset + 1;
  const showingEnd = Math.min(offset + photos.length, total);
  const hasPreviousPage = offset > 0;
  const hasNextPage = offset + PAGE_SIZE < total;
  const photoIds = useMemo(() => photos.map((photo) => photo.ID), [photos]);
  const resultCopy = hasActiveFilter
    ? `${formatNumber(total)} matching ${total === 1 ? "image" : "images"}`
    : `${formatNumber(total)} cataloged ${total === 1 ? "image" : "images"} from provider downloads`;

  function selectProvider(provider: string) {
    setProviderFilter(provider);
    setSourceFilter("");
    setOffset(0);
  }

  function selectSource(source: string) {
    setSourceFilter(source);
    setOffset(0);
  }

  function clearFilters() {
    setProviderFilter("");
    setSourceFilter("");
    setOffset(0);
  }

  if (isLoading) {
    return (
      <div className="flex min-h-64 items-center justify-center text-sm text-gray-600" role="status">
        Loading OK Folio gallery...
      </div>
    );
  }

  if (error) {
    return (
      <div className="rounded border border-red-200 bg-red-50 p-4">
        <p className="text-sm font-medium text-red-900">Failed to load OK Folio gallery.</p>
        <p className="mt-1 text-sm text-red-800">
          The gallery uses the local catalog and image endpoints. Check the API service, then retry.
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <section className="flex flex-col gap-4 border-b border-gray-200 pb-5 md:flex-row md:items-end md:justify-between">
        <div>
          <h2 className="text-2xl font-semibold text-gray-950">Gallery</h2>
          <p className="mt-1 text-sm text-gray-600">
            {resultCopy}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2 text-sm text-gray-600">
          <Link
            to="/operations"
            className="rounded border border-gray-300 bg-white px-3 py-2 text-gray-800 hover:bg-gray-50"
          >
            Operations
          </Link>
          <span>
            Showing {formatNumber(showingStart)}-{formatNumber(showingEnd)}
          </span>
          <button
            type="button"
            className="rounded border border-gray-300 bg-white px-3 py-2 text-gray-800 disabled:cursor-not-allowed disabled:opacity-40"
            disabled={!hasPreviousPage || isFetching}
            onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
          >
            Previous
          </button>
          <button
            type="button"
            className="rounded border border-gray-300 bg-white px-3 py-2 text-gray-800 disabled:cursor-not-allowed disabled:opacity-40"
            disabled={!hasNextPage || isFetching}
            onClick={() => setOffset(offset + PAGE_SIZE)}
          >
            Next
          </button>
        </div>
      </section>

      <section className="flex flex-col gap-3 border-b border-gray-200 pb-5 md:flex-row md:items-end">
        <label className="min-w-0 text-sm font-medium text-gray-700">
          <span className="mb-1 block">Provider</span>
          <select
            className="w-full rounded border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 md:w-56"
            value={providerFilter}
            onChange={(event) => selectProvider(event.target.value)}
          >
            <option value="">All providers</option>
            {providers.map((provider) => (
              <option key={provider.id} value={provider.id}>
                {provider.display_name} ({formatNumber(provider.count)})
              </option>
            ))}
          </select>
        </label>
        <label className="min-w-0 text-sm font-medium text-gray-700">
          <span className="mb-1 block">Source</span>
          <select
            className="w-full rounded border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 md:w-80"
            value={sourceFilter}
            onChange={(event) => selectSource(event.target.value)}
          >
            <option value="">All sources</option>
            {sourceOptions.map((source) => (
              <option key={source.id} value={source.id}>
                {source.display_name} ({formatNumber(source.count)})
              </option>
            ))}
          </select>
        </label>
        {hasActiveFilter && (
          <button
            type="button"
            className="w-fit rounded border border-gray-300 bg-white px-3 py-2 text-sm text-gray-800 hover:bg-gray-50"
            onClick={clearFilters}
          >
            Clear filters
          </button>
        )}
        {isFetching && (
          <span className="text-sm text-gray-500" role="status">
            Refreshing catalog...
          </span>
        )}
      </section>

      {total === 0 ? (
        <section className="rounded border border-dashed border-gray-300 bg-white px-6 py-14 text-center">
          <h3 className="text-base font-medium text-gray-900">
            {hasActiveFilter ? "No images match these filters" : "No cataloged images yet"}
          </h3>
          <p className="mt-2 text-sm text-gray-600">
            {hasActiveFilter
              ? "Try another provider or source to browse the local catalog."
              : "Run an extraction from Operations, then downloaded provider media will appear here."}
          </p>
          <div className="mt-5 flex justify-center gap-2">
            {hasActiveFilter && (
              <button
                type="button"
                className="rounded border border-gray-300 bg-white px-3 py-2 text-sm text-gray-800 hover:bg-gray-50"
                onClick={clearFilters}
              >
                Clear filters
              </button>
            )}
            <Link
              to="/operations"
              className="rounded border border-gray-300 bg-white px-3 py-2 text-sm text-gray-800 hover:bg-gray-50"
            >
              Operations
            </Link>
          </div>
        </section>
      ) : (
        <section className="grid grid-cols-2 gap-x-4 gap-y-6 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
          {photos.map((photo) => (
            <article key={photo.ID} className="min-w-0">
              <ImageThumbnail
                photoId={photo.ID}
                title={photo.Title}
                onClick={() => setSelectedPhotoId(photo.ID)}
                className="aspect-[4/3] w-full rounded"
              />
              <div className="mt-2 min-w-0 text-xs">
                <button
                  type="button"
                  className="block w-full truncate text-left font-medium text-gray-950"
                  title={photo.Title}
                  onClick={() => setSelectedPhotoId(photo.ID)}
                >
                  {photo.Title || photo.FileName}
                </button>
                <Link
                  to={`/artists/${encodeURIComponent(photo.Artist)}`}
                  className="mt-1 block truncate text-gray-600 hover:text-gray-950"
                  title={photo.Artist}
                >
                  {photo.Artist || "Unknown artist"}
                </Link>
                <div className="mt-1 truncate text-gray-500">
                  {formatDate(photo.DownloadedAt)} · {formatBytes(photo.FileSize)}
                </div>
              </div>
            </article>
          ))}
        </section>
      )}

      {selectedPhotoId && (
        <ImageModal
          photoId={selectedPhotoId}
          photoIds={photoIds}
          onClose={() => setSelectedPhotoId(null)}
          onNavigate={(id) => setSelectedPhotoId(id)}
        />
      )}
    </div>
  );
}
