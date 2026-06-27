import { useLocation, useNavigate } from "react-router-dom";
import { useFolio } from "./context";
import { BrandMark, Hov, MoonIcon, PlusIcon, SearchIcon } from "./ui";

interface NavItem {
  label: string;
  path: string;
  badge?: number;
}

export default function Nav() {
  const navigate = useNavigate();
  const location = useLocation();
  const { query, setQuery, toggleTheme, openAdd } = useFolio();

  const items: NavItem[] = [
    { label: "Gallery", path: "/" },
    { label: "Folios", path: "/folios" },
    { label: "Inbox", path: "/inbox", badge: 0 },
    { label: "Streams", path: "/streams" },
    { label: "Settings", path: "/settings" },
  ];

  const isActive = (path: string) =>
    path === "/" ? location.pathname === "/" : location.pathname.startsWith(path);

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
