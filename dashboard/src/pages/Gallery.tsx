import { useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { fetchGalleryCatalog } from "../api";
import ImageThumbnail from "../components/ImageThumbnail";
import type { GalleryFavoriteFacet, Photo } from "../types";
import { formatBytes, formatDate, formatNumber } from "../utils";

const PAGE_SIZE = 100;
const ALL_FAVORITES = "all";
const ALL_ARTISTS = "__all_artists__";

type FavoriteFilter = typeof ALL_FAVORITES | "true" | "false";
type GalleryMode = "library" | "magazine" | "wall";

const galleryModes: { id: GalleryMode; label: string }[] = [
  { id: "library", label: "Library" },
  { id: "magazine", label: "Magazine" },
  { id: "wall", label: "Wall" },
];

function pieceTitle(photo: Photo) {
  return photo.Title || photo.FileName || `Piece ${photo.ID}`;
}

export default function Gallery() {
  const [offset, setOffset] = useState(0);
  const [mode, setMode] = useState<GalleryMode>("library");
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
      <div className="flex min-h-64 items-center justify-center text-sm text-[color:var(--folio-graphite)]" role="status">
        Loading OK Folio gallery...
      </div>
    );
  }

  if (error) {
    return (
      <div className="border border-red-300 bg-red-50 p-4 dark:bg-red-950/30">
        <p className="text-sm font-medium text-red-900 dark:text-red-100">Failed to load OK Folio gallery.</p>
        <p className="mt-1 text-sm text-red-800 dark:text-red-200">
          The gallery uses the local catalog and image endpoints. Check the API service, then retry.
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <section className="flex flex-col gap-4 border-b border-[color:var(--folio-line)] pb-5 md:flex-row md:items-end md:justify-between">
        <div>
          <p className="text-xs font-medium uppercase tracking-[0.18em] text-[color:var(--folio-accent)]">
            Aggregated catalog
          </p>
          <h2 className="mt-1 font-serif text-3xl text-[color:var(--folio-ink)]">Gallery</h2>
          <p className="mt-1 text-sm text-[color:var(--folio-graphite)]">{resultCopy}</p>
        </div>
        <div className="flex flex-wrap items-center gap-3 text-sm text-[color:var(--folio-graphite)]">
          <div className="inline-flex border border-[color:var(--folio-line)] bg-[color:var(--folio-surface)] p-1">
            {galleryModes.map((galleryMode) => (
              <button
                key={galleryMode.id}
                type="button"
                className={`px-3 py-1.5 text-sm font-medium transition-colors ${
                  mode === galleryMode.id
                    ? "bg-[color:var(--folio-accent)] text-white"
                    : "text-[color:var(--folio-graphite)] hover:text-[color:var(--folio-ink)]"
                }`}
                onClick={() => setMode(galleryMode.id)}
                aria-pressed={mode === galleryMode.id}
              >
                {galleryMode.label}
              </button>
            ))}
          </div>
          <span>
            Showing {formatNumber(showingStart)}-{formatNumber(showingEnd)}
          </span>
          <button
            type="button"
            className="border border-[color:var(--folio-line)] bg-[color:var(--folio-surface)] px-3 py-2 text-[color:var(--folio-ink)] disabled:cursor-not-allowed disabled:opacity-40"
            disabled={!hasPreviousPage || isFetching}
            onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
          >
            Previous
          </button>
          <button
            type="button"
            className="border border-[color:var(--folio-line)] bg-[color:var(--folio-surface)] px-3 py-2 text-[color:var(--folio-ink)] disabled:cursor-not-allowed disabled:opacity-40"
            disabled={!hasNextPage || isFetching}
            onClick={() => setOffset(offset + PAGE_SIZE)}
          >
            Next
          </button>
        </div>
      </section>

      <div className="grid gap-6 lg:grid-cols-[18rem_minmax(0,1fr)]">
        <aside className="space-y-5 border-b border-[color:var(--folio-line)] pb-5 lg:border-b-0 lg:border-r lg:pb-0 lg:pr-6">
          <div className="space-y-2">
            <label htmlFor="gallery-search" className="text-sm font-medium text-[color:var(--folio-ink)]">
              Search
            </label>
            <input
              id="gallery-search"
              type="search"
              className="w-full border border-[color:var(--folio-line)] bg-[color:var(--folio-surface)] px-3 py-2 text-sm text-[color:var(--folio-ink)]"
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

          <label className="block text-sm font-medium text-[color:var(--folio-ink)]">
            <span className="mb-1 block">Favorites</span>
            <select
              className="w-full border border-[color:var(--folio-line)] bg-[color:var(--folio-surface)] px-3 py-2 text-sm text-[color:var(--folio-ink)]"
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
            <div className="space-y-3 border-t border-[color:var(--folio-line)] pt-4">
              <div className="flex items-center justify-between gap-3">
                <h3 className="text-sm font-medium text-[color:var(--folio-ink)]">Active filters</h3>
                <button
                  type="button"
                  className="text-sm text-[color:var(--folio-graphite)] hover:text-[color:var(--folio-ink)]"
                  onClick={clearFilters}
                >
                  Clear
                </button>
              </div>
              <dl className="space-y-2">
                {activeFilters.map((filter) => (
                  <div key={`${filter.label}:${filter.value}`} className="min-w-0">
                    <dt className="text-xs uppercase text-[color:var(--folio-graphite)]">{filter.label}</dt>
                    <dd className="truncate text-sm text-[color:var(--folio-ink)]" title={filter.value}>
                      {filter.value}
                    </dd>
                  </div>
                ))}
              </dl>
            </div>
          )}

          {isFetching && (
            <span className="text-sm text-[color:var(--folio-graphite)]" role="status">
              Refreshing catalog...
            </span>
          )}
        </aside>

        <div className="min-w-0">
          {total === 0 ? (
            <section className="border border-dashed border-[color:var(--folio-line)] bg-[color:var(--folio-surface)] px-6 py-14 text-center">
              <h3 className="text-base font-medium text-[color:var(--folio-ink)]">
                {hasActiveFilter ? "No pieces match these filters" : "No cataloged pieces yet"}
              </h3>
              <p className="mt-2 text-sm text-[color:var(--folio-graphite)]">
                {hasActiveFilter
                  ? "Try another search term or relax one of the facets."
                  : "Connect streams, then downloaded provider media will appear here."}
              </p>
              <div className="mt-5 flex justify-center gap-2">
                {hasActiveFilter && (
                  <button
                    type="button"
                    className="border border-[color:var(--folio-line)] bg-[color:var(--folio-surface)] px-3 py-2 text-sm text-[color:var(--folio-ink)] hover:bg-[color:var(--folio-surface-muted)]"
                    onClick={clearFilters}
                  >
                    Clear filters
                  </button>
                )}
                <Link
                  to="/streams"
                  className="border border-[color:var(--folio-line)] bg-[color:var(--folio-surface)] px-3 py-2 text-sm text-[color:var(--folio-ink)] hover:bg-[color:var(--folio-surface-muted)]"
                >
                  Streams
                </Link>
              </div>
            </section>
          ) : (
            <GalleryModeView mode={mode} photos={photos} />
          )}
        </div>
      </div>
    </div>
  );
}

