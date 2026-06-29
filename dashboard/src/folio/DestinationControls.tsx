import { useState, type CSSProperties } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { backfillConnectorSource, createFolio, fetchFolios } from "../api";
import type { ConnectorSourceBackfillResult } from "../types";

type Props = {
  targetFolioId: number | null;
  showInLibrary: boolean;
  onTargetFolioIdChange: (id: number | null) => void;
  onShowInLibraryChange: (show: boolean) => void;
  disabled?: boolean;
  sourceId?: number;
  canBackfill?: boolean;
  backfillBlockedMessage?: string;
  compact?: boolean;
};

const fieldBase: CSSProperties = {
  width: "100%",
  minHeight: 44,
  border: "1px solid var(--line)",
  borderRadius: 6,
  background: "var(--surface-2)",
  color: "var(--ink)",
  padding: "10px 12px",
  fontFamily: "var(--sans)",
  fontSize: 14,
  outline: "none",
};

const labelStyle: CSSProperties = {
  display: "block",
  marginBottom: 7,
  fontFamily: "var(--sans)",
  fontSize: 12,
  fontWeight: 700,
  color: "var(--graphite)",
};

const buttonStyle: CSSProperties = {
  minHeight: 44,
  padding: "0 16px",
  border: "1px solid var(--line)",
  borderRadius: 99,
  background: "transparent",
  color: "var(--graphite)",
  fontFamily: "var(--sans)",
  fontSize: 13,
  fontWeight: 700,
  cursor: "pointer",
};

const BACKFILL_BATCH_SIZE = 1000;

function backfillSummary(result: ConnectorSourceBackfillResult): string {
  return `Added ${result.added_to_folio.toLocaleString()} existing pieces`;
}

function emptyBackfillResult(): ConnectorSourceBackfillResult {
  return {
    matched: 0,
    updated: 0,
    added_to_folio: 0,
    target_folio_id: null,
    show_in_library: true,
  };
}

function mergeBackfillResult(total: ConnectorSourceBackfillResult, next: ConnectorSourceBackfillResult): ConnectorSourceBackfillResult {
  return {
    matched: total.matched + next.matched,
    updated: total.updated + next.updated,
    added_to_folio: total.added_to_folio + next.added_to_folio,
    target_folio_id: next.target_folio_id ?? total.target_folio_id,
    show_in_library: next.show_in_library,
  };
}

export function destinationGateMessage(targetFolioId: number | null, showInLibrary: boolean): string {
  if (!showInLibrary && targetFolioId == null) {
    return "Select a target folio before hiding pieces from the gallery.";
  }
  return "";
}

