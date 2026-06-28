import { useMemo, useRef, useState, type ReactNode } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { useFolio } from "./context";
import { useViewport } from "./useViewport";
import { BrandMark, CloseIcon, Hov, MoonIcon, PlusIcon, SearchIcon } from "./ui";

interface NavItem {
  label: string;
  path: string;
  badge?: number;
}

const inactiveTab = "color-mix(in srgb, var(--ink) 50%, transparent)";

function TabIcon({ label }: { label: string }) {
  const common = {
    width: 22,
    height: 22,
    viewBox: "0 0 24 24",
    fill: "none",
    stroke: "currentColor",
    strokeWidth: 1.75,
    strokeLinecap: "round" as const,
    strokeLinejoin: "round" as const,
    style: { display: "block" },
  };

  if (label === "Gallery") {
    return (
      <svg {...common}>
        <rect x="4" y="5" width="16" height="14" rx="1.8" />
        <path d="M8 15.5 11 12.5 13.4 14.7 15.5 12.6 20 17" />
        <circle cx="9" cy="9" r="1.2" />
      </svg>
    );
  }
  if (label === "Folios") {
    return (
      <svg {...common}>
        <path d="M7 5.5h10a2 2 0 0 1 2 2v10" />
        <rect x="4" y="8" width="13" height="12" rx="1.8" />
      </svg>
    );
  }
  if (label === "Inbox") {
    return (
      <svg {...common}>
        <path d="M5 7.5h14l-1.3 9.2a2 2 0 0 1-2 1.8H8.3a2 2 0 0 1-2-1.8L5 7.5Z" />
        <path d="M8.5 12.5h2.2a1.8 1.8 0 0 0 2.6 0h2.2" />
      </svg>
    );
  }
  if (label === "Streams") {
    return (
      <svg {...common}>
        <path d="M5 8.2a11 11 0 0 1 14 0" />
        <path d="M8 11.5a6.5 6.5 0 0 1 8 0" />
        <path d="M11 15a2.2 2.2 0 0 1 2 0" />
        <circle cx="12" cy="18" r="1" fill="currentColor" stroke="none" />
      </svg>
    );
  }
  return (
    <svg {...common}>
      <circle cx="12" cy="12" r="3.2" />
      <path d="M12 3.8v2M12 18.2v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M3.8 12h2M18.2 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" />
    </svg>
  );
}

function screenTitle(pathname: string): string {
  if (pathname === "/") return "Gallery";
  if (pathname.startsWith("/folios")) return "Folios";
  if (pathname.startsWith("/inbox")) return "Inbox";
  if (pathname.startsWith("/streams")) return "Streams";
  if (pathname.startsWith("/settings")) return "Settings";
  return "OK Folio";
}

function IconButton({
  label,
  onClick,
  filled = false,
  children,
}: {
  label: string;
  onClick: () => void;
  filled?: boolean;
  children: ReactNode;
}) {
  return (
    <Hov
      as="button"
      onClick={onClick}
      aria-label={label}
      title={label}
      style={{
        appearance: "none",
        width: 40,
        height: 40,
        flex: "0 0 40px",
        borderRadius: 99,
        border: filled ? "1px solid var(--accent)" : "1px solid var(--line)",
        background: filled ? "var(--accent)" : "var(--surface)",
        color: filled ? "var(--on-accent)" : "var(--graphite)",
        cursor: "pointer",
        display: "inline-flex",
        alignItems: "center",
        justifyContent: "center",
      }}
      hover={filled ? { filter: "brightness(0.98)" } : { borderColor: "var(--line-2)", color: "var(--ink)" }}
    >
      {children}
    </Hov>
  );
}

