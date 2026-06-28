import { useEffect, useMemo, useRef, useState } from "react";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { fetchInbox, fetchInboxCounts, getPhotoThumbnailUrl } from "../api";
import type { InboxItem } from "../types";
import { useFolio } from "./context";
import { Hov, OkfImage, PageHeader } from "./ui";

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
  const { dismissInboxAction } = useFolio();
  const title = item.title.trim() || "Untitled piece";
  const artist = item.artist.trim() || "Unknown artist";
  const source = item.source_url.trim();
  const sourceLink = sourceURL(source);
  const sourceLabel = sourceLink ? sourceLink.hostname.replace(/^www\./, "") : source;
  const coverPhotoId = item.cover_photo_id;

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
      <Hov
        as="button"
        onClick={() => dismissInboxAction(item.id)}
        style={{
          flex: "none",
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
        Dismiss
      </Hov>
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

export default function Inbox() {
  const [status, setStatus] = useState<InboxStatus>("");
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
  const total = inbox.data?.pages[0]?.total ?? counts.data?.total ?? 0;

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
