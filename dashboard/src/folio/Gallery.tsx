import { useEffect, useMemo, useRef, useState, type CSSProperties, type ChangeEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import { fetchArtists, fetchGalleryCatalog, getPhotoThumbnailUrl } from "../api";
import type { GalleryFacet } from "../types";
import { useFolio, type GalleryMode, type PieceVM } from "./context";
import { BrandMark, CloseIcon, HeartIcon, Hov, OkfImage, PageHeader, SearchIcon } from "./ui";
import { useViewport } from "./useViewport";

const MODES: { key: GalleryMode; label: string }[] = [
  { key: "magazine", label: "Magazine" },
  { key: "library", label: "Library" },
  { key: "wall", label: "Wall" },
];

const MOBILE_MEDIUMS = ["Painting", "Photography", "Drawing", "Print", "Sculpture"];

function SlidersIcon({ size = 16 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round">
      <path d="M4 7h9M17 7h3M4 17h3M11 17h9" />
      <circle cx="15" cy="7" r="2" />
      <circle cx="9" cy="17" r="2" />
    </svg>
  );
}

function MagnifierMinusIcon({ size = 28 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.65" strokeLinecap="round">
      <circle cx="11" cy="11" r="7" />
      <path d="M8.5 11h5M20 20l-4-4" />
    </svg>
  );
}

function Spinner({ size = 22 }: { size?: number }) {
  return (
    <span
      aria-hidden="true"
      style={{
        width: size,
        height: size,
        borderRadius: 99,
        border: "2px solid color-mix(in srgb, var(--accent) 18%, transparent)",
        borderTopColor: "var(--accent)",
        animation: "okf-spin .8s linear infinite",
      }}
    />
  );
}

function useArtistSuggestions(draft: string) {
  const { query, favoriteOnly } = useFolio();
  const trimmedQuery = query.trim();
  const useScopedSuggestions = favoriteOnly || !!trimmedQuery;
  const artistsQuery = useQuery({
    queryKey: ["folio-artists"],
    queryFn: () => fetchArtists(500, 0, "count"),
    enabled: !useScopedSuggestions,
  });
  const scopedArtistsQuery = useQuery({
    queryKey: ["folio-artist-facets", trimmedQuery, favoriteOnly],
    queryFn: () => {
      const filters: Parameters<typeof fetchGalleryCatalog>[2] = {};
      if (trimmedQuery) filters.query = trimmedQuery;
      if (favoriteOnly) filters.favorite = true;
      return fetchGalleryCatalog(1, 0, filters);
    },
    enabled: useScopedSuggestions,
  });
  const normalized = draft.trim().toLowerCase();
  const suggestions = useMemo(() => {
    const artists = useScopedSuggestions
      ? (scopedArtistsQuery.data?.facets.artists ?? []).map((item) => ({
          artist: item.display_name,
          photo_count: item.count,
        }))
      : (artistsQuery.data?.artists ?? []);
    return artists
      .filter((item) => !normalized || item.artist.toLowerCase().includes(normalized))
      .slice(0, 8);
  }, [artistsQuery.data?.artists, normalized, scopedArtistsQuery.data?.facets.artists, useScopedSuggestions]);
  return { suggestions, normalized };
}

function ModeTabs() {
  const { mode, setMode } = useFolio();
  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        gap: 3,
        padding: 4,
        border: "1px solid var(--line)",
        borderRadius: 99,
        background: "var(--surface)",
      }}
    >
      {MODES.map((m) => {
        const active = mode === m.key;
        return (
          <button
            key={m.key}
            onClick={() => setMode(m.key)}
            style={{
              appearance: "none",
              cursor: "pointer",
              fontFamily: "var(--sans)",
              fontSize: 13.5,
              letterSpacing: "0.1px",
              padding: "8px 17px",
              border: 0,
              borderRadius: 99,
              color: active ? "var(--ink)" : "var(--graphite)",
              background: active ? "var(--surface-2)" : "transparent",
              boxShadow: active ? "0 1px 4px var(--shadow)" : "none",
            }}
          >
            {m.label}
          </button>
        );
      })}
    </div>
  );
}

