import { useMemo, useState, type CSSProperties } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  createConnectorSource,
  deleteConnectorSource,
  fetchConnectorSources,
  fetchConnectorStatus,
  previewConnectorSource,
  updateConnectorSource,
} from "../api";
import type {
  ConnectorSourcePreviewResponse,
  ConnectorSourcesResponse,
  ConnectorSourceSetting,
  ConnectorStatus,
  WebGalleryConfig,
  WebGalleryFieldSelector,
  WebGalleryPaginationConfig,
} from "../types";
import DestinationControls, { destinationGateMessage } from "./DestinationControls";
import { ConfirmationDialog, DotsIcon, OutlineButton, PageHeader, PlusIcon } from "./ui";
import { useViewport } from "./useViewport";

type EditorMode = { kind: "create" } | { kind: "edit"; source: ConnectorSourceSetting };

type StreamNotice = { tone: "guidance" | "error"; message: string };

type WebGalleryForm = {
  label: string;
  listURL: string;
  schedule: string;
  enabled: boolean;
  paginationStrategy: WebGalleryPaginationConfig["strategy"];
  pageParam: string;
  startIndex: string;
  nextLinkSelector: string;
  itemLinkSelector: string;
  imageSelector: string;
  imageAttr: string;
  artistSelector: string;
  artistAttr: string;
  titleSelector: string;
  titleAttr: string;
  dateSelector: string;
  dateAttr: string;
  itemLinkFilter: string;
  targetFolioId: number | null;
  showInLibrary: boolean;
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

function formatAgo(iso: string | null | undefined): string {
  if (!iso) return "never";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "never";
  const s = Math.floor((Date.now() - d.getTime()) / 1000);
  if (s < 60) return "just now";
  if (s < 3600) return `${Math.floor(s / 60)} min ago`;
  if (s < 86400) return `${Math.floor(s / 3600)} hours ago`;
  const days = Math.floor(s / 86400);
  if (days === 1) return "yesterday";
  if (days < 30) return `${days} days ago`;
  return d.toLocaleDateString(undefined, { month: "short", day: "numeric" });
}

function weekCount(s: ConnectorStatus): number {
  const cutoff = Date.now() - 7 * 86_400_000;
  return (s.recent_runs ?? []).reduce((sum, r) => {
    const t = new Date(r.start_time).getTime();
    return !Number.isNaN(t) && t >= cutoff ? sum + (r.photos_downloaded || 0) : sum;
  }, 0);
}

function statusView(health: ConnectorStatus["health"]): {
  label: string;
  dot: string;
  color: string;
} {
  switch (health) {
    case "healthy":
      return { label: "Active", dot: "var(--accent)", color: "var(--graphite)" };
    case "syncing":
      return { label: "Syncing", dot: "var(--accent)", color: "var(--graphite)" };
    case "degraded":
      return { label: "Degraded", dot: "var(--accent)", color: "var(--graphite)" };
    case "error":
      return { label: "Error", dot: "var(--accent)", color: "var(--graphite)" };
    default:
      return { label: "Idle", dot: "var(--faint)", color: "var(--muted)" };
  }
}

function sourceType(s: ConnectorStatus): string {
  if (s.id === "webgallery") return "Web";
  if (s.id === "telegram") return "Telegram";
  const id = `${s.id} ${s.display_name}`.toLowerCase();
  if (id.includes("rss")) return "RSS";
  if (id.includes("manual")) return "Manual";
  return "API";
}

function defaultConfig(listURL = ""): WebGalleryConfig {
  return {
    list_url: listURL,
    pagination: { strategy: "page_param", param_name: "pager", start_index: 1 },
    selectors: {
      item_link: "div.photo-item a",
      image: { selector: "img#big_photo", attr: "src" },
      title: { selector: "h1[itemprop='name']" },
      artist: { selector: "span[itemprop='name']" },
      date: { selector: "meta[itemprop='datePublished']", attr: "content" },
    },
    item_link_filter: ["javascript", "users"],
  };
}

function sourceConfig(source?: ConnectorSourceSetting): WebGalleryConfig {
  if (source?.config) return { ...defaultConfig(source.config.list_url), ...source.config };
  return defaultConfig("");
}

function latestStreamIssue(s: ConnectorStatus): string {
  const message = s.recent_errors?.[0]?.message?.trim();
  return message || "";
}

function initialForm(source?: ConnectorSourceSetting): WebGalleryForm {
  const cfg = sourceConfig(source);
  return {
    label: source?.label ?? "",
    listURL: cfg.list_url,
    schedule: cfg.schedule ?? "",
    enabled: source?.enabled ?? false,
    paginationStrategy: cfg.pagination.strategy,
    pageParam: cfg.pagination.param_name ?? "pager",
    startIndex: String(cfg.pagination.start_index ?? 1),
    nextLinkSelector: cfg.pagination.next_link_selector ?? "",
    itemLinkSelector: cfg.selectors.item_link,
    imageSelector: cfg.selectors.image.selector,
    imageAttr: cfg.selectors.image.attr ?? "src",
    artistSelector: cfg.selectors.artist?.selector ?? "",
    artistAttr: cfg.selectors.artist?.attr ?? "",
    titleSelector: cfg.selectors.title?.selector ?? "",
    titleAttr: cfg.selectors.title?.attr ?? "",
    dateSelector: cfg.selectors.date?.selector ?? "",
    dateAttr: cfg.selectors.date?.attr ?? "",
    itemLinkFilter: cfg.item_link_filter.join("\n"),
    targetFolioId: source?.target_folio_id ?? null,
    showInLibrary: source?.show_in_library ?? true,
  };
}

function optionalField(selector: string, attr: string): WebGalleryFieldSelector | undefined {
  const cleanSelector = selector.trim();
  if (!cleanSelector) return undefined;
  const cleanAttr = attr.trim();
  return cleanAttr ? { selector: cleanSelector, attr: cleanAttr } : { selector: cleanSelector };
}

function formConfig(form: WebGalleryForm): WebGalleryConfig {
  const pagination: WebGalleryPaginationConfig = { strategy: form.paginationStrategy };
  if (form.paginationStrategy === "page_param") {
    pagination.param_name = form.pageParam.trim() || "pager";
    const startIndex = Number.parseInt(form.startIndex, 10);
    pagination.start_index = Number.isNaN(startIndex) ? 1 : startIndex;
  }
  if (form.paginationStrategy === "next_link") {
    pagination.next_link_selector = form.nextLinkSelector.trim();
  }
  const image: WebGalleryFieldSelector = { selector: form.imageSelector.trim() };
  if (form.imageAttr.trim()) image.attr = form.imageAttr.trim();
  return {
    list_url: form.listURL.trim(),
    schedule: form.schedule.trim() || undefined,
    pagination,
    selectors: {
      item_link: form.itemLinkSelector.trim(),
      image,
      artist: optionalField(form.artistSelector, form.artistAttr),
      title: optionalField(form.titleSelector, form.titleAttr),
      date: optionalField(form.dateSelector, form.dateAttr),
    },
    item_link_filter: form.itemLinkFilter
      .split(/\r?\n|,/)
      .map((item) => item.trim())
      .filter(Boolean),
  };
}

function sourceTitle(source: ConnectorSourceSetting): string {
  return source.label || source.config?.list_url || source.chat_id || `Source ${source.id}`;
}

function Toggle({ on, onClick, label, disabled = false }: { on: boolean; onClick: () => void; label: string; disabled?: boolean }) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={on}
      aria-label={label}
      disabled={disabled}
      onClick={onClick}
      style={{
        flex: "none",
        width: 48,
        height: 29,
        padding: 3,
        border: 0,
        borderRadius: 99,
        background: on ? "var(--accent)" : "var(--line-2)",
        display: "flex",
        justifyContent: on ? "flex-end" : "flex-start",
        cursor: disabled ? "not-allowed" : "pointer",
        opacity: disabled ? 0.55 : 1,
      }}
    >
      <span
        style={{
          width: 23,
          height: 23,
          borderRadius: 99,
          background: on ? "var(--on-accent)" : "var(--surface)",
          boxShadow: "0 1px 4px var(--shadow)",
        }}
      />
    </button>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label style={{ display: "block", minWidth: 0 }}>
      <span style={labelStyle}>{label}</span>
      {children}
    </label>
  );
}

