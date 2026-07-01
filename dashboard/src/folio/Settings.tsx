import { useEffect, useState, type CSSProperties, type ReactNode } from "react";
import { createConnectorSource, deleteConnectorSource, fetchConnectorSources, fetchConnectorStatus, updateConnectorSource } from "../api";
import type { ConnectorSourceSetting, ConnectorSourceStatus } from "../types";
import { useStorageEstimate } from "../hooks/useStorageEstimate";
import { useFolio, formatBytes } from "./context";
import DestinationControls, { destinationGateMessage } from "./DestinationControls";
import { BrandMark, Hov, PageHeader } from "./ui";
import { useViewport } from "./useViewport";

function useLocalPref<T extends string | boolean>(key: string, initial: T): [T, (v: T) => void] {
  const [value, setValue] = useState<T>(() => {
    try {
      const raw = localStorage.getItem(key);
      if (raw != null) return JSON.parse(raw) as T;
    } catch {
      /* ignore */
    }
    return initial;
  });
  const set = (v: T) => {
    setValue(v);
    try {
      localStorage.setItem(key, JSON.stringify(v));
    } catch {
      /* ignore */
    }
  };
  return [value, set];
}

const SECTION: CSSProperties = {
  fontFamily: "var(--sans)",
  fontWeight: 600,
  fontSize: 11.5,
  letterSpacing: "0.18em",
  textTransform: "uppercase",
  color: "var(--faint)",
  margin: "44px 0 4px",
};
const ROW: CSSProperties = {
  display: "flex",
  alignItems: "center",
  justifyContent: "space-between",
  gap: 24,
  padding: "20px 0",
  borderBottom: "1px solid var(--line)",
};
const ROW_TITLE: CSSProperties = { fontFamily: "var(--sans)", fontSize: 15, color: "var(--ink)" };
const ROW_DESC: CSSProperties = { fontFamily: "var(--sans)", fontSize: 12.5, color: "var(--muted)", marginTop: 3 };

function Switch({ on, onClick }: { on: boolean; onClick: () => void }) {
  return (
    <div
      onClick={onClick}
      role="switch"
      aria-checked={on}
      style={{
        width: 44,
        height: 25,
        borderRadius: 99,
        background: on ? "var(--accent)" : "var(--line-2)",
        position: "relative",
        cursor: "pointer",
        flex: "none",
        transition: "background .15s ease",
      }}
    >
      <span
        style={{
          position: "absolute",
          top: 3,
          left: on ? 22 : 3,
          width: 19,
          height: 19,
          borderRadius: 99,
          background: on ? "var(--on-accent)" : "var(--surface-2)",
          boxShadow: on ? undefined : "0 1px 3px var(--shadow)",
          transition: "left .15s ease",
        }}
      />
    </div>
  );
}

function Row({ title, desc, children }: { title: string; desc: string; children: React.ReactNode }) {
  return (
    <div style={ROW}>
      <div>
        <div style={ROW_TITLE}>{title}</div>
        <div style={ROW_DESC}>{desc}</div>
      </div>
      {children}
    </div>
  );
}

