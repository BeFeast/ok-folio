import { useEffect, useMemo, useRef, useState } from "react";
import { useInfiniteQuery, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { dismissInboxItem, fetchFolios, fetchInbox, fetchInboxCounts, getPhotoThumbnailUrl } from "../api";
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

function statusLabel(status: InboxItem["status"]): string {
  return status === "duplicate" ? "Duplicate" : "Ambiguous";
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

function InboxRow({ item }: { item: InboxItem }) {
  const navigate = useNavigate();
  const { keepInboxAction, skipInboxAction, moveInboxToFolioAction } = useFolio();
  const folios = useQuery({ queryKey: ["folios"], queryFn: fetchFolios });
  const [selectedFolio, setSelectedFolio] = useState("");
  const title = item.title.trim() || "Untitled piece";
  const artist = item.artist.trim() || "Unknown artist";
  const source = item.source_url.trim();
  const sourceLink = sourceURL(source);
  const sourceLabel = sourceLink ? sourceLink.hostname.replace(/^www\./, "") : source;
  const coverPhotoId = item.cover_photo_id;
  const folioId = Number(selectedFolio);
  const canMove = coverPhotoId != null && Number.isFinite(folioId) && folioId > 0;

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
      {coverPhotoId != null ? (
        <button
          type="button"
          onClick={() => navigate(`/pieces/${coverPhotoId}`)}
          aria-label={`Open matched piece: ${title}`}
          title="Open matched piece"
          style={{
            flex: "0 0 78px",
            width: 78,
            height: 78,
            position: "relative",
            padding: 0,
            cursor: "zoom-in",
            overflow: "hidden",
            border: "1px solid var(--line)",
            borderRadius: 6,
            background: "var(--surface-2)",
          }}
        >
          <OkfImage
            src={getPhotoThumbnailUrl(coverPhotoId, 180)}
            alt={`Matched piece for ${title}`}
            title={title}
            artist={artist}
            imgStyle={{ width: "100%", height: "100%", display: "block", objectFit: "cover" }}
            matteStyle={{
              width: "100%",
              height: "100%",
              boxSizing: "border-box",
              padding: 10,
              flexDirection: "column",
              justifyContent: "center",
              gap: 4,
              background: "var(--surface-2)",
              color: "var(--ink)",
              textAlign: "left",
            }}
            matteTitleStyle={{ fontFamily: "var(--serif)", fontSize: 12.5, lineHeight: 1.1 }}
            matteArtistStyle={{ fontFamily: "var(--sans)", fontSize: 10.5, color: "var(--muted)" }}
          />
          <span
            style={{
              position: "absolute",
              left: 6,
              bottom: 6,
              padding: "3px 6px",
              borderRadius: 99,
              background: "rgba(255, 255, 255, 0.88)",
              color: "var(--graphite)",
              fontFamily: "var(--sans)",
              fontSize: 10,
              fontWeight: 600,
              letterSpacing: "0.04em",
              textTransform: "uppercase",
              boxShadow: "0 1px 4px var(--shadow)",
            }}
          >
            Matches
          </span>
        </button>
      ) : null}
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{ display: "flex", alignItems: "center", gap: 10, minWidth: 0, flexWrap: "wrap" }}>
          <div style={{ fontFamily: "var(--sans)", fontSize: 15, fontWeight: 500, color: "var(--ink)" }}>{title}</div>
          <span
            style={{
              flex: "none",
              fontFamily: "var(--sans)",
              fontSize: 11,
              fontWeight: 600,
              letterSpacing: "0.08em",
              textTransform: "uppercase",
              color: "var(--graphite)",
              border: "1px solid var(--line)",
              borderRadius: 99,
              padding: "4px 8px",
              background: "var(--surface-2)",
            }}
          >
            {statusLabel(item.status)}
          </span>
        </div>
        <div style={{ fontFamily: "var(--sans)", fontSize: 12.5, color: "var(--muted)", marginTop: 4 }}>{artist}</div>
        <div style={{ fontFamily: "var(--sans)", fontSize: 13, color: "var(--graphite)", lineHeight: 1.5, marginTop: 12 }}>
          {item.reason || "No reason provided."}
        </div>
        {sourceLink ? (
          <Hov
            as="a"
            href={sourceLink.toString()}
            target="_blank"
            rel="noreferrer"
            style={{
              display: "inline-flex",
              marginTop: 10,
              fontFamily: "var(--sans)",
              fontSize: 12.5,
              color: "var(--muted)",
              textDecoration: "none",
            }}
            hover={{ color: "var(--ink)" }}
          >
            {sourceLabel || "Open source"}
          </Hov>
        ) : source ? (
          <div style={{ marginTop: 10, fontFamily: "var(--sans)", fontSize: 12.5, color: "var(--muted)" }}>{sourceLabel}</div>
        ) : null}
      </div>
      <div style={{ flex: "0 0 220px", display: "flex", flexDirection: "column", alignItems: "stretch", gap: 8 }}>
        {coverPhotoId != null ? (
          <div style={{ display: "flex", gap: 7 }}>
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
            <Hov
              as="button"
              disabled={!canMove}
              onClick={() => {
                if (canMove) {
                  moveInboxToFolioAction(item.id, folioId, coverPhotoId);
                }
              }}
              style={{
                flex: "none",
                appearance: "none",
                cursor: canMove ? "pointer" : "not-allowed",
                opacity: canMove ? 1 : 0.55,
                fontFamily: "var(--sans)",
                fontSize: 12.5,
                fontWeight: 500,
                padding: "9px 11px",
                borderRadius: 99,
                border: 0,
                background: "var(--accent)",
                color: "var(--on-accent)",
              }}
              hover={canMove ? { filter: "brightness(1.06)" } : undefined}
            >
              Add
            </Hov>
          </div>
        ) : null}
        <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
          <ActionButton onClick={() => keepInboxAction(item.id)}>Keep</ActionButton>
          <ActionButton onClick={() => skipInboxAction(item.id)}>Skip</ActionButton>
        </div>
      </div>
    </div>
  );
}