function TextField({
  label,
  value,
  onChange,
  placeholder,
  type = "text",
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  type?: string;
}) {
  return (
    <Field label={label}>
      <input value={value} onChange={(e) => onChange(e.target.value)} placeholder={placeholder} type={type} style={fieldBase} />
    </Field>
  );
}

function SelectorPair({
  label,
  selector,
  attr,
  onSelector,
  onAttr,
  selectorPlaceholder,
  attrPlaceholder = "attr",
}: {
  label: string;
  selector: string;
  attr: string;
  onSelector: (value: string) => void;
  onAttr: (value: string) => void;
  selectorPlaceholder?: string;
  attrPlaceholder?: string;
}) {
  return (
    <div style={{ display: "grid", gridTemplateColumns: "minmax(0, 1fr) 112px", gap: 10 }}>
      <TextField label={label} value={selector} onChange={onSelector} placeholder={selectorPlaceholder} />
      <TextField label="Attribute" value={attr} onChange={onAttr} placeholder={attrPlaceholder} />
    </div>
  );
}

function MobileStreamCard({
  s,
  settings,
  busy,
  onToggle,
}: {
  s: ConnectorStatus;
  settings: ConnectorSourceSetting[];
  busy: boolean;
  onToggle: (connector: ConnectorStatus, sources: ConnectorSourceSetting[]) => void;
}) {
  const initial = (s.display_name || "?").trim().charAt(0).toUpperCase() || "?";
  const managedSources = settings.filter((source) => source.type === s.id);
  const enabled = managedSources.length > 0 ? managedSources.some((source) => source.enabled) : s.health !== "idle" && s.health !== "error";
  const canToggle = managedSources.length > 0 && !busy;
  const pieces = s.counts?.total ?? s.counts?.downloaded ?? 0;
  const issue = latestStreamIssue(s);
  return (
    <article
      style={{
        display: "grid",
        gridTemplateColumns: "44px minmax(0, 1fr) auto",
        alignItems: "center",
        gap: 13,
        padding: "14px 0",
        borderBottom: "1px solid var(--line)",
      }}
    >
      <div
        style={{
          width: 44,
          height: 44,
          borderRadius: 10,
          border: "1px solid var(--line)",
          background: "var(--surface)",
          color: "var(--accent)",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          fontFamily: "var(--serif)",
          fontSize: 19,
          fontWeight: 500,
        }}
      >
        {initial}
      </div>
      <div style={{ minWidth: 0 }}>
        <h2 style={{ margin: 0, fontFamily: "var(--serif)", fontSize: 18, fontWeight: 500, lineHeight: 1.12, color: "var(--ink)" }}>
          {s.display_name}
        </h2>
        <div style={{ marginTop: 4, fontFamily: "var(--sans)", fontSize: 12.5, color: "var(--muted)", whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>
          {sourceType(s)} · synced {formatAgo(s.last_sync)} · {pieces.toLocaleString()} pieces
        </div>
        {issue ? (
          <div style={{ marginTop: 4, fontFamily: "var(--sans)", fontSize: 12, color: "var(--accent)", whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>
            Latest issue: {issue}
          </div>
        ) : null}
      </div>
      <Toggle on={enabled} onClick={() => onToggle(s, managedSources)} label={`Toggle ${s.display_name}`} disabled={!canToggle} />
    </article>
  );
}

function SourceStatus({ source }: { source: ConnectorSourceSetting }) {
  return (
    <div style={{ marginTop: 6, fontFamily: "var(--sans)", fontSize: 12, color: source.last_error ? "var(--accent)" : "var(--muted)", lineHeight: 1.45 }}>
      Last seen {formatAgo(source.last_seen_at)}
      {source.last_error ? ` · ${source.last_error}` : ""}
    </div>
  );
}

function WebGallerySourceRow({
  source,
  busy,
  compact = false,
  onEdit,
  onToggle,
  onDelete,
}: {
  source: ConnectorSourceSetting;
  busy: boolean;
  compact?: boolean;
  onEdit: (source: ConnectorSourceSetting) => void;
  onToggle: (source: ConnectorSourceSetting) => void;
  onDelete: (source: ConnectorSourceSetting) => void;
}) {
  return (
    <div
      style={{
        display: "grid",
        gridTemplateColumns: compact ? "minmax(0, 1fr) auto" : "minmax(0, 1fr) auto auto auto",
        alignItems: "center",
        gap: 10,
        padding: "13px 0",
        borderTop: "1px solid var(--line)",
      }}
    >
      <div style={{ minWidth: 0 }}>
        <div style={{ fontFamily: "var(--sans)", fontSize: 14, color: "var(--ink)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
          {sourceTitle(source)}
        </div>
        <div style={{ marginTop: 3, fontFamily: "var(--sans)", fontSize: 12, color: "var(--muted)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
          {source.config?.list_url ?? source.chat_id} · {source.config?.schedule || "default schedule"}
        </div>
        <SourceStatus source={source} />
      </div>
      <Toggle on={source.enabled} onClick={() => onToggle(source)} label={`Toggle ${sourceTitle(source)}`} disabled={busy} />
      <div style={{ display: "flex", gap: 8, gridColumn: compact ? "1 / -1" : undefined, justifyContent: compact ? "flex-end" : undefined }}>
        <button type="button" onClick={() => onEdit(source)} disabled={busy} style={smallButtonStyle}>
          Edit
        </button>
        <button type="button" onClick={() => onDelete(source)} disabled={busy} style={{ ...smallButtonStyle, color: "var(--accent)" }}>
          Delete
        </button>
      </div>
    </div>
  );
}

const smallButtonStyle: CSSProperties = {
  minHeight: 36,
  padding: "0 12px",
  border: "1px solid var(--line)",
  borderRadius: 99,
  background: "transparent",
  color: "var(--graphite)",
  fontFamily: "var(--sans)",
  fontSize: 12.5,
  cursor: "pointer",
};

function StreamNoticeBanner({ notice, onDismiss }: { notice: StreamNotice; onDismiss: () => void }) {
  return (
    <div
      role={notice.tone === "error" ? "alert" : "status"}
      style={{
        display: "flex",
        alignItems: "start",
        justifyContent: "space-between",
        gap: 12,
        padding: "12px 14px",
        borderRadius: 6,
        border: notice.tone === "error" ? "1px solid var(--accent-line)" : "1px solid var(--line)",
        background: notice.tone === "error" ? "var(--accent-soft)" : "var(--surface-2)",
        color: notice.tone === "error" ? "var(--accent)" : "var(--graphite)",
        fontFamily: "var(--sans)",
        fontSize: 13,
        lineHeight: 1.45,
      }}
    >
      <span>{notice.message}</span>
      <button
        type="button"
        onClick={onDismiss}
        aria-label="Dismiss streams notice"
        style={{
          flex: "none",
          appearance: "none",
          border: 0,
          background: "transparent",
          color: "inherit",
          cursor: "pointer",
          fontFamily: "var(--sans)",
          fontSize: 18,
          lineHeight: 1,
          padding: "0 2px",
        }}
      >
        ×
      </button>
    </div>
  );
}

function StreamRow({
  s,
  settings,
  busy,
  onEditWebGallery,
  onToggleSource,
  onDeleteSource,
}: {
  s: ConnectorStatus;
  settings: ConnectorSourceSetting[];
  busy: boolean;
  onEditWebGallery: (source: ConnectorSourceSetting) => void;
  onToggleSource: (source: ConnectorSourceSetting) => void;
  onDeleteSource: (source: ConnectorSourceSetting) => void;
}) {
  const sv = statusView(s.health);
  const initial = (s.display_name || "?").trim().charAt(0).toUpperCase() || "?";
  const sourceCount = s.sources?.length ?? 0;
  const managedSources = settings.filter((source) => source.type === s.id);
  const kind = managedSources.length > 0 ? `Connector · ${managedSources.length} managed sources` : sourceCount > 1 ? `Connector · ${sourceCount} sources` : "Connector";
  const issue = latestStreamIssue(s);
  return (
    <div
      style={{
        border: "1px solid var(--line)",
        borderRadius: 6,
        background: "var(--surface)",
        overflow: "hidden",
      }}
    >
      <div style={{ display: "flex", alignItems: "center", gap: 20, padding: "18px 22px" }}>
        <div
          style={{
            flex: "none",
            width: 46,
            height: 46,
            borderRadius: 8,
            background: "var(--surface-2)",
            border: "1px solid var(--line)",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            fontFamily: "var(--serif)",
            fontSize: 19,
            color: "var(--graphite)",
          }}
        >
          {initial}
        </div>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ fontFamily: "var(--sans)", fontSize: 15, fontWeight: 500, color: "var(--ink)" }}>{s.display_name}</div>
          <div style={{ fontFamily: "var(--sans)", fontSize: 12.5, color: "var(--muted)", marginTop: 3 }}>
            {kind} · last gathered {formatAgo(s.last_sync)}
          </div>
          {issue ? (
            <div style={{ fontFamily: "var(--sans)", fontSize: 12, color: "var(--accent)", marginTop: 5, whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>
              Latest issue: {issue}
            </div>
          ) : null}
        </div>
        <div style={{ flex: "none", display: "flex", alignItems: "center", gap: 8, width: 104 }}>
          <span style={{ width: 7, height: 7, borderRadius: 99, background: sv.dot }} />
          <span style={{ fontFamily: "var(--sans)", fontSize: 13, color: sv.color }}>{sv.label}</span>
        </div>
        <div style={{ flex: "none", textAlign: "right", width: 96 }}>
          <div style={{ fontFamily: "var(--serif)", fontSize: 22, color: "var(--ink)", lineHeight: 1 }}>{weekCount(s)}</div>
          <div style={{ fontFamily: "var(--sans)", fontSize: 11, letterSpacing: "0.08em", textTransform: "uppercase", color: "var(--faint)", marginTop: 4 }}>this week</div>
        </div>
        <button
          aria-label="Stream settings"
          style={{
            flex: "none",
            appearance: "none",
            cursor: "pointer",
            width: 34,
            height: 34,
            borderRadius: 99,
            border: "1px solid var(--line)",
            background: "transparent",
            color: "var(--muted)",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
          }}
        >
          <DotsIcon />
        </button>
      </div>
      {s.id === "webgallery" && managedSources.length > 0 ? (
        <div style={{ padding: "0 22px 5px" }}>
          {managedSources.map((source) => (
            <WebGallerySourceRow
              key={source.id}
              source={source}
              busy={busy}
              onEdit={onEditWebGallery}
              onToggle={onToggleSource}
              onDelete={onDeleteSource}
            />
          ))}
        </div>
      ) : null}
    </div>
  );
}

function PreviewPanel({ preview, error }: { preview: ConnectorSourcePreviewResponse | null; error: string }) {
  if (error) {
    return (
      <div style={{ padding: 12, borderRadius: 6, border: "1px solid var(--accent-line)", background: "var(--accent-soft)", color: "var(--accent)", fontFamily: "var(--sans)", fontSize: 13, lineHeight: 1.45 }}>
        {error}
      </div>
    );
  }
  if (!preview) return null;
  return (
    <div style={{ padding: 12, borderRadius: 6, border: "1px solid var(--line)", background: "var(--surface-2)" }}>
      <div style={{ fontFamily: "var(--sans)", fontSize: 13, fontWeight: 700, color: "var(--ink)" }}>
        {preview.items_found.toLocaleString()} items found
      </div>
      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(112px, 1fr))", gap: 10, marginTop: 12 }}>
        {preview.sample.map((item) => (
          <div key={item.source_url} style={{ minWidth: 0 }}>
            <div style={{ aspectRatio: "1 / 1", borderRadius: 6, background: "var(--wall)", overflow: "hidden", border: "1px solid var(--line)" }}>
              {item.image_url ? <img src={item.image_url} alt={item.title || "Preview item"} style={{ width: "100%", height: "100%", objectFit: "cover", display: "block" }} /> : null}
            </div>
            <div style={{ marginTop: 6, fontFamily: "var(--sans)", fontSize: 12, color: "var(--ink)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
              {item.title || "Untitled"}
            </div>
            <div style={{ marginTop: 1, fontFamily: "var(--sans)", fontSize: 11.5, color: "var(--muted)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
              {item.artist || item.image_url || "No artist"}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function MobileWebGallerySources({
  sources,
  busy,
  onEdit,
  onToggle,
  onDelete,
}: {
  sources: ConnectorSourceSetting[];
  busy: boolean;
  onEdit: (source: ConnectorSourceSetting) => void;
  onToggle: (source: ConnectorSourceSetting) => void;
  onDelete: (source: ConnectorSourceSetting) => void;
}) {
  if (sources.length === 0) return null;
  return (
    <section style={{ padding: "18px 0 0" }}>
      <h2 style={{ margin: "0 0 4px", fontFamily: "var(--sans)", fontSize: 12, fontWeight: 800, letterSpacing: "0.12em", textTransform: "uppercase", color: "var(--faint)" }}>
        Web galleries
      </h2>
      {sources.map((source) => (
        <WebGallerySourceRow
          key={source.id}
          source={source}
          busy={busy}
          compact
          onEdit={onEdit}
          onToggle={onToggle}
          onDelete={onDelete}
        />
      ))}
    </section>
  );
}

function WebGalleryEditor({
  mode,
  isMobile,
  busy,
  onClose,
  onSaved,
}: {
  mode: EditorMode;
  isMobile: boolean;
  busy: boolean;
  onClose: () => void;
  onSaved: () => void;
}) {
  const source = mode.kind === "edit" ? mode.source : undefined;
  const [form, setForm] = useState<WebGalleryForm>(() => initialForm(source));
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [preview, setPreview] = useState<ConnectorSourcePreviewResponse | null>(null);
  const [previewError, setPreviewError] = useState("");
  const [localBusy, setLocalBusy] = useState(false);
  const [saveError, setSaveError] = useState("");
  const [previewKey, setPreviewKey] = useState<string>(() => (source?.enabled ? JSON.stringify(formConfig(initialForm(source))) : ""));
  const config = useMemo(() => formConfig(form), [form]);
  const currentKey = JSON.stringify(config);
  const previewOK = previewKey === currentKey && (preview != null || Boolean(source?.enabled));
  const canSaveEnabled = !form.enabled || previewOK;

  const setValue = <K extends keyof WebGalleryForm>(key: K, value: WebGalleryForm[K]) => {
    setForm((current) => ({ ...current, [key]: value }));
    if (key !== "label" && key !== "enabled" && key !== "targetFolioId" && key !== "showInLibrary") {
      setPreview(null);
      setPreviewError("");
      setPreviewKey("");
    }
  };

  const test = async () => {
    setPreviewError("");
    setSaveError("");
    setLocalBusy(true);
    try {
      const result = await previewConnectorSource({ config, limit: 6 });
      setPreview(result);
      setPreviewKey(currentKey);
    } catch (err) {
      setPreview(null);
      setPreviewKey("");
      setPreviewError(err instanceof Error ? err.message : "Preview failed");
    } finally {
      setLocalBusy(false);
    }
  };

  const save = async () => {
    setSaveError("");
    if (!canSaveEnabled) {
      setSaveError("Run a successful test before enabling this web gallery.");
      return;
    }
    const destinationError = destinationGateMessage(form.targetFolioId, form.showInLibrary);
    if (destinationError) {
      setSaveError(destinationError);
      return;
    }
    setLocalBusy(true);
    try {
      if (mode.kind === "edit") {
        await updateConnectorSource(mode.source.id, {
          type: "webgallery",
          label: form.label,
          config: { ...mode.source.config, ...config },
          enabled: form.enabled,
          target_folio_id: form.targetFolioId,
          show_in_library: form.showInLibrary,
        });
      } else {
        await createConnectorSource({
          type: "webgallery",
          label: form.label,
          config,
          enabled: form.enabled,
          target_folio_id: form.targetFolioId,
          show_in_library: form.showInLibrary,
        });
      }
      onSaved();
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : "Failed to save web gallery source");
    } finally {
      setLocalBusy(false);
    }
  };
  const destinationError = destinationGateMessage(form.targetFolioId, form.showInLibrary);
  const canBackfill =
    mode.kind === "edit" &&
    form.targetFolioId != null &&
    form.targetFolioId === (mode.source.target_folio_id ?? null) &&
    form.showInLibrary === (mode.source.show_in_library ?? true);

  const shellStyle: CSSProperties = isMobile
    ? {
        position: "fixed",
        left: 0,
        right: 0,
        bottom: 0,
        maxHeight: "92dvh",
        overflow: "auto",
        borderRadius: "18px 18px 0 0",
        background: "var(--surface)",
        padding: "18px 18px calc(18px + var(--safe-bottom))",
        boxShadow: "0 -18px 55px var(--shadow-2)",
      }
    : {
        width: "min(760px, calc(100vw - 40px))",
        maxHeight: "86vh",
        overflow: "auto",
        borderRadius: 8,
        background: "var(--surface)",
        padding: 22,
        boxShadow: "0 24px 70px var(--shadow-2)",
      };

  return (
    <div style={{ position: "fixed", inset: 0, zIndex: 50, display: "flex", alignItems: isMobile ? "flex-end" : "center", justifyContent: "center", background: "rgba(0,0,0,0.34)", padding: isMobile ? 0 : 20 }}>
      <div role="dialog" aria-modal="true" aria-label={mode.kind === "edit" ? "Edit web gallery" : "Add web gallery"} style={shellStyle}>
        <div style={{ display: "flex", alignItems: "start", justifyContent: "space-between", gap: 16 }}>
          <div>
            <div style={{ fontFamily: "var(--sans)", fontSize: 12, fontWeight: 800, letterSpacing: "0.12em", textTransform: "uppercase", color: "var(--faint)" }}>
              Web gallery
            </div>
            <h2 style={{ margin: "4px 0 0", fontFamily: "var(--serif)", fontSize: isMobile ? 24 : 28, fontWeight: 500, color: "var(--ink)", lineHeight: 1.05 }}>
              {mode.kind === "edit" ? "Edit source" : "Add source"}
            </h2>
          </div>
          <button type="button" onClick={onClose} style={{ ...smallButtonStyle, minWidth: 44, height: 44, padding: 0 }} aria-label="Close editor">
            ×
          </button>
        </div>

        <div style={{ display: "grid", gap: 14, marginTop: 20 }}>
          <div style={{ display: "grid", gridTemplateColumns: isMobile ? "1fr" : "minmax(0, 1fr) minmax(0, 1.35fr)", gap: 12 }}>
            <TextField label="Label" value={form.label} onChange={(value) => setValue("label", value)} placeholder="Weekend archive" />
            <TextField label="List URL" value={form.listURL} onChange={(value) => setValue("listURL", value)} placeholder="https://example.test/gallery" type="url" />
          </div>
          <div style={{ display: "grid", gridTemplateColumns: isMobile ? "1fr" : "minmax(0, 1fr) 150px", gap: 12, alignItems: "end" }}>
            <TextField label="Schedule" value={form.schedule} onChange={(value) => setValue("schedule", value)} placeholder="default, @every 6h, 0 */4 * * *" />
            <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", minHeight: 44, padding: "0 2px" }}>
              <span style={{ fontFamily: "var(--sans)", fontSize: 13, color: "var(--graphite)" }}>Enabled</span>
              <Toggle on={form.enabled} onClick={() => setValue("enabled", !form.enabled)} label="Enable web gallery source" />
            </div>
          </div>

          <button
            type="button"
            onClick={() => setAdvancedOpen((open) => !open)}
            style={{ minHeight: 44, border: "1px solid var(--line)", borderRadius: 6, background: "var(--surface-2)", color: "var(--ink)", fontFamily: "var(--sans)", fontSize: 14, fontWeight: 700, cursor: "pointer" }}
          >
            {advancedOpen ? "Hide advanced" : "Show advanced"}
          </button>

          {advancedOpen ? (
            <div style={{ display: "grid", gap: 14, padding: 14, border: "1px solid var(--line)", borderRadius: 6, background: "var(--surface-2)" }}>
              <Field label="Pagination">
                <select value={form.paginationStrategy} onChange={(e) => setValue("paginationStrategy", e.target.value as WebGalleryPaginationConfig["strategy"])} style={fieldBase}>
                  <option value="page_param">Page parameter</option>
                  <option value="next_link">Next link</option>
                  <option value="none">Single page</option>
                </select>
              </Field>
              {form.paginationStrategy === "page_param" ? (
                <div style={{ display: "grid", gridTemplateColumns: isMobile ? "1fr" : "minmax(0, 1fr) 140px", gap: 10 }}>
                  <TextField label="Page parameter" value={form.pageParam} onChange={(value) => setValue("pageParam", value)} placeholder="pager" />
                  <TextField label="Start index" value={form.startIndex} onChange={(value) => setValue("startIndex", value)} placeholder="1" />
                </div>
              ) : null}
              {form.paginationStrategy === "next_link" ? (
                <TextField label="Next link selector" value={form.nextLinkSelector} onChange={(value) => setValue("nextLinkSelector", value)} placeholder="a.next" />
              ) : null}
              <TextField label="Item link selector" value={form.itemLinkSelector} onChange={(value) => setValue("itemLinkSelector", value)} placeholder="div.photo-item a" />
              <SelectorPair label="Image selector" selector={form.imageSelector} attr={form.imageAttr} onSelector={(value) => setValue("imageSelector", value)} onAttr={(value) => setValue("imageAttr", value)} selectorPlaceholder="img#big_photo" attrPlaceholder="src" />
              <SelectorPair label="Title selector" selector={form.titleSelector} attr={form.titleAttr} onSelector={(value) => setValue("titleSelector", value)} onAttr={(value) => setValue("titleAttr", value)} selectorPlaceholder="h1[itemprop='name']" />
              <SelectorPair label="Artist selector" selector={form.artistSelector} attr={form.artistAttr} onSelector={(value) => setValue("artistSelector", value)} onAttr={(value) => setValue("artistAttr", value)} selectorPlaceholder="span[itemprop='name']" />
              <SelectorPair label="Date selector" selector={form.dateSelector} attr={form.dateAttr} onSelector={(value) => setValue("dateSelector", value)} onAttr={(value) => setValue("dateAttr", value)} selectorPlaceholder="meta[itemprop='datePublished']" attrPlaceholder="content" />
              <Field label="Item link filters">
                <textarea value={form.itemLinkFilter} onChange={(e) => setValue("itemLinkFilter", e.target.value)} rows={3} style={{ ...fieldBase, resize: "vertical" }} />
              </Field>
            </div>
          ) : null}

          <PreviewPanel preview={preview} error={previewError} />
          <DestinationControls
            targetFolioId={form.targetFolioId}
            showInLibrary={form.showInLibrary}
            onTargetFolioIdChange={(value) => setValue("targetFolioId", value)}
            onShowInLibraryChange={(value) => setValue("showInLibrary", value)}
            disabled={localBusy || busy}
            sourceId={mode.kind === "edit" ? mode.source.id : undefined}
            canBackfill={canBackfill}
            backfillBlockedMessage={mode.kind === "edit" ? "Save a target folio before backfill." : "Create and save this source before backfill."}
          />
          {!canSaveEnabled || destinationError || saveError ? (
            <div style={{ fontFamily: "var(--sans)", fontSize: 13, color: "var(--accent)", lineHeight: 1.45 }}>
              {saveError || destinationError || "Run a successful test before enabling this source."}
            </div>
          ) : null}
          <div style={{ display: "flex", justifyContent: "flex-end", gap: 10, flexWrap: "wrap" }}>
            <button type="button" onClick={test} disabled={localBusy || busy} style={{ ...smallButtonStyle, minHeight: 44, padding: "0 18px" }}>
              {localBusy ? "Working..." : "Test"}
            </button>
            <button type="button" onClick={save} disabled={localBusy || busy || !canSaveEnabled || Boolean(destinationError)} style={{ minHeight: 44, padding: "0 20px", border: "1px solid var(--accent)", borderRadius: 99, background: "var(--accent)", color: "var(--on-accent)", fontFamily: "var(--sans)", fontSize: 14, fontWeight: 800, cursor: "pointer", opacity: localBusy || busy || !canSaveEnabled || Boolean(destinationError) ? 0.55 : 1 }}>
              Save source
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

export default function Streams() {
  const { isMobile } = useViewport();
  const queryClient = useQueryClient();
  const [busyConnectors, setBusyConnectors] = useState<Record<string, boolean>>({});
  const [editor, setEditor] = useState<EditorMode | null>(null);
  const [menuOpen, setMenuOpen] = useState(false);
  const [notice, setNotice] = useState<StreamNotice | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<ConnectorSourceSetting | null>(null);
  const { data, isLoading, isError } = useQuery({
    queryKey: ["folio-connectors"],
    queryFn: fetchConnectorStatus,
  });
  const sourceSettings = useQuery({
    queryKey: ["connector-sources"],
    queryFn: () => fetchConnectorSources(""),
  });
  const connectors = data?.connectors ?? [];
  const settings = sourceSettings.data?.sources ?? [];
  const webGallerySources = settings.filter((source) => source.type === "webgallery");
  const webGalleryBusy = !!busyConnectors.webgallery || sourceSettings.isLoading;

  const reload = () => {
    void queryClient.invalidateQueries({ queryKey: ["connector-sources"] });
    void queryClient.invalidateQueries({ queryKey: ["folio-connectors"] });
  };

  const toggleConnector = (connector: ConnectorStatus, sources: ConnectorSourceSetting[]) => {
    if (sources.length === 0 || busyConnectors[connector.id]) return;
    const enabled = sources.some((source) => source.enabled);
    const nextEnabled = !enabled;
    if (connector.id === "webgallery" && nextEnabled) {
      setNotice({ tone: "guidance", message: "Open a web gallery source, run Test, then save it enabled." });
      return;
    }
    const previous = queryClient.getQueryData<ConnectorSourcesResponse>(["connector-sources"]);
    queryClient.setQueryData<ConnectorSourcesResponse>(["connector-sources"], (current) => {
      if (!current) return current;
      return {
        sources: current.sources.map((source) => (source.type === connector.id ? { ...source, enabled: nextEnabled } : source)),
      };
    });
    setBusyConnectors((current) => ({ ...current, [connector.id]: true }));
    Promise.all(
      sources.map((source) =>
        updateConnectorSource(source.id, {
          type: source.type,
          chat_id: source.chat_id,
          label: source.label,
          config: source.config ?? undefined,
          enabled: nextEnabled,
          target_folio_id: source.target_folio_id ?? null,
          show_in_library: source.show_in_library ?? true,
        }),
      ),
    )
      .then(reload)
      .catch(() => {
        if (previous) queryClient.setQueryData(["connector-sources"], previous);
        reload();
        setNotice({ tone: "error", message: "Some stream sources could not be updated. The source list was refreshed." });
      })
      .finally(() => {
        setBusyConnectors((current) => {
          const next = { ...current };
          delete next[connector.id];
          return next;
        });
      });
  };

  const toggleSource = (source: ConnectorSourceSetting) => {
    if (source.type === "webgallery" && !source.enabled) {
      setEditor({ kind: "edit", source });
      return;
    }
    setBusyConnectors((current) => ({ ...current, [source.type]: true }));
    updateConnectorSource(source.id, {
      type: source.type,
      chat_id: source.chat_id,
      label: source.label,
      config: source.config ?? undefined,
      enabled: !source.enabled,
      target_folio_id: source.target_folio_id ?? null,
      show_in_library: source.show_in_library ?? true,
    })
      .then(reload)
      .catch(() => {
        reload();
        setNotice({ tone: "error", message: "The stream source could not be updated." });
      })
      .finally(() => {
        setBusyConnectors((current) => {
          const next = { ...current };
          delete next[source.type];
          return next;
        });
      });
  };

  const removeSource = (source: ConnectorSourceSetting) => {
    setDeleteTarget(source);
  };

  const confirmRemoveSource = async (source: ConnectorSourceSetting) => {
    setBusyConnectors((current) => ({ ...current, [source.type]: true }));
    try {
      await deleteConnectorSource(source.id);
      reload();
    } catch {
      reload();
      setNotice({ tone: "error", message: "The stream source could not be deleted." });
    } finally {
      setBusyConnectors((current) => {
        const next = { ...current };
        delete next[source.type];
        return next;
      });
    }
  };

  const confirmation = deleteTarget ? (
    <ConfirmationDialog
      eyebrow="Delete source"
      title={`Delete ${sourceTitle(deleteTarget)}?`}
      description="This stream source will stop gathering pieces. Pieces already in your gallery stay where they are."
      confirmLabel="Delete"
      busyLabel="Deleting"
      destructive
      onCancel={() => setDeleteTarget(null)}
      onConfirm={async () => {
        const source = deleteTarget;
        setDeleteTarget(null);
        await confirmRemoveSource(source);
      }}
    />
  ) : null;

  const addAction = (
    <div style={{ position: "relative" }}>
      <OutlineButton onClick={() => setMenuOpen((open) => !open)}>
        <span style={{ display: "inline-flex", alignItems: "center", gap: 8 }}>
          <PlusIcon size={13} /> Add a stream
        </span>
      </OutlineButton>
      {menuOpen ? (
        <div style={{ position: "absolute", right: 0, top: "calc(100% + 8px)", zIndex: 10, minWidth: 190, padding: 6, border: "1px solid var(--line)", borderRadius: 8, background: "var(--surface)", boxShadow: "0 16px 48px var(--shadow-2)" }}>
          <button type="button" onClick={() => { setMenuOpen(false); setEditor({ kind: "create" }); }} style={{ ...smallButtonStyle, width: "100%", borderRadius: 6, textAlign: "left" }}>
            Add web gallery
          </button>
          <div style={{ padding: "8px 10px", fontFamily: "var(--sans)", fontSize: 12, color: "var(--muted)" }}>
            Telegram sources stay in Settings.
          </div>
        </div>
      ) : null}
    </div>
  );

  if (isMobile) {
    return (
      <div>
        <div style={{ marginTop: 2, display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12 }}>
          <div style={{ fontFamily: "var(--sans)", fontSize: 13, color: "var(--muted)" }}>
            Sources with routing for Folios, Gallery, or review
          </div>
          <button
            type="button"
            onClick={() => setEditor({ kind: "create" })}
            style={{
              width: 44,
              height: 44,
              border: "1px solid var(--accent)",
              borderRadius: 99,
              background: "var(--accent)",
              color: "var(--on-accent)",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
            }}
            aria-label="Add web gallery"
          >
            <PlusIcon size={18} />
          </button>
        </div>
        {notice ? <div style={{ marginTop: 14 }}><StreamNoticeBanner notice={notice} onDismiss={() => setNotice(null)} /></div> : null}
        <section style={{ padding: "18px 0 0" }}>
          {isError ? (
            <div style={{ padding: "60px 0", fontFamily: "var(--serif)", fontSize: 20, color: "var(--graphite)" }}>
              The streams could not be reached.
            </div>
          ) : isLoading ? (
            <div style={{ padding: "60px 0", fontFamily: "var(--sans)", fontSize: 14, color: "var(--muted)" }}>Loading streams...</div>
          ) : connectors.length === 0 ? (
            <div style={{ padding: "60px 0", fontFamily: "var(--serif)", fontSize: 20, color: "var(--graphite)" }}>
              No streams yet.
            </div>
          ) : (
            connectors.map((s) => (
              <MobileStreamCard
                key={s.id}
                s={s}
                settings={settings}
                busy={!!busyConnectors[s.id] || sourceSettings.isLoading}
                onToggle={toggleConnector}
              />
            ))
          )}
        </section>
        <MobileWebGallerySources
          sources={webGallerySources}
          busy={webGalleryBusy}
          onEdit={(source) => setEditor({ kind: "edit", source })}
          onToggle={toggleSource}
          onDelete={removeSource}
        />
        {editor ? <WebGalleryEditor mode={editor} isMobile={isMobile} busy={webGalleryBusy} onClose={() => setEditor(null)} onSaved={() => { setEditor(null); reload(); }} /> : null}
        {confirmation}
      </div>
    );
  }

  return (
    <div>
      <PageHeader
        eyebrow="Streams · backstage"
        title="Where pieces come in"
        subcopy="The quiet machinery behind incoming pieces, routing, and visibility."
        action={addAction}
      />
      <section style={{ maxWidth: 920, padding: "34px 0 0", display: "flex", flexDirection: "column", gap: 12 }}>
        {notice ? <StreamNoticeBanner notice={notice} onDismiss={() => setNotice(null)} /> : null}
        {isError ? (
          <div style={{ padding: "60px 0", fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 20, color: "var(--graphite)" }}>
            The streams could not be reached.
          </div>
        ) : isLoading ? (
          <div style={{ padding: "60px 0", fontFamily: "var(--sans)", fontSize: 14, color: "var(--muted)" }}>Loading streams...</div>
        ) : connectors.length === 0 ? (
          <div style={{ padding: "60px 0", fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 20, color: "var(--graphite)" }}>
            No streams yet.
          </div>
        ) : (
          connectors.map((s) => (
            <StreamRow
              key={s.id}
              s={s}
              settings={settings}
              busy={!!busyConnectors[s.id] || sourceSettings.isLoading}
              onEditWebGallery={(source) => setEditor({ kind: "edit", source })}
              onToggleSource={toggleSource}
              onDeleteSource={removeSource}
            />
          ))
        )}
        <p style={{ fontFamily: "var(--sans)", fontSize: 12.5, color: "var(--faint)", margin: "14px 2px 0", lineHeight: 1.6 }}>
          Streams run on their own schedule. Depending on their destination, new pieces can route to a Folio, appear in the
          Gallery for Library browsing, wait in Inbox review, or stay hidden from the Gallery. No source defines the collection -
          it is yours.
        </p>
      </section>
      {editor ? <WebGalleryEditor mode={editor} isMobile={isMobile} busy={webGalleryBusy} onClose={() => setEditor(null)} onSaved={() => { setEditor(null); reload(); }} /> : null}
      {confirmation}
    </div>
  );
}