function GalleryFilterBar() {
  const { favoriteOnly, setFavoriteOnly, artist, setArtist } = useFolio();
  const [draft, setDraft] = useState("");
  const [open, setOpen] = useState(false);
  const { suggestions, normalized } = useArtistSuggestions(draft);

  const chooseArtist = (name: string) => {
    const next = name.trim();
    if (!next) return;
    setArtist(next);
    setDraft("");
    setOpen(false);
  };
  const submitArtist = () => {
    const exact = suggestions.find((item) => item.artist.toLowerCase() === normalized);
    const fallback = suggestions[0]?.artist ?? draft.trim();
    chooseArtist(exact?.artist ?? fallback);
  };

  const showSuggestions = open && suggestions.length > 0;

  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        gap: 10,
        flexWrap: "wrap",
        padding: "18px 0 0",
      }}
    >
      <Hov
        as="button"
        type="button"
        aria-pressed={favoriteOnly}
        onClick={() => setFavoriteOnly(!favoriteOnly)}
        style={{
          appearance: "none",
          cursor: "pointer",
          height: 39,
          padding: "0 15px",
          borderRadius: 99,
          border: `1px solid ${favoriteOnly ? "var(--accent)" : "var(--line)"}`,
          background: favoriteOnly ? "var(--accent)" : "var(--surface)",
          color: favoriteOnly ? "var(--on-accent)" : "var(--graphite)",
          display: "inline-flex",
          alignItems: "center",
          gap: 8,
          fontFamily: "var(--sans)",
          fontSize: 13.5,
          fontWeight: 500,
          boxShadow: favoriteOnly ? "0 1px 7px var(--shadow)" : "none",
        }}
        hover={{ borderColor: "var(--accent-line)" }}
      >
        <HeartIcon size={15} fill={favoriteOnly ? "currentColor" : "transparent"} stroke="currentColor" />
        Favorites
      </Hov>

      <div style={{ position: "relative", width: "min(320px, 100%)" }}>
        <input
          value={draft}
          onChange={(e: ChangeEvent<HTMLInputElement>) => {
            setDraft(e.target.value);
            setOpen(true);
          }}
          onFocus={() => setOpen(true)}
          onBlur={() => window.setTimeout(() => setOpen(false), 120)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && (draft.trim() || suggestions[0])) {
              e.preventDefault();
              submitArtist();
            }
            if (e.key === "Escape") setOpen(false);
          }}
          placeholder="Filter by author or artist"
          aria-label="Filter by author or artist"
          style={{
            width: "100%",
            height: 39,
            boxSizing: "border-box",
            borderRadius: 99,
            border: "1px solid var(--line)",
            outline: "none",
            background: "var(--surface)",
            color: "var(--ink)",
            padding: "0 15px",
            fontFamily: "var(--sans)",
            fontSize: 13.5,
          }}
        />
        {showSuggestions ? (
          <div
            style={{
              position: "absolute",
              top: "calc(100% + 7px)",
              left: 0,
              right: 0,
              zIndex: 12,
              border: "1px solid var(--line)",
              borderRadius: 8,
              background: "var(--surface)",
              boxShadow: "0 12px 30px var(--shadow)",
              overflow: "hidden",
            }}
          >
            {suggestions.map((item) => (
              <button
                key={item.artist}
                type="button"
                onMouseDown={(e) => e.preventDefault()}
                onClick={() => chooseArtist(item.artist)}
                style={{
                  appearance: "none",
                  cursor: "pointer",
                  width: "100%",
                  border: 0,
                  borderBottom: "1px solid var(--line)",
                  background: "transparent",
                  color: "var(--ink)",
                  padding: "10px 12px",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "space-between",
                  gap: 14,
                  fontFamily: "var(--sans)",
                  fontSize: 13,
                  textAlign: "left",
                }}
              >
                <span style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{item.artist}</span>
                <span style={{ flex: "none", color: "var(--muted)", fontSize: 12 }}>{item.photo_count.toLocaleString()}</span>
              </button>
            ))}
          </div>
        ) : null}
      </div>

      {artist ? (
        <div
          style={{
            height: 39,
            borderRadius: 99,
            border: "1px solid var(--accent-line)",
            background: "var(--surface-2)",
            color: "var(--ink)",
            padding: "0 8px 0 14px",
            display: "inline-flex",
            alignItems: "center",
            gap: 8,
            fontFamily: "var(--sans)",
            fontSize: 13,
          }}
        >
          <span style={{ maxWidth: 240, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{artist}</span>
          <button
            type="button"
            aria-label="Clear artist filter"
            onClick={() => setArtist("")}
            style={{
              appearance: "none",
              cursor: "pointer",
              width: 25,
              height: 25,
              borderRadius: 99,
              border: 0,
              background: "transparent",
              color: "var(--graphite)",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
            }}
          >
            <CloseIcon size={14} />
          </button>
        </div>
      ) : null}
    </div>
  );
}

function MobileModeTabs() {
  const { mode, setMode } = useFolio();
  return (
    <div
      role="tablist"
      aria-label="Gallery mode"
      style={{
        display: "grid",
        gridTemplateColumns: "repeat(3, minmax(0, 1fr))",
        gap: 3,
        minWidth: 0,
        flex: "1 1 auto",
        height: 44,
        padding: 3,
        borderRadius: 14,
        background: "#E7E1D4",
      }}
    >
      {MODES.map((m) => {
        const active = mode === m.key;
        return (
          <button
            key={m.key}
            type="button"
            role="tab"
            aria-selected={active}
            onClick={() => setMode(m.key)}
            style={{
              appearance: "none",
              minWidth: 0,
              border: 0,
              borderRadius: 11,
              background: active ? "var(--accent)" : "transparent",
              color: active ? "var(--on-accent)" : "var(--graphite)",
              cursor: "pointer",
              fontFamily: "var(--sans)",
              fontSize: 12.5,
              fontWeight: 700,
            }}
          >
            {m.label}
          </button>
        );
      })}
    </div>
  );
}

function MobileFilterChip({ label, onRemove }: { label: string; onRemove: () => void }) {
  return (
    <button
      type="button"
      onClick={onRemove}
      style={{
        appearance: "none",
        minHeight: 34,
        maxWidth: "100%",
        border: "1px solid var(--accent-line)",
        borderRadius: 99,
        background: "var(--surface)",
        color: "var(--ink)",
        cursor: "pointer",
        display: "inline-flex",
        alignItems: "center",
        gap: 7,
        padding: "0 10px 0 12px",
        fontFamily: "var(--sans)",
        fontSize: 12.5,
        fontWeight: 600,
      }}
    >
      <span style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{label}</span>
      <CloseIcon size={13} />
    </button>
  );
}

function ActiveMobileFilters() {
  const { query, setQuery, favoriteOnly, setFavoriteOnly, artist, setArtist, category, setCategory } = useFolio();
  if (!query.trim() && !favoriteOnly && !artist && !category) return null;
  return (
    <div style={{ display: "flex", flexWrap: "wrap", gap: 8, padding: "0 0 12px" }}>
      {query.trim() ? <MobileFilterChip label={`Search: ${query.trim()}`} onRemove={() => setQuery("")} /> : null}
      {favoriteOnly ? <MobileFilterChip label="Favorites" onRemove={() => setFavoriteOnly(false)} /> : null}
      {artist ? <MobileFilterChip label={artist} onRemove={() => setArtist("")} /> : null}
      {category ? <MobileFilterChip label={category} onRemove={() => setCategory("")} /> : null}
    </div>
  );
}

function categoryForMedium(categories: GalleryFacet[], medium: string): GalleryFacet | undefined {
  const target = medium.trim().toLowerCase();
  return categories.find((facet) => {
    const id = facet.id.trim().toLowerCase();
    const label = facet.display_name.trim().toLowerCase();
    return id === target || label === target;
  });
}

