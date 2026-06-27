// Shared presentational primitives for the OK Folio surfaces.
// Inline styles are used deliberately: the canonical Claude Design expresses
// every surface as inline style + CSS custom properties, so porting them
// verbatim keeps the implementation pixel-faithful to the source.

import {
  useState,
  type CSSProperties,
  type ElementType,
  type ReactNode,
} from "react";

type HovProps = {
  as?: ElementType;
  style?: CSSProperties;
  hover?: CSSProperties;
  focus?: CSSProperties;
  children?: ReactNode;
} & Record<string, unknown>;

/**
 * Hov renders an element with base inline styles and merges `hover`/`focus`
 * deltas on the matching pointer/focus state — the React equivalent of the
 * design's `style-hover` / `style-focus` attributes.
 */
export function Hov({
  as: Tag = "div",
  style,
  hover,
  focus,
  children,
  ...rest
}: HovProps) {
  const [h, setH] = useState(false);
  const [f, setF] = useState(false);
  const merged: CSSProperties = {
    ...style,
    ...(h && hover ? hover : null),
    ...(f && focus ? focus : null),
  };
  return (
    <Tag
      style={merged}
      onMouseEnter={() => hover && setH(true)}
      onMouseLeave={() => hover && setH(false)}
      onFocus={focus ? () => setF(true) : undefined}
      onBlur={focus ? () => setF(false) : undefined}
      {...rest}
    >
      {children}
    </Tag>
  );
}

type OkfImageProps = {
  src: string;
  alt: string;
  title: string;
  artist?: string;
  /** Style for the <img> element. */
  imgStyle?: CSSProperties;
  /** Style for the matte fallback shown when the image fails to load. */
  matteStyle?: CSSProperties;
  matteTitleStyle?: CSSProperties;
  matteArtistStyle?: CSSProperties;
  loading?: "eager" | "lazy";
  onClick?: (e: React.MouseEvent) => void;
};

/**
 * OkfImage shows the artwork and, on load failure, swaps to a typographic
 * matte (title + artist) exactly as the design does for missing originals.
 */
export function OkfImage({
  src,
  alt,
  title,
  artist,
  imgStyle,
  matteStyle,
  matteTitleStyle,
  matteArtistStyle,
  loading = "lazy",
  onClick,
}: OkfImageProps) {
  const [failed, setFailed] = useState(false);
  if (failed) {
    return (
      <div onClick={onClick} style={{ display: "flex", ...matteStyle }}>
        <div style={matteTitleStyle}>{title}</div>
        {artist ? <div style={matteArtistStyle}>{artist}</div> : null}
      </div>
    );
  }
  return (
    <img
      src={src}
      alt={alt}
      loading={loading}
      onClick={onClick}
      onError={() => setFailed(true)}
      style={imgStyle}
    />
  );
}

/* ---- Icons (stroke/fill driven by currentColor + props) ---- */

export function HeartIcon({
  size = 15,
  fill,
  stroke,
  strokeWidth = 1.7,
}: {
  size?: number;
  fill: string;
  stroke: string;
  strokeWidth?: number;
}) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill={fill} stroke={stroke} strokeWidth={strokeWidth}>
      <path d="M12 20.4 C12 20.4 3.6 14.6 3.6 8.9 C3.6 6.2 5.7 4.2 8.2 4.2 C9.9 4.2 11.3 5.2 12 6.6 C12.7 5.2 14.1 4.2 15.8 4.2 C18.3 4.2 20.4 6.2 20.4 8.9 C20.4 14.6 12 20.4 12 20.4 Z" />
    </svg>
  );
}

export function SearchIcon() {
  return (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="var(--muted)" strokeWidth="1.8" style={{ flex: "none" }}>
      <circle cx="11" cy="11" r="7" />
      <path d="M20 20 L16 16" />
    </svg>
  );
}

export function MoonIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6">
      <path d="M20.5 13.2 A8.3 8.3 0 1 1 10.8 3.5 A6.4 6.4 0 0 0 20.5 13.2 Z" fill="currentColor" opacity="0.16" />
      <path d="M20.5 13.2 A8.3 8.3 0 1 1 10.8 3.5 A6.4 6.4 0 0 0 20.5 13.2 Z" />
    </svg>
  );
}

