import { useState, type CSSProperties } from "react";
import { useFolio } from "./context";

const inputStyle: CSSProperties = {
  minHeight: 44,
  minWidth: 0,
  borderRadius: 8,
  border: "1px solid var(--line)",
  background: "var(--surface)",
  color: "var(--ink)",
  outline: "none",
  padding: "0 12px",
  fontFamily: "var(--sans)",
  fontSize: 13.5,
};

function splitKeywords(value: string): string[] {
  return value
    .split(",")
    .map((keyword) => keyword.trim())
    .filter(Boolean);
}

export default function BulkEditBar({
  selectedIds,
  onClear,
}: {
  selectedIds: number[];
  onClear: () => void;
}) {
  const { bulkEditPieces } = useFolio();
  const [artist, setArtist] = useState("");
  const [date, setDate] = useState("");
  const [addKeywords, setAddKeywords] = useState("");
  const [removeKeywords, setRemoveKeywords] = useState("");
  const [busy, setBusy] = useState(false);
  const count = selectedIds.length;
  const canApply = count > 0 && (artist.trim() || date.trim() || addKeywords.trim() || removeKeywords.trim());

  const apply = () => {
    if (!canApply || busy) return;
    setBusy(true);
    void bulkEditPieces({
      ids: selectedIds,
      ...(artist.trim() ? { set_artist: artist.trim() } : {}),
      ...(date.trim() ? { set_date: date.trim() } : {}),
      ...(addKeywords.trim() ? { add_keywords: splitKeywords(addKeywords) } : {}),
      ...(removeKeywords.trim() ? { remove_keywords: splitKeywords(removeKeywords) } : {}),
    }).then((ok) => {
      setBusy(false);
      if (!ok) return;
      setArtist("");
      setDate("");
      setAddKeywords("");
      setRemoveKeywords("");
      onClear();
    });
  };

  return (
    <div
      style={{
        position: "fixed",
        left: "calc(16px + var(--safe-left))",
        right: "calc(16px + var(--safe-right))",
        bottom: "calc(18px + var(--safe-bottom))",
        zIndex: 40,
        display: "flex",
        flexWrap: "wrap",
        gap: 8,
        alignItems: "center",
        padding: 10,
        borderRadius: 10,
        border: "1px solid var(--line)",
        background: "color-mix(in srgb, var(--surface) 94%, transparent)",
        boxShadow: "0 14px 42px var(--shadow-2)",
        backdropFilter: "blur(16px)",
        WebkitBackdropFilter: "blur(16px)",
      }}
    >
      <div style={{ padding: "0 8px", fontFamily: "var(--sans)", fontSize: 13, fontWeight: 800, color: "var(--ink)", whiteSpace: "nowrap" }}>
        {count} selected
      </div>
      <input value={artist} onChange={(event) => setArtist(event.target.value)} placeholder="Set artist" aria-label="Set artist" style={{ ...inputStyle, flex: "1 1 140px" }} />
      <input value={date} onChange={(event) => setDate(event.target.value)} placeholder="Set date" aria-label="Set date" type="date" style={{ ...inputStyle, flex: "1 1 136px" }} />
      <input value={addKeywords} onChange={(event) => setAddKeywords(event.target.value)} placeholder="Add keywords" aria-label="Add keywords" style={{ ...inputStyle, flex: "1 1 150px" }} />
      <input value={removeKeywords} onChange={(event) => setRemoveKeywords(event.target.value)} placeholder="Remove keywords" aria-label="Remove keywords" style={{ ...inputStyle, flex: "1 1 150px" }} />
      <button
        type="button"
        onClick={apply}
        disabled={!canApply || busy}
        style={{ minHeight: 44, borderRadius: 8, border: 0, background: "var(--accent)", color: "var(--on-accent)", opacity: !canApply || busy ? 0.55 : 1, padding: "0 16px", fontFamily: "var(--sans)", fontSize: 13.5, fontWeight: 800, cursor: !canApply || busy ? "not-allowed" : "pointer", whiteSpace: "nowrap" }}
      >
        {busy ? "Applying..." : "Apply"}
      </button>
      <button type="button" onClick={onClear} style={{ minHeight: 44, borderRadius: 8, border: "1px solid var(--line)", background: "transparent", color: "var(--ink)", padding: "0 14px", fontFamily: "var(--sans)", fontSize: 13.5, fontWeight: 800, cursor: "pointer" }}>
        Clear
      </button>
    </div>
  );
}
