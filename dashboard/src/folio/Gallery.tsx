import { useEffect, useMemo, useRef, useState, type CSSProperties, type ChangeEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import { fetchArtists, fetchGalleryCatalog, getPhotoThumbnailUrl } from "../api";
import { useFolio, type GalleryMode, type PieceVM } from "./context";
import { CloseIcon, HeartIcon, Hov, OkfImage, PageHeader } from "./ui";

const MODES: { key: GalleryMode; label: string }[] = [
  { key: "magazine", label: "Magazine" },
  { key: "library", label: "Library" },
  { key: "wall", label: "Wall" },
];

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
  const { query, favoriteOnly, setFavoriteOnly, artist, setArtist } = useFolio();
  const [draft, setDraft] = useState("");
  const [open, setOpen] = useState(false);
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
  if (pieces.length === 0) return null;
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

function LibraryView({ pieces, total }: { pieces: PieceVM[]; total: number }) {
  return (
    <div>
      <div style={{ display: "flex", alignItems: "center", gap: 14, padding: "30px 0 22px", fontFamily: "var(--sans)", fontSize: 13, color: "var(--muted)" }}>
        <span>{total.toLocaleString()} pieces</span>
        <span style={{ opacity: 0.5 }}>·</span>
        <span>Newest first</span>
        <span style={{ flex: 1 }} />
        <span style={{ color: "var(--faint)" }}>Hover a piece to preview · click to open</span>
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

/* ---- Screen ---- */

export default function Gallery() {
  const { pieces, total, mode, isLoading, isError } = useFolio();

  const subcopy = isLoading
    ? "Gathering your pieces…"
    : `${total.toLocaleString()} pieces, kept with intention.`;

  return (
    <div>
      <PageHeader eyebrow="Gallery" title="Your gathered pieces" subcopy={subcopy} action={<ModeTabs />} pad="54px 0 26px" />
      <GalleryFilterBar />
      {isError ? (
        <div style={{ padding: "90px 0", textAlign: "center", fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 22, color: "var(--graphite)" }}>
          The gallery could not be reached.
        </div>
      ) : !isLoading && pieces.length === 0 ? (
        <div style={{ padding: "90px 0", textAlign: "center", color: "var(--muted)" }}>
          <div style={{ fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 24, color: "var(--graphite)" }}>Nothing here yet.</div>
          <div style={{ fontFamily: "var(--sans)", fontSize: 14, marginTop: 10 }}>Pieces will appear as your streams gather them.</div>
        </div>
      ) : (
        <>
          {mode === "magazine" ? (
            <MagazineView pieces={pieces} />
          ) : mode === "wall" ? (
            <WallView pieces={pieces} />
          ) : (
            <LibraryView pieces={pieces} total={total} />
          )}
          {mode !== "wall" ? <LoadMoreSentinel /> : null}
        </>
      )}
    </div>
  );
}
