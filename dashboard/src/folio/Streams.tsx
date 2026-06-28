import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { fetchConnectorStatus } from "../api";
import type { ConnectorStatus } from "../types";
import { DotsIcon, OutlineButton, PageHeader } from "./ui";
import { useViewport } from "./useViewport";

function formatAgo(iso: string | null): string {
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
  const id = `${s.id} ${s.display_name}`.toLowerCase();
  if (id.includes("rss")) return "RSS";
  if (id.includes("manual")) return "Manual";
  return "API";
}

function Toggle({ on, onClick, label }: { on: boolean; onClick: () => void; label: string }) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={on}
      aria-label={label}
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

function MobileStreamCard({ s }: { s: ConnectorStatus }) {
  const initial = (s.display_name || "?").trim().charAt(0).toUpperCase() || "?";
  const [enabled, setEnabled] = useState(() => s.health !== "idle" && s.health !== "error");
  const pieces = s.counts?.total ?? s.counts?.downloaded ?? 0;
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
      </div>
      <Toggle on={enabled} onClick={() => setEnabled((current) => !current)} label={`Toggle ${s.display_name}`} />
    </article>
  );
}

function StreamRow({ s }: { s: ConnectorStatus }) {
  const sv = statusView(s.health);
  const initial = (s.display_name || "?").trim().charAt(0).toUpperCase() || "?";
  const sourceCount = s.sources?.length ?? 0;
  const kind = sourceCount > 1 ? `Connector · ${sourceCount} sources` : "Connector";
  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        gap: 20,
        padding: "18px 22px",
        border: "1px solid var(--line)",
        borderRadius: 6,
        background: "var(--surface)",
      }}
    >
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
  );
}

export default function Streams() {
  const { isMobile } = useViewport();
  const { data, isLoading, isError } = useQuery({
    queryKey: ["folio-connectors"],
    queryFn: fetchConnectorStatus,
  });
  const connectors = data?.connectors ?? [];

  if (isMobile) {
    return (
      <div>
        <div style={{ marginTop: 2, display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12 }}>
          <div style={{ fontFamily: "var(--sans)", fontSize: 13, color: "var(--muted)" }}>Sources that fill your folio</div>
          <button
            type="button"
            style={{
              width: 38,
              height: 38,
              border: "1px solid var(--accent)",
              borderRadius: 99,
              background: "var(--accent)",
              color: "var(--on-accent)",
              fontFamily: "var(--sans)",
              fontSize: 23,
              lineHeight: 1,
            }}
            aria-label="Add stream"
          >
            +
          </button>
        </div>
        <section style={{ padding: "18px 0 0" }}>
          {isError ? (
            <div style={{ padding: "60px 0", fontFamily: "var(--serif)", fontSize: 20, color: "var(--graphite)" }}>
              The streams could not be reached.
            </div>
          ) : isLoading ? (
            <div style={{ padding: "60px 0", fontFamily: "var(--sans)", fontSize: 14, color: "var(--muted)" }}>Loading streams…</div>
          ) : connectors.length === 0 ? (
            <div style={{ padding: "60px 0", fontFamily: "var(--serif)", fontSize: 20, color: "var(--graphite)" }}>
              No streams yet.
            </div>
          ) : (
            connectors.map((s) => <MobileStreamCard key={s.id} s={s} />)
          )}
        </section>
      </div>
    );
  }

  return (
    <div>
      <PageHeader
        eyebrow="Streams · backstage"
        title="Where pieces come in"
        subcopy="The quiet machinery behind the gallery. Tend it when you need to."
        action={<OutlineButton>Add a stream</OutlineButton>}
      />
      <section style={{ maxWidth: 920, padding: "34px 0 0", display: "flex", flexDirection: "column", gap: 12 }}>
        {isError ? (
          <div style={{ padding: "60px 0", fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 20, color: "var(--graphite)" }}>
            The streams could not be reached.
          </div>
        ) : isLoading ? (
          <div style={{ padding: "60px 0", fontFamily: "var(--sans)", fontSize: 14, color: "var(--muted)" }}>Loading streams…</div>
        ) : connectors.length === 0 ? (
          <div style={{ padding: "60px 0", fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 20, color: "var(--graphite)" }}>
            No streams yet.
          </div>
        ) : (
          connectors.map((s) => <StreamRow key={s.id} s={s} />)
        )}
        <p style={{ fontFamily: "var(--sans)", fontSize: 12.5, color: "var(--faint)", margin: "14px 2px 0", lineHeight: 1.6 }}>
          Streams run on their own schedule. New pieces land in your Inbox for review before they settle into the Gallery. No
          source defines the collection — it is yours.
        </p>
      </section>
    </div>
  );
}
