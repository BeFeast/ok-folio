import { useEffect, useMemo, useRef, useState, type CSSProperties } from "react";
import { useInfiniteQuery, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { fetchFolios, fetchInbox, fetchInboxCounts, getPhotoThumbnailUrl, skipInboxItem } from "../api";
import type { InboxItem } from "../types";
import { useFolio } from "./context";
import { useViewport } from "./useViewport";
import { CloseIcon, Hov, OkfImage, PageHeader } from "./ui";

const PAGE_SIZE = 50;

type InboxStatus = "" | InboxItem["status"];

const STATUSES: { key: InboxStatus; label: string }[] = [
  { key: "", label: "All" },
  { key: "duplicate", label: "Duplicate" },
  { key: "ambiguous", label: "Ambiguous" },
];

function StatusTabs({
  status,
  setStatus,
  counts,
}: {
  status: InboxStatus;
  setStatus: (status: InboxStatus) => void;
  counts?: { duplicate: number; ambiguous: number };
}) {
  const countFor = (key: InboxStatus) => {
    if (!counts) return undefined;
    if (key === "duplicate") return counts.duplicate;
    if (key === "ambiguous") return counts.ambiguous;
    return counts.duplicate + counts.ambiguous;
  };

  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        gap: 3,
        padding: 4,
        border: "1px solid var(--line)",
        borderRadius: 99,
        background: "var(--surface)",
      }}
    >
      {STATUSES.map((s) => {
        const active = status === s.key;
        const count = countFor(s.key);
        return (
          <button
            key={s.key || "all"}
            onClick={() => setStatus(s.key)}
            style={{
              appearance: "none",
              cursor: "pointer",
              fontFamily: "var(--sans)",
              fontSize: 13.5,
              letterSpacing: "0.1px",
              padding: "8px 14px",
              border: 0,
              borderRadius: 99,
              color: active ? "var(--ink)" : "var(--graphite)",
              background: active ? "var(--surface-2)" : "transparent",
              boxShadow: active ? "0 1px 4px var(--shadow)" : "none",
            }}
          >
            {s.label}
            {count !== undefined ? (
              <span style={{ color: active ? "var(--graphite)" : "var(--muted)", marginLeft: 6 }}>{count}</span>
            ) : null}
          </button>
        );
      })}
    </div>
  );
}

function sourceURL(value: string): URL | null {
  if (!value) return null;
  try {
    const url = new URL(value);
    if (url.protocol !== "http:" && url.protocol !== "https:") return null;
    return url;
  } catch {
    return null;
  }
}

const THUMB_W = 104;
const THUMB_H = 128;

// Editorial portrait thumbnail — fixed 104x128, soft shadow, no border, its
// serif-italic matte always present (matched photo or missing-match placeholder).
const THUMB_BOX: CSSProperties = {
  flex: `0 0 ${THUMB_W}px`,
  width: THUMB_W,
  height: THUMB_H,
  overflow: "hidden",
  background: "var(--surface-2)",
  boxShadow: "0 1px 8px var(--shadow)",
};

const MATTE_TITLE: CSSProperties = {
  fontFamily: "var(--serif)",
  fontStyle: "italic",
  fontSize: 12,
  lineHeight: 1.2,
};

// Three-tier vertical action stack: Keep (accent) > Add to folio (outline) > Dismiss (quiet).
const ACTION_BASE: CSSProperties = {
  appearance: "none",
  cursor: "pointer",
  fontFamily: "var(--sans)",
  fontSize: 13,
  fontWeight: 500,
  padding: "9px 14px",
  borderRadius: 99,
  textAlign: "center",
};
const ACTION_PRIMARY: CSSProperties = { ...ACTION_BASE, border: 0, background: "var(--accent)", color: "var(--on-accent)" };
const ACTION_SECONDARY: CSSProperties = { ...ACTION_BASE, width: "100%", border: "1px solid var(--line-2)", background: "var(--surface)", color: "var(--ink)" };
const ACTION_TERTIARY: CSSProperties = { ...ACTION_BASE, border: 0, background: "transparent", color: "var(--muted)" };