function MobileFiltersSheet({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { query, setQuery, favoriteOnly, setFavoriteOnly, artist, setArtist, category, setCategory, total } = useFolio();
  const [artistDraft, setArtistDraft] = useState("");
  const [artistOpen, setArtistOpen] = useState(false);
  const { suggestions, normalized } = useArtistSuggestions(artistDraft);
  const filtersForFacets = useMemo(() => {
    const filters: Parameters<typeof fetchGalleryCatalog>[2] = {};
    if (query.trim()) filters.query = query.trim();
    if (favoriteOnly) filters.favorite = true;
    if (artist) filters.artist = artist;
    return filters;
  }, [artist, favoriteOnly, query]);
  const facetsQuery = useQuery({
    queryKey: ["folio-mobile-filter-facets", filtersForFacets],
    queryFn: () => fetchGalleryCatalog(1, 0, filtersForFacets),
    enabled: open,
  });
  const categories = facetsQuery.data?.facets.categories ?? [];
  const hasDisabledMediums = MOBILE_MEDIUMS.some((medium) => !categoryForMedium(categories, medium));
  const showSuggestions = artistOpen && suggestions.length > 0;

  useEffect(() => {
    if (!open) return;
    const previous = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = previous;
    };
  }, [open]);

  useEffect(() => {
    if (open) setArtistDraft(artist);
  }, [artist, open]);

  const chooseArtist = (name: string) => {
    const next = name.trim();
    setArtist(next);
    setArtistDraft(next);
    setArtistOpen(false);
  };
  const submitArtist = () => {
    const exact = suggestions.find((item) => item.artist.toLowerCase() === normalized);
    const fallback = suggestions[0]?.artist ?? artistDraft.trim();
    chooseArtist(exact?.artist ?? fallback);
  };
  const reset = () => {
    setQuery("");
    setFavoriteOnly(false);
    setArtist("");
    setArtistDraft("");
    setCategory("");
  };

  if (!open) return null;
  return (
    <div role="dialog" aria-modal="true" aria-label="Filters">
      <button
        type="button"
        aria-label="Close filters"
        onClick={onClose}
        style={{
          position: "fixed",
          inset: 0,
          zIndex: 100,
          border: 0,
          background: "rgba(20,14,10,.5)",
          cursor: "pointer",
        }}
      />
      <section
        style={{
          position: "fixed",
          left: 0,
          right: 0,
          bottom: 0,
          zIndex: 101,
          maxHeight: "min(82vh, 690px)",
          overflow: "auto",
          padding: "10px calc(20px + var(--safe-right)) calc(18px + var(--safe-bottom)) calc(20px + var(--safe-left))",
          borderRadius: "24px 24px 0 0",
          background: "var(--bg)",
          boxShadow: "0 -18px 40px rgba(0,0,0,.25)",
        }}
      >
        <div style={{ width: 42, height: 4, borderRadius: 99, background: "color-mix(in srgb, var(--ink) 22%, transparent)", margin: "0 auto 17px" }} />
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 14 }}>
          <h2 style={{ margin: 0, fontFamily: "var(--serif)", fontWeight: 500, fontSize: 26, lineHeight: 1, color: "var(--ink)" }}>Filters</h2>
          <button
            type="button"
            onClick={reset}
            style={{
              appearance: "none",
              border: 0,
              background: "transparent",
              color: "var(--accent)",
              cursor: "pointer",
              fontFamily: "var(--sans)",
              fontSize: 14,
              fontWeight: 700,
              padding: "8px 0 8px 12px",
            }}
          >
            Reset
          </button>
        </div>

        <button
          type="button"
          aria-pressed={favoriteOnly}
          onClick={() => setFavoriteOnly(!favoriteOnly)}
          style={{
            appearance: "none",
            width: "100%",
            minHeight: 58,
            marginTop: 20,
            border: "1px solid var(--line)",
            borderRadius: 14,
            background: "var(--surface)",
            color: "var(--ink)",
            cursor: "pointer",
            display: "flex",
            alignItems: "center",
            gap: 12,
            padding: "0 14px",
            fontFamily: "var(--sans)",
            fontSize: 15,
            fontWeight: 700,
            textAlign: "left",
          }}
        >
          <HeartIcon size={18} fill={favoriteOnly ? "var(--accent)" : "transparent"} stroke={favoriteOnly ? "var(--accent)" : "currentColor"} />
          <span style={{ flex: 1 }}>Favorites only</span>
          <span
            aria-hidden="true"
            style={{
              width: 48,
              height: 28,
              borderRadius: 99,
              background: favoriteOnly ? "var(--accent)" : "color-mix(in srgb, var(--ink) 13%, transparent)",
              padding: 3,
              boxSizing: "border-box",
              display: "flex",
              justifyContent: favoriteOnly ? "flex-end" : "flex-start",
            }}
          >
            <span style={{ width: 22, height: 22, borderRadius: 99, background: favoriteOnly ? "var(--on-accent)" : "var(--surface)" }} />
          </span>
        </button>

        <div style={{ marginTop: 23 }}>
          <div style={{ fontFamily: "var(--sans)", fontSize: 11, fontWeight: 700, letterSpacing: "0.06em", color: "var(--muted)" }}>ARTIST</div>
          <div style={{ position: "relative", marginTop: 9 }}>
            <div
              style={{
                height: 48,
                border: "1px solid var(--line)",
                borderRadius: 13,
                background: "var(--surface)",
                display: "flex",
                alignItems: "center",
                gap: 9,
                padding: "0 12px",
              }}
            >
              <SearchIcon />
              <input
                value={artistDraft}
                onChange={(e: ChangeEvent<HTMLInputElement>) => {
                  setArtistDraft(e.target.value);
                  setArtistOpen(true);
                }}
                onFocus={() => setArtistOpen(true)}
                onBlur={() => window.setTimeout(() => setArtistOpen(false), 120)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" && (artistDraft.trim() || suggestions[0])) {
                    e.preventDefault();
                    submitArtist();
                  }
                  if (e.key === "Escape") setArtistOpen(false);
                }}
                placeholder="Type to find an artist..."
                aria-label="Type to find an artist"
                style={{
                  appearance: "none",
                  minWidth: 0,
                  flex: 1,
                  border: 0,
                  outline: 0,
                  background: "transparent",
                  color: "var(--ink)",
                  fontFamily: "var(--sans)",
                  fontSize: 15,
                }}
              />
              {artist ? (
                <button
                  type="button"
                  aria-label="Clear artist filter"
                  onClick={() => chooseArtist("")}
                  style={{ border: 0, background: "transparent", color: "var(--muted)", padding: 4, display: "flex" }}
                >
                  <CloseIcon size={14} />
                </button>
              ) : null}
            </div>
            {showSuggestions ? (
              <div
                style={{
                  position: "absolute",
                  left: 0,
                  right: 0,
                  top: "calc(100% + 7px)",
                  zIndex: 2,
                  border: "1px solid var(--line)",
                  borderRadius: 12,
                  background: "var(--surface)",
                  boxShadow: "0 12px 30px var(--shadow)",
                  overflow: "hidden",
                }}
              >
                {suggestions.map((item) => (
                  <button
                    key={item.artist}
                    type="button"
                    onMouseDown={(e) => e.preventDefault()}
                    onClick={() => chooseArtist(item.artist)}
                    style={{
                      appearance: "none",
                      width: "100%",
                      minHeight: 44,
                      border: 0,
                      borderBottom: "1px solid var(--line)",
                      background: "transparent",
                      color: "var(--ink)",
                      cursor: "pointer",
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "space-between",
                      gap: 12,
                      padding: "0 13px",
                      fontFamily: "var(--sans)",
                      fontSize: 14,
                      textAlign: "left",
                    }}
                  >
                    <span style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{item.artist}</span>
                    <span style={{ color: "var(--muted)", fontSize: 12 }}>{item.photo_count.toLocaleString()}</span>
                  </button>
                ))}
              </div>
            ) : null}
          </div>
        </div>

        <div style={{ marginTop: 23 }}>
          <div style={{ fontFamily: "var(--sans)", fontSize: 11, fontWeight: 700, letterSpacing: "0.06em", color: "var(--muted)" }}>MEDIUM</div>
          <div style={{ display: "flex", flexWrap: "wrap", gap: 8, marginTop: 10 }}>
            {MOBILE_MEDIUMS.map((medium) => {
              const facet = categoryForMedium(categories, medium);
              const selected = !!facet && category === facet.id;
              return (
                <button
                  key={medium}
                  type="button"
                  disabled={!facet}
                  onClick={() => facet && setCategory(selected ? "" : facet.id)}
                  style={{
                    appearance: "none",
                    minHeight: 36,
                    border: `1px solid ${selected ? "var(--accent)" : "var(--line-2)"}`,
                    borderRadius: 99,
                    background: selected ? "var(--accent)" : "transparent",
                    color: selected ? "var(--on-accent)" : facet ? "var(--ink)" : "var(--muted)",
                    cursor: facet ? "pointer" : "not-allowed",
                    opacity: facet ? 1 : 0.48,
                    padding: "0 13px",
                    fontFamily: "var(--sans)",
                    fontSize: 13,
                    fontWeight: 700,
                  }}
                >
                  {medium}
                </button>
              );
            })}
          </div>
          {hasDisabledMediums ? (
            <p style={{ margin: "9px 0 0", fontFamily: "var(--sans)", fontSize: 12.5, lineHeight: 1.35, color: "var(--muted)" }}>
              Some medium chips are coming soon for catalogs without matching category facets.
            </p>
          ) : null}
        </div>

        <div
          style={{
            position: "sticky",
            bottom: "calc(0px + var(--safe-bottom))",
            marginTop: 26,
            paddingTop: 12,
            background: "linear-gradient(to top, var(--bg) 76%, color-mix(in srgb, var(--bg) 0%, transparent))",
          }}
        >
          <button
            type="button"
            onClick={onClose}
            style={{
              appearance: "none",
              width: "100%",
              height: 52,
              border: 0,
              borderRadius: 13,
              background: "var(--accent)",
              color: "var(--on-accent)",
              cursor: "pointer",
              boxShadow: "0 8px 20px rgba(124,36,32,.3)",
              fontFamily: "var(--sans)",
              fontSize: 15,
              fontWeight: 800,
            }}
          >
            Show {total.toLocaleString()} pieces
          </button>
        </div>
      </section>
    </div>
  );
}

