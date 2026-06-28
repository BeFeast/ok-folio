import type { CSSProperties } from "react";
import { useFolio, type ToastStatus } from "./context";

function Spinner() {
  return (
    <span
      style={{
        width: 16,
        height: 16,
        borderRadius: 99,
        border: "2px solid var(--line-2)",
        borderTopColor: "var(--accent)",
        display: "inline-block",
        animation: "okf-spin 0.7s linear infinite",
      }}
    />
  );
}

function CheckGlyph() {
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
      <path d="M3.2 8.4l3 3 6.6-7" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function AlertGlyph() {
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
      <path d="M8 4.4v4.2" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
      <circle cx="8" cy="11.4" r="0.95" fill="currentColor" />
    </svg>
  );
}

function glyph(status: ToastStatus) {
  if (status === "loading") return <Spinner />;
  if (status === "success") return <CheckGlyph />;
  return <AlertGlyph />;
}

const CARD: CSSProperties = {
  pointerEvents: "auto",
  display: "flex",
  alignItems: "center",
  gap: 12,
  minWidth: 240,
  maxWidth: 360,
  padding: "13px 16px",
  borderRadius: 12,
  background: "var(--surface)",
  border: "1px solid var(--line)",
  boxShadow: "0 18px 50px rgba(0,0,0,0.22)",
  animation: "okf-rise 0.28s ease",
};

export default function Toaster() {
  const { toasts, dismissToast } = useFolio();
  if (toasts.length === 0) return null;

  return (
    <div
      style={{
        position: "fixed",
        right: 24,
        bottom: 24,
        zIndex: 200,
        display: "flex",
        flexDirection: "column",
        gap: 10,
        pointerEvents: "none",
      }}
    >
      {toasts.map((t) => {
        const dismissable = t.status !== "loading";
        return (
          <div
            key={t.id}
            role="status"
            onClick={() => dismissable && dismissToast(t.id)}
            title={dismissable ? "Dismiss" : undefined}
            style={{ ...CARD, cursor: dismissable ? "pointer" : "default" }}
          >
            <span
              style={{
                flex: "none",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                width: 18,
                height: 18,
                color: t.status === "error" ? "var(--danger, #b42318)" : "var(--accent)",
              }}
            >
              {glyph(t.status)}
            </span>
            <div style={{ flex: 1, minWidth: 0 }}>
              <div style={{ fontFamily: "var(--sans)", fontSize: 13.5, fontWeight: 500, color: "var(--ink)" }}>{t.title}</div>
              {t.detail ? (
                <div
                  style={{
                    fontFamily: "var(--sans)",
                    fontSize: 12,
                    color: "var(--muted)",
                    marginTop: 2,
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    whiteSpace: "nowrap",
                  }}
                >
                  {t.detail}
                </div>
              ) : null}
            </div>
          </div>
        );
      })}
    </div>
  );
}
