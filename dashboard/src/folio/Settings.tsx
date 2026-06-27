import { useEffect, useState, type CSSProperties } from "react";
import { createConnectorSource, deleteConnectorSource, fetchConnectorSources, updateConnectorSource } from "../api";
import type { ConnectorSourceSetting } from "../types";
import { useFolio, formatBytes } from "./context";
import { BrandMark, Hov, PageHeader } from "./ui";

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
  const { theme, setTheme, totalPhotos, totalSizeBytes } = useFolio();

  const [reduceMotion, setReduceMotion] = useLocalPref<boolean>("okfolio-reduce-motion", false);
  const [autoCovers, setAutoCovers] = useLocalPref<boolean>("okfolio-auto-covers", true);
  const [suggestFolios, setSuggestFolios] = useLocalPref<boolean>("okfolio-suggest-folios", true);
  const [folioName, setFolioName] = useLocalPref<string>("okfolio-name", "OK Folio");
  const [sources, setSources] = useState<ConnectorSourceSetting[]>([]);
  const [sourceLabel, setSourceLabel] = useState("");
  const [sourceChatID, setSourceChatID] = useState("");
  const [sourceBusy, setSourceBusy] = useState(false);
  const [sourceError, setSourceError] = useState("");

  useEffect(() => {
    document.documentElement.dataset.reduceMotion = reduceMotion ? "1" : "0";
  }, [reduceMotion]);

  const reloadSources = async () => {
    const response = await fetchConnectorSources("telegram");
    setSources(response.sources);
  };

  useEffect(() => {
    reloadSources().catch((err: Error) => setSourceError(err.message));
  }, []);

  const saveSource = async () => {
    setSourceError("");
    setSourceBusy(true);
    try {
      await createConnectorSource({
        type: "telegram",
        chat_id: sourceChatID,
        label: sourceLabel,
        enabled: true,
      });
      setSourceLabel("");
      setSourceChatID("");
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
      });
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

  return (
    <div>
      <PageHeader eyebrow="Settings" title="Preferences" pad="54px 0 8px" border={false} />
      <section style={{ maxWidth: 660, padding: "20px 0 0" }}>
        <h2 style={{ ...SECTION, margin: "34px 0 4px" }}>Appearance</h2>
        <Row title="Theme" desc="Light editorial paper, or dark gallery viewing.">
          <div style={{ display: "flex", alignItems: "center", gap: 3, padding: 4, border: "1px solid var(--line)", borderRadius: 99, background: "var(--surface)", flex: "none" }}>
            <button onClick={() => setTheme("light")} style={seg(theme !== "dark")}>Light</button>
            <button onClick={() => setTheme("dark")} style={seg(theme === "dark")}>Dark</button>
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
        <Row title="Address" desc="Self-hosted, on your own machine.">
          <div style={{ fontFamily: "var(--sans)", fontSize: 13.5, color: "var(--graphite)", display: "flex", alignItems: "center", gap: 8 }}>
            <span style={{ width: 6, height: 6, borderRadius: 99, background: "var(--accent)" }} />
            folio.oklabs.uk
          </div>
        </Row>
        <Row title="Storage" desc="Where your pieces live.">
          <div style={{ fontFamily: "var(--sans)", fontSize: 13.5, color: "var(--graphite)", textAlign: "right" }}>
            {totalPhotos.toLocaleString()} pieces · {formatBytes(totalSizeBytes)}
          </div>
        </Row>

        <h2 style={SECTION}>Connectors</h2>
        <div style={{ padding: "18px 0", borderBottom: "1px solid var(--line)" }}>
          <div style={{ ...ROW_TITLE, marginBottom: 5 }}>Telegram sources</div>
          <div style={{ ...ROW_DESC, marginBottom: 16 }}>Enabled chat IDs are polled by the connector without redeploying.</div>
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
              disabled={sourceBusy || sourceChatID.trim() === ""}
              style={{ appearance: "none", cursor: sourceBusy || sourceChatID.trim() === "" ? "not-allowed" : "pointer", fontFamily: "var(--sans)", fontSize: 13, border: 0, borderRadius: 6, color: "var(--on-accent)", background: "var(--accent)", padding: "10px 14px", opacity: sourceBusy || sourceChatID.trim() === "" ? 0.55 : 1 }}
            >
              Add
            </button>
          </div>
          {sourceError && <div style={{ ...ROW_DESC, color: "var(--danger, #b42318)", marginTop: 10 }}>{sourceError}</div>}
          <div style={{ marginTop: 12 }}>
            {sources.length === 0 ? (
              <div style={{ ...ROW_DESC, padding: "10px 0" }}>No managed Telegram sources.</div>
            ) : sources.map((source) => (
              <div key={source.id} style={{ display: "grid", gridTemplateColumns: "1fr auto auto", gap: 12, alignItems: "center", padding: "11px 0", borderTop: "1px solid var(--line)" }}>
                <div style={{ minWidth: 0 }}>
                  <div style={{ fontFamily: "var(--sans)", fontSize: 13.5, color: "var(--ink)", overflowWrap: "anywhere" }}>{source.label || "Telegram source"}</div>
                  <div style={{ ...ROW_DESC, overflowWrap: "anywhere" }}>{source.chat_id}</div>
                </div>
                <Switch on={source.enabled} onClick={() => toggleSource(source)} />
                <button
                  onClick={() => removeSource(source)}
                  disabled={sourceBusy}
                  style={{ appearance: "none", cursor: sourceBusy ? "not-allowed" : "pointer", fontFamily: "var(--sans)", fontSize: 12.5, border: "1px solid var(--line)", borderRadius: 6, color: "var(--graphite)", background: "var(--surface)", padding: "7px 10px", opacity: sourceBusy ? 0.55 : 1 }}
                >
                  Remove
                </button>
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
