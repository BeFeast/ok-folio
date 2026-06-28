import { useCallback, useEffect, useState, type CSSProperties } from "react";
import { useFolio } from "./context";
import { ChevronIcon, CloseIcon, HeartIcon, OkfImage } from "./ui";

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

function stop(e: React.MouseEvent) {
  e.stopPropagation();
}

export default function PieceViewer() {
  const { selected, closePiece, stepPiece, isFav, toggleFav, selIndex, selCount, filterByArtist } = useFolio();

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

  const togglePanel = useCallback(() => setPanelOpen((open) => !open), []);

  // Keyed on the piece id (a primitive), NOT the `selected` object — its
  // identity churns on every context re-render.
  const pieceId = selected?.id ?? null;
  useEffect(() => {
    setPanelOpen(false);
  }, [pieceId]);

  if (!selected) return null;
  const p = selected;
  const fav = isFav(p.id);
  const eyebrow = [p.kind || p.src, p.y].filter(Boolean).join(" · ");
  const canFilterArtist = p.a !== "Unknown";

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
          <div style={ROW_KEY}>Date</div>
          <div style={ROW_VAL}>{p.y || "—"}</div>
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
              <div style={META_VAL}>{p.added}</div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
