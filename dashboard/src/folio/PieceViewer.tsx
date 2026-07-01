import { useCallback, useEffect, useRef, useState, type CSSProperties } from "react";
import { useQuery } from "@tanstack/react-query";
import { fetchFolios, type PieceMetadataPatch } from "../api";
import { useFolio } from "./context";
import { ChevronIcon, CloseIcon, HeartIcon, OkfImage } from "./ui";
import { useViewport } from "./useViewport";

const LABEL: CSSProperties = {
  fontFamily: "var(--sans)",
  fontSize: 10.5,
  letterSpacing: "0.18em",
  textTransform: "uppercase",
  color: "rgba(251,246,238,0.42)",
};
const ROW: CSSProperties = {
  display: "flex",
  gap: 16,
  padding: "11px 0",
  borderBottom: "1px solid rgba(251,246,238,0.12)",
};
const ROW_KEY: CSSProperties = {
  flex: "none",
  width: 86,
  fontFamily: "var(--sans)",
  fontSize: 12,
  color: "rgba(251,246,238,0.52)",
  paddingTop: 3,
};
const ROW_VAL: CSSProperties = {
  flex: 1,
  fontFamily: "var(--sans)",
  fontSize: 14,
  color: "#FBF6EE",
};
const KEYWORD_CHIP: CSSProperties = {
  display: "inline-flex",
  alignItems: "center",
  minHeight: 24,
  padding: "3px 9px",
  borderRadius: 999,
  border: "1px solid rgba(251,246,238,0.16)",
  background: "rgba(251,246,238,0.07)",
  fontFamily: "var(--sans)",
  fontSize: 12,
  color: "rgba(251,246,238,0.78)",
};
const EDIT_INPUT: CSSProperties = {
  width: "100%",
  minHeight: 44,
  boxSizing: "border-box",
  borderRadius: 8,
  border: "1px solid rgba(251,246,238,0.18)",
  background: "rgba(251,246,238,0.08)",
  color: "#FBF6EE",
  outline: "none",
  padding: "0 12px",
  fontFamily: "var(--sans)",
  fontSize: 14,
};
const EDIT_LABEL: CSSProperties = {
  display: "flex",
  alignItems: "center",
  justifyContent: "space-between",
  gap: 10,
  marginBottom: 6,
  fontFamily: "var(--sans)",
  fontSize: 11,
  color: "rgba(251,246,238,0.56)",
};
const EDITED_MARK: CSSProperties = {
  display: "inline-flex",
  alignItems: "center",
  minHeight: 22,
  borderRadius: 99,
  border: "1px solid rgba(220,138,112,0.34)",
  background: "rgba(220,138,112,0.12)",
  color: "#DC8A70",
  padding: "0 8px",
  fontFamily: "var(--sans)",
  fontSize: 10.5,
  fontWeight: 800,
};
const META_KEY: CSSProperties = { fontFamily: "var(--sans)", fontSize: 11, color: "rgba(251,246,238,0.52)" };
const META_VAL: CSSProperties = { fontFamily: "var(--sans)", fontSize: 13, color: "rgba(251,246,238,0.78)", marginTop: 2 };
const VIEWER_CHROME_SIZE = 42;
const MOBILE_CHROME_SIZE = 44;
const VIEWER_CHROME_BUTTON: CSSProperties = {
  appearance: "none",
  border: "1px solid rgba(251,246,238,0.12)",
  background: "rgba(20,14,10,.4)",
  backdropFilter: "blur(14px)",
  WebkitBackdropFilter: "blur(14px)",
  color: "#FBF6EE",
  width: VIEWER_CHROME_SIZE,
  height: VIEWER_CHROME_SIZE,
  borderRadius: 999,
  display: "flex",
  alignItems: "center",
  justifyContent: "center",
  padding: 0,
  flex: "none",
};
const MOBILE_CHROME: CSSProperties = { ...VIEWER_CHROME_BUTTON, width: MOBILE_CHROME_SIZE, height: MOBILE_CHROME_SIZE, minWidth: MOBILE_CHROME_SIZE };
const VIEWER_CHROME_ACTIVE: CSSProperties = {
  borderColor: "rgba(220,138,112,0.5)",
  background: "rgba(220,138,112,0.16)",
  color: "#FBF6EE",
};
const MOBILE_SHEET_ROW: CSSProperties = {
  display: "grid",
  gridTemplateColumns: "92px minmax(0, 1fr)",
  gap: 14,
  padding: "12px 0",
  borderBottom: "1px solid rgba(236,230,218,.1)",
};

function useReducedMotion(): boolean {
  const [reduced, setReduced] = useState(false);
  useEffect(() => {
    if (typeof window === "undefined" || typeof window.matchMedia !== "function") return;
    const query = window.matchMedia("(prefers-reduced-motion: reduce)");
    const update = () => setReduced(query.matches);
    update();
    if (typeof query.addEventListener === "function") {
      query.addEventListener("change", update);
      return () => query.removeEventListener("change", update);
    }
    query.addListener(update);
    return () => query.removeListener(update);
  }, []);
  return reduced;
}

function stop(e: React.MouseEvent) {
  e.stopPropagation();
}

function UpChevronIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M5 15 L12 8 L19 15" />
    </svg>
  );
}

function ShareIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 16 V4" />
      <path d="M8 8 L12 4 L16 8" />
      <path d="M5 13 V19 H19 V13" />
    </svg>
  );
}

function AddToFolioIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M4.8 7.2 H10.2 L12 9 H19.2 V18.6 H4.8 Z" />
      <path d="M12 12.4 V16.2" />
      <path d="M10.1 14.3 H13.9" />
    </svg>
  );
}

function InfoIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="8.2" />
      <path d="M12 10.8 V16" />
      <path d="M12 8 H12.01" />
    </svg>
  );
}

function PencilIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M4.5 19.5 L8.8 18.6 L18.6 8.8 C19.4 8 19.4 6.8 18.6 6 L18 5.4 C17.2 4.6 16 4.6 15.2 5.4 L5.4 15.2 L4.5 19.5 Z" />
      <path d="M13.8 6.8 L17.2 10.2" />
    </svg>
  );
}

interface EditDraft {
  title: string;
  artist: string;
  date: string;
  keywords: string[];
}

function normalizeDraftKeywords(keywords: string[]): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const raw of keywords) {
    const value = raw.trim();
    const key = value.toLowerCase();
    if (!value || seen.has(key)) continue;
    seen.add(key);
    out.push(value);
  }
  return out;
}

function sameKeywords(a: string[], b: string[]): boolean {
  if (a.length !== b.length) return false;
  return a.every((keyword, index) => keyword === b[index]);
}

function editedMarker(fields: string[], field: string) {
  return fields.includes(field) ? <span style={EDITED_MARK}>Edited</span> : null;
}

export default function PieceViewer() {
  const {
    selected,
    closePiece,
    stepPiece,
    isFav,
    toggleFav,
    selIndex,
    selCount,
    filterByArtist,
    addPieceToFolioAction,
    editPieceMetadata,
    infoPanelMode,
    infoPanelRememberedOpen,
    setInfoPanelRememberedOpen,
  } = useFolio();
  const { isMobile, width: viewportWidth } = useViewport();
  const reducedMotion = useReducedMotion();

  useEffect(() => {
    if (!selected) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") closePiece();
      else if (e.key === "ArrowLeft") stepPiece(-1);
      else if (e.key === "ArrowRight") stepPiece(1);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [selected, closePiece, stepPiece]);

  const [transientPanelOpen, setTransientPanelOpen] = useState(false);
  const [mobilePinnedOpen, setMobilePinnedOpen] = useState(true);
  const [chromeVisible, setChromeVisible] = useState(true);
  const [drag, setDrag] = useState({ x: 0, y: 0, active: false });
  const [folioPickerOpen, setFolioPickerOpen] = useState(false);
  const [editing, setEditing] = useState(false);
  const [savingEdit, setSavingEdit] = useState(false);
  const [keywordDraft, setKeywordDraft] = useState("");
  const [editDraft, setEditDraft] = useState<EditDraft>({ title: "", artist: "", date: "", keywords: [] });
  const touchStartRef = useRef<{ x: number; y: number; target: "art" | "sheet" } | null>(null);
  const lastTouchDeltaRef = useRef({ x: 0, y: 0 });
  const suppressClickRef = useRef(false);
  const chromeTimerRef = useRef<number | null>(null);
  const folios = useQuery({ queryKey: ["folios"], queryFn: fetchFolios, enabled: !!selected });

  const pinnedDesktop = infoPanelMode === "pinned" && !isMobile;
  const panelOpen = pinnedDesktop
    ? true
    : infoPanelMode === "remember"
      ? infoPanelRememberedOpen
      : infoPanelMode === "pinned"
        ? mobilePinnedOpen
        : transientPanelOpen;
  const setPanelOpen = useCallback((open: boolean) => {
    if (pinnedDesktop) return;
    if (infoPanelMode === "remember") {
      setInfoPanelRememberedOpen(open);
    } else if (infoPanelMode === "pinned") {
      setMobilePinnedOpen(open);
    } else {
      setTransientPanelOpen(open);
    }
  }, [infoPanelMode, pinnedDesktop, setInfoPanelRememberedOpen]);
  const togglePanel = useCallback(() => setPanelOpen(!panelOpen), [panelOpen, setPanelOpen]);
  const showChrome = useCallback(() => {
    setChromeVisible(true);
    if (chromeTimerRef.current != null) window.clearTimeout(chromeTimerRef.current);
    chromeTimerRef.current = window.setTimeout(() => setChromeVisible(false), 2500);
  }, []);

  // Keyed on the piece id (a primitive), NOT the `selected` object — its
  // identity churns on every context re-render.
  const pieceId = selected?.id ?? null;
  useEffect(() => {
    if (infoPanelMode === "hidden") setTransientPanelOpen(false);
    setFolioPickerOpen(false);
    setEditing(false);
    setSavingEdit(false);
    setKeywordDraft("");
    setChromeVisible(true);
    setDrag({ x: 0, y: 0, active: false });
  }, [infoPanelMode, pieceId]);

  useEffect(() => {
    if (infoPanelMode === "pinned" && isMobile) setMobilePinnedOpen(true);
    if (infoPanelMode === "hidden") setTransientPanelOpen(false);
  }, [infoPanelMode, isMobile]);

  useEffect(() => {
    if (!selected || editing) return;
    setEditDraft({ title: selected.t, artist: selected.a === "Unknown" ? "" : selected.a, date: selected.editDate, keywords: selected.keywords });
  }, [editing, selected]);

  useEffect(() => {
    if (!panelOpen) setFolioPickerOpen(false);
  }, [panelOpen]);

  useEffect(() => {
    if (!selected || !isMobile) return;
    showChrome();
    return () => {
      if (chromeTimerRef.current != null) window.clearTimeout(chromeTimerRef.current);
    };
  }, [selected, isMobile, showChrome]);

  const handleShare = useCallback(() => {
    if (!selected) return;
    const pieceUrl = new URL(`/pieces/${selected.id}`, window.location.origin).toString();
    const shareData = { title: selected.t, text: selected.a, url: pieceUrl };
    if (navigator.share) {
      void navigator.share(shareData).catch(() => undefined);
      return;
    }
    void navigator.clipboard?.writeText(pieceUrl).catch(() => undefined);
  }, [selected]);

  const handleAddToFolio = useCallback((folioId: number, photoId: number) => {
    addPieceToFolioAction(folioId, photoId);
    setFolioPickerOpen(false);
  }, [addPieceToFolioAction]);

  const startEditing = useCallback(() => {
    if (!selected || editing) return;
    setEditDraft({ title: selected.t, artist: selected.a === "Unknown" ? "" : selected.a, date: selected.editDate, keywords: selected.keywords });
    setKeywordDraft("");
    setEditing(true);
  }, [editing, selected]);

  const addKeyword = useCallback(() => {
    const value = keywordDraft.trim();
    if (!value) return;
    setEditDraft((draft) => {
      if (draft.keywords.some((keyword) => keyword.toLowerCase() === value.toLowerCase())) return draft;
      return { ...draft, keywords: [...draft.keywords, value] };
    });
    setKeywordDraft("");
  }, [keywordDraft]);

  const saveEdit = useCallback(() => {
    if (!selected || savingEdit) return;
    const nextTitle = editDraft.title.trim();
    const nextArtist = editDraft.artist.trim();
    const currentArtist = selected.a === "Unknown" ? "" : selected.a.trim();
    const nextDate = editDraft.date.trim();
    const nextKeywords = normalizeDraftKeywords(editDraft.keywords);
    const currentKeywords = normalizeDraftKeywords(selected.keywords);
    const patch: PieceMetadataPatch = {};
    if (nextTitle !== selected.t.trim()) patch.title = nextTitle;
    if (nextArtist !== currentArtist) patch.artist = nextArtist;
    if (nextDate !== selected.editDate) patch.date = nextDate || null;
    if (!sameKeywords(nextKeywords, currentKeywords)) patch.keywords = nextKeywords;
    if (Object.keys(patch).length === 0) {
      setEditing(false);
      return;
    }
    setSavingEdit(true);
    void editPieceMetadata(selected.id, patch).then((ok) => {
      setSavingEdit(false);
      if (ok) setEditing(false);
    });
  }, [editDraft, editPieceMetadata, savingEdit, selected]);

  const onArtworkTouchStart = useCallback((e: React.TouchEvent) => {
    const touch = e.touches[0];
    if (!touch) return;
    touchStartRef.current = { x: touch.clientX, y: touch.clientY, target: "art" };
    lastTouchDeltaRef.current = { x: 0, y: 0 };
    suppressClickRef.current = false;
    showChrome();
  }, [showChrome]);

  const onArtworkTouchMove = useCallback((e: React.TouchEvent) => {
    const start = touchStartRef.current;
    const touch = e.touches[0];
    if (!start || start.target !== "art" || !touch) return;
    const dx = touch.clientX - start.x;
    const dy = touch.clientY - start.y;
    if (Math.abs(dx) < 8 && Math.abs(dy) < 8) return;
    lastTouchDeltaRef.current = { x: dx, y: dy };
    suppressClickRef.current = true;
    const horizontal = Math.abs(dx) > Math.abs(dy);
    if (reducedMotion) {
      setDrag({ x: 0, y: 0, active: true });
      return;
    }
    setDrag({
      x: horizontal ? Math.max(-92, Math.min(92, dx * 0.36)) : 0,
      y: !horizontal && dy > 0 ? Math.min(132, dy * 0.52) : !horizontal && dy < 0 ? Math.max(-48, dy * 0.28) : 0,
      active: true,
    });
  }, [reducedMotion]);

  const onArtworkTouchEnd = useCallback(() => {
    const start = touchStartRef.current;
    touchStartRef.current = null;
    if (!start || start.target !== "art") return;
    const { x: dx, y: dy } = lastTouchDeltaRef.current;
    setDrag({ x: 0, y: 0, active: false });
    if (Math.abs(dx) > 72 && Math.abs(dx) > Math.abs(dy)) {
      stepPiece(dx < 0 ? 1 : -1);
    } else if (dy > 86) {
      closePiece();
    } else if (dy < -64) {
      setPanelOpen(true);
      showChrome();
    }
  }, [closePiece, setPanelOpen, showChrome, stepPiece]);

  const onArtworkClick = useCallback((e: React.MouseEvent) => {
    stop(e);
    if (suppressClickRef.current) {
      suppressClickRef.current = false;
      return;
    }
    setChromeVisible((visible) => {
      const next = !visible;
      if (next) showChrome();
      return next;
    });
  }, [showChrome]);

  const onSheetTouchStart = useCallback((e: React.TouchEvent) => {
    const touch = e.touches[0];
    if (!touch) return;
    touchStartRef.current = { x: touch.clientX, y: touch.clientY, target: "sheet" };
  }, []);

  const onSheetTouchEnd = useCallback((e: React.TouchEvent) => {
    const start = touchStartRef.current;
    const touch = e.changedTouches[0];
    touchStartRef.current = null;
    if (!start || start.target !== "sheet" || !touch) return;
    if (touch.clientY - start.y > 58) setPanelOpen(false);
  }, [setPanelOpen]);

  if (!selected) return null;
  const p = selected;
  const fav = isFav(p.id);
  const eyebrow = [p.kind || p.src, p.y].filter(Boolean).join(" · ");
  const canFilterArtist = p.a !== "Unknown";
  const artistDate = [p.a, p.y].filter(Boolean).join(" · ");
  const fileDetail = [p.dim, p.size].filter((value) => value && value !== "—").join(" · ") || "—";
  const artworkTransform = reducedMotion
    ? "none"
    : `translate3d(${drag.x}px, ${drag.y}px, 0) scale(${drag.y > 0 ? Math.max(0.92, 1 - drag.y / 900) : 1})`;
  const artworkOpacity = reducedMotion && drag.active ? 0.72 : 1;
  const editedFields = p.editedFields;
  const renderReadTitle = () => (
    <>
      <h2 style={{ margin: 0, fontFamily: "var(--serif)", fontWeight: 400, fontSize: isMobile ? 30 : 31, lineHeight: 1.02, color: "#ECE6DA" }}>{p.t || "Untitled"}</h2>
      <button
        type="button"
        disabled={!canFilterArtist}
        onClick={() => filterByArtist(p.a)}
        style={{
          appearance: "none",
          border: 0,
          background: "transparent",
          padding: "9px 0 0",
          color: "#C75D49",
          fontFamily: "var(--sans)",
          fontSize: 15,
          fontWeight: 600,
          textAlign: "left",
          cursor: canFilterArtist ? "pointer" : "default",
        }}
      >
        {p.a}
      </button>
    </>
  );
  const renderEditForm = () => (
    <div style={{ display: "grid", gap: 13, marginTop: 4 }}>
      {[
        ["Title", "title", "title"],
        ["Artist", "artist", "artist"],
        ["Date", "date", "date"],
      ].map(([label, key, type]) => (
        <label key={key} style={{ display: "grid" }}>
          <span style={EDIT_LABEL}>
            {label}
            {editedMarker(editedFields, key)}
          </span>
          <input
            type={type}
            value={key === "title" ? editDraft.title : key === "artist" ? editDraft.artist : editDraft.date}
            onChange={(event) => {
              const value = event.target.value;
              setEditDraft((draft) => ({ ...draft, [key]: value }));
            }}
            style={EDIT_INPUT}
          />
        </label>
      ))}
      <div>
        <div style={EDIT_LABEL}>
          Keywords
          {editedMarker(editedFields, "keywords")}
        </div>
        <div style={{ display: "flex", flexWrap: "wrap", gap: 8, marginBottom: 8 }}>
          {editDraft.keywords.map((keyword) => (
            <button
              key={keyword}
              type="button"
              onClick={() => setEditDraft((draft) => ({ ...draft, keywords: draft.keywords.filter((item) => item !== keyword) }))}
              style={{ ...KEYWORD_CHIP, minHeight: 32, cursor: "pointer", color: "#FBF6EE" }}
              aria-label={`Remove ${keyword}`}
            >
              {keyword}
              <span aria-hidden="true" style={{ marginLeft: 7, fontSize: 15 }}>×</span>
            </button>
          ))}
        </div>
        <input
          value={keywordDraft}
          onChange={(event) => setKeywordDraft(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === "Enter") {
              event.preventDefault();
              addKeyword();
            }
          }}
          onBlur={addKeyword}
          placeholder="Type keyword and press Enter"
          style={EDIT_INPUT}
        />
      </div>
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 10, marginTop: 4 }}>
        <button
          type="button"
          onClick={() => {
            setEditing(false);
            setKeywordDraft("");
          }}
          disabled={savingEdit}
          style={{ minHeight: 46, borderRadius: 12, border: "1px solid rgba(251,246,238,0.18)", background: "transparent", color: "#FBF6EE", fontFamily: "var(--sans)", fontSize: 14, fontWeight: 800, cursor: savingEdit ? "wait" : "pointer" }}
        >
          Cancel
        </button>
        <button
          type="button"
          onClick={saveEdit}
          disabled={savingEdit}
          style={{ minHeight: 46, borderRadius: 12, border: 0, background: "#C75D49", color: "#16130E", fontFamily: "var(--sans)", fontSize: 14, fontWeight: 800, cursor: savingEdit ? "wait" : "pointer", opacity: savingEdit ? 0.7 : 1 }}
        >
          {savingEdit ? "Saving..." : "Save"}
        </button>
      </div>
    </div>
  );
  const renderEditButton = () => (
    <button
      type="button"
      onClick={startEditing}
      aria-label="Edit metadata"
      title="Edit"
      style={{ ...VIEWER_CHROME_BUTTON, cursor: "pointer", background: "rgba(251,246,238,0.06)" }}
    >
      <PencilIcon />
    </button>
  );
  const renderFolioPicker = (style: CSSProperties) => (
    <div
      role="menu"
      aria-label="Choose folio"
      onClick={(e) => e.stopPropagation()}
      style={style}
    >
      {folios.isLoading ? (
        <div style={{ minHeight: 48, display: "flex", alignItems: "center", padding: "0 12px", fontFamily: "var(--sans)", fontSize: 13, color: "rgba(236,230,218,.62)" }}>
          Loading folios...
        </div>
      ) : folios.isError ? (
        <div style={{ minHeight: 48, display: "flex", alignItems: "center", padding: "0 12px", fontFamily: "var(--sans)", fontSize: 13, color: "rgba(236,230,218,.62)" }}>
          Folios unavailable
        </div>
      ) : (folios.data?.folios ?? []).length === 0 ? (
        <div style={{ minHeight: 48, display: "flex", alignItems: "center", padding: "0 12px", fontFamily: "var(--sans)", fontSize: 13, color: "rgba(236,230,218,.62)" }}>
          No folios yet
        </div>
      ) : (
        (folios.data?.folios ?? []).map((folio) => (
          <button
            key={folio.id}
            type="button"
            role="menuitem"
            onClick={() => handleAddToFolio(folio.id, p.id)}
            style={{
              appearance: "none",
              width: "100%",
              minHeight: 48,
              border: 0,
              borderRadius: 11,
              background: "transparent",
              color: "#ECE6DA",
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              gap: 12,
              padding: "0 12px",
              fontFamily: "var(--sans)",
              fontSize: 14,
              textAlign: "left",
              cursor: "pointer",
            }}
          >
            <span style={{ minWidth: 0, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{folio.name}</span>
            <span style={{ flex: "none", fontSize: 12, color: "rgba(236,230,218,.48)" }}>
              {folio.piece_count.toLocaleString()}
            </span>
          </button>
        ))
      )}
    </div>
  );

  if (isMobile) {
    const compactMobileChrome = viewportWidth > 0 && viewportWidth <= 360;
    const mobileChromeInset = compactMobileChrome ? 12 : 16;
    const mobileChromeGap = compactMobileChrome ? 6 : 8;
    const mobileChromeButton: CSSProperties = compactMobileChrome
      ? { ...MOBILE_CHROME, width: 40, height: 40, minWidth: 40 }
      : MOBILE_CHROME;

    return (
      <div
        style={{
          position: "fixed",
          inset: 0,
          zIndex: 100,
          background: "#0c0a08",
          overflow: "hidden",
          touchAction: panelOpen ? "pan-y" : "none",
          color: "#ECE6DA",
        }}
      >
        <div
          onClick={onArtworkClick}
          onTouchStart={onArtworkTouchStart}
          onTouchMove={onArtworkTouchMove}
          onTouchEnd={onArtworkTouchEnd}
          style={{
            position: "absolute",
            inset: 0,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            padding: "env(safe-area-inset-top) 0 env(safe-area-inset-bottom)",
            transform: artworkTransform,
            opacity: artworkOpacity,
            transition: drag.active || reducedMotion ? "opacity .16s ease" : "transform .24s ease, opacity .2s ease",
            willChange: "transform, opacity",
          }}
        >
          <OkfImage
            src={p.img}
            alt={p.t}
            title={p.t}
            artist={p.a}
            loading="eager"
            imgStyle={{
              display: "block",
              width: "100vw",
              height: "100vh",
              objectFit: "contain",
              userSelect: "none",
              WebkitUserSelect: "none",
            }}
            matteStyle={{
              flexDirection: "column",
              alignItems: "center",
              justifyContent: "center",
              gap: 10,
              width: "100vw",
              height: "100vh",
              padding: 28,
              textAlign: "center",
              background: "linear-gradient(155deg, #232019, #15110d)",
            }}
            matteTitleStyle={{ fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 28, color: "#FBF6EE" }}
            matteArtistStyle={{ fontFamily: "var(--sans)", fontSize: 12, letterSpacing: "0.14em", textTransform: "uppercase", color: "rgba(251,246,238,0.58)" }}
          />
        </div>

        <div
          style={{
            position: "absolute",
            left: mobileChromeInset,
            right: mobileChromeInset,
            top: "calc(env(safe-area-inset-top) + 12px)",
            zIndex: 12,
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            gap: 8,
            minWidth: 0,
            opacity: chromeVisible ? 1 : 0,
            transform: chromeVisible ? "translateY(0)" : "translateY(-8px)",
            transition: reducedMotion ? "opacity .16s ease" : "opacity .22s ease, transform .22s ease",
            pointerEvents: chromeVisible ? "auto" : "none",
          }}
        >
          <div
            style={{
              minHeight: 36,
              padding: "0 12px",
              borderRadius: 999,
              background: "rgba(20,14,10,.4)",
              backdropFilter: "blur(14px)",
              WebkitBackdropFilter: "blur(14px)",
              border: "1px solid rgba(251,246,238,0.12)",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              fontFamily: "var(--sans)",
              fontSize: 12,
              fontWeight: 600,
              color: "rgba(251,246,238,.86)",
              flex: "1 1 auto",
              minWidth: 0,
              maxWidth: compactMobileChrome ? 92 : 120,
              whiteSpace: "nowrap",
              overflow: "hidden",
              textOverflow: "ellipsis",
            }}
          >
            {selIndex >= 0 ? `${selIndex + 1} / ${selCount}` : ""}
          </div>
          <div style={{ display: "flex", alignItems: "center", gap: mobileChromeGap, flex: "0 0 auto" }}>
            <button
              type="button"
              onClick={(e) => {
                stop(e);
                togglePanel();
                showChrome();
              }}
              aria-label={panelOpen ? "Hide info" : "Show info"}
              aria-pressed={panelOpen}
              style={{ ...mobileChromeButton, ...(panelOpen ? VIEWER_CHROME_ACTIVE : null), cursor: "pointer" }}
            >
              <InfoIcon />
            </button>
            <button
              type="button"
              onClick={(e) => {
                stop(e);
                setPanelOpen(true);
                startEditing();
                showChrome();
              }}
              aria-label="Edit metadata"
              style={{ ...mobileChromeButton, ...(editing ? VIEWER_CHROME_ACTIVE : null), cursor: "pointer" }}
            >
              <PencilIcon />
            </button>
            <button
              onClick={(e) => {
                stop(e);
                toggleFav(p.id);
                showChrome();
              }}
              aria-label={fav ? "Remove favorite" : "Favorite"}
              style={{ ...mobileChromeButton, cursor: "pointer" }}
            >
              <HeartIcon size={20} fill={fav ? "#DC8A70" : "transparent"} stroke={fav ? "#DC8A70" : "#FBF6EE"} strokeWidth={1.7} />
            </button>
            <button
              onClick={(e) => {
                stop(e);
                closePiece();
              }}
              aria-label="Close"
              style={{ ...mobileChromeButton, cursor: "pointer" }}
            >
              <CloseIcon />
            </button>
          </div>
        </div>

        <button
          type="button"
          onClick={(e) => {
            stop(e);
            setPanelOpen(true);
            showChrome();
          }}
          aria-label="Show details"
          style={{
            position: "absolute",
            left: 0,
            right: 0,
            bottom: 0,
            zIndex: 7,
            appearance: "none",
            border: 0,
            cursor: "pointer",
            minHeight: 154,
            padding: "42px 20px calc(env(safe-area-inset-bottom) + 18px)",
            textAlign: "left",
            background: "linear-gradient(to top, rgba(12,10,8,.88), rgba(12,10,8,.5) 54%, rgba(12,10,8,0))",
            color: "#FBF6EE",
            opacity: chromeVisible && !panelOpen ? 1 : 0,
            transition: reducedMotion ? "opacity .16s ease" : "opacity .24s ease",
            pointerEvents: chromeVisible && !panelOpen ? "auto" : "none",
          }}
        >
          <span style={{ display: "block", fontFamily: "var(--serif)", fontSize: 28, lineHeight: 1.02, fontWeight: 400 }}>{p.t}</span>
          <span style={{ display: "block", marginTop: 5, fontFamily: "var(--sans)", fontSize: 13.5, color: "rgba(251,246,238,.72)" }}>{artistDate || p.src}</span>
          <span style={{ display: "flex", alignItems: "center", gap: 5, marginTop: 14, fontFamily: "var(--sans)", fontSize: 12, color: "rgba(251,246,238,.58)" }}>
            Swipe up for details <UpChevronIcon />
          </span>
        </button>

        {panelOpen ? (
          <div
            onClick={(e) => {
              stop(e);
              setPanelOpen(false);
            }}
            style={{
              position: "absolute",
              inset: 0,
              zIndex: 9,
              background: "rgba(20,14,10,.5)",
              opacity: 1,
              transition: "opacity .18s ease",
            }}
          />
        ) : null}

        <section
          role="dialog"
          aria-modal="true"
          aria-label="Piece details"
          onClick={(e) => e.stopPropagation()}
          onTouchStart={onSheetTouchStart}
          onTouchEnd={onSheetTouchEnd}
          style={{
            position: "absolute",
            left: 0,
            right: 0,
            bottom: 0,
            zIndex: 10,
            maxHeight: "min(82vh, 720px)",
            display: "flex",
            flexDirection: "column",
            borderRadius: "24px 24px 0 0",
            background: "#161310",
            boxShadow: "0 -18px 40px rgba(0,0,0,.25)",
            transform: panelOpen ? "translateY(0)" : "translateY(104%)",
            opacity: panelOpen || !reducedMotion ? 1 : 0,
            transition: reducedMotion ? "opacity .18s ease" : "transform .28s cubic-bezier(.2,.8,.2,1)",
            pointerEvents: panelOpen ? "auto" : "none",
            overflow: "hidden",
            touchAction: "pan-y",
          }}
        >
          <div style={{ flex: 1, overflowY: "auto", padding: "10px 22px 112px" }}>
            <div style={{ width: 46, height: 5, borderRadius: 99, background: "rgba(236,230,218,.28)", margin: "0 auto 20px" }} />
            <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 14 }}>
              <div style={{ minWidth: 0, flex: 1 }}>{editing ? renderEditForm() : renderReadTitle()}</div>
              {!editing ? renderEditButton() : null}
            </div>

            <div style={{ marginTop: 20 }}>
              {[
                ["Medium", p.med || "—"],
                ["Source", p.src || "—"],
                ["Dimensions", p.dim || "—"],
                ["File", `${p.file}${fileDetail !== "—" ? ` · ${fileDetail}` : ""}`],
                ...(p.captured ? [["Captured", p.captured]] : []),
                ...(p.camera ? [["Camera", p.camera]] : []),
                ...(p.lens ? [["Lens", p.lens]] : []),
              ].map(([key, value]) => (
                <div key={key} style={MOBILE_SHEET_ROW}>
                  <div style={{ fontFamily: "var(--sans)", fontSize: 12, color: "rgba(236,230,218,.5)" }}>{key}</div>
                  <div style={{ minWidth: 0, overflowWrap: "anywhere", fontFamily: "var(--sans)", fontSize: 14, color: "#ECE6DA" }}>{value}</div>
                </div>
              ))}
            </div>

            {!editing && p.keywords.length ? (
              <div style={{ marginTop: 22 }}>
                <div style={{ ...LABEL, marginBottom: 10, color: "rgba(236,230,218,.46)" }}>Keywords</div>
                <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
                  {p.keywords.map((keyword) => (
                    <span key={keyword} style={{ ...KEYWORD_CHIP, minHeight: 30, color: "rgba(236,230,218,.82)", background: "rgba(236,230,218,.07)" }}>
                      {keyword}
                    </span>
                  ))}
                </div>
              </div>
            ) : null}
          </div>

          {folioPickerOpen
            ? renderFolioPicker({
                position: "absolute",
                left: 18,
                right: 18,
                bottom: "calc(env(safe-area-inset-bottom) + 86px)",
                zIndex: 2,
                maxHeight: "min(44vh, 320px)",
                overflowY: "auto",
                padding: 8,
                borderRadius: 16,
                border: "1px solid rgba(236,230,218,.14)",
                background: "rgba(22,19,16,.96)",
                boxShadow: "0 20px 70px rgba(0,0,0,.42)",
                backdropFilter: "blur(14px)",
                WebkitBackdropFilter: "blur(14px)",
              })
            : null}

          <div
            style={{
              position: "absolute",
              left: 0,
              right: 0,
              bottom: 0,
              padding: "12px 18px calc(env(safe-area-inset-bottom) + 14px)",
              display: "grid",
              gridTemplateColumns: "1fr 48px 48px",
              gap: 10,
              background: "linear-gradient(to top, #161310 76%, rgba(22,19,16,0))",
            }}
          >
            <button
              type="button"
              aria-expanded={folioPickerOpen}
              onClick={(e) => {
                e.stopPropagation();
                setFolioPickerOpen((open) => !open);
              }}
              style={{
                appearance: "none",
                border: 0,
                borderRadius: 13,
                minHeight: 52,
                background: "#C75D49",
                color: "#16130E",
                fontFamily: "var(--sans)",
                fontSize: 14.5,
                fontWeight: 700,
                cursor: "pointer",
              }}
            >
              Add to folio
            </button>
            <button type="button" onClick={handleShare} aria-label="Share" style={{ ...MOBILE_CHROME, height: 52, borderRadius: 13 }}>
              <ShareIcon />
            </button>
            <button
              type="button"
              onClick={() => toggleFav(p.id)}
              aria-label={fav ? "Remove favorite" : "Favorite"}
              style={{ ...MOBILE_CHROME, height: 52, borderRadius: 13 }}
            >
              <HeartIcon size={20} fill={fav ? "#C75D49" : "transparent"} stroke={fav ? "#C75D49" : "#FBF6EE"} strokeWidth={1.7} />
            </button>
          </div>
        </section>
      </div>
    );
  }

  return (
    <div
      onClick={closePiece}
      style={{
        position: "fixed",
        inset: 0,
        zIndex: 100,
        background: "rgba(9,7,5,0.94)",
        backdropFilter: "blur(10px)",
        WebkitBackdropFilter: "blur(10px)",
        overflow: "hidden",
      }}
    >
      <div style={{ position: "absolute", inset: 0, display: "flex", alignItems: "center", justifyContent: "center" }}>
        <OkfImage
          src={p.img}
          alt={p.t}
          title={p.t}
          artist={p.a}
          onClick={stop}
          imgStyle={{ display: "block", width: "100%", height: "100%", objectFit: "contain", animation: "okf-fade .3s ease" }}
          matteStyle={{
            flexDirection: "column",
            alignItems: "center",
            justifyContent: "center",
            gap: 12,
            width: "min(440px,86vw)",
            height: "min(560px,80vh)",
            padding: 40,
            textAlign: "center",
            background: "linear-gradient(155deg, #232019, #1A1712)",
          }}
          matteTitleStyle={{ fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 26, color: "#FBF6EE" }}
          matteArtistStyle={{ fontFamily: "var(--sans)", fontSize: 12, letterSpacing: "0.16em", textTransform: "uppercase", color: "rgba(251,246,238,0.55)" }}
        />
      </div>

      <div
        onClick={stop}
        style={{
          position: "absolute",
          top: 22,
          right: 24,
          zIndex: 8,
          display: "flex",
          alignItems: "center",
          gap: 8,
        }}
      >
        <button
          type="button"
          onClick={() => setFolioPickerOpen((open) => !open)}
          aria-label="Add to folio"
          aria-expanded={folioPickerOpen}
          title="Add to folio"
          style={{
            ...VIEWER_CHROME_BUTTON,
            ...(folioPickerOpen ? VIEWER_CHROME_ACTIVE : null),
            cursor: "pointer",
          }}
        >
          <AddToFolioIcon />
        </button>
        <button
          type="button"
          onClick={handleShare}
          aria-label="Share or copy link"
          title="Share or copy link"
          style={{ ...VIEWER_CHROME_BUTTON, cursor: "pointer" }}
        >
          <ShareIcon />
        </button>
        <button
          type="button"
          onClick={togglePanel}
          aria-label={panelOpen ? "Hide info" : "Show info"}
          aria-pressed={panelOpen}
          title={pinnedDesktop ? "Info panel pinned in Settings" : panelOpen ? "Hide info" : "Show info"}
          style={{
            ...VIEWER_CHROME_BUTTON,
            ...(panelOpen ? VIEWER_CHROME_ACTIVE : null),
            cursor: pinnedDesktop ? "default" : "pointer",
          }}
        >
          <InfoIcon />
        </button>
        <button
          type="button"
          onClick={() => {
            setPanelOpen(true);
            startEditing();
          }}
          aria-label="Edit metadata"
          title="Edit"
          style={{
            ...VIEWER_CHROME_BUTTON,
            ...(editing ? VIEWER_CHROME_ACTIVE : null),
            cursor: "pointer",
          }}
        >
          <PencilIcon />
        </button>
        <button
          type="button"
          onClick={() => toggleFav(p.id)}
          aria-label={fav ? "Remove favorite" : "Favorite"}
          title={fav ? "Remove favorite" : "Favorite"}
          style={{ ...VIEWER_CHROME_BUTTON, cursor: "pointer" }}
        >
          <HeartIcon size={19} fill={fav ? "#DC8A70" : "transparent"} stroke={fav ? "#DC8A70" : "#FBF6EE"} strokeWidth={1.6} />
        </button>
        <button
          type="button"
          onClick={closePiece}
          aria-label="Close"
          title="Close"
          style={{ ...VIEWER_CHROME_BUTTON, cursor: "pointer" }}
        >
          <CloseIcon />
        </button>
        {folioPickerOpen
          ? renderFolioPicker({
              position: "absolute",
              top: 52,
              right: 0,
              width: 270,
              maxHeight: "min(50vh, 340px)",
              overflowY: "auto",
              padding: 8,
              borderRadius: 16,
              border: "1px solid rgba(236,230,218,.14)",
              background: "rgba(22,19,16,.94)",
              boxShadow: "0 20px 70px rgba(0,0,0,.42)",
              backdropFilter: "blur(14px)",
              WebkitBackdropFilter: "blur(14px)",
            })
          : null}
      </div>

      <button
        onClick={(e) => {
          stop(e);
          stepPiece(-1);
        }}
        aria-label="Previous"
        style={{
          position: "absolute",
          left: 22,
          top: "50%",
          transform: "translateY(-50%)",
          zIndex: 6,
          appearance: "none",
          cursor: "pointer",
          width: 48,
          height: 48,
          borderRadius: 99,
          border: 0,
          background: "rgba(251,246,238,0.1)",
          backdropFilter: "blur(6px)",
          WebkitBackdropFilter: "blur(6px)",
          color: "#FBF6EE",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
        }}
      >
        <ChevronIcon dir="left" />
      </button>
      <button
        onClick={(e) => {
          stop(e);
          stepPiece(1);
        }}
        aria-label="Next"
        style={{
          position: "absolute",
          right: panelOpen ? "calc(min(416px, 92vw) + 18px)" : 24,
          top: "50%",
          transform: "translateY(-50%)",
          transition: "right .35s ease",
          zIndex: 6,
          appearance: "none",
          cursor: "pointer",
          width: 48,
          height: 48,
          borderRadius: 99,
          border: 0,
          background: "rgba(251,246,238,0.1)",
          backdropFilter: "blur(6px)",
          WebkitBackdropFilter: "blur(6px)",
          color: "#FBF6EE",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
        }}
      >
        <ChevronIcon dir="right" />
      </button>

      <div
        style={{
          position: "absolute",
          left: 26,
          bottom: 22,
          zIndex: 6,
          fontFamily: "var(--sans)",
          fontSize: 12,
          letterSpacing: "0.1em",
          color: "rgba(251,246,238,0.66)",
          pointerEvents: "none",
        }}
      >
        {selIndex >= 0 ? `${selIndex + 1} / ${selCount}` : ""}
      </div>

      <div
        onClick={stop}
        style={{
          position: "absolute",
          top: 0,
          right: 0,
          bottom: 0,
          zIndex: 7,
          width: "min(416px, 92vw)",
          padding: "70px 34px 40px",
          overflowY: "auto",
          background: "rgba(16,12,9,0.72)",
          backdropFilter: "blur(24px)",
          WebkitBackdropFilter: "blur(24px)",
          borderLeft: "1px solid rgba(251,246,238,0.12)",
          boxShadow: "-30px 0 80px rgba(0,0,0,0.4)",
          opacity: panelOpen ? 1 : 0,
          transform: panelOpen ? "translateX(0)" : "translateX(100%)",
          transition: "opacity .35s ease, transform .35s ease",
          pointerEvents: panelOpen ? "auto" : "none",
        }}
      >
        <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 16, paddingRight: 44 }}>
          <div style={{ fontFamily: "var(--sans)", fontSize: 11, letterSpacing: "0.18em", textTransform: "uppercase", color: "#DC8A70", paddingTop: 6 }}>
            {eyebrow}
          </div>
          <div style={{ display: "flex", gap: 8, alignItems: "center", flex: "none" }}>
            {!editing ? renderEditButton() : null}
            <button
              onClick={() => toggleFav(p.id)}
              aria-label={fav ? "Remove favorite" : "Favorite"}
              style={{
                ...VIEWER_CHROME_BUTTON,
                cursor: "pointer",
                background: "rgba(251,246,238,0.06)",
              }}
            >
              <HeartIcon size={19} fill={fav ? "#DC8A70" : "transparent"} stroke={fav ? "#DC8A70" : "#FBF6EE"} strokeWidth={1.6} />
            </button>
          </div>
        </div>
        {editing ? (
          <div style={{ marginTop: 18 }}>{renderEditForm()}</div>
        ) : (
          <>
            <h2 style={{ fontFamily: "var(--serif)", fontWeight: 300, fontSize: 31, lineHeight: 1.08, margin: "12px 0 0", color: "#FBF6EE", letterSpacing: "-0.01em" }}>
              {p.t || "Untitled"}
            </h2>
            <div style={{ display: "flex", alignItems: "center", gap: 8, fontFamily: "var(--sans)", fontSize: 14.5, color: "rgba(251,246,238,0.74)", marginTop: 8 }}>
              {p.a}
              {editedMarker(editedFields, "artist")}
            </div>
          </>
        )}
        {p.note ? (
          <div style={{ fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 17, lineHeight: 1.5, color: "rgba(251,246,238,0.8)", margin: "18px 0 0" }}>
            {p.note}
          </div>
        ) : null}

        <div style={{ height: 1, background: "rgba(251,246,238,0.12)", margin: "24px 0 18px" }} />
        <div style={{ ...LABEL, marginBottom: 6 }}>Museum label</div>
        {!editing && p.keywords.length ? (
          <div style={{ display: "flex", flexWrap: "wrap", gap: 7, margin: "0 0 10px" }}>
            {p.keywords.map((keyword) => (
              <span key={keyword} style={KEYWORD_CHIP}>
                {keyword}
              </span>
            ))}
          </div>
        ) : null}

        <div style={ROW}>
          <div style={ROW_KEY}>Source</div>
          <div style={ROW_VAL}>{p.src}</div>
        </div>
        <div style={ROW}>
          <div style={ROW_KEY}>Artist</div>
          <button
            type="button"
            disabled={!canFilterArtist}
            onClick={() => filterByArtist(p.a)}
            style={{
              ...ROW_VAL,
              appearance: "none",
              cursor: canFilterArtist ? "pointer" : "default",
              border: 0,
              background: "transparent",
              padding: 0,
              textAlign: "left",
              textDecoration: canFilterArtist ? "underline" : "none",
              textDecorationColor: "rgba(251,246,238,0.32)",
              textUnderlineOffset: 3,
            }}
          >
            {p.a}
          </button>
        </div>
        <div style={ROW}>
          <div style={ROW_KEY}>Date</div>
          <div style={ROW_VAL}>
            {p.editDate || "—"} {editedMarker(editedFields, "date")}
          </div>
        </div>
        <div style={ROW}>
          <div style={ROW_KEY}>Medium</div>
          <div style={{ ...ROW_VAL, color: "rgba(251,246,238,0.5)" }}>{p.med || "—"}</div>
        </div>

        <div style={{ marginTop: 24, paddingTop: 20, borderTop: "1px solid rgba(251,246,238,0.12)" }}>
          <div style={{ ...LABEL, marginBottom: 12 }}>File &amp; metadata</div>
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "12px 22px" }}>
            <div>
              <div style={META_KEY}>Dimensions</div>
              <div style={META_VAL}>{p.dim || "—"}</div>
            </div>
            <div>
              <div style={META_KEY}>File</div>
              <div style={META_VAL}>{p.file}</div>
            </div>
            <div>
              <div style={META_KEY}>Size</div>
              <div style={META_VAL}>{p.size}</div>
            </div>
            <div>
              <div style={META_KEY}>Added</div>
              <div
                style={p.addedExact ? { ...META_VAL, cursor: "help" } : META_VAL}
                title={p.addedExact || undefined}
              >
                {p.added}
              </div>
            </div>
            <div>
              <div style={META_KEY}>Captured</div>
              <div style={META_VAL}>{p.captured || "—"}</div>
            </div>
            <div>
              <div style={META_KEY}>Camera</div>
              <div style={META_VAL}>{p.camera || "—"}</div>
            </div>
            {p.lens ? (
              <div style={{ gridColumn: "1 / -1" }}>
                <div style={META_KEY}>Lens</div>
                <div style={META_VAL}>{p.lens}</div>
              </div>
            ) : null}
          </div>
        </div>
      </div>
    </div>
  );
}