function InboxRow({ item }: { item: InboxItem }) {
  const navigate = useNavigate();
  const { keepInboxAction, skipInboxAction, moveInboxToFolioAction } = useFolio();
  const folios = useQuery({ queryKey: ["folios"], queryFn: fetchFolios });
  const [pickerOpen, setPickerOpen] = useState(false);
  const title = item.title.trim() || "Untitled piece";
  const artist = item.artist.trim() || "Unknown artist";
  const source = item.source_url.trim();
  const sourceLink = sourceURL(source);
  const sourceLabel = sourceLink ? sourceLink.hostname.replace(/^www\./, "") : source;
  const coverPhotoId = item.cover_photo_id;
  const folioList = folios.data?.folios ?? [];

  // Suggested-folio chip: InboxItem carries no suggested-folio field yet, so we
  // surface a stable client-side hint (prefer folios that already hold pieces)
  // until the API returns a real suggestion.
  const suggestedFolio = useMemo(() => {
    const list = folios.data?.folios ?? [];
    const withPieces = list.filter((folio) => folio.piece_count > 0);
    const pool = withPieces.length ? withPieces : list;
    if (pool.length === 0) return null;
    return pool[item.id % pool.length];
  }, [folios.data, item.id]);

  useEffect(() => {
    if (!pickerOpen) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") setPickerOpen(false);
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [pickerOpen]);

  const chooseFolio = (folioId: number) => {
    if (coverPhotoId != null) moveInboxToFolioAction(item.id, folioId, coverPhotoId);
    setPickerOpen(false);
  };

  return (
    <div style={{ display: "flex", alignItems: "center", gap: 22, padding: "22px 4px", borderBottom: "1px solid var(--line)" }}>
      {coverPhotoId != null ? (
        <button
          type="button"
          onClick={() => navigate(`/pieces/${coverPhotoId}`)}
          aria-label={`Open matched piece: ${title}`}
          title="Open matched piece"
          style={{ ...THUMB_BOX, position: "relative", padding: 0, border: 0, cursor: "zoom-in", display: "block" }}
        >
          <OkfImage
            src={getPhotoThumbnailUrl(coverPhotoId, 220)}
            alt={`Matched piece for ${title}`}
            title={title}
            artist={artist}
            imgStyle={{ width: "100%", height: "100%", display: "block", objectFit: "cover" }}
            matteStyle={{
              width: "100%",
              height: "100%",
              boxSizing: "border-box",
              padding: 12,
              flexDirection: "column",
              justifyContent: "center",
              gap: 4,
              textAlign: "left",
            }}
            matteTitleStyle={MATTE_TITLE}
            matteArtistStyle={{ fontFamily: "var(--sans)", fontSize: 10.5, color: "var(--muted)" }}
          />
        </button>
      ) : (
        <div
          aria-label={`No match for ${title}`}
          style={{ ...THUMB_BOX, display: "flex", flexDirection: "column", justifyContent: "center", gap: 8, padding: 12, color: "color-mix(in srgb, var(--ink) 40%, transparent)" }}
        >
          <PlaceholderGlyph />
          <div
            style={{
              ...MATTE_TITLE,
              maxWidth: "100%",
              overflow: "hidden",
              display: "-webkit-box",
              WebkitBoxOrient: "vertical",
              WebkitLineClamp: 3,
              textOverflow: "ellipsis",
              color: "color-mix(in srgb, var(--ink) 62%, transparent)",
            }}
          >
            {title}
          </div>
        </div>
      )}
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{ fontFamily: "var(--serif)", fontWeight: 400, fontSize: 21, lineHeight: 1.15, color: "var(--ink)" }}>{title}</div>
        <div style={{ fontFamily: "var(--sans)", fontSize: 13.5, color: "var(--graphite)", marginTop: 5 }}>{artist}</div>
        <div style={{ fontFamily: "var(--sans)", fontSize: 13, color: "var(--graphite)", lineHeight: 1.5, marginTop: 10 }}>
          {item.reason || "No reason provided."}
        </div>
        {sourceLink ? (
          <Hov
            as="a"
            href={sourceLink.toString()}
            target="_blank"
            rel="noreferrer"
            style={{ display: "inline-flex", marginTop: 8, fontFamily: "var(--sans)", fontSize: 12, color: "var(--faint)", textDecoration: "none" }}
            hover={{ color: "var(--ink)" }}
          >
            From {sourceLabel}
          </Hov>
        ) : source ? (
          <div style={{ marginTop: 8, fontFamily: "var(--sans)", fontSize: 12, color: "var(--faint)" }}>From {sourceLabel}</div>
        ) : null}
        {suggestedFolio ? (
          <div style={{ marginTop: 12 }}>
            <span
              style={{
                display: "inline-flex",
                alignItems: "center",
                gap: 6,
                padding: "5px 11px",
                borderRadius: 99,
                background: "var(--accent-soft)",
                color: "var(--accent)",
                fontFamily: "var(--sans)",
                fontSize: 12,
              }}
            >
              <span aria-hidden style={{ width: 5, height: 5, borderRadius: 99, background: "var(--accent)" }} />
              Suggested folio · {suggestedFolio.name}
            </span>
          </div>
        ) : null}
      </div>
      <div style={{ flex: "0 0 148px", display: "flex", flexDirection: "column", gap: 8 }}>
        <Hov as="button" onClick={() => keepInboxAction(item.id)} style={ACTION_PRIMARY} hover={{ filter: "brightness(1.06)" }}>
          Keep
        </Hov>
        {coverPhotoId != null ? (
          <div style={{ position: "relative" }}>
            <Hov
              as="button"
              onClick={() => setPickerOpen((open) => !open)}
              aria-haspopup="menu"
              aria-expanded={pickerOpen}
              aria-label={`Add ${title} to a folio`}
              style={ACTION_SECONDARY}
              hover={{ borderColor: "var(--accent-line)" }}
            >
              Add to folio
            </Hov>
            {pickerOpen ? (
              <>
                <div aria-hidden onClick={() => setPickerOpen(false)} style={{ position: "fixed", inset: 0, zIndex: 60 }} />
                <div
                  role="menu"
                  aria-label={`Folios for ${title}`}
                  style={{
                    position: "absolute",
                    top: "calc(100% + 6px)",
                    left: 0,
                    right: 0,
                    zIndex: 61,
                    background: "var(--surface)",
                    border: "1px solid var(--line)",
                    borderRadius: 10,
                    boxShadow: "0 14px 32px var(--shadow)",
                    padding: 6,
                    maxHeight: 220,
                    overflowY: "auto",
                  }}
                >
                  {folioList.length === 0 ? (
                    <div style={{ padding: "8px 10px", fontFamily: "var(--sans)", fontSize: 12.5, color: "var(--muted)" }}>No folios yet</div>
                  ) : (
                    folioList.map((folio) => (
                      <Hov
                        key={folio.id}
                        as="button"
                        role="menuitem"
                        onClick={() => chooseFolio(folio.id)}
                        style={{
                          display: "block",
                          width: "100%",
                          appearance: "none",
                          cursor: "pointer",
                          textAlign: "left",
                          border: 0,
                          background: "transparent",
                          borderRadius: 7,
                          padding: "8px 10px",
                          fontFamily: "var(--sans)",
                          fontSize: 12.5,
                          color: "var(--ink)",
                        }}
                        hover={{ background: "var(--surface-2)" }}
                      >
                        {folio.name}
                      </Hov>
                    ))
                  )}
                </div>
              </>
            ) : null}
          </div>
        ) : null}
        <Hov as="button" onClick={() => skipInboxAction(item.id)} style={ACTION_TERTIARY} hover={{ color: "var(--ink)" }}>
          Dismiss
        </Hov>
      </div>
    </div>
  );
}