export function PlusIcon({ size = 13 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M12 5 V19 M5 12 H19" />
    </svg>
  );
}

export function CloseIcon({ size = 17 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7">
      <path d="M6 6 L18 18 M18 6 L6 18" />
    </svg>
  );
}

export function ChevronIcon({ dir }: { dir: "left" | "right" }) {
  return (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6">
      {dir === "left" ? <path d="M15 5 L8 12 L15 19" /> : <path d="M9 5 L16 12 L9 19" />}
    </svg>
  );
}

export function DotsIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
      <circle cx="5" cy="12" r="1.7" />
      <circle cx="12" cy="12" r="1.7" />
      <circle cx="19" cy="12" r="1.7" />
    </svg>
  );
}

const EYEBROW: CSSProperties = {
  fontFamily: "var(--sans)",
  fontSize: 11.5,
  letterSpacing: "0.22em",
  textTransform: "uppercase",
  color: "var(--muted)",
};
const PAGE_TITLE: CSSProperties = {
  margin: "12px 0 0",
  fontFamily: "var(--serif)",
  fontWeight: 300,
  fontSize: 48,
  lineHeight: 1.0,
  letterSpacing: "-0.012em",
  color: "var(--ink)",
};
const SUBCOPY: CSSProperties = {
  margin: "12px 0 0",
  fontFamily: "var(--serif)",
  fontStyle: "italic",
  fontSize: 17,
  color: "var(--graphite)",
};

/** The eyebrow + serif headline + italic subcopy header used on every surface. */
export function PageHeader({
  eyebrow,
  title,
  subcopy,
  action,
  pad = "54px 0 30px",
  border = true,
}: {
  eyebrow: string;
  title: string;
  subcopy?: ReactNode;
  action?: ReactNode;
  pad?: string;
  border?: boolean;
}) {
  return (
    <header
      style={{
        padding: pad,
        display: "flex",
        alignItems: "flex-end",
        justifyContent: "space-between",
        gap: 28,
        flexWrap: "wrap",
        borderBottom: border ? "1px solid var(--line)" : undefined,
      }}
    >
      <div>
        <div style={EYEBROW}>{eyebrow}</div>
        <h1 style={PAGE_TITLE}>{title}</h1>
        {subcopy ? <p style={SUBCOPY}>{subcopy}</p> : null}
      </div>
      {action ?? null}
    </header>
  );
}

/** Quiet outline action (e.g. "New folio", "Add a stream"). */
export function OutlineButton({
  children,
  onClick,
}: {
  children: ReactNode;
  onClick?: () => void;
}) {
  return (
    <Hov
      as="button"
      onClick={onClick}
      style={{
        appearance: "none",
        cursor: "pointer",
        fontFamily: "var(--sans)",
        fontSize: 13.5,
        fontWeight: 500,
        padding: "11px 18px",
        borderRadius: 99,
        border: "1px solid var(--line-2)",
        background: "var(--surface)",
        color: "var(--ink)",
        display: "flex",
        alignItems: "center",
        gap: 8,
      }}
      hover={{ borderColor: "var(--accent-line)", color: "var(--accent)" }}
    >
      <PlusIcon />
      {children}
    </Hov>
  );
}

/** The stacked-folio brand mark. */
export function BrandMark({ width = 22, height = 25 }: { width?: number; height?: number }) {
  return (
    <svg width={width} height={height} viewBox="0 0 22 25" fill="none" style={{ display: "block", flex: "none" }}>
      <rect x="6.4" y="3.3" width="12.4" height="15.6" rx="0.6" fill="var(--bg)" stroke="var(--accent)" strokeWidth="1.3" />
      <rect x="3.2" y="5.4" width="12.4" height="15.6" rx="0.6" fill="var(--bg)" stroke="var(--ink)" strokeWidth="1.4" />
    </svg>
  );
}
