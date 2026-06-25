import { useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { fetchGalleryCatalog } from "../api";
import ImageThumbnail from "../components/ImageThumbnail";
import type { GalleryFavoriteFacet } from "../types";
import { formatBytes, formatDate, formatNumber } from "../utils";

const PAGE_SIZE = 100;
const ALL_FAVORITES = "all";
const ALL_ARTISTS = "__all_artists__";

type FavoriteFilter = typeof ALL_FAVORITES | "true" | "false";

export default function Gallery() {
  const [offset, setOffset] = useState(0);
  const [providerFilter, setProviderFilter] = useState("");
  const [sourceFilter, setSourceFilter] = useState("");
  const [categoryFilter, setCategoryFilter] = useState("");
  const [artistFilter, setArtistFilter] = useState<string | undefined>();
  const [favoriteFilter, setFavoriteFilter] = useState<FavoriteFilter>(ALL_FAVORITES);
  const [query, setQuery] = useState("");

  const trimmedQuery = query.trim();
  const favoriteValue =
    favoriteFilter === ALL_FAVORITES ? undefined : favoriteFilter === "true";

  const { data, isLoading, error, isFetching } = useQuery({
    queryKey: [
      "gallery-catalog",
      PAGE_SIZE,
      offset,
      providerFilter,
      sourceFilter,
      categoryFilter,
      artistFilter,
      favoriteValue,
      trimmedQuery,
    ],
    queryFn: () =>
      fetchGalleryCatalog(PAGE_SIZE, offset, {
        provider: providerFilter,
        source: sourceFilter,
        category: categoryFilter,
        artist: artistFilter,
        favorite: favoriteValue,
        query: trimmedQuery,
      }),
    placeholderData: (previousData) => previousData,
    refetchInterval: 60000,
  });

  const photos = data?.photos ?? [];
  const total = data?.total ?? 0;
  const providers = data?.providers ?? [];
  const facets = data?.facets;
  const selectedProvider = providers.find((provider) => provider.id === providerFilter);
  const sourceOptions = selectedProvider?.sources ?? facets?.sources ?? [];
  const categoryOptions = facets?.categories ?? [];
  const artistOptions = facets?.artists ?? [];
  const favoriteOptions = facets?.favorites ?? [];
  const hasActiveFilter =
    providerFilter !== "" ||
    sourceFilter !== "" ||
    categoryFilter !== "" ||
    artistFilter !== undefined ||
    favoriteFilter !== ALL_FAVORITES ||
    trimmedQuery !== "";
  const showingStart = total === 0 ? 0 : offset + 1;
  const showingEnd = Math.min(offset + photos.length, total);
  const hasPreviousPage = offset > 0;
  const hasNextPage = offset + PAGE_SIZE < total;
  const resultCopy = hasActiveFilter
    ? `${formatNumber(total)} matching ${total === 1 ? "piece" : "pieces"}`
    : `${formatNumber(total)} cataloged ${total === 1 ? "piece" : "pieces"} from provider downloads`;

  const activeFilters = useMemo(() => {
    const filters: { label: string; value: string }[] = [];
    if (trimmedQuery) filters.push({ label: "Search", value: trimmedQuery });
    if (providerFilter) filters.push({ label: "Provider", value: selectedProvider?.display_name ?? providerFilter });
    if (sourceFilter) filters.push({ label: "Source", value: sourceOptions.find((source) => source.id === sourceFilter)?.display_name ?? sourceFilter });
    if (categoryFilter) filters.push({ label: "Category", value: categoryOptions.find((category) => category.id === categoryFilter)?.display_name ?? categoryFilter });
    if (artistFilter !== undefined) filters.push({ label: "Artist", value: artistOptions.find((artist) => artist.id === artistFilter)?.display_name ?? "Unknown artist" });
    if (favoriteFilter !== ALL_FAVORITES) filters.push({ label: "Favorite", value: favoriteLabel(favoriteFilter, favoriteOptions) });
    return filters;
  }, [
    artistFilter,
    artistOptions,
    categoryFilter,
    categoryOptions,
    favoriteFilter,
    favoriteOptions,
    providerFilter,
    selectedProvider?.display_name,
    sourceFilter,
    sourceOptions,
    trimmedQuery,
  ]);

  function resetPage() {
    setOffset(0);
  }

  function selectProvider(provider: string) {
    setProviderFilter(provider);
    setSourceFilter("");
    resetPage();
  }

  function clearFilters() {
    setProviderFilter("");
    setSourceFilter("");
    setCategoryFilter("");
    setArtistFilter(undefined);
    setFavoriteFilter(ALL_FAVORITES);
    setQuery("");
    resetPage();
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
          <p className="mt-1 text-sm text-gray-600">{resultCopy}</p>
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

      <div className="grid gap-6 lg:grid-cols-[18rem_minmax(0,1fr)]">
        <aside className="space-y-5 border-b border-gray-200 pb-5 lg:border-b-0 lg:border-r lg:pb-0 lg:pr-6">
          <div className="space-y-2">
            <label htmlFor="gallery-search" className="text-sm font-medium text-gray-800">
              Search
            </label>
            <input
              id="gallery-search"
              type="search"
              className="w-full rounded border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900"
              value={query}
              onChange={(event) => {
                setQuery(event.target.value);
                resetPage();
              }}
              placeholder="Title, artist, filename, source"
            />
          </div>

          <FacetSelect
            label="Source"
            value={sourceFilter}
            allLabel="All sources"
            options={sourceOptions}
            onChange={(value) => {
              setSourceFilter(value);
              resetPage();
            }}
          />

          <FacetSelect
            label="Provider"
            value={providerFilter}
            allLabel="All providers"
            options={providers}
            onChange={selectProvider}
          />

          <FacetSelect
            label="Category"
            value={categoryFilter}
            allLabel="All categories"
            options={categoryOptions}
            onChange={(value) => {
              setCategoryFilter(value);
              resetPage();
            }}
          />

          <FacetSelect
            label="Artist"
            value={artistFilter}
            allLabel="All artists"
            allValue={ALL_ARTISTS}
            options={artistOptions}
            onChange={(value) => {
              setArtistFilter(value === ALL_ARTISTS ? undefined : value);
              resetPage();
            }}
          />

          <label className="block text-sm font-medium text-gray-800">
            <span className="mb-1 block">Favorites</span>
            <select
              className="w-full rounded border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900"
              value={favoriteFilter}
              onChange={(event) => {
                setFavoriteFilter(event.target.value as FavoriteFilter);
                resetPage();
              }}
            >
              <option value={ALL_FAVORITES}>All pieces</option>
              {favoriteOptions.map((favorite) => (
                <option key={favorite.id} value={String(favorite.favorite)}>
                  {favorite.display_name} ({formatNumber(favorite.count)})
                </option>
              ))}
            </select>
          </label>

          {activeFilters.length > 0 && (
            <div className="space-y-3 border-t border-gray-200 pt-4">
              <div className="flex items-center justify-between gap-3">
                <h3 className="text-sm font-medium text-gray-900">Active filters</h3>
                <button
                  type="button"
                  className="text-sm text-gray-600 hover:text-gray-950"
                  onClick={clearFilters}
                >
                  Clear
                </button>
              </div>
              <dl className="space-y-2">
                {activeFilters.map((filter) => (
                  <div key={`${filter.label}:${filter.value}`} className="min-w-0">
                    <dt className="text-xs uppercase text-gray-500">{filter.label}</dt>
                    <dd className="truncate text-sm text-gray-900" title={filter.value}>
                      {filter.value}
                    </dd>
                  </div>
                ))}
              </dl>
            </div>
          )}

          {isFetching && (
            <span className="text-sm text-gray-500" role="status">
              Refreshing catalog...
            </span>
          )}
        </aside>

        <div className="min-w-0">
          {total === 0 ? (
            <section className="rounded border border-dashed border-gray-300 bg-white px-6 py-14 text-center">
              <h3 className="text-base font-medium text-gray-900">
                {hasActiveFilter ? "No pieces match these filters" : "No cataloged pieces yet"}
              </h3>
              <p className="mt-2 text-sm text-gray-600">
                {hasActiveFilter
                  ? "Try another search term or relax one of the facets."
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
            <section className="grid grid-cols-2 gap-x-4 gap-y-6 sm:grid-cols-3 md:grid-cols-4 xl:grid-cols-5">
              {photos.map((photo) => (
                <article key={photo.ID} className="min-w-0">
                  <Link to={`/pieces/${photo.ID}`} className="block">
                    <ImageThumbnail
                      photoId={photo.ID}
                      title={photo.Title}
                      className="aspect-[4/3] w-full rounded"
                    />
                  </Link>
                  <div className="mt-2 min-w-0 text-xs">
                    <Link
                      to={`/pieces/${photo.ID}`}
                      className="block w-full truncate text-left font-medium text-gray-950"
                      title={photo.Title}
                    >
                      {photo.Title || photo.FileName}
                    </Link>
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
        </div>
      </div>
    </div>
  );
}

interface FacetSelectProps {
  label: string;
  value: string | undefined;
  allLabel: string;
  allValue?: string;
  options: { id: string; display_name: string; count: number }[];
  onChange: (value: string) => void;
}

function FacetSelect({ label, value, allLabel, allValue = "", options, onChange }: FacetSelectProps) {
  return (
    <label className="block text-sm font-medium text-gray-800">
      <span className="mb-1 block">{label}</span>
      <select
        className="w-full rounded border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900"
        value={value ?? allValue}
        onChange={(event) => onChange(event.target.value)}
      >
        <option value={allValue}>{allLabel}</option>
        {options.map((option) => (
          <option key={option.id} value={option.id}>
            {option.display_name} ({formatNumber(option.count)})
          </option>
        ))}
      </select>
    </label>
  );
}

function favoriteLabel(value: FavoriteFilter, options: GalleryFavoriteFacet[]) {
  const match = options.find((favorite) => String(favorite.favorite) === value);
  return match?.display_name ?? (value === "true" ? "Favorites" : "Not favorites");
}