export default function Settings() {
  const { theme, setTheme, mode, setMode, infoPanelMode, setInfoPanelMode, totalPhotos, totalSizeBytes } = useFolio();
  const { isMobile } = useViewport();
  const currentHost = typeof window !== "undefined" ? window.location.host : "Current address";

  const [reduceMotion, setReduceMotion] = useLocalPref<boolean>("okfolio-reduce-motion", false);
  const [autoCovers, setAutoCovers] = useLocalPref<boolean>("okfolio-auto-covers", true);
  const [suggestFolios, setSuggestFolios] = useLocalPref<boolean>("okfolio-suggest-folios", true);
  const [syncCellular, setSyncCellular] = useLocalPref<boolean>("okfolio-sync-cellular", false);
  const [folioName, setFolioName] = useLocalPref<string>("okfolio-name", "OK Folio");
  const [sources, setSources] = useState<ConnectorSourceSetting[]>([]);
  const [sourceLabel, setSourceLabel] = useState("");
  const [sourceChatID, setSourceChatID] = useState("");
  const [sourceTargetFolioId, setSourceTargetFolioId] = useState<number | null>(null);
  const [sourceShowInLibrary, setSourceShowInLibrary] = useState(true);
  const [editingSourceID, setEditingSourceID] = useState<number | null>(null);
  const [editSourceLabel, setEditSourceLabel] = useState("");
  const [editSourceChatID, setEditSourceChatID] = useState("");
  const [editSourceTargetFolioId, setEditSourceTargetFolioId] = useState<number | null>(null);
  const [editSourceShowInLibrary, setEditSourceShowInLibrary] = useState(true);
  const [runtimeTelegramSources, setRuntimeTelegramSources] = useState<ConnectorSourceStatus[]>([]);
  const [sourceBusy, setSourceBusy] = useState(false);
  const [sourceError, setSourceError] = useState("");
  const storageEstimate = useStorageEstimate();
  const offlineCacheSize =
    storageEstimate.supported && storageEstimate.usage != null
      ? formatBytes(storageEstimate.usage)
      : "Unavailable";
  const offlineCacheDetail =
    storageEstimate.supported && storageEstimate.quota != null
      ? `Offline app-shell and media caches on this device. ${formatBytes(storageEstimate.quota)} browser quota available.`
      : "Offline cache estimate is not available on this device.";

  useEffect(() => {
    document.documentElement.dataset.reduceMotion = reduceMotion ? "1" : "0";
  }, [reduceMotion]);

  const reloadSources = async () => {
    const [settingsResponse, statusResponse] = await Promise.all([
      fetchConnectorSources("telegram"),
      fetchConnectorStatus(),
    ]);
    setSources(settingsResponse.sources);
    setRuntimeTelegramSources(statusResponse.connectors.find((connector) => connector.id === "telegram")?.sources ?? []);
  };

  useEffect(() => {
    reloadSources().catch((err: Error) => setSourceError(err.message));
  }, []);

  const saveSource = async () => {
    setSourceError("");
    const destinationError = destinationGateMessage(sourceTargetFolioId, sourceShowInLibrary);
    if (destinationError) {
      setSourceError(destinationError);
      return;
    }
    setSourceBusy(true);
    try {
      await createConnectorSource({
        type: "telegram",
        chat_id: sourceChatID,
        label: sourceLabel,
        enabled: true,
        target_folio_id: sourceTargetFolioId,
        show_in_library: sourceShowInLibrary,
      });
      setSourceLabel("");
      setSourceChatID("");
      setSourceTargetFolioId(null);
      setSourceShowInLibrary(true);
      await reloadSources();
    } catch (err) {
      setSourceError(err instanceof Error ? err.message : "Failed to save connector source");
    } finally {
      setSourceBusy(false);
    }
  };

  const toggleSource = async (source: ConnectorSourceSetting) => {
    setSourceError("");
    setSourceBusy(true);
    try {
      await updateConnectorSource(source.id, {
        type: source.type,
        chat_id: source.chat_id,
        label: source.label,
        enabled: !source.enabled,
        target_folio_id: source.target_folio_id ?? null,
        show_in_library: source.show_in_library ?? true,
      });
      await reloadSources();
    } catch (err) {
      setSourceError(err instanceof Error ? err.message : "Failed to update connector source");
    } finally {
      setSourceBusy(false);
    }
  };

  const beginEditSource = (source: ConnectorSourceSetting) => {
    setSourceError("");
    setEditingSourceID(source.id);
    setEditSourceLabel(source.label);
    setEditSourceChatID(source.chat_id);
    setEditSourceTargetFolioId(source.target_folio_id ?? null);
    setEditSourceShowInLibrary(source.show_in_library ?? true);
  };

  const cancelEditSource = () => {
    setEditingSourceID(null);
    setEditSourceLabel("");
    setEditSourceChatID("");
    setEditSourceTargetFolioId(null);
    setEditSourceShowInLibrary(true);
  };

  const saveEditedSource = async (source: ConnectorSourceSetting) => {
    setSourceError("");
    const destinationError = destinationGateMessage(editSourceTargetFolioId, editSourceShowInLibrary);
    if (destinationError) {
      setSourceError(destinationError);
      return;
    }
    setSourceBusy(true);
    try {
      await updateConnectorSource(source.id, {
        type: source.type,
        chat_id: editSourceChatID,
        label: editSourceLabel,
        enabled: source.enabled,
        target_folio_id: editSourceTargetFolioId,
        show_in_library: editSourceShowInLibrary,
      });
      cancelEditSource();
      await reloadSources();
    } catch (err) {
      setSourceError(err instanceof Error ? err.message : "Failed to update connector source");
    } finally {
      setSourceBusy(false);
    }
  };

  const removeSource = async (source: ConnectorSourceSetting) => {
    setSourceError("");
    setSourceBusy(true);
    try {
      await deleteConnectorSource(source.id);
      await reloadSources();
    } catch (err) {
      setSourceError(err instanceof Error ? err.message : "Failed to remove connector source");
    } finally {
      setSourceBusy(false);
    }
  };

  const seg = (active: boolean): CSSProperties => ({
    appearance: "none",
    cursor: "pointer",
    fontFamily: "var(--sans)",
    fontSize: 13,
    padding: "7px 16px",
    border: 0,
    borderRadius: 99,
    color: active ? "var(--ink)" : "var(--graphite)",
    background: active ? "var(--surface-2)" : "transparent",
    boxShadow: active ? "0 1px 4px var(--shadow)" : "none",
  });

  const mobileSegment = (active: boolean): CSSProperties => ({
    flex: 1,
    minHeight: 36,
    border: 0,
    borderRadius: 11,
    background: active ? "var(--surface)" : "transparent",
    color: active ? "var(--accent)" : "var(--graphite)",
    boxShadow: active ? "0 1px 4px var(--shadow)" : "none",
    fontFamily: "var(--sans)",
    fontSize: 13,
    fontWeight: 800,
  });

  const mobileRow = (title: string, detail: ReactNode, accessory?: ReactNode) => (
    <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 14, minHeight: 52, padding: "13px 0", borderBottom: "1px solid var(--line)" }}>
      <div style={{ minWidth: 0 }}>
        <div style={{ fontFamily: "var(--sans)", fontSize: 15, color: "var(--ink)" }}>{title}</div>
        {detail ? <div style={{ marginTop: 3, fontFamily: "var(--sans)", fontSize: 12.5, color: "var(--muted)" }}>{detail}</div> : null}
      </div>
      {accessory}
    </div>
  );

  const sourceMatchesManaged = (source: ConnectorSourceStatus) =>
    sources.some((managed) => {
      const label = managed.label.trim();
      const chatID = managed.chat_id.trim();
      return (
        (chatID !== "" && (source.id.includes(chatID) || source.display_name.includes(chatID))) ||
        (label !== "" && source.display_name === label)
      );
    });
  const hasManagedTelegramSources = sources.length > 0;
  const unmanagedTelegramSources = runtimeTelegramSources.filter((source) => {
    if (sourceMatchesManaged(source)) return false;
    // Telegram runtime status is grouped by piece provenance URL/channel, while
    // Settings stores chat IDs. When managed rows exist, an unmatched runtime row
    // may be the same source and should not be reported as unmanaged.
    return !hasManagedTelegramSources;
  });
  const telegramSourceSummary =
    sources.length > 0
      ? `${sources.length} managed${unmanagedTelegramSources.length > 0 ? `, ${unmanagedTelegramSources.length} runtime-managed` : ""}`
      : unmanagedTelegramSources.length > 0
        ? `${unmanagedTelegramSources.length} runtime-managed`
        : "No sources";

  if (isMobile) {
    return (
      <div>
        <section style={{ padding: "18px 0 0" }}>
          <h2 style={{ ...SECTION, margin: "0 0 8px", letterSpacing: "0.06em" }}>Appearance</h2>
          <div style={{ padding: "4px 0 20px", borderBottom: "1px solid var(--line)" }}>
            <div style={{ display: "flex", gap: 3, padding: 4, borderRadius: 13, background: "var(--wall)" }}>
              <button type="button" onClick={() => setTheme("light")} style={mobileSegment(theme === "light")}>Light</button>
              <button type="button" onClick={() => setTheme("dark")} style={mobileSegment(theme === "dark")}>Dark</button>
              <button type="button" onClick={() => setTheme("auto")} style={mobileSegment(theme === "auto")}>Auto</button>
            </div>
            {mobileRow("Reduce motion", "Calmer transitions throughout.", <Switch on={reduceMotion} onClick={() => setReduceMotion(!reduceMotion)} />)}
            {mobileRow(
              "Default gallery mode",
              mode.charAt(0).toUpperCase() + mode.slice(1),
              <select
                value={mode}
                onChange={(e) => setMode(e.target.value as typeof mode)}
                aria-label="Default gallery mode"
                style={{ border: 0, background: "transparent", color: "var(--accent)", fontFamily: "var(--sans)", fontSize: 14, fontWeight: 800 }}
              >
                <option value="magazine">Magazine</option>
                <option value="library">Library</option>
                <option value="wall">Wall</option>
              </select>,
            )}
            <div style={{ padding: "14px 0 0" }}>
              <div style={{ fontFamily: "var(--sans)", fontSize: 15, color: "var(--ink)", marginBottom: 8 }}>Info panel</div>
              <div style={{ display: "flex", gap: 3, padding: 4, borderRadius: 13, background: "var(--wall)" }}>
                <button type="button" onClick={() => setInfoPanelMode("pinned")} style={mobileSegment(infoPanelMode === "pinned")}>Pinned</button>
                <button type="button" onClick={() => setInfoPanelMode("remember")} style={mobileSegment(infoPanelMode === "remember")}>Remember</button>
                <button type="button" onClick={() => setInfoPanelMode("hidden")} style={mobileSegment(infoPanelMode === "hidden")}>Hidden</button>
              </div>
            </div>
          </div>

          <h2 style={{ ...SECTION, margin: "28px 0 8px", letterSpacing: "0.06em" }}>Storage & Sync</h2>
          <div>
            {mobileRow("Offline cache (size)", offlineCacheDetail, <span style={{ fontFamily: "var(--sans)", fontSize: 13, color: "var(--graphite)", whiteSpace: "nowrap" }}>{offlineCacheSize}</span>)}
            {mobileRow("Sync over cellular", "Stored on this device.", <Switch on={syncCellular} onClick={() => setSyncCellular(!syncCellular)} />)}
            {mobileRow("App address", "Current self-hosted address.", <span style={{ fontFamily: "var(--sans)", fontSize: 13, color: "var(--graphite)", overflowWrap: "anywhere", textAlign: "right" }}>{currentHost}</span>)}
          </div>

          <h2 style={{ ...SECTION, margin: "28px 0 8px", letterSpacing: "0.06em" }}>Connectors</h2>
          <div>
            {mobileRow(
              "Telegram sources",
              unmanagedTelegramSources.length > 0
                ? "Some Telegram activity is runtime-managed outside Settings."
                : "Managed sources can be edited on desktop.",
              <span style={{ fontFamily: "var(--sans)", fontSize: 13, color: "var(--graphite)", whiteSpace: "nowrap" }}>{telegramSourceSummary}</span>,
            )}
          </div>

          <h2 style={{ ...SECTION, margin: "28px 0 8px", letterSpacing: "0.06em" }}>Folios</h2>
          <div>
            {mobileRow(
              "Folio name",
              "Shown across this instance.",
              <Hov
                as="input"
                value={folioName}
                onChange={(e: React.ChangeEvent<HTMLInputElement>) => setFolioName(e.target.value)}
                style={{ appearance: "none", fontFamily: "var(--serif)", fontSize: 15, color: "var(--ink)", textAlign: "right", border: 0, borderBottom: "1px solid var(--line-2)", background: "transparent", outline: "none", padding: "4px 2px", width: 130 }}
                focus={{ borderColor: "var(--accent)" }}
              />,
            )}
            {mobileRow("Auto-select covers", "Choose a fitting cover until you set one yourself.", <Switch on={autoCovers} onClick={() => setAutoCovers(!autoCovers)} />)}
            {mobileRow("Suggest folios", "Quietly group related pieces.", <Switch on={suggestFolios} onClick={() => setSuggestFolios(!suggestFolios)} />)}
          </div>

          <footer style={{ padding: "34px 0 10px", display: "flex", alignItems: "center", gap: 11, color: "var(--muted)" }}>
            <BrandMark width={22} height={25} />
            <div style={{ fontFamily: "var(--sans)", fontSize: 13 }}>
              OK Folio · Version 2.4 · Installed
            </div>
          </footer>
        </section>
      </div>
    );
  }

  return (
    <div>
      <PageHeader eyebrow="Settings" title="Preferences" pad="54px 0 8px" border={false} />
      <section style={{ maxWidth: 660, padding: "20px 0 0" }}>
        <h2 style={{ ...SECTION, margin: "34px 0 4px" }}>Appearance</h2>
        <Row title="Theme" desc="Light editorial paper, or dark gallery viewing.">
          <div style={{ display: "flex", alignItems: "center", gap: 3, padding: 4, border: "1px solid var(--line)", borderRadius: 99, background: "var(--surface)", flex: "none" }}>
            <button type="button" onClick={() => setTheme("light")} style={seg(theme === "light")}>Light</button>
            <button type="button" onClick={() => setTheme("dark")} style={seg(theme === "dark")}>Dark</button>
            <button type="button" onClick={() => setTheme("auto")} style={seg(theme === "auto")}>Auto</button>
          </div>
        </Row>
        <Row title="Info panel" desc="Choose how the viewer keeps details visible while browsing pieces.">
          <div style={{ display: "flex", alignItems: "center", gap: 3, padding: 4, border: "1px solid var(--line)", borderRadius: 99, background: "var(--surface)", flex: "none" }}>
            <button type="button" onClick={() => setInfoPanelMode("pinned")} style={seg(infoPanelMode === "pinned")}>Pinned</button>
            <button type="button" onClick={() => setInfoPanelMode("remember")} style={seg(infoPanelMode === "remember")}>Remember</button>
            <button type="button" onClick={() => setInfoPanelMode("hidden")} style={seg(infoPanelMode === "hidden")}>Hidden</button>
          </div>
        </Row>
        <Row title="Reduce motion" desc="Calmer transitions throughout.">
          <Switch on={reduceMotion} onClick={() => setReduceMotion(!reduceMotion)} />
        </Row>

        <h2 style={SECTION}>Instance</h2>
        <Row title="Folio name" desc="Shown across this instance.">
          <Hov
            as="input"
            value={folioName}
            onChange={(e: React.ChangeEvent<HTMLInputElement>) => setFolioName(e.target.value)}
            style={{ appearance: "none", fontFamily: "var(--serif)", fontSize: 16, color: "var(--ink)", textAlign: "right", border: 0, borderBottom: "1px solid var(--line-2)", background: "transparent", outline: "none", padding: "4px 2px", width: 180 }}
            focus={{ borderColor: "var(--accent)" }}
          />
        </Row>
        <Row title="Address" desc="Current self-hosted address.">
          <div style={{ fontFamily: "var(--sans)", fontSize: 13.5, color: "var(--graphite)", display: "flex", alignItems: "center", gap: 8 }}>
            <span style={{ width: 6, height: 6, borderRadius: 99, background: "var(--accent)" }} />
            <span style={{ overflowWrap: "anywhere" }}>{currentHost}</span>
          </div>
        </Row>
        <Row title="Storage" desc="Where your pieces live.">
          <div style={{ fontFamily: "var(--sans)", fontSize: 13.5, color: "var(--graphite)", textAlign: "right" }}>
            {totalPhotos.toLocaleString()} pieces · {formatBytes(totalSizeBytes)}
          </div>
        </Row>
        <Row title="Offline cache (size)" desc={offlineCacheDetail}>
          <div style={{ fontFamily: "var(--sans)", fontSize: 13.5, color: "var(--graphite)", textAlign: "right" }}>
            {offlineCacheSize}
          </div>
        </Row>

        <h2 style={SECTION}>Connectors</h2>
        <div style={{ padding: "18px 0", borderBottom: "1px solid var(--line)" }}>
          <div style={{ ...ROW_TITLE, marginBottom: 5 }}>Telegram sources</div>
          <div style={{ ...ROW_DESC, marginBottom: 16 }}>
            Enabled chat IDs are polled by the connector. Runtime-managed sources are shown below when they are not managed here.
          </div>
          <div style={{ display: "grid", gridTemplateColumns: "minmax(150px, 1fr) minmax(180px, 1.2fr) auto", gap: 10, alignItems: "end" }}>
            <Hov
              as="input"
              value={sourceLabel}
              placeholder="Label"
              onChange={(e: React.ChangeEvent<HTMLInputElement>) => setSourceLabel(e.target.value)}
              style={{ appearance: "none", fontFamily: "var(--sans)", fontSize: 13.5, color: "var(--ink)", border: "1px solid var(--line)", borderRadius: 6, background: "var(--surface)", outline: "none", padding: "9px 10px", minWidth: 0 }}
              focus={{ borderColor: "var(--accent)" }}
            />
            <Hov
              as="input"
              value={sourceChatID}
              placeholder="Chat ID"
              onChange={(e: React.ChangeEvent<HTMLInputElement>) => setSourceChatID(e.target.value)}
              style={{ appearance: "none", fontFamily: "var(--sans)", fontSize: 13.5, color: "var(--ink)", border: "1px solid var(--line)", borderRadius: 6, background: "var(--surface)", outline: "none", padding: "9px 10px", minWidth: 0 }}
              focus={{ borderColor: "var(--accent)" }}
            />
            <button
              onClick={saveSource}
              disabled={sourceBusy || sourceChatID.trim() === "" || Boolean(destinationGateMessage(sourceTargetFolioId, sourceShowInLibrary))}
              style={{ appearance: "none", cursor: sourceBusy || sourceChatID.trim() === "" || Boolean(destinationGateMessage(sourceTargetFolioId, sourceShowInLibrary)) ? "not-allowed" : "pointer", fontFamily: "var(--sans)", fontSize: 13, border: 0, borderRadius: 6, color: "var(--on-accent)", background: "var(--accent)", padding: "10px 14px", minHeight: 44, opacity: sourceBusy || sourceChatID.trim() === "" || Boolean(destinationGateMessage(sourceTargetFolioId, sourceShowInLibrary)) ? 0.55 : 1 }}
            >
              Add
            </button>
          </div>
          <DestinationControls
            targetFolioId={sourceTargetFolioId}
            showInLibrary={sourceShowInLibrary}
            onTargetFolioIdChange={setSourceTargetFolioId}
            onShowInLibraryChange={setSourceShowInLibrary}
            disabled={sourceBusy}
            backfillBlockedMessage="Create and save this source before backfill."
            compact
          />
          {sourceError && <div style={{ ...ROW_DESC, color: "var(--danger, #b42318)", marginTop: 10 }}>{sourceError}</div>}
          <div style={{ marginTop: 12 }}>
            {sources.length === 0 ? (
              <div style={{ ...ROW_DESC, padding: "10px 0" }}>No managed Telegram sources.</div>
            ) : sources.map((source) => (
              <div key={source.id} style={{ padding: "11px 0", borderTop: "1px solid var(--line)" }}>
                {editingSourceID === source.id ? (
                  <div style={{ display: "grid", gap: 12 }}>
                    <div style={{ display: "grid", gridTemplateColumns: "minmax(150px, 1fr) minmax(180px, 1.2fr)", gap: 10 }}>
                      <Hov
                        as="input"
                        value={editSourceLabel}
                        placeholder="Label"
                        onChange={(e: React.ChangeEvent<HTMLInputElement>) => setEditSourceLabel(e.target.value)}
                        style={{ appearance: "none", fontFamily: "var(--sans)", fontSize: 13.5, color: "var(--ink)", border: "1px solid var(--line)", borderRadius: 6, background: "var(--surface)", outline: "none", padding: "9px 10px", minHeight: 44, minWidth: 0 }}
                        focus={{ borderColor: "var(--accent)" }}
                      />
                      <Hov
                        as="input"
                        value={editSourceChatID}
                        placeholder="Chat ID"
                        onChange={(e: React.ChangeEvent<HTMLInputElement>) => setEditSourceChatID(e.target.value)}
                        style={{ appearance: "none", fontFamily: "var(--sans)", fontSize: 13.5, color: "var(--ink)", border: "1px solid var(--line)", borderRadius: 6, background: "var(--surface)", outline: "none", padding: "9px 10px", minHeight: 44, minWidth: 0 }}
                        focus={{ borderColor: "var(--accent)" }}
                      />
                    </div>
                    <DestinationControls
                      targetFolioId={editSourceTargetFolioId}
                      showInLibrary={editSourceShowInLibrary}
                      onTargetFolioIdChange={setEditSourceTargetFolioId}
                      onShowInLibraryChange={setEditSourceShowInLibrary}
                      disabled={sourceBusy}
                      sourceId={source.id}
                      canBackfill={
                        editSourceTargetFolioId != null &&
                        editSourceTargetFolioId === (source.target_folio_id ?? null) &&
                        editSourceShowInLibrary === (source.show_in_library ?? true)
                      }
                      backfillBlockedMessage="Save a target folio before backfill."
                      compact
                    />
                    <div style={{ display: "flex", justifyContent: "flex-end", gap: 8, flexWrap: "wrap" }}>
                      <button
                        onClick={cancelEditSource}
                        disabled={sourceBusy}
                        style={{ appearance: "none", cursor: sourceBusy ? "not-allowed" : "pointer", fontFamily: "var(--sans)", fontSize: 12.5, border: "1px solid var(--line)", borderRadius: 6, color: "var(--graphite)", background: "var(--surface)", padding: "9px 12px", minHeight: 44, opacity: sourceBusy ? 0.55 : 1 }}
                      >
                        Cancel
                      </button>
                      <button
                        onClick={() => saveEditedSource(source)}
                        disabled={sourceBusy || editSourceChatID.trim() === "" || Boolean(destinationGateMessage(editSourceTargetFolioId, editSourceShowInLibrary))}
                        style={{ appearance: "none", cursor: sourceBusy || editSourceChatID.trim() === "" || Boolean(destinationGateMessage(editSourceTargetFolioId, editSourceShowInLibrary)) ? "not-allowed" : "pointer", fontFamily: "var(--sans)", fontSize: 12.5, border: 0, borderRadius: 6, color: "var(--on-accent)", background: "var(--accent)", padding: "9px 12px", minHeight: 44, opacity: sourceBusy || editSourceChatID.trim() === "" || Boolean(destinationGateMessage(editSourceTargetFolioId, editSourceShowInLibrary)) ? 0.55 : 1 }}
                      >
                        Save
                      </button>
                    </div>
                  </div>
                ) : (
                  <div style={{ display: "grid", gridTemplateColumns: "1fr auto auto auto", gap: 12, alignItems: "center" }}>
                    <div style={{ minWidth: 0 }}>
                      <div style={{ fontFamily: "var(--sans)", fontSize: 13.5, color: "var(--ink)", overflowWrap: "anywhere" }}>{source.label || "Telegram source"}</div>
                      <div style={{ ...ROW_DESC, overflowWrap: "anywhere" }}>
                        {source.chat_id}
                        {source.target_folio_id ? " · routed to folio" : ""}
                        {source.show_in_library === false ? " · hidden from library" : ""}
                      </div>
                    </div>
                    <Switch on={source.enabled} onClick={() => toggleSource(source)} />
                    <button
                      onClick={() => beginEditSource(source)}
                      disabled={sourceBusy}
                      style={{ appearance: "none", cursor: sourceBusy ? "not-allowed" : "pointer", fontFamily: "var(--sans)", fontSize: 12.5, border: "1px solid var(--line)", borderRadius: 6, color: "var(--graphite)", background: "var(--surface)", padding: "9px 10px", minHeight: 44, opacity: sourceBusy ? 0.55 : 1 }}
                    >
                      Edit
                    </button>
                    <button
                      onClick={() => removeSource(source)}
                      disabled={sourceBusy}
                      style={{ appearance: "none", cursor: sourceBusy ? "not-allowed" : "pointer", fontFamily: "var(--sans)", fontSize: 12.5, border: "1px solid var(--line)", borderRadius: 6, color: "var(--graphite)", background: "var(--surface)", padding: "9px 10px", minHeight: 44, opacity: sourceBusy ? 0.55 : 1 }}
                    >
                      Remove
                    </button>
                  </div>
                )}
              </div>
            ))}
            {unmanagedTelegramSources.map((source) => (
              <div key={source.id || source.display_name} style={{ padding: "11px 0", borderTop: "1px solid var(--line)" }}>
                <div style={{ display: "grid", gridTemplateColumns: "1fr auto", gap: 12, alignItems: "center" }}>
                  <div style={{ minWidth: 0 }}>
                    <div style={{ fontFamily: "var(--sans)", fontSize: 13.5, color: "var(--ink)", overflowWrap: "anywhere" }}>{source.display_name || "Telegram source"}</div>
                    <div style={{ ...ROW_DESC, overflowWrap: "anywhere" }}>
                      Runtime-managed · {source.counts.downloaded.toLocaleString()} kept{source.counts.failed ? ` · ${source.counts.failed.toLocaleString()} failed` : ""}
                    </div>
                  </div>
                  <span style={{ fontFamily: "var(--sans)", fontSize: 12.5, color: "var(--muted)", border: "1px solid var(--line)", borderRadius: 99, padding: "6px 10px", whiteSpace: "nowrap" }}>
                    Unmanaged
                  </span>
                </div>
              </div>
            ))}
          </div>
        </div>

        <h2 style={SECTION}>Folios</h2>
        <Row title="Auto-select covers" desc="Choose a fitting cover until you set one yourself.">
          <Switch on={autoCovers} onClick={() => setAutoCovers(!autoCovers)} />
        </Row>
        <Row title="Suggest folios" desc="Quietly group related pieces. You decide what to keep.">
          <Switch on={suggestFolios} onClick={() => setSuggestFolios(!suggestFolios)} />
        </Row>

        <h2 style={SECTION}>About</h2>
        <div style={{ padding: "20px 0 0" }}>
          <div style={{ display: "flex", alignItems: "center", gap: 11 }}>
            <BrandMark width={20} height={23} />
            <span style={{ fontFamily: "var(--serif)", fontSize: 18, color: "var(--ink)" }}>
              <span style={{ color: "var(--graphite)" }}>OK</span> Folio
            </span>
          </div>
          <p style={{ fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 16, color: "var(--graphite)", margin: "16px 0 0", maxWidth: 460, lineHeight: 1.5 }}>
            A beautiful folio for visual discoveries.
          </p>
          <div style={{ fontFamily: "var(--sans)", fontSize: 12.5, color: "var(--muted)", marginTop: 14 }}>
            MIT License · Open source · Self-hosted
          </div>
        </div>
      </section>
    </div>
  );
}