function MobileGalleryHeader() {
  const [filtersOpen, setFiltersOpen] = useState(false);
  return (
    <>
      <div style={{ display: "flex", alignItems: "center", gap: 10, padding: "4px 0 12px" }}>
        <MobileModeTabs />
        <button
          type="button"
          onClick={() => setFiltersOpen(true)}
          style={{
            appearance: "none",
            height: 44,
            flex: "0 0 auto",
            border: "1px solid var(--line-2)",
            borderRadius: 13,
            background: "var(--surface)",
            color: "var(--ink)",
            cursor: "pointer",
            display: "inline-flex",
            alignItems: "center",
            gap: 7,
            padding: "0 12px",
            fontFamily: "var(--sans)",
            fontSize: 13,
            fontWeight: 800,
          }}
        >
          <SlidersIcon />
          Filters
        </button>
      </div>
      <ActiveMobileFilters />
      <MobileFiltersSheet open={filtersOpen} onClose={() => setFiltersOpen(false)} />
    </>
  );
}

function Heart({ id, size = 15, top, right, dim = 31 }: { id: number; size?: number; top: number; right: number; dim?: number }) {
  const { isFav, toggleFav } = useFolio();
  const [hover, setHover] = useState(false);
  const fav = isFav(id);
  return (
    <button
      aria-label={fav ? "Saved" : "Favorite"}
      onClick={(e) => {
        e.stopPropagation();
        toggleFav(id);
      }}
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
      style={{
        position: "absolute",
        top,
        right,
        zIndex: 4,
        appearance: "none",
        border: 0,
        cursor: "pointer",
        width: dim,
        height: dim,
        borderRadius: 99,
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        background: "color-mix(in srgb, var(--surface) 64%, transparent)",
        backdropFilter: "blur(5px)",
        WebkitBackdropFilter: "blur(5px)",
        boxShadow: "0 1px 5px var(--shadow)",
        color: "var(--graphite)",
        transform: hover ? "scale(1.12)" : "none",
        transition: "transform .15s ease",
      }}
    >
      <HeartIcon size={size} fill={fav ? "var(--accent)" : "transparent"} stroke={fav ? "var(--accent)" : "currentColor"} />
    </button>
  );
}

function MobileThumb({
  piece,
  size,
  height,
  style,
  loading = "lazy",
}: {
  piece: PieceVM;
  size: number;
  height: number | string;
  style?: CSSProperties;
  loading?: "eager" | "lazy";
}) {
  const { openPiece } = useFolio();
  const matte: CSSProperties = {
    position: "absolute",
    inset: 0,
    flexDirection: "column",
    alignItems: "center",
    justifyContent: "center",
    gap: 5,
    padding: 14,
    textAlign: "center",
    background: "linear-gradient(155deg, var(--surface-2), var(--surface))",
  };
  return (
    <figure
      onClick={() => openPiece(piece.id)}
      style={{
        margin: 0,
        position: "relative",
        height,
        minWidth: 0,
        overflow: "hidden",
        borderRadius: 3,
        background: "var(--surface)",
        cursor: "pointer",
        ...style,
      }}
    >
      <OkfImage
        src={getPhotoThumbnailUrl(piece.id, size)}
        alt={piece.t}
        title={piece.t}
        artist={piece.a}
        loading={loading}
        imgStyle={{ position: "absolute", inset: 0, width: "100%", height: "100%", objectFit: "cover", zIndex: 1 }}
        matteStyle={matte}
        matteTitleStyle={{ fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 14, lineHeight: 1.2, color: "var(--ink)" }}
        matteArtistStyle={{ fontFamily: "var(--sans)", fontSize: 9.5, letterSpacing: "0.12em", textTransform: "uppercase", color: "var(--muted)" }}
      />
    </figure>
  );
}