function LoadMoreSentinel({
  hasMore,
  loadingMore,
  loadMore,
}: {
  hasMore: boolean;
  loadingMore: boolean;
  loadMore: () => void;
}) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = ref.current;
    if (!el || !hasMore) return;
    const obs = new IntersectionObserver(
      (entries) => {
        if (entries.some((e) => e.isIntersecting)) loadMore();
      },
      { rootMargin: "600px" },
    );
    obs.observe(el);
    return () => obs.disconnect();
  }, [hasMore, loadMore]);
  if (!hasMore && !loadingMore) return null;
  return (
    <div
      ref={ref}
      style={{ padding: "44px 0 12px", textAlign: "center", fontFamily: "var(--sans)", fontSize: 12.5, letterSpacing: "0.04em", color: "var(--faint)" }}
    >
      {loadingMore ? "Loading more…" : ""}
    </div>
  );
}

function actionLabel(item: InboxItem): string {
  const reason = `${item.status} ${item.reason}`.toLowerCase();
  if (reason.includes("duplicate")) return "Compare";
  if (reason.includes("missing") || reason.includes("image")) return "Retry import";
  return "View source";
}

function displayStatus(item: InboxItem): { label: string; color: string; background: string; border: string } {
  const reason = `${item.status} ${item.reason}`.toLowerCase();
  if (reason.includes("removed") || reason.includes("gone") || reason.includes("unavailable")) {
    return { label: "Source removed", color: "var(--danger, #c0392b)", background: "rgba(192,57,43,.1)", border: "rgba(192,57,43,.22)" };
  }
  if (reason.includes("missing") || reason.includes("image")) {
    return { label: "Missing image", color: "#8a6d1a", background: "rgba(180,140,30,.16)", border: "rgba(180,140,30,.22)" };
  }
  return { label: "Possible duplicate", color: "var(--accent)", background: "var(--accent-soft)", border: "var(--accent-line)" };
}