export default function Nav() {
  const navigate = useNavigate();
  const location = useLocation();
  const { query, setQuery, toggleTheme, openAdd, inboxCount } = useFolio();
  const { isMobile } = useViewport();
  const [mobileSearchOpen, setMobileSearchOpen] = useState(false);
  const mobileSearchRef = useRef<HTMLInputElement | null>(null);

  const items: NavItem[] = useMemo(
    () => [
      { label: "Gallery", path: "/" },
      { label: "Folios", path: "/folios" },
      { label: "Inbox", path: "/inbox", badge: inboxCount },
      { label: "Streams", path: "/streams" },
      { label: "Settings", path: "/settings" },
    ],
    [inboxCount],
  );

  const isActive = (path: string) =>
    path === "/" ? location.pathname === "/" : location.pathname.startsWith(path);

  if (isMobile) {
    return (
      <>
        <nav
          style={{
            position: "sticky",
            top: 0,
            zIndex: 60,
            padding: "calc(var(--safe-top) + 10px) calc(20px + var(--safe-right)) 10px calc(20px + var(--safe-left))",
            background: "color-mix(in srgb, var(--bg) 88%, transparent)",
            backdropFilter: "saturate(1.12) blur(18px)",
            WebkitBackdropFilter: "saturate(1.12) blur(18px)",
            borderBottom: "1px solid color-mix(in srgb, var(--ink) 10%, transparent)",
          }}
        >
          {mobileSearchOpen ? (
            <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
              <div
                style={{
                  minWidth: 0,
                  flex: 1,
                  display: "flex",
                  alignItems: "center",
                  gap: 8,
                  height: 44,
                  padding: "0 12px",
                  border: "1px solid var(--accent)",
                  borderRadius: 13,
                  background: "var(--surface)",
                  boxShadow: "0 0 0 4px rgba(124,36,32,.08)",
                }}
              >
                <SearchIcon />
                <input
                  ref={mobileSearchRef}
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  placeholder="Search pieces"
                  autoFocus
                  style={{
                    appearance: "none",
                    minWidth: 0,
                    flex: 1,
                    border: 0,
                    outline: 0,
                    background: "transparent",
                    fontFamily: "var(--sans)",
                    fontSize: 15,
                    color: "var(--ink)",
                  }}
                />
                {query ? (
                  <Hov
                    as="button"
                    onClick={() => {
                      setQuery("");
                      mobileSearchRef.current?.focus();
                    }}
                    aria-label="Clear search"
                    title="Clear search"
                    style={{
                      appearance: "none",
                      cursor: "pointer",
                      flex: "none",
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "center",
                      border: 0,
                      padding: 0,
                      background: "transparent",
                      color: "var(--muted)",
                    }}
                    hover={{ color: "var(--ink)" }}
                  >
                    <CloseIcon size={15} />
                  </Hov>
                ) : null}
              </div>
              <Hov
                as="button"
                onClick={() => setMobileSearchOpen(false)}
                style={{
                  appearance: "none",
                  border: 0,
                  background: "transparent",
                  color: "var(--accent)",
                  cursor: "pointer",
                  fontFamily: "var(--sans)",
                  fontSize: 14.5,
                  fontWeight: 600,
                  padding: "8px 0 8px 2px",
                }}
                hover={{ color: "var(--ink)" }}
              >
                Cancel
              </Hov>
            </div>
          ) : (
            <div style={{ display: "flex", alignItems: "center", gap: 12, minHeight: 42 }}>
              <h1
                style={{
                  margin: 0,
                  minWidth: 0,
                  flex: 1,
                  fontFamily: "var(--serif)",
                  fontWeight: 500,
                  fontSize: 26,
                  lineHeight: 1,
                  color: "var(--ink)",
                }}
              >
                {screenTitle(location.pathname)}
              </h1>
              <IconButton label="Search" onClick={() => setMobileSearchOpen(true)}>
                <SearchIcon />
              </IconButton>
              <IconButton label="Toggle theme" onClick={toggleTheme}>
                <MoonIcon />
              </IconButton>
              <IconButton label="Add piece" onClick={openAdd} filled>
                <PlusIcon size={17} />
              </IconButton>
            </div>
          )}
        </nav>

        <nav
          aria-label="Primary"
          style={{
            position: "fixed",
            left: 0,
            right: 0,
            bottom: 0,
            zIndex: 70,
            height: "calc(var(--mobile-tab-height) + var(--safe-bottom))",
            padding: "7px calc(10px + var(--safe-right)) calc(7px + var(--safe-bottom)) calc(10px + var(--safe-left))",
            display: "grid",
            gridTemplateColumns: "repeat(5, minmax(0, 1fr))",
            alignItems: "start",
            background: "color-mix(in srgb, var(--bg) 82%, transparent)",
            backdropFilter: "blur(18px)",
            WebkitBackdropFilter: "blur(18px)",
            borderTop: "1px solid color-mix(in srgb, var(--ink) 11%, transparent)",
          }}
        >
          {items.map((n) => {
            const active = isActive(n.path);
            const color = active ? "var(--accent)" : inactiveTab;
            return (
              <button
                key={n.path}
                type="button"
                onClick={() => navigate(n.path)}
                aria-current={active ? "page" : undefined}
                style={{
                  appearance: "none",
                  border: 0,
                  background: "transparent",
                  color,
                  minWidth: 0,
                  minHeight: 56,
                  padding: "5px 2px 0",
                  cursor: "pointer",
                  display: "flex",
                  flexDirection: "column",
                  alignItems: "center",
                  justifyContent: "flex-start",
                  gap: 3,
                  fontFamily: "var(--sans)",
                  fontSize: 10,
                  fontWeight: active ? 600 : 500,
                  lineHeight: 1.05,
                }}
              >
                <span style={{ position: "relative", display: "inline-flex", alignItems: "center", justifyContent: "center", height: 24 }}>
                  <TabIcon label={n.label} />
                  {n.badge ? (
                    <span
                      style={{
                        position: "absolute",
                        top: -5,
                        right: -10,
                        minWidth: 17,
                        height: 17,
                        padding: "0 4px",
                        display: "inline-flex",
                        alignItems: "center",
                        justifyContent: "center",
                        borderRadius: 99,
                        border: "1.5px solid var(--bg)",
                        background: "var(--accent)",
                        color: "var(--on-accent)",
                        fontFamily: "var(--sans)",
                        fontSize: 10,
                        fontWeight: 700,
                        lineHeight: 1,
                      }}
                    >
                      {n.badge}
                    </span>
                  ) : null}
                </span>
                <span style={{ overflow: "hidden", textOverflow: "ellipsis", maxWidth: "100%" }}>{n.label}</span>
              </button>
            );
          })}
        </nav>
      </>
    );
  }

  return (
    <nav
      style={{
        position: "sticky",
        top: 0,
        zIndex: 50,
        display: "flex",
        alignItems: "center",
        gap: 30,
        padding: "0 30px",
        height: 64,
        background: "color-mix(in srgb, var(--bg) 86%, transparent)",
        backdropFilter: "saturate(1.15) blur(16px)",
        WebkitBackdropFilter: "saturate(1.15) blur(16px)",
        borderBottom: "1px solid var(--line)",
      }}
    >
      <div
        onClick={() => navigate("/")}
        style={{ display: "flex", alignItems: "center", gap: 11, cursor: "pointer" }}
      >
        <BrandMark />
        <span
          style={{
            fontFamily: "var(--serif)",
            fontSize: 19,
            letterSpacing: "0.1px",
            display: "inline-flex",
            gap: "0.26em",
          }}
        >
          <span style={{ color: "var(--graphite)", fontWeight: 500 }}>OK</span>
          <span style={{ color: "var(--ink)", fontWeight: 500 }}>Folio</span>
        </span>
      </div>

      <div style={{ display: "flex", alignItems: "center", gap: 2 }}>
        {items.map((n) => {
          const active = isActive(n.path);
          return (
            <Hov
              as="button"
              key={n.path}
              onClick={() => navigate(n.path)}
              style={{
                appearance: "none",
                background: "transparent",
                border: 0,
                cursor: "pointer",
                fontFamily: "var(--sans)",
                fontSize: 14.5,
                padding: "9px 13px",
                color: active ? "var(--ink)" : "var(--graphite)",
                position: "relative",
                letterSpacing: "0.1px",
              }}
              hover={active ? undefined : { color: "var(--ink)" }}
            >
              {n.label}
              {n.badge ? (
                <span
                  style={{
                    display: "inline-flex",
                    alignItems: "center",
                    justifyContent: "center",
                    minWidth: 17,
                    height: 17,
                    padding: "0 4px",
                    marginLeft: 6,
                    fontSize: 10.5,
                    fontWeight: 600,
                    borderRadius: 99,
                    background: "var(--accent)",
                    color: "var(--on-accent)",
                    verticalAlign: 1.5,
                  }}
                >
                  {n.badge}
                </span>
              ) : null}
              <span
                style={{
                  position: "absolute",
                  left: 13,
                  right: 13,
                  bottom: -1,
                  height: 2,
                  background: active ? "var(--accent)" : "transparent",
                }}
              />
            </Hov>
          );
        })}
      </div>

      <div style={{ marginLeft: "auto", display: "flex", alignItems: "center", gap: 14 }}>
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: 8,
            padding: "7px 12px",
            border: "1px solid var(--line)",
            borderRadius: 99,
            background: "var(--surface)",
          }}
        >
          <SearchIcon />
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search pieces"
            style={{
              appearance: "none",
              border: 0,
              outline: 0,
              background: "transparent",
              fontFamily: "var(--sans)",
              fontSize: 13.5,
              color: "var(--ink)",
              width: 128,
            }}
          />
          {query ? (
            <Hov
              as="button"
              onClick={() => setQuery("")}
              aria-label="Clear search"
              title="Clear search"
              style={{
                appearance: "none",
                cursor: "pointer",
                flex: "none",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                border: 0,
                padding: 0,
                background: "transparent",
                color: "var(--muted)",
              }}
              hover={{ color: "var(--ink)" }}
            >
              <CloseIcon size={13} />
            </Hov>
          ) : null}
        </div>
        <Hov
          as="button"
          onClick={toggleTheme}
          aria-label="Toggle theme"
          style={{
            appearance: "none",
            width: 36,
            height: 36,
            borderRadius: 99,
            border: "1px solid var(--line)",
            background: "var(--surface)",
            cursor: "pointer",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            color: "var(--graphite)",
          }}
          hover={{ borderColor: "var(--line-2)" }}
        >
          <MoonIcon />
        </Hov>
        <Hov
          as="button"
          onClick={openAdd}
          style={{
            appearance: "none",
            cursor: "pointer",
            fontFamily: "var(--sans)",
            fontSize: 13.5,
            fontWeight: 500,
            letterSpacing: "0.1px",
            padding: "9px 16px",
            borderRadius: 99,
            border: "1px solid var(--accent-line)",
            background: "transparent",
            color: "var(--accent)",
            display: "flex",
            alignItems: "center",
            gap: 7,
          }}
          hover={{ background: "var(--accent-soft)" }}
        >
          <PlusIcon />
          Add Piece
        </Hov>
      </div>
    </nav>
  );
}