function GalleryModeView({ mode, photos }: { mode: GalleryMode; photos: Photo[] }) {
  if (mode === "magazine") {
    const [feature, ...rest] = photos;
    return (
      <section className="space-y-6">
        {feature && (
          <article className="grid gap-5 border border-[color:var(--folio-line)] bg-[color:var(--folio-surface)] p-4 shadow-[var(--folio-shadow)] md:grid-cols-[minmax(0,1.4fr)_minmax(16rem,0.6fr)]">
            <Link to={`/pieces/${feature.ID}`} className="block">
              <ImageThumbnail
                photoId={feature.ID}
                title={pieceTitle(feature)}
                className="aspect-[16/10] w-full"
              />
            </Link>
            <PieceText photo={feature} size="large" />
          </article>
        )}
        <section className="grid grid-cols-1 gap-5 sm:grid-cols-2 xl:grid-cols-3">
          {rest.map((photo) => (
            <article key={photo.ID} className="min-w-0 border border-[color:var(--folio-line)] bg-[color:var(--folio-surface)] p-3">
              <Link to={`/pieces/${photo.ID}`} className="block">
                <ImageThumbnail
                  photoId={photo.ID}
                  title={pieceTitle(photo)}
                  className="aspect-[5/4] w-full"
                />
              </Link>
              <PieceText photo={photo} />
            </article>
          ))}
        </section>
      </section>
    );
  }

  if (mode === "wall") {
    return (
      <section className="columns-2 gap-3 sm:columns-3 lg:columns-4 xl:columns-5">
        {photos.map((photo, index) => (
          <Link
            key={photo.ID}
            to={`/pieces/${photo.ID}`}
            className="mb-3 block break-inside-avoid border border-[color:var(--folio-line)] bg-[color:var(--folio-surface)] p-2"
            title={pieceTitle(photo)}
          >
            <ImageThumbnail
              photoId={photo.ID}
              title={pieceTitle(photo)}
              className={`${index % 5 === 0 ? "aspect-[3/4]" : index % 3 === 0 ? "aspect-square" : "aspect-[4/3]"} w-full`}
            />
          </Link>
        ))}
      </section>
    );
  }

  return (
    <section className="grid grid-cols-3 gap-x-3 gap-y-5 sm:grid-cols-4 md:grid-cols-5 xl:grid-cols-7">
      {photos.map((photo) => (
        <article key={photo.ID} className="min-w-0">
          <Link to={`/pieces/${photo.ID}`} className="block">
            <ImageThumbnail
              photoId={photo.ID}
              title={pieceTitle(photo)}
              className="aspect-[4/3] w-full"
            />
          </Link>
          <PieceText photo={photo} compact />
        </article>
      ))}
    </section>
  );
}