function MobileMagazineHero({ piece }: { piece: PieceVM }) {
  const { openPiece } = useFolio();
  return (
    <figure
      onClick={() => openPiece(piece.id)}
      style={{
        margin: 0,
        position: "relative",
        height: 196,
        overflow: "hidden",
        borderRadius: 3,
        background: "var(--surface)",
        cursor: "pointer",
      }}
    >
      <OkfImage
        src={getPhotoThumbnailUrl(piece.id, 430)}
        alt={piece.t}
        title={piece.t}
        artist={piece.a}
        loading="eager"
        imgStyle={{ position: "absolute", inset: 0, width: "100%", height: "100%", objectFit: "cover", zIndex: 1 }}
        matteStyle={{
          position: "absolute",
          inset: 0,
          flexDirection: "column",
          alignItems: "center",
          justifyContent: "center",
          gap: 5,
          padding: 16,
          textAlign: "center",
          background: "linear-gradient(155deg, var(--surface-2), var(--surface))",
        }}
        matteTitleStyle={{ fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 17, color: "var(--ink)" }}
        matteArtistStyle={{ fontFamily: "var(--sans)", fontSize: 10, letterSpacing: "0.12em", textTransform: "uppercase", color: "var(--muted)" }}
      />
      <div style={{ position: "absolute", inset: "42% 0 0", zIndex: 2, background: "linear-gradient(to top, rgba(12,10,7,.78), rgba(12,10,7,0))" }} />
      <figcaption style={{ position: "absolute", left: 13, right: 13, bottom: 13, zIndex: 3, pointerEvents: "none" }}>
        <div style={{ fontFamily: "var(--serif)", fontWeight: 500, fontSize: 17, lineHeight: 1.08, color: "#FBF6EE" }}>{piece.t}</div>
        <div style={{ marginTop: 4, fontFamily: "var(--sans)", fontSize: 12, color: "rgba(251,246,238,.78)" }}>
          {piece.a}
          {piece.y ? ` · ${piece.y}` : ""}
        </div>
      </figcaption>
    </figure>
  );
}

function MobileMagazineView({ pieces }: { pieces: PieceVM[] }) {
  if (pieces.length === 0) return null;
  const [hero, ...rest] = pieces;
  return (
    <section style={{ display: "grid", gap: 12, padding: "0 0 4px" }}>
      <MobileMagazineHero piece={hero} />
      {rest.length ? (
        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 8 }}>
          {rest[0] ? <MobileThumb piece={rest[0]} size={320} height={246} /> : null}
          <div style={{ display: "grid", gap: 8 }}>
            {rest[1] ? <MobileThumb piece={rest[1]} size={220} height={119} /> : null}
            {rest[2] ? <MobileThumb piece={rest[2]} size={220} height={119} /> : null}
          </div>
        </div>
      ) : null}
      {rest[3] ? <MobileThumb piece={rest[3]} size={430} height={148} /> : null}
      {rest.slice(4).map((p, index) => (
        <MobileThumb key={p.id} piece={p} size={index % 3 === 0 ? 430 : 260} height={index % 3 === 0 ? 162 : 132} />
      ))}
    </section>
  );
}

/* ---- Magazine ---- */

function FeaturePiece({ piece }: { piece: PieceVM }) {
  const { openPiece, isFav, toggleFav } = useFolio();
  const [favHover, setFavHover] = useState(false);
  const fav = isFav(piece.id);
  const matte: CSSProperties = {
    flexDirection: "column",
    alignItems: "center",
    justifyContent: "center",
    gap: 8,
    aspectRatio: "16 / 10",
    width: "100%",
    padding: 24,
    textAlign: "center",
    background: "linear-gradient(155deg, var(--surface-2), var(--surface))",
  };
  return (
    <section
      style={{
        display: "grid",
        gridTemplateColumns: "1.72fr 1fr",
        gap: 46,
        alignItems: "center",
        padding: "46px 0 14px",
      }}
    >
      <figure
        onClick={() => openPiece(piece.id)}
        style={{
          margin: 0,
          position: "relative",
          cursor: "zoom-in",
          background: "var(--surface)",
          boxShadow: "0 3px 40px var(--shadow), 0 1px 3px var(--shadow)",
        }}
      >
        <OkfImage
          src={piece.img}
          alt={piece.t}
          title={piece.t}
          artist={piece.a}
          loading="eager"
          imgStyle={{ display: "block", width: "100%", height: "auto", maxHeight: "62vh", objectFit: "cover", position: "relative", zIndex: 1 }}
          matteStyle={matte}
          matteTitleStyle={{ fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 22, color: "var(--ink)" }}
          matteArtistStyle={{ fontFamily: "var(--sans)", fontSize: 12, letterSpacing: "0.14em", textTransform: "uppercase", color: "var(--muted)" }}
        />
      </figure>
      <div>
        <div style={{ fontFamily: "var(--sans)", fontSize: 11.5, letterSpacing: "0.22em", textTransform: "uppercase", color: "var(--accent)" }}>
          Featured
        </div>
        <h2 style={{ fontFamily: "var(--serif)", fontWeight: 300, fontSize: 37, lineHeight: 1.06, margin: "16px 0 0", color: "var(--ink)", letterSpacing: "-0.01em" }}>
          {piece.t}
        </h2>
        <div style={{ fontFamily: "var(--sans)", fontSize: 14.5, color: "var(--graphite)", marginTop: 12 }}>
          {piece.a}
          {piece.y ? ` · ${piece.y}` : ""}
        </div>
        {piece.note ? (
          <p style={{ fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 18.5, lineHeight: 1.5, color: "var(--graphite)", margin: "20px 0 0" }}>
            {piece.note}
          </p>
        ) : null}
        <div style={{ marginTop: 26, display: "flex", gap: 11, alignItems: "center" }}>
          <button
            onClick={() => openPiece(piece.id)}
            style={{ appearance: "none", cursor: "pointer", fontFamily: "var(--sans)", fontSize: 14, fontWeight: 500, padding: "11px 20px", borderRadius: 99, border: 0, background: "var(--accent)", color: "var(--on-accent)" }}
          >
            View piece
          </button>
          <button
            aria-label="Favorite"
            onClick={() => toggleFav(piece.id)}
            onMouseEnter={() => setFavHover(true)}
            onMouseLeave={() => setFavHover(false)}
            style={{ appearance: "none", cursor: "pointer", width: 42, height: 42, borderRadius: 99, border: `1px solid ${favHover ? "var(--accent-line)" : "var(--line-2)"}`, background: "var(--surface)", display: "flex", alignItems: "center", justifyContent: "center", color: "var(--graphite)" }}
          >
            <HeartIcon size={18} fill={fav ? "var(--accent)" : "transparent"} stroke={fav ? "var(--accent)" : "currentColor"} />
          </button>
        </div>
      </div>
    </section>
  );
}