export default function DestinationControls({
  targetFolioId,
  showInLibrary,
  onTargetFolioIdChange,
  onShowInLibraryChange,
  disabled = false,
  sourceId,
  canBackfill = false,
  backfillBlockedMessage,
  compact = false,
}: Props) {
  const queryClient = useQueryClient();
  const folios = useQuery({ queryKey: ["folios"], queryFn: fetchFolios });
  const [newFolioName, setNewFolioName] = useState("");
  const [createError, setCreateError] = useState("");
  const [creating, setCreating] = useState(false);
  const [backfillStatus, setBackfillStatus] = useState("");
  const [backfilling, setBackfilling] = useState(false);
  const folioList = folios.data?.folios ?? [];
  const selectedFolio = folioList.find((folio) => folio.id === targetFolioId);
  const gateMessage = destinationGateMessage(targetFolioId, showInLibrary);
  const backfillDisabled = disabled || backfilling || !sourceId || !canBackfill;

  const create = async () => {
    const name = newFolioName.trim();
    if (!name) return;
    setCreateError("");
    setCreating(true);
    try {
      const folio = await createFolio({ name });
      onTargetFolioIdChange(folio.id);
      setNewFolioName("");
      await queryClient.invalidateQueries({ queryKey: ["folios"] });
    } catch (err) {
      setCreateError(err instanceof Error ? err.message : "Failed to create folio");
    } finally {
      setCreating(false);
    }
  };

  const backfill = async () => {
    if (!sourceId || backfillDisabled) return;
    if (!window.confirm("Backfill existing pieces for this source into the saved destination?")) return;
    setBackfillStatus("Backfilling...");
    setBackfilling(true);
    try {
      let result = emptyBackfillResult();
      let batch: ConnectorSourceBackfillResult;
      do {
        batch = await backfillConnectorSource(sourceId);
        result = mergeBackfillResult(result, batch);
      } while (batch.matched >= BACKFILL_BATCH_SIZE);
      setBackfillStatus(backfillSummary(result));
      await queryClient.invalidateQueries({ queryKey: ["folios"] });
      await queryClient.invalidateQueries({ queryKey: ["connector-sources"] });
      await queryClient.invalidateQueries({ queryKey: ["folio-catalog"] });
      await queryClient.invalidateQueries({ queryKey: ["folio-pieces"] });
    } catch (err) {
      setBackfillStatus(err instanceof Error ? err.message : "Backfill failed");
    } finally {
      setBackfilling(false);
    }
  };

  return (
    <section style={{ display: "grid", gap: 12, padding: compact ? "12px 0" : 14, border: compact ? 0 : "1px solid var(--line)", borderRadius: compact ? 0 : 6, background: compact ? "transparent" : "var(--surface-2)" }}>
      <div>
        <div style={{ fontFamily: "var(--sans)", fontSize: 12, fontWeight: 800, letterSpacing: "0.12em", textTransform: "uppercase", color: "var(--faint)" }}>
          Destination
        </div>
        <div style={{ marginTop: 3, fontFamily: "var(--sans)", fontSize: 12.5, color: "var(--muted)", lineHeight: 1.45 }}>
          Route incoming pieces into a folio and choose whether they also appear in the main gallery.
        </div>
      </div>

      <label style={{ display: "block" }}>
        <span style={labelStyle}>Target Folio</span>
        <select
          value={targetFolioId ?? ""}
          onChange={(e) => onTargetFolioIdChange(e.target.value ? Number(e.target.value) : null)}
          disabled={disabled || folios.isLoading}
          style={fieldBase}
        >
          <option value="">No target folio</option>
          {folioList.map((folio) => (
            <option key={folio.id} value={folio.id}>
              {folio.name}
            </option>
          ))}
        </select>
      </label>

      <div style={{ display: "grid", gridTemplateColumns: compact ? "1fr" : "minmax(0, 1fr) auto", gap: 10, alignItems: "end" }}>
        <label style={{ display: "block" }}>
          <span style={labelStyle}>Create Folio</span>
          <input
            value={newFolioName}
            onChange={(e) => setNewFolioName(e.target.value)}
            placeholder="New folio name"
            disabled={disabled || creating}
            style={fieldBase}
          />
        </label>
        <button
          type="button"
          onClick={create}
          disabled={disabled || creating || newFolioName.trim() === ""}
          style={{ ...buttonStyle, opacity: disabled || creating || newFolioName.trim() === "" ? 0.55 : 1, cursor: disabled || creating || newFolioName.trim() === "" ? "not-allowed" : "pointer" }}
        >
          {creating ? "Creating..." : "Create"}
        </button>
      </div>
      {createError ? <div style={{ fontFamily: "var(--sans)", fontSize: 12.5, color: "var(--accent)" }}>{createError}</div> : null}

      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, minHeight: 44 }}>
        <div>
          <div style={{ fontFamily: "var(--sans)", fontSize: 13.5, color: "var(--ink)" }}>Show in Library</div>
          {!showInLibrary ? (
            <div style={{ marginTop: 3, fontFamily: "var(--sans)", fontSize: 12.5, color: gateMessage ? "var(--accent)" : "var(--muted)", lineHeight: 1.45 }}>
              {gateMessage || `Pieces appear only in ${selectedFolio?.name ?? "the selected folio"}, hidden from the gallery`}
            </div>
          ) : null}
        </div>
        <button
          type="button"
          role="switch"
          aria-checked={showInLibrary}
          aria-label="Show in Library"
          disabled={disabled}
          onClick={() => onShowInLibraryChange(!showInLibrary)}
          style={{
            flex: "none",
            width: 48,
            height: 29,
            padding: 3,
            border: 0,
            borderRadius: 99,
            background: showInLibrary ? "var(--accent)" : "var(--line-2)",
            display: "flex",
            justifyContent: showInLibrary ? "flex-end" : "flex-start",
            cursor: disabled ? "not-allowed" : "pointer",
            opacity: disabled ? 0.55 : 1,
          }}
        >
          <span style={{ width: 23, height: 23, borderRadius: 99, background: showInLibrary ? "var(--on-accent)" : "var(--surface)", boxShadow: "0 1px 4px var(--shadow)" }} />
        </button>
      </div>

      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, flexWrap: "wrap" }}>
        <button type="button" onClick={backfill} disabled={backfillDisabled} style={{ ...buttonStyle, opacity: backfillDisabled ? 0.55 : 1, cursor: backfillDisabled ? "not-allowed" : "pointer" }}>
          {backfilling ? "Backfilling..." : "Backfill existing"}
        </button>
        <div style={{ flex: 1, minWidth: 180, fontFamily: "var(--sans)", fontSize: 12.5, color: backfillStatus.includes("failed") || backfillStatus.includes("Failed") ? "var(--accent)" : "var(--muted)", lineHeight: 1.45 }}>
          {backfillStatus || (canBackfill ? "Uses the saved destination." : backfillBlockedMessage || "Save a target folio before backfill.")}
        </div>
      </div>
    </section>
  );
}