function PieceText({ photo, compact = false, size = "normal" }: { photo: Photo; compact?: boolean; size?: "normal" | "large" }) {
  const title = pieceTitle(photo);
  const artist = photo.Artist.trim();
  return (
    <div className={`${compact ? "mt-1.5" : "mt-3"} min-w-0 text-xs`}>
      <Link
        to={`/pieces/${photo.ID}`}
        className={`${size === "large" ? "font-serif text-2xl" : "font-medium"} block w-full truncate text-left text-[color:var(--folio-ink)]`}
        title={title}
      >
        {title}
      </Link>
      {artist ? (
        <Link
          to={`/artists/${encodeURIComponent(artist)}`}
          className={`${compact ? "mt-0.5" : "mt-1"} block truncate text-[color:var(--folio-graphite)] hover:text-[color:var(--folio-ink)]`}
          title={artist}
        >
          {artist}
        </Link>
      ) : (
        <span className={`${compact ? "mt-0.5" : "mt-1"} block truncate text-[color:var(--folio-graphite)]`}>
          Unknown artist
        </span>
      )}
      {!compact && (
        <div className="mt-1 truncate text-[color:var(--folio-graphite)]">
          {formatDate(photo.DownloadedAt)} - {formatBytes(photo.FileSize)}
        </div>
      )}
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
    <label className="block text-sm font-medium text-[color:var(--folio-ink)]">
      <span className="mb-1 block">{label}</span>
      <select
        className="w-full border border-[color:var(--folio-line)] bg-[color:var(--folio-surface)] px-3 py-2 text-sm text-[color:var(--folio-ink)]"
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
