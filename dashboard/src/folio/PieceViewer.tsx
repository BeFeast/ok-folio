import { useCallback, useEffect, useRef, useState, type CSSProperties } from "react";
import { useQuery } from "@tanstack/react-query";
import { fetchFolios } from "../api";
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
const META_KEY: CSSProperties = { fontFamily: "var(--sans)", fontSize: 11, color: "rgba(251,246,238,0.52)" };
const META_VAL: CSSProperties = { fontFamily: "var(--sans)", fontSize: 13, color: "rgba(251,246,238,0.78)", marginTop: 2 };
const MOBILE_CHROME: CSSProperties = {
  appearance: "none",
  border: "1px solid rgba(251,246,238,0.12)",
  background: "rgba(20,14,10,.4)",
  backdropFilter: "blur(14px)",
  WebkitBackdropFilter: "blur(14px)",
  color: "#FBF6EE",
  minWidth: 44,
  height: 44,
  borderRadius: 999,
  display: "flex",
  alignItems: "center",
  justifyContent: "center",
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

export default function PieceViewer() {
  const { selected, closePiece, stepPiece, isFav, toggleFav, selIndex, selCount, filterByArtist, addPieceToFolioAction } = useFolio();
  const { isMobile } = useViewport();
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

  // The info panel is hidden by default so the artwork shows unobstructed; the
  // edge handle (top-right, beside Close) toggles it open/closed on click, and
  // it resets to hidden on each new piece.
  const [panelOpen, setPanelOpen] = useState(false);
  const [chromeVisible, setChromeVisible] = useState(true);
  const [drag, setDrag] = useState({ x: 0, y: 0, active: false });
  const [folioPickerOpen, setFolioPickerOpen] = useState(false);
  const touchStartRef = useRef<{ x: number; y: number; target: "art" | "sheet" } | null>(null);
  const lastTouchDeltaRef = useRef({ x: 0, y: 0 });
  const suppressClickRef = useRef(false);
  const chromeTimerRef = useRef<number | null>(null);
  const folios = useQuery({ queryKey: ["folios"], queryFn: fetchFolios, enabled: !!selected && isMobile });

  const togglePanel = useCallback(() => setPanelOpen((open) => !open), []);
  const showChrome = useCallback(() => {
    setChromeVisible(true);
    if (chromeTimerRef.current != null) window.clearTimeout(chromeTimerRef.current);
    chromeTimerRef.current = window.setTimeout(() => setChromeVisible(false), 2500);
  }, []);

  // Keyed on the piece id (a primitive), NOT the `selected` object — its
  // identity churns on every context re-render.
  const pieceId = selected?.id ?? null;
  useEffect(() => {
    setPanelOpen(false);
    setFolioPickerOpen(false);
    setChromeVisible(true);
    setDrag({ x: 0, y: 0, active: false });
  }, [pieceId]);

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
  }, [closePiece, showChrome, stepPiece]);

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
  }, []);

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

  if (isMobile) {
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
            left: 16,
            right: 16,
            top: "calc(env(safe-area-inset-top) + 12px)",
            zIndex: 8,
            display: "grid",
            gridTemplateColumns: "44px minmax(0, 1fr) 44px",
            alignItems: "center",
            gap: 12,
            opacity: chromeVisible && !panelOpen ? 1 : 0,
            transform: chromeVisible && !panelOpen ? "translateY(0)" : "translateY(-8px)",
            transition: reducedMotion ? "opacity .16s ease" : "opacity .22s ease, transform .22s ease",
            pointerEvents: chromeVisible && !panelOpen ? "auto" : "none",
          }}
        >
          <button
            onClick={(e) => {
              stop(e);
              closePiece();
            }}
            aria-label="Close"
            style={{ ...MOBILE_CHROME, cursor: "pointer" }}
          >
            <CloseIcon />
          </button>
          <div
            style={{
              justifySelf: "center",
              minHeight: 36,
              padding: "0 14px",
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
            }}
          >
            {selIndex >= 0 ? `${selIndex + 1} / ${selCount}` : ""}
          </div>
          <button
            onClick={(e) => {
              stop(e);
              toggleFav(p.id);
              showChrome();
            }}
            aria-label={fav ? "Remove favorite" : "Favorite"}
            style={{ ...MOBILE_CHROME, cursor: "pointer" }}
          >
            <HeartIcon size={20} fill={fav ? "#C75D49" : "transparent"} stroke={fav ? "#C75D49" : "#FBF6EE"} strokeWidth={1.7} />
          </button>
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
            <h2 style={{ margin: 0, fontFamily: "var(--serif)", fontWeight: 400, fontSize: 30, lineHeight: 1.02, color: "#ECE6DA" }}>{p.t}</h2>
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

            {p.keywords.length ? (
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

          {folioPickerOpen ? (
            <div
              role="menu"
              aria-label="Choose folio"
              onClick={(e) => e.stopPropagation()}
              style={{
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
              }}
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
          ) : null}

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

      <button
        onClick={(e) => {
          stop(e);
          closePiece();
        }}
        aria-label="Close"
        style={{
          position: "absolute",
          top: 22,
          right: 24,
          zIndex: 8,
          appearance: "none",
          cursor: "pointer",
          width: 42,
          height: 42,
          borderRadius: 99,
          border: 0,
          background: "rgba(251,246,238,0.12)",
          backdropFilter: "blur(6px)",
          WebkitBackdropFilter: "blur(6px)",
          color: "#FBF6EE",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
        }}
      >
        <CloseIcon />
      </button>

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

      <button
        onClick={(e) => {
          stop(e);
          togglePanel();
        }}
        aria-label={panelOpen ? "Hide details" : "Show details"}
        title={panelOpen ? "Hide details" : "Show details"}
        style={{
          position: "absolute",
          top: 22,
          right: 76,
          zIndex: 8,
          appearance: "none",
          cursor: "pointer",
          width: 42,
          height: 42,
          borderRadius: 99,
          border: 0,
          background: "rgba(251,246,238,0.12)",
          backdropFilter: "blur(6px)",
          WebkitBackdropFilter: "blur(6px)",
          color: "#FBF6EE",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
        }}
      >
        <ChevronIcon dir={panelOpen ? "right" : "left"} />
      </button>

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
          <button
            onClick={() => toggleFav(p.id)}
            aria-label="Favorite"
            style={{
              flex: "none",
              appearance: "none",
              cursor: "pointer",
              width: 40,
              height: 40,
              borderRadius: 99,
              border: "1px solid rgba(251,246,238,0.22)",
              background: "rgba(251,246,238,0.06)",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              color: "#FBF6EE",
            }}
          >
            <HeartIcon size={19} fill={fav ? "#DC8A70" : "transparent"} stroke={fav ? "#DC8A70" : "#FBF6EE"} strokeWidth={1.6} />
          </button>
        </div>
        <h2 style={{ fontFamily: "var(--serif)", fontWeight: 300, fontSize: 31, lineHeight: 1.08, margin: "12px 0 0", color: "#FBF6EE", letterSpacing: "-0.01em" }}>
          {p.t}
        </h2>
        <div style={{ fontFamily: "var(--sans)", fontSize: 14.5, color: "rgba(251,246,238,0.74)", marginTop: 8 }}>{p.a}</div>
        {p.note ? (
          <div style={{ fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 17, lineHeight: 1.5, color: "rgba(251,246,238,0.8)", margin: "18px 0 0" }}>
            {p.note}
          </div>
        ) : null}

        <div style={{ height: 1, background: "rgba(251,246,238,0.12)", margin: "24px 0 18px" }} />
        <div style={{ ...LABEL, marginBottom: 6 }}>Museum label</div>
        {p.keywords.length ? (
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