function reasonText(item: InboxItem): string {
  if (item.reason.trim()) return item.reason.trim();
  if (item.status === "duplicate") return "This looks like a piece already in your folio.";
  return "This import needs a quick decision before it enters the gallery.";
}

function ExternalArrow() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <path d="M7 17 17 7" />
      <path d="M9 7h8v8" />
    </svg>
  );
}

function PlaceholderGlyph() {
  return (
    <svg width="23" height="23" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.55" strokeLinecap="round" strokeLinejoin="round">
      <rect x="4.5" y="5.5" width="15" height="13" rx="1.8" />
      <path d="m7.5 15 3.2-3.2 2.8 2.7 2-2 3 3" />
      <circle cx="9" cy="9.2" r="1.2" />
    </svg>
  );
}

function MobileInboxRow({ item, onDismiss }: { item: InboxItem; onDismiss: (item: InboxItem) => void }) {
  const navigate = useNavigate();
  const { keepInboxAction, moveInboxToFolioAction } = useFolio();
  const folios = useQuery({ queryKey: ["folios"], queryFn: fetchFolios });
  const [dragX, setDragX] = useState(0);
  const [selectedFolio, setSelectedFolio] = useState("");
  const touchStart = useRef<number | null>(null);
  const title = item.title.trim() || "Untitled piece";
  const artist = item.artist.trim() || "Unknown artist";
  const source = item.source_url.trim();
  const sourceLink = sourceURL(source);
  const status = displayStatus(item);
  const revealing = dragX < -4;
  const revealed = dragX < -42;
  const translateX = revealed ? -92 : Math.min(0, Math.max(-92, dragX));
  const coverPhotoId = item.cover_photo_id;
  const folioId = Number(selectedFolio);
  const canMove = coverPhotoId != null && Number.isFinite(folioId) && folioId > 0;

  const finishTouch = () => {
    if (dragX < -74) {
      setDragX(-92);
    } else {
      setDragX(0);
    }
    touchStart.current = null;
  };

  return (
    <div style={{ position: "relative", overflow: "hidden", borderRadius: 14 }}>
      <button
        type="button"
        onClick={() => onDismiss(item)}
        style={{
          position: "absolute",
          inset: "0 0 0 auto",
          width: 96,
          border: 0,
          background: revealing || revealed ? "var(--danger, #c0392b)" : "transparent",
          color: "#fff",
          fontFamily: "var(--sans)",
          fontSize: 12,
          fontWeight: 700,
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          justifyContent: "center",
          gap: 4,
        }}
      >
        <CloseIcon size={16} />
        Dismiss
      </button>
      <article
        onTouchStart={(e) => {
          touchStart.current = e.touches[0]?.clientX ?? null;
        }}
        onTouchMove={(e) => {
          if (touchStart.current == null) return;
          setDragX(e.touches[0].clientX - touchStart.current);
        }}
        onTouchEnd={finishTouch}
        onTouchCancel={finishTouch}
        style={{
          position: "relative",
          zIndex: 1,
          width: "100%",
          boxSizing: "border-box",
          transform: `translateX(${translateX}px)`,
          transition: touchStart.current == null ? "transform .18s ease" : undefined,
          display: "grid",
          gridTemplateColumns: "54px minmax(0, 1fr)",
          gap: 13,
          padding: "14px 14px 15px",
          border: "1px solid var(--line)",
          borderRadius: 14,
          background: "var(--surface)",
          boxShadow: "0 8px 22px var(--shadow)",
        }}
      >
        {item.cover_photo_id != null ? (
          <button
            type="button"
            onClick={() => navigate(`/pieces/${item.cover_photo_id}`)}
            aria-label={`Open matched piece: ${title}`}
            style={{
              width: 54,
              height: 54,
              padding: 0,
              overflow: "hidden",
              border: "1px solid var(--line)",
              borderRadius: 10,
              background: "var(--wall)",
            }}
          >
            <OkfImage
              src={getPhotoThumbnailUrl(item.cover_photo_id, 140)}
              alt={`Matched piece for ${title}`}
              title={title}
              artist={artist}
              imgStyle={{ width: "100%", height: "100%", objectFit: "cover", display: "block" }}
              matteStyle={{ width: "100%", height: "100%", alignItems: "center", justifyContent: "center", background: "var(--wall)", color: "var(--muted)" }}
              matteTitleStyle={{ display: "none" }}
            />
          </button>
        ) : (
          <div
            aria-hidden
            style={{
              width: 54,
              height: 54,
              border: "1px solid var(--line)",
              borderRadius: 10,
              background: "var(--wall)",
              color: "var(--muted)",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
            }}
          >
            <PlaceholderGlyph />
          </div>
        )}
        <div style={{ minWidth: 0 }}>
          <div style={{ display: "inline-flex", alignItems: "center", border: `1px solid ${status.border}`, borderRadius: 99, padding: "4px 8px", background: status.background, color: status.color, fontFamily: "var(--sans)", fontSize: 12, fontWeight: 700 }}>
            {status.label}
          </div>
          <h2 style={{ margin: "8px 0 0", fontFamily: "var(--serif)", fontSize: 18, fontWeight: 500, lineHeight: 1.12, color: "var(--ink)" }}>{title}</h2>
          <div style={{ marginTop: 3, fontFamily: "var(--sans)", fontSize: 13, color: "var(--graphite)" }}>{artist}</div>
          <p style={{ margin: "9px 0 0", fontFamily: "var(--sans)", fontSize: 13.5, lineHeight: 1.4, color: "var(--muted)" }}>{reasonText(item)}</p>
          {sourceLink ? (
            <a
              href={sourceLink.toString()}
              target="_blank"
              rel="noreferrer"
              style={{ display: "inline-flex", alignItems: "center", gap: 6, marginTop: 10, color: "var(--accent)", fontFamily: "var(--sans)", fontSize: 13, fontWeight: 700, textDecoration: "none" }}
            >
              <ExternalArrow />
              {actionLabel(item)}
            </a>
          ) : (
            <button
              type="button"
              onClick={() => item.cover_photo_id != null && navigate(`/pieces/${item.cover_photo_id}`)}
              style={{ display: "inline-flex", alignItems: "center", gap: 6, marginTop: 10, padding: 0, border: 0, background: "transparent", color: "var(--accent)", fontFamily: "var(--sans)", fontSize: 13, fontWeight: 700 }}
            >
              <ExternalArrow />
              {actionLabel(item)}
            </button>
          )}
        </div>
        <div
          style={{
            gridColumn: "1 / -1",
            display: "flex",
            flexDirection: "column",
            gap: 9,
            paddingTop: 3,
          }}
        >
          {coverPhotoId != null ? (
            <div style={{ display: "flex", gap: 8 }}>
              <select
                value={selectedFolio}
                onChange={(event) => setSelectedFolio(event.target.value)}
                aria-label={`Choose folio for ${title}`}
                style={{
                  minWidth: 0,
                  flex: 1,
                  border: "1px solid var(--line)",
                  borderRadius: 99,
                  background: "var(--surface)",
                  color: "var(--graphite)",
                  fontFamily: "var(--sans)",
                  fontSize: 12.5,
                  padding: "9px 10px",
                }}
              >
                <option value="">Folio</option>
                {(folios.data?.folios ?? []).map((folio) => (
                  <option key={folio.id} value={folio.id}>
                    {folio.name}
                  </option>
                ))}
              </select>
              <MobileActionButton
                tone="accent"
                disabled={!canMove}
                onClick={() => {
                  if (canMove) {
                    moveInboxToFolioAction(item.id, folioId, coverPhotoId);
                  }
                }}
              >
                Add to folio
              </MobileActionButton>
            </div>
          ) : null}
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 8 }}>
            <MobileActionButton onClick={() => keepInboxAction(item.id)}>Keep</MobileActionButton>
            <MobileActionButton onClick={() => onDismiss(item)}>Dismiss</MobileActionButton>
          </div>
        </div>
      </article>
    </div>
  );
}