function MagazineCard({ piece }: { piece: PieceVM }) {
  const { openPiece } = useFolio();
  const matte: CSSProperties = {
    flexDirection: "column",
    alignItems: "center",
    justifyContent: "center",
    gap: 7,
    aspectRatio: "4 / 5",
    width: "100%",
    padding: 22,
    textAlign: "center",
    background: "linear-gradient(155deg, var(--surface-2), var(--surface))",
    borderBottom: "1px solid var(--line)",
  };
  return (
    <figure
      onClick={() => openPiece(piece.id)}
      style={{
        breakInside: "avoid",
        margin: "0 0 26px",
        position: "relative",
        cursor: "zoom-in",
        background: "var(--surface)",
        boxShadow: "0 1px 14px var(--shadow)",
      }}
    >
      <Heart id={piece.id} top={11} right={11} dim={31} size={15} />
      <OkfImage
        src={getPhotoThumbnailUrl(piece.id, 700)}
        alt={piece.t}
        title={piece.t}
        artist={piece.a}
        imgStyle={{ display: "block", width: "100%", height: "auto", position: "relative", zIndex: 1 }}
        matteStyle={matte}
        matteTitleStyle={{ fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 18, lineHeight: 1.25, color: "var(--ink)" }}
        matteArtistStyle={{ fontFamily: "var(--sans)", fontSize: 11, letterSpacing: "0.13em", textTransform: "uppercase", color: "var(--muted)" }}
      />
      <figcaption style={{ padding: "13px 15px 15px" }}>
        <div style={{ fontFamily: "var(--serif)", fontSize: 16.5, lineHeight: 1.2, color: "var(--ink)" }}>{piece.t}</div>
        <div style={{ fontFamily: "var(--sans)", fontSize: 12.5, color: "var(--muted)", marginTop: 4 }}>
          {piece.a}
          {piece.y ? ` · ${piece.y}` : ""}
        </div>
      </figcaption>
    </figure>
  );
}

function MagazineView({ pieces }: { pieces: PieceVM[] }) {
  const { isMobile } = useViewport();
  if (pieces.length === 0) return null;
  if (isMobile) return <MobileMagazineView pieces={pieces} />;
  const featured = pieces.find((p) => p.fav) ?? pieces[0];
  const grid = pieces.filter((p) => p.id !== featured.id);
  return (
    <div>
      <FeaturePiece piece={featured} />
      <div style={{ display: "flex", alignItems: "center", gap: 18, padding: "40px 0 28px" }}>
        <span style={{ fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 20, color: "var(--ink)" }}>Recently gathered</span>
        <span style={{ flex: 1, height: 1, background: "var(--line)" }} />
      </div>
      <section style={{ columns: "3 290px", columnGap: 26 }}>
        {grid.map((p) => (
          <MagazineCard key={p.id} piece={p} />
        ))}
      </section>
    </div>
  );
}

/* ---- Library ---- */

function LibraryTile({ piece }: { piece: PieceVM }) {
  const { openPiece } = useFolio();
  const [hover, setHover] = useState(false);
  const matte: CSSProperties = {
    position: "absolute",
    inset: 0,
    flexDirection: "column",
    alignItems: "center",
    justifyContent: "center",
    gap: 5,
    padding: 16,
    textAlign: "center",
    background: "linear-gradient(155deg, var(--surface-2), var(--surface))",
  };
  return (
    <figure
      onClick={() => openPiece(piece.id)}
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
      style={{ margin: 0, position: "relative", aspectRatio: "1 / 1", cursor: "zoom-in", overflow: "hidden", background: "var(--surface)", boxShadow: "0 1px 8px var(--shadow)" }}
    >
      <OkfImage
        src={piece.thumb}
        alt={piece.t}
        title={piece.t}
        artist={piece.a}
        imgStyle={{ position: "absolute", inset: 0, width: "100%", height: "100%", objectFit: "cover", zIndex: 1 }}
        matteStyle={matte}
        matteTitleStyle={{ fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 14, lineHeight: 1.2, color: "var(--ink)" }}
        matteArtistStyle={{ fontFamily: "var(--sans)", fontSize: 9.5, letterSpacing: "0.12em", textTransform: "uppercase", color: "var(--muted)" }}
      />
      <Heart id={piece.id} top={9} right={9} dim={28} size={13} />
      <figcaption
        style={{
          position: "absolute",
          left: 0,
          right: 0,
          bottom: 0,
          zIndex: 3,
          padding: "26px 12px 11px",
          opacity: hover ? 1 : 0,
          transition: "opacity .2s ease",
          background: "linear-gradient(to top, rgba(12,10,7,0.78), rgba(12,10,7,0))",
        }}
      >
        <div style={{ fontFamily: "var(--serif)", fontSize: 13.5, lineHeight: 1.2, color: "#FBF6EE" }}>{piece.t}</div>
        <div style={{ fontFamily: "var(--sans)", fontSize: 10.5, color: "rgba(251,246,238,0.7)", marginTop: 2 }}>{piece.a}</div>
      </figcaption>
    </figure>
  );
}