function ActionButton({ children, onClick }: { children: string; onClick: () => void }) {
  return (
    <Hov
      as="button"
      onClick={onClick}
      style={{
        appearance: "none",
        cursor: "pointer",
        fontFamily: "var(--sans)",
        fontSize: 13,
        fontWeight: 500,
        padding: "10px 14px",
        borderRadius: 99,
        border: "1px solid var(--line)",
        background: "transparent",
        color: "var(--graphite)",
      }}
      hover={{ color: "var(--ink)", borderColor: "var(--accent)" }}
    >
      {children}
    </Hov>
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
  const [dragX, setDragX] = useState(0);
  const touchStart = useRef<number | null>(null);
  const title = item.title.trim() || "Untitled piece";
  const artist = item.artist.trim() || "Unknown artist";
  const source = item.source_url.trim();
  const sourceLink = sourceURL(source);
  const status = displayStatus(item);
  const revealed = dragX < -42;
  const translateX = revealed ? -92 : Math.min(0, Math.max(-92, dragX));

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
          background: "var(--danger, #c0392b)",
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
          transform: `translateX(${translateX}px)`,
          transition: touchStart.current == null ? "transform .18s ease" : undefined,
          display: "grid",
          gridTemplateColumns: "54px minmax(0, 1fr)",
          gap: 13,
          padding: "14px 42px 15px 14px",
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
        <button
          type="button"
          onClick={() => onDismiss(item)}
          aria-label={`Dismiss ${title}`}
          style={{
            position: "absolute",
            top: 9,
            right: 9,
            width: 32,
            height: 32,
            border: 0,
            borderRadius: 99,
            background: "transparent",
            color: "var(--muted)",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
          }}
        >
          <CloseIcon size={16} />
        </button>
      </article>
    </div>
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
  const total = inbox.data?.pages[0]?.total ?? counts.data?.total ?? 0;

  const dismissWithUndo = (item: InboxItem) => {
    if (dismissTimers.current[item.id]) window.clearTimeout(dismissTimers.current[item.id]);
    setDismissed((prev) => ({ ...prev, [item.id]: item }));
    setUndoItem(item);
    dismissTimers.current[item.id] = window.setTimeout(() => {
      dismissInboxItem(item.id)
        .then(() => {
          void queryClient.invalidateQueries({ queryKey: ["inbox"] });
          void queryClient.invalidateQueries({ queryKey: ["inbox-counts"] });
        })
        .catch(() => {
          setDismissed((prev) => {
            const next = { ...prev };
            delete next[item.id];
            return next;
          });
        });
      delete dismissTimers.current[item.id];
      setUndoItem((current) => (current?.id === item.id ? null : current));
    }, 3400);
  };

  const undoDismiss = () => {
    if (!undoItem) return;
    const item = undoItem;
    if (dismissTimers.current[item.id]) {
      window.clearTimeout(dismissTimers.current[item.id]);
      delete dismissTimers.current[item.id];
    }
    setDismissed((prev) => {
      const next = { ...prev };
      delete next[item.id];
      return next;
    });
    setUndoItem(null);
  };

  useEffect(() => {
    return () => {
      Object.values(dismissTimers.current).forEach((timer) => window.clearTimeout(timer));
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
        subcopy={inbox.isLoading ? "Gathering exceptions…" : `${total.toLocaleString()} exceptions waiting for review.`}
        action={<StatusTabs status={status} setStatus={setStatus} counts={counts.data?.counts} />}
      />
      <section style={{ maxWidth: 920, padding: "34px 0 0", display: "flex", flexDirection: "column", gap: 12 }}>
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