function MobileActionButton({
  children,
  disabled,
  onClick,
  tone = "secondary",
}: {
  children: string;
  disabled?: boolean;
  onClick: () => void;
  tone?: "accent" | "secondary";
}) {
  const accent = tone === "accent";
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={onClick}
      style={{
        minHeight: 38,
        minWidth: accent ? 108 : undefined,
        padding: "0 12px",
        appearance: "none",
        cursor: disabled ? "not-allowed" : "pointer",
        opacity: disabled ? 0.55 : 1,
        fontFamily: "var(--sans)",
        fontSize: 12.5,
        fontWeight: 700,
        borderRadius: 99,
        border: accent ? 0 : "1px solid var(--line)",
        background: accent ? "var(--accent)" : "var(--surface-2)",
        color: accent ? "var(--on-accent)" : "var(--graphite)",
        whiteSpace: "nowrap",
      }}
    >
      {children}
    </button>
  );
}

function MobileUndoToast({ title, onUndo }: { title: string; onUndo: () => void }) {
  return (
    <div
      role="status"
      style={{
        position: "fixed",
        left: "calc(20px + var(--safe-left))",
        right: "calc(20px + var(--safe-right))",
        bottom: "calc(var(--mobile-tab-height) + var(--safe-bottom) + 14px)",
        zIndex: 180,
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        gap: 12,
        padding: "13px 14px",
        borderRadius: 13,
        background: "var(--ink)",
        color: "var(--bg)",
        boxShadow: "0 14px 34px rgba(0,0,0,.3)",
        fontFamily: "var(--sans)",
        fontSize: 13,
      }}
    >
      <span style={{ minWidth: 0, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{title} dismissed</span>
      <button type="button" onClick={onUndo} style={{ border: 0, background: "transparent", color: "var(--accent)", fontFamily: "var(--sans)", fontSize: 13, fontWeight: 800 }}>
        Undo
      </button>
    </div>
  );
}

export default function Inbox() {
  const [status, setStatus] = useState<InboxStatus>("");
  const { isMobile } = useViewport();
  const queryClient = useQueryClient();
  const dismissTimers = useRef<Record<number, number>>({});
  const pendingDismisses = useRef<Record<number, InboxItem>>({});
  const mounted = useRef(true);
  const [dismissed, setDismissed] = useState<Record<number, InboxItem>>({});
  const [undoItem, setUndoItem] = useState<InboxItem | null>(null);
  const counts = useQuery({ queryKey: ["inbox-counts"], queryFn: fetchInboxCounts });
  const inbox = useInfiniteQuery({
    queryKey: ["inbox", status],
    queryFn: ({ pageParam }) => fetchInbox(status, PAGE_SIZE, pageParam as number),
    initialPageParam: 0,
    getNextPageParam: (lastPage, allPages) => {
      const loaded = allPages.reduce((n, pg) => n + pg.items.length, 0);
      return loaded < lastPage.total ? loaded : undefined;
    },
  });
  const items = useMemo(() => inbox.data?.pages.flatMap((page) => page.items) ?? [], [inbox.data]);
  const visibleItems = useMemo(() => items.filter((item) => !dismissed[item.id]), [dismissed, items]);

  const commitDismiss = (item: InboxItem, restoreOnError: boolean) => {
    delete pendingDismisses.current[item.id];
    if (dismissTimers.current[item.id]) {
      window.clearTimeout(dismissTimers.current[item.id]);
      delete dismissTimers.current[item.id];
    }
    skipInboxItem(item.id)
      .then(() => {
        void queryClient.invalidateQueries({ queryKey: ["inbox"] });
        void queryClient.invalidateQueries({ queryKey: ["inbox-counts"] });
      })
      .catch(() => {
        if (restoreOnError && mounted.current) {
          setDismissed((prev) => {
            const next = { ...prev };
            delete next[item.id];
            return next;
          });
        }
      })
      .finally(() => {
        if (mounted.current) {
          setUndoItem((current) => (current?.id === item.id ? null : current));
        }
      });
  };

  const dismissWithUndo = (item: InboxItem) => {
    if (dismissTimers.current[item.id]) window.clearTimeout(dismissTimers.current[item.id]);
    pendingDismisses.current[item.id] = item;
    setDismissed((prev) => ({ ...prev, [item.id]: item }));
    setUndoItem(item);
    dismissTimers.current[item.id] = window.setTimeout(() => {
      commitDismiss(item, true);
    }, 3400);
  };

  const undoDismiss = () => {
    if (!undoItem) return;
    const item = undoItem;
    if (dismissTimers.current[item.id]) {
      window.clearTimeout(dismissTimers.current[item.id]);
      delete dismissTimers.current[item.id];
    }
    delete pendingDismisses.current[item.id];
    setDismissed((prev) => {
      const next = { ...prev };
      delete next[item.id];
      return next;
    });
    setUndoItem(null);
  };

  useEffect(() => {
    return () => {
      mounted.current = false;
      Object.values(dismissTimers.current).forEach((timer) => window.clearTimeout(timer));
      Object.values(pendingDismisses.current).forEach((item) => commitDismiss(item, false));
    };
  }, []);

  if (isMobile) {
    return (
      <div>
        <div style={{ marginTop: 2, fontFamily: "var(--sans)", fontSize: 13, color: "var(--muted)" }}>
          {inbox.isLoading ? "Gathering exceptions…" : `${visibleItems.length.toLocaleString()} to review`}
        </div>
        <section style={{ padding: "18px 0 0", display: "flex", flexDirection: "column", gap: 12 }}>
          {inbox.isError ? (
            <div style={{ padding: "60px 0", fontFamily: "var(--serif)", fontSize: 20, color: "var(--graphite)" }}>
              The inbox could not be reached.
            </div>
          ) : inbox.isLoading ? (
            <div style={{ padding: "60px 0", fontFamily: "var(--sans)", fontSize: 14, color: "var(--muted)" }}>Loading inbox…</div>
          ) : visibleItems.length === 0 ? (
            <div style={{ textAlign: "center", padding: "88px 14px", color: "var(--muted)" }}>
              <div
                style={{
                  width: 58,
                  height: 58,
                  margin: "0 auto 18px",
                  borderRadius: 99,
                  background: "var(--accent-soft)",
                  color: "var(--accent)",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  fontFamily: "var(--sans)",
                  fontSize: 28,
                  fontWeight: 600,
                }}
              >
                ✓
              </div>
              <div style={{ fontFamily: "var(--serif)", fontSize: 22, fontWeight: 500, color: "var(--ink)" }}>All caught up</div>
              <div style={{ fontFamily: "var(--sans)", fontSize: 14, marginTop: 8, lineHeight: 1.45 }}>
                New stream exceptions will appear here when they need review.
              </div>
            </div>
          ) : (
            <>
              {visibleItems.map((item) => (
                <MobileInboxRow key={item.id} item={item} onDismiss={dismissWithUndo} />
              ))}
              <LoadMoreSentinel
                hasMore={!!inbox.hasNextPage}
                loadingMore={inbox.isFetchingNextPage}
                loadMore={() => {
                  if (inbox.hasNextPage && !inbox.isFetchingNextPage) {
                    void inbox.fetchNextPage();
                  }
                }}
              />
            </>
          )}
        </section>
        {undoItem ? <MobileUndoToast title={undoItem.title.trim() || "Inbox item"} onUndo={undoDismiss} /> : null}
      </div>
    );
  }

  return (
    <div>
      <PageHeader
        eyebrow="Inbox"
        title="To review"
        subcopy="New pieces gathered from your streams. Review at your pace — nothing is urgent."
        action={<StatusTabs status={status} setStatus={setStatus} counts={counts.data?.counts} />}
      />
      <section style={{ maxWidth: 880, padding: "18px 0 0", display: "flex", flexDirection: "column" }}>
        {inbox.isError ? (
          <div style={{ padding: "60px 0", fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 20, color: "var(--graphite)" }}>
            The inbox could not be reached.
          </div>
        ) : inbox.isLoading ? (
          <div style={{ padding: "60px 0", fontFamily: "var(--sans)", fontSize: 14, color: "var(--muted)" }}>Loading inbox…</div>
        ) : items.length === 0 ? (
          <div style={{ textAlign: "center", padding: "90px 0", color: "var(--muted)" }}>
            <div style={{ fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 24, color: "var(--graphite)" }}>All caught up.</div>
            <div style={{ fontFamily: "var(--sans)", fontSize: 14, marginTop: 10 }}>
              Nothing waiting. Pieces will appear here as your streams gather them.
            </div>
          </div>
        ) : (
          <>
            {items.map((item) => (
              <InboxRow key={item.id} item={item} />
            ))}
            <LoadMoreSentinel
              hasMore={!!inbox.hasNextPage}
              loadingMore={inbox.isFetchingNextPage}
              loadMore={() => {
                if (inbox.hasNextPage && !inbox.isFetchingNextPage) {
                  void inbox.fetchNextPage();
                }
              }}
            />
          </>
        )}
      </section>
    </div>
  );
}