function MobileLibraryTile({ piece }: { piece: PieceVM }) {
  const { openPiece } = useFolio();
  const label = [piece.a, piece.y].filter(Boolean).join(" · ");
  return (
    <div style={{ minWidth: 0 }}>
      <MobileThumb piece={piece} size={240} height={124} />
      <button
        type="button"
        onClick={() => openPiece(piece.id)}
        style={{
          appearance: "none",
          width: "100%",
          border: 0,
          background: "transparent",
          cursor: "pointer",
          textAlign: "left",
          padding: "7px 1px 1px",
        }}
      >
        <div style={{ fontFamily: "var(--serif)", fontWeight: 500, fontSize: 14.5, lineHeight: 1.1, color: "var(--ink)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{piece.t}</div>
        <div style={{ marginTop: 3, fontFamily: "var(--sans)", fontSize: 11.5, lineHeight: 1.2, color: "var(--muted)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{label || "Unknown"}</div>
      </button>
    </div>
  );
}

function LibraryView({ pieces, total }: { pieces: PieceVM[]; total: number }) {
  const { isMobile } = useViewport();
  if (isMobile) {
    return (
      <section style={{ display: "grid", gridTemplateColumns: "repeat(2, minmax(0, 1fr))", gap: "14px 12px", padding: "0 0 4px" }}>
        {pieces.map((p) => (
          <MobileLibraryTile key={p.id} piece={p} />
        ))}
      </section>
    );
  }
  return (
    <div>
      <div style={{ display: "flex", alignItems: "center", gap: 14, padding: isMobile ? "6px 0 14px" : "30px 0 22px", fontFamily: "var(--sans)", fontSize: 13, color: "var(--muted)" }}>
        <span>{total.toLocaleString()} pieces</span>
        <span style={{ opacity: 0.5 }}>·</span>
        <span>Newest first</span>
        <span style={{ flex: 1 }} />
        {isMobile ? null : <span style={{ color: "var(--faint)" }}>Hover a piece to preview · click to open</span>}
      </div>
      <section style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(166px, 1fr))", gap: 13 }}>
        {pieces.map((p) => (
          <LibraryTile key={p.id} piece={p} />
        ))}
      </section>
    </div>
  );
}

/* ---- Wall ---- */

function WallPiece({ piece }: { piece: PieceVM }) {
  const { openPiece } = useFolio();
  const matte: CSSProperties = {
    flexDirection: "column",
    alignItems: "center",
    justifyContent: "center",
    gap: 8,
    aspectRatio: "4 / 3",
    width: "100%",
    padding: 30,
    textAlign: "center",
    background: "linear-gradient(155deg, var(--surface-2), var(--surface))",
  };
  const meta = [piece.a, piece.y, piece.med].filter(Boolean).join(" · ");
  return (
    <figure onClick={() => openPiece(piece.id)} style={{ margin: 0, width: "min(720px, 86vw)", cursor: "zoom-in" }}>
      <div style={{ background: "var(--surface)", boxShadow: "0 40px 90px var(--shadow-2), 0 2px 8px var(--shadow)" }}>
        <OkfImage
          src={piece.img}
          alt={piece.t}
          title={piece.t}
          artist={piece.a}
          imgStyle={{ display: "block", width: "100%", height: "auto", position: "relative", zIndex: 1 }}
          matteStyle={matte}
          matteTitleStyle={{ fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 24, color: "var(--ink)" }}
          matteArtistStyle={{ fontFamily: "var(--sans)", fontSize: 11, letterSpacing: "0.16em", textTransform: "uppercase", color: "var(--muted)" }}
        />
      </div>
      <figcaption style={{ textAlign: "center", marginTop: 22 }}>
        <div style={{ fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 21, color: "var(--ink)" }}>{piece.t}</div>
        <div style={{ fontFamily: "var(--sans)", fontSize: 11, letterSpacing: "0.16em", textTransform: "uppercase", color: "var(--muted)", marginTop: 8 }}>{meta}</div>
      </figcaption>
    </figure>
  );
}

function WallView({ pieces }: { pieces: PieceVM[] }) {
  const { isMobile } = useViewport();
  if (isMobile) {
    return (
      <section
        style={{
          marginLeft: "calc(-20px - var(--safe-left))",
          marginRight: "calc(-20px - var(--safe-right))",
          display: "grid",
          gridTemplateColumns: "1fr 1fr 1fr",
          gridAutoRows: 84,
          gap: 3,
          padding: "0 0 4px",
          overflow: "hidden",
        }}
      >
        {pieces.map((p, index) => (
          <MobileThumb
            key={p.id}
            piece={p}
            size={180}
            height="100%"
            style={{
              borderRadius: 0,
              gridRow: index % 7 === 0 || index % 7 === 4 ? "span 2" : "span 1",
            }}
          />
        ))}
      </section>
    );
  }
  const wall = pieces.slice(0, 8);
  return (
    <div style={{ marginLeft: "calc(50% - 50vw)", marginRight: "calc(50% - 50vw)", width: "100vw", background: "var(--wall)", padding: "78px 0 96px", marginTop: 34 }}>
      <div style={{ textAlign: "center", marginBottom: 64 }}>
        <div style={{ fontFamily: "var(--sans)", fontSize: 11.5, letterSpacing: "0.24em", textTransform: "uppercase", color: "var(--muted)" }}>Wall · quiet viewing</div>
      </div>
      <div style={{ display: "flex", flexDirection: "column", gap: 104, alignItems: "center" }}>
        {wall.map((p) => (
          <WallPiece key={p.id} piece={p} />
        ))}
      </div>
    </div>
  );
}

/* ---- Infinite scroll ---- */

function LoadMoreSentinel() {
  const { loadMore, hasMore, loadingMore } = useFolio();
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = ref.current;
    if (!el || !hasMore) return;
    const obs = new IntersectionObserver(
      (entries) => {
        if (entries.some((e) => e.isIntersecting)) loadMore();
      },
      { rootMargin: "600px" },
    );
    obs.observe(el);
    return () => obs.disconnect();
  }, [hasMore, loadMore]);
  if (!hasMore && !loadingMore) return null;
  return (
    <div
      ref={ref}
      style={{ padding: "44px 0 12px", textAlign: "center", fontFamily: "var(--sans)", fontSize: 12.5, letterSpacing: "0.04em", color: "var(--faint)" }}
    >
      {loadingMore ? "Gathering more…" : ""}
    </div>
  );
}

function MobileLoadingState({ mode }: { mode: GalleryMode }) {
  const tiles = mode === "wall" ? Array.from({ length: 12 }) : mode === "library" ? Array.from({ length: 8 }) : Array.from({ length: 5 });
  return (
    <div style={{ position: "relative", paddingBottom: 32 }}>
      <section
        aria-hidden="true"
        style={
          mode === "wall"
            ? {
                marginLeft: "calc(-20px - var(--safe-left))",
                marginRight: "calc(-20px - var(--safe-right))",
                display: "grid",
                gridTemplateColumns: "repeat(3, 1fr)",
                gridAutoRows: 84,
                gap: 3,
              }
            : mode === "library"
              ? { display: "grid", gridTemplateColumns: "repeat(2, minmax(0, 1fr))", gap: "14px 12px" }
              : { display: "grid", gap: 12 }
        }
      >
        {tiles.map((_, index) => (
          <div
            key={index}
            className="okf-shimmer"
            style={{
              height: mode === "wall" ? "100%" : mode === "library" ? 124 : index === 0 ? 196 : 132,
              borderRadius: mode === "wall" ? 0 : 3,
              gridRow: mode === "wall" && (index % 7 === 0 || index % 7 === 4) ? "span 2" : undefined,
              background: "#E7E1D4",
            }}
          />
        ))}
      </section>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "center", gap: 10, padding: "24px 0 0", fontFamily: "var(--sans)", fontSize: 14, color: "var(--muted)" }}>
        <Spinner />
        Loading your folio...
      </div>
    </div>
  );
}

