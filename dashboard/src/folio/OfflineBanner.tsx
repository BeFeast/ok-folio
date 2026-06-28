import { useOnlineStatus } from "../hooks/useOnlineStatus";
import { useViewport } from "./useViewport";

function OfflineIcon({ size = 20 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M2.5 8.5a17.5 17.5 0 0 1 19 0" />
      <path d="M6 12a11.5 11.5 0 0 1 12 0" />
      <path d="M9.8 15.5a5.2 5.2 0 0 1 4.4 0" />
      <path d="M12 19h.01" />
      <path d="M4 4l16 16" />
    </svg>
  );
}

export default function OfflineBanner() {
  const online = useOnlineStatus();
  const { isMobile } = useViewport();

  if (online) return null;

  return (
    <div
      role="status"
      aria-live="polite"
      style={{
        maxWidth: 1340,
        margin: "0 auto",
        padding: isMobile
          ? "10px calc(20px + var(--safe-right)) 0 calc(20px + var(--safe-left))"
          : "14px 30px 0",
      }}
    >
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: 12,
          border: "1px solid var(--line)",
          borderRadius: 14,
          background: "var(--surface)",
          color: "var(--ink)",
          padding: isMobile ? "11px 13px" : "12px 15px",
          boxShadow: "0 8px 24px var(--shadow)",
        }}
      >
        <span style={{ color: "var(--accent)", flex: "none", display: "inline-flex" }}>
          <OfflineIcon />
        </span>
        <div style={{ minWidth: 0 }}>
          <div style={{ fontFamily: "var(--sans)", fontSize: 14, fontWeight: 800 }}>
            You're offline
          </div>
          <div style={{ marginTop: 2, fontFamily: "var(--sans)", fontSize: 12.5, lineHeight: 1.35, color: "var(--graphite)" }}>
            Showing your saved pieces — new imports resume when you reconnect.
          </div>
        </div>
      </div>
    </div>
  );
}