function MobileEmptyState() {
  const { openAdd } = useFolio();
  return (
    <div style={{ minHeight: "52vh", display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", textAlign: "center", padding: "34px 16px 64px", color: "var(--graphite)" }}>
      <div style={{ width: 66, height: 66, borderRadius: 18, border: "1px solid var(--line-2)", display: "flex", alignItems: "center", justifyContent: "center", background: "var(--surface)" }}>
        <BrandMark width={30} height={34} />
      </div>
      <h2 style={{ margin: "17px 0 0", fontFamily: "var(--serif)", fontSize: 22, fontWeight: 500, lineHeight: 1.05, color: "var(--ink)" }}>Nothing here yet</h2>
      <p style={{ margin: "9px 0 0", maxWidth: 260, fontFamily: "var(--sans)", fontSize: 14.5, lineHeight: 1.42, color: "var(--muted)" }}>
        Add a piece to start shaping your folio.
      </p>
      <button
        type="button"
        onClick={openAdd}
        style={{
          appearance: "none",
          marginTop: 21,
          minHeight: 48,
          border: 0,
          borderRadius: 13,
          background: "var(--accent)",
          color: "var(--on-accent)",
          cursor: "pointer",
          padding: "0 18px",
          fontFamily: "var(--sans)",
          fontSize: 14.5,
          fontWeight: 800,
        }}
      >
        Add your first piece
      </button>
    </div>
  );
}

function MobileFilteredEmptyState() {
  const { setQuery, setFavoriteOnly, setArtist, setCategory } = useFolio();
  const clear = () => {
    setQuery("");
    setFavoriteOnly(false);
    setArtist("");
    setCategory("");
  };
  return (
    <div style={{ minHeight: "48vh", display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", textAlign: "center", padding: "30px 16px 64px", color: "var(--graphite)" }}>
      <div style={{ width: 58, height: 58, borderRadius: 99, border: "1px solid var(--line-2)", display: "flex", alignItems: "center", justifyContent: "center", color: "var(--muted)", background: "var(--surface)" }}>
        <MagnifierMinusIcon />
      </div>
      <h2 style={{ margin: "16px 0 0", fontFamily: "var(--serif)", fontSize: 22, fontWeight: 500, lineHeight: 1.05, color: "var(--ink)" }}>No pieces match</h2>
      <p style={{ margin: "8px 0 0", fontFamily: "var(--sans)", fontSize: 14.5, color: "var(--muted)" }}>Try loosening one filter.</p>
      <button
        type="button"
        onClick={clear}
        style={{
          appearance: "none",
          marginTop: 20,
          minHeight: 46,
          border: "1px solid var(--line-2)",
          borderRadius: 13,
          background: "transparent",
          color: "var(--ink)",
          cursor: "pointer",
          padding: "0 18px",
          fontFamily: "var(--sans)",
          fontSize: 14,
          fontWeight: 800,
        }}
      >
        Clear filters
      </button>
    </div>
  );
}

/* ---- Screen ---- */

export default function Gallery() {
  const { pieces, total, totalPhotos, mode, isLoading, isError, query, favoriteOnly, artist, category } = useFolio();
  const { isMobile } = useViewport();
  const hasActiveFilters = !!query.trim() || favoriteOnly || !!artist || !!category;

  const subcopy = isLoading
    ? "Gathering your pieces…"
    : `${total.toLocaleString()} pieces, kept with intention.`;

  return (
    <div>
      {isMobile ? (
        <MobileGalleryHeader />
      ) : (
        <PageHeader eyebrow="Gallery" title="Your gathered pieces" subcopy={subcopy} action={<ModeTabs />} pad="54px 0 26px" />
      )}
      {isMobile ? null : <GalleryFilterBar />}
      {isError ? (
        <div style={{ padding: "90px 0", textAlign: "center", fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 22, color: "var(--graphite)" }}>
          The gallery could not be reached.
        </div>
      ) : isMobile && isLoading ? (
        <MobileLoadingState mode={mode} />
      ) : !isLoading && pieces.length === 0 ? (
        isMobile ? (
          hasActiveFilters || totalPhotos > 0 ? <MobileFilteredEmptyState /> : <MobileEmptyState />
        ) : (
          <div style={{ padding: "90px 0", textAlign: "center", color: "var(--muted)" }}>
            <div style={{ fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 24, color: "var(--graphite)" }}>Nothing here yet.</div>
            <div style={{ fontFamily: "var(--sans)", fontSize: 14, marginTop: 10 }}>Pieces will appear as your streams gather them.</div>
          </div>
        )
      ) : (
        <>
          {mode === "magazine" ? (
            <MagazineView pieces={pieces} />
          ) : mode === "wall" ? (
            <WallView pieces={pieces} />
          ) : (
            <LibraryView pieces={pieces} total={total} />
          )}
          {mode !== "wall" || isMobile ? <LoadMoreSentinel /> : null}
        </>
      )}
    </div>
  );
}
