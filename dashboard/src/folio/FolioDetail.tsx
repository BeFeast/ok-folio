import { useEffect, useMemo, useState, type CSSProperties, type ReactNode } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { fetchFolioPieces, fetchFolios, fetchGalleryCatalog, getPhotoThumbnailUrl } from "../api";
import type { Folio, Photo } from "../types";
import BulkEditBar from "./BulkEditBar";
import { mapPhoto, useFolio, type PieceVM } from "./context";
import { FolioNameModal } from "./Folios";
import { BrandMark, ChevronIcon, CloseIcon, ConfirmationDialog, DotsIcon, Hov, OkfImage, PageHeader, PlusIcon } from "./ui";
import { useViewport } from "./useViewport";

const PAGE_SIZE = 100;
const PICKER_PAGE_SIZE = 80;

const TILE_MATTE: CSSProperties = {
  position: "absolute",
  inset: 0,
  flexDirection: "column",
  alignItems: "center",
  justifyContent: "center",
  gap: 5,
  padding: 16,
  textAlign: "center",
  background: "linear-gradient(155deg, var(--surface-2), var(--surface))",
};

function pieceCountLabel(count: number): string {
  return `${count.toLocaleString()} ${count === 1 ? "piece" : "pieces"}`;
}

function updatedLabel(value?: string): string {
  if (!value) return "updated recently";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "updated recently";
  const days = Math.floor((Date.now() - date.getTime()) / 86_400_000);
  if (days <= 0) return "updated today";
  if (days === 1) return "updated yesterday";
  if (days < 30) return `updated ${days} days ago`;
  return `updated ${date.toLocaleDateString(undefined, { month: "short", day: "numeric" })}`;
}

function PieceTile({ piece }: { piece: PieceVM }) {
  const { openPiece } = useFolio();
  return (
    <figure
      onClick={() => openPiece(piece.id)}
      style={{ margin: 0, position: "relative", aspectRatio: "1 / 1", cursor: "zoom-in", overflow: "hidden", background: "var(--surface)", boxShadow: "0 1px 8px var(--shadow)" }}
    >
      <OkfImage
        src={piece.thumb}
        alt={piece.t}
        title={piece.t}
        artist={piece.a}
        imgStyle={{ position: "absolute", inset: 0, width: "100%", height: "100%", objectFit: "cover", zIndex: 1 }}
        matteStyle={TILE_MATTE}
        matteTitleStyle={{ fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 14, lineHeight: 1.2, color: "var(--ink)" }}
        matteArtistStyle={{ fontFamily: "var(--sans)", fontSize: 9.5, letterSpacing: "0.12em", textTransform: "uppercase", color: "var(--muted)" }}
      />
    </figure>
  );
}

function SelectionBadge({ selected }: { selected: boolean }) {
  return (
    <span
      aria-hidden="true"
      style={{
        position: "absolute",
        top: 8,
        right: 8,
        zIndex: 4,
        width: 24,
        height: 24,
        borderRadius: 99,
        border: selected ? 0 : "2px solid rgba(255,255,255,.9)",
        background: selected ? "var(--accent)" : "rgba(20,14,10,.18)",
        color: "var(--on-accent)",
        display: "grid",
        placeItems: "center",
        boxShadow: "0 1px 5px rgba(0,0,0,.22)",
      }}
    >
      {selected ? (
        <svg width="13" height="13" viewBox="0 0 16 16" fill="none">
          <path d="M3.2 8.4l3 3 6.6-7" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      ) : null}
    </span>
  );
}

function SelectPieceTile({ piece, selected, onToggle }: { piece: PieceVM; selected: boolean; onToggle: () => void }) {
  return (
    <button
      type="button"
      aria-pressed={selected}
      onClick={onToggle}
      style={{
        position: "relative",
        aspectRatio: "1 / 1",
        border: 0,
        borderRadius: 3,
        padding: 0,
        overflow: "hidden",
        background: "var(--wall)",
        cursor: "pointer",
        boxShadow: selected ? "0 0 0 3px var(--accent)" : "0 1px 8px var(--shadow)",
      }}
    >
      <OkfImage
        src={piece.thumb}
        alt={piece.t}
        title={piece.t}
        artist={piece.a}
        imgStyle={{ position: "absolute", inset: 0, width: "100%", height: "100%", objectFit: "cover", zIndex: 1 }}
        matteStyle={TILE_MATTE}
        matteTitleStyle={{ fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 14, lineHeight: 1.2, color: "var(--ink)" }}
        matteArtistStyle={{ fontFamily: "var(--sans)", fontSize: 9.5, letterSpacing: "0.12em", color: "var(--muted)" }}
      />
      {selected ? <span style={{ position: "absolute", inset: 0, zIndex: 2, background: "rgba(124,36,32,.18)" }} /> : null}
      <SelectionBadge selected={selected} />
    </button>
  );
}

function PickerTile({
  photo,
  selected,
  disabled,
  onToggle,
}: {
  photo: Photo;
  selected: boolean;
  disabled: boolean;
  onToggle: () => void;
}) {
  const piece = mapPhoto(photo);
  return (
    <button
      type="button"
      aria-pressed={selected}
      aria-label={piece.a ? `${piece.t} — ${piece.a}` : piece.t}
      title={piece.a ? `${piece.t} — ${piece.a}` : piece.t}
      disabled={disabled}
      onClick={onToggle}
      style={{
        position: "relative",
        aspectRatio: "1 / 1",
        border: 0,
        borderRadius: 4,
        padding: 0,
        overflow: "hidden",
        background: "var(--wall)",
        cursor: disabled ? "not-allowed" : "pointer",
        opacity: disabled ? 0.45 : 1,
        transition: "transform .12s ease, box-shadow .12s ease",
        transform: selected ? "scale(0.93)" : "none",
        boxShadow: selected ? "0 0 0 3px var(--accent)" : "0 1px 5px var(--shadow)",
      }}
    >
      <OkfImage
        src={getPhotoThumbnailUrl(photo.ID, 400)}
        alt={piece.t}
        title={piece.t}
        artist={piece.a}
        imgStyle={{ position: "absolute", inset: 0, width: "100%", height: "100%", objectFit: "cover", zIndex: 1 }}
        matteStyle={TILE_MATTE}
        matteTitleStyle={{ fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 13, lineHeight: 1.2, color: "var(--ink)" }}
        matteArtistStyle={{ fontFamily: "var(--sans)", fontSize: 9, letterSpacing: "0.12em", textTransform: "uppercase", color: "var(--muted)" }}
      />
      {selected ? <span style={{ position: "absolute", inset: 0, zIndex: 2, background: "var(--accent-soft)" }} /> : null}
      <span
        aria-hidden="true"
        style={{
          position: "absolute",
          top: 8,
          right: 8,
          zIndex: 4,
          width: 22,
          height: 22,
          borderRadius: 99,
          border: selected ? 0 : "1.5px solid rgba(255,255,255,0.85)",
          background: selected ? "var(--accent)" : "rgba(20,16,10,0.28)",
          color: "var(--on-accent)",
          display: "grid",
          placeItems: "center",
          boxShadow: "0 1px 5px rgba(0,0,0,.22)",
        }}
      >
        {selected ? (
          <svg width="12" height="12" viewBox="0 0 16 16" fill="none">
            <path d="M3.2 8.4l3 3 6.6-7" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
        ) : null}
      </span>
    </button>
  );
}

function MobilePickerTile({
  photo,
  selected,
  disabled,
  onToggle,
}: {
  photo: Photo;
  selected: boolean;
  disabled: boolean;
  onToggle: () => void;
}) {
  const piece = mapPhoto(photo);
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={onToggle}
      aria-pressed={selected}
      style={{
        position: "relative",
        aspectRatio: "1 / 1",
        border: 0,
        borderRadius: 3,
        padding: 0,
        overflow: "hidden",
        background: "var(--wall)",
        opacity: disabled ? 0.35 : 1,
        boxShadow: selected ? "0 0 0 3px var(--accent)" : "none",
      }}
    >
      <OkfImage
        src={getPhotoThumbnailUrl(photo.ID, 400)}
        alt={piece.t}
        title={piece.t}
        artist={piece.a}
        imgStyle={{ position: "absolute", inset: 0, width: "100%", height: "100%", objectFit: "cover", zIndex: 1 }}
        matteStyle={TILE_MATTE}
      />
      {selected ? <span style={{ position: "absolute", inset: 0, zIndex: 2, background: "rgba(124,36,32,.18)" }} /> : null}
      <span
        aria-hidden="true"
        style={{
          position: "absolute",
          top: 7,
          right: 7,
          zIndex: 3,
          width: 23,
          height: 23,
          borderRadius: 99,
          border: selected ? "0" : "2px solid rgba(255,255,255,.9)",
          background: selected ? "var(--accent)" : "rgba(20,14,10,.18)",
          color: "var(--on-accent)",
          display: "grid",
          placeItems: "center",
          boxShadow: "0 1px 5px rgba(0,0,0,.22)",
        }}
      >
        {selected ? (
          <svg width="13" height="13" viewBox="0 0 16 16" fill="none">
            <path d="M3.2 8.4l3 3 6.6-7" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
        ) : null}
      </span>
    </button>
  );
}

function AddPiecesPicker({
  folioId,
  folioName,
  existingIds,
  onClose,
}: {
  folioId: number;
  folioName?: string;
  existingIds: Set<number>;
  onClose: () => void;
}) {
  const { addPiecesToFolioAction } = useFolio();
  const { isMobile } = useViewport();
  const [selected, setSelected] = useState<Set<number>>(() => new Set());
  const [adding, setAdding] = useState(false);
  const [search, setSearch] = useState("");
  const [query, setQuery] = useState("");
  useEffect(() => {
    const handle = setTimeout(() => setQuery(search.trim()), 220);
    return () => clearTimeout(handle);
  }, [search]);
  const catalog = useInfiniteQuery({
    queryKey: ["folio-piece-picker", folioId, query],
    queryFn: ({ pageParam }) => fetchGalleryCatalog(PICKER_PAGE_SIZE, pageParam as number, query ? { query } : {}),
    initialPageParam: 0,
    getNextPageParam: (lastPage, allPages) => {
      const loaded = allPages.reduce((n, pg) => n + pg.photos.length, 0);
      return loaded < lastPage.total ? loaded : undefined;
    },
  });
  const photos = catalog.data?.pages.flatMap((page) => page.photos) ?? [];

  const toggle = (photoId: number) => {
    if (adding) return;
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(photoId)) {
        next.delete(photoId);
      } else {
        next.add(photoId);
      }
      return next;
    });
  };

  const addSelected = async () => {
    if (adding || selected.size === 0) return;
    setAdding(true);
    try {
      const added = await addPiecesToFolioAction(folioId, Array.from(selected), existingIds);
      if (added) {
        onClose();
      }
    } finally {
      setAdding(false);
    }
  };

  if (isMobile) {
    return (
      <div
        role="dialog"
        aria-modal="true"
        style={{
          position: "fixed",
          inset: 0,
          zIndex: 130,
          background: "var(--bg)",
          color: "var(--ink)",
          display: "flex",
          flexDirection: "column",
          padding: "calc(10px + var(--safe-top)) calc(16px + var(--safe-right)) calc(var(--safe-bottom) + 86px) calc(16px + var(--safe-left))",
        }}
      >
        <header style={{ height: 48, display: "grid", gridTemplateColumns: "68px 1fr 68px", alignItems: "center", gap: 8 }}>
          <button type="button" onClick={onClose} disabled={adding} style={{ border: 0, background: "transparent", color: "var(--accent)", fontFamily: "var(--sans)", fontSize: 14, fontWeight: 700, textAlign: "left", padding: 0, opacity: adding ? 0.6 : 1 }}>
            Cancel
          </button>
          <div style={{ textAlign: "center", minWidth: 0 }}>
            <div style={{ fontFamily: "var(--serif)", fontSize: 20, lineHeight: 1.05, color: "var(--ink)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>Add to {folioName ?? "folio"}</div>
          </div>
          <button type="button" onClick={() => void addSelected()} disabled={selected.size === 0 || adding} style={{ border: 0, background: "transparent", color: selected.size && !adding ? "var(--accent)" : "var(--muted)", fontFamily: "var(--sans)", fontSize: 14, fontWeight: 700, textAlign: "right", padding: 0 }}>
            {adding ? "Adding" : "Done"}
          </button>
        </header>

        <div style={{ flex: 1, overflow: "auto", paddingTop: 10, paddingBottom: 12 }}>
          {catalog.isError ? (
            <div style={{ padding: "58px 8px", textAlign: "center", fontFamily: "var(--serif)", fontSize: 21 }}>The gallery could not be reached.</div>
          ) : catalog.isLoading ? (
            <section style={{ display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 8 }}>
              {Array.from({ length: 15 }, (_, index) => (
                <div key={index} className="okf-shimmer" style={{ aspectRatio: "1 / 1", borderRadius: 3, background: "var(--wall)" }} />
              ))}
            </section>
          ) : (
            <section style={{ display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 8 }}>
              {photos.map((photo) => (
                <MobilePickerTile
                  key={photo.ID}
                  photo={photo}
                  selected={selected.has(photo.ID)}
                  disabled={existingIds.has(photo.ID) || adding}
                  onToggle={() => toggle(photo.ID)}
                />
              ))}
            </section>
          )}
          {catalog.hasNextPage ? (
            <div style={{ display: "flex", justifyContent: "center", padding: "18px 0 2px" }}>
              <button
                type="button"
                onClick={() => void catalog.fetchNextPage()}
                disabled={catalog.isFetchingNextPage || adding}
                style={{ height: 40, borderRadius: 99, border: "1px solid var(--line-2)", background: "var(--surface)", color: "var(--ink)", fontFamily: "var(--sans)", fontSize: 13, fontWeight: 600, padding: "0 16px" }}
              >
                {catalog.isFetchingNextPage ? "Loading..." : "Load more"}
              </button>
            </div>
          ) : null}
        </div>

        <div
          style={{
            position: "fixed",
            left: 0,
            right: 0,
            bottom: 0,
            zIndex: 2,
            padding: "22px calc(16px + var(--safe-right)) calc(14px + var(--safe-bottom)) calc(16px + var(--safe-left))",
            background: "linear-gradient(to top, var(--bg) 68%, rgba(243,239,231,0))",
          }}
        >
          <button
            type="button"
            onClick={() => void addSelected()}
            disabled={selected.size === 0 || adding}
            style={{ width: "100%", height: 52, borderRadius: 14, border: 0, background: "var(--accent)", color: "var(--on-accent)", fontFamily: "var(--sans)", fontSize: 15, fontWeight: 800, opacity: selected.size && !adding ? 1 : 0.55, boxShadow: selected.size && !adding ? "0 8px 20px rgba(124,36,32,.3)" : "none" }}
          >
            {adding ? "Adding..." : `Add ${selected.size.toLocaleString()} ${selected.size === 1 ? "piece" : "pieces"}`}
          </button>
        </div>
      </div>
    );
  }

  const hasSelection = selected.size > 0;
  const confirmLabel = adding
    ? "Adding..."
    : hasSelection
      ? `Add ${selected.size.toLocaleString()} ${selected.size === 1 ? "piece" : "pieces"}`
      : "Add pieces";

  return (
    <div
      onClick={onClose}
      style={{
        position: "fixed",
        inset: 0,
        zIndex: 100,
        background: "rgba(12,9,6,0.62)",
        backdropFilter: "blur(6px)",
        WebkitBackdropFilter: "blur(6px)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        padding: 34,
        animation: "okf-fade .2s ease",
      }}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Add pieces"
        onClick={(e) => e.stopPropagation()}
        style={{
          width: "min(880px, 96vw)",
          maxHeight: "88vh",
          display: "flex",
          flexDirection: "column",
          borderRadius: 16,
          overflow: "hidden",
          background: "var(--surface)",
          boxShadow: "0 50px 130px rgba(0,0,0,0.5)",
          animation: "okf-rise .3s cubic-bezier(0.22,1,0.36,1)",
        }}
      >
        <div style={{ flex: "none", padding: "22px 26px", borderBottom: "1px solid var(--line)", display: "flex", alignItems: "center", justifyContent: "space-between", gap: 20 }}>
          <div style={{ minWidth: 0 }}>
            <div style={{ fontFamily: "var(--sans)", fontSize: 11, letterSpacing: "0.2em", textTransform: "uppercase", color: "var(--accent)" }}>Add to folio</div>
            <h2 style={{ margin: "7px 0 0", fontFamily: "var(--serif)", fontWeight: 300, fontSize: 23, color: "var(--ink)", letterSpacing: "-0.01em", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{folioName ?? "folio"}</h2>
          </div>
          <div style={{ display: "flex", alignItems: "center", gap: 12, flex: "none" }}>
            <label style={{ display: "flex", alignItems: "center", gap: 8, width: 230, borderRadius: 11, background: "var(--bg)", border: "1px solid var(--line)", padding: "9px 14px" }}>
              <span aria-hidden="true" style={{ display: "flex", color: "var(--muted)", flex: "none" }}>
                <svg width="15" height="15" viewBox="0 0 20 20" fill="none">
                  <circle cx="9" cy="9" r="6" stroke="currentColor" strokeWidth="1.7" />
                  <path d="M13.5 13.5L18 18" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
                </svg>
              </span>
              <input
                type="text"
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                placeholder="Search pieces"
                aria-label="Search pieces"
                disabled={adding}
                style={{ flex: 1, minWidth: 0, border: 0, background: "transparent", outline: "none", fontFamily: "var(--sans)", fontSize: 13.5, color: "var(--ink)" }}
              />
            </label>
            <Hov
              as="button"
              onClick={onClose}
              aria-label="Close"
              disabled={adding}
              style={{ appearance: "none", cursor: adding ? "not-allowed" : "pointer", width: 36, height: 36, borderRadius: 99, border: "1px solid var(--line)", background: "transparent", color: "var(--muted)", display: "flex", alignItems: "center", justifyContent: "center", flex: "none", opacity: adding ? 0.6 : 1 }}
              hover={{ color: "var(--ink)", borderColor: "var(--line-2)" }}
            >
              <CloseIcon size={15} />
            </Hov>
          </div>
        </div>

        <div style={{ flex: 1, minHeight: 0, overflow: "auto", padding: "18px 22px" }}>
          {catalog.isError ? (
            <div style={{ padding: "50px 0", fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 20, color: "var(--graphite)", textAlign: "center" }}>
              The gallery could not be reached.
            </div>
          ) : catalog.isLoading ? (
            <section style={{ display: "grid", gridTemplateColumns: "repeat(5, 1fr)", gap: 10 }}>
              {Array.from({ length: 15 }, (_, index) => (
                <div key={index} className="okf-shimmer" style={{ aspectRatio: "1 / 1", borderRadius: 4, background: "var(--wall)" }} />
              ))}
            </section>
          ) : photos.length === 0 ? (
            <div style={{ padding: "50px 0", fontFamily: "var(--sans)", fontSize: 14, color: "var(--muted)", textAlign: "center" }}>
              {query ? "No pieces match your search." : "No pieces to add yet."}
            </div>
          ) : (
            <section style={{ display: "grid", gridTemplateColumns: "repeat(5, 1fr)", gap: 10 }}>
              {photos.map((photo) => (
                <PickerTile
                  key={photo.ID}
                  photo={photo}
                  selected={selected.has(photo.ID)}
                  disabled={existingIds.has(photo.ID) || adding}
                  onToggle={() => toggle(photo.ID)}
                />
              ))}
            </section>
          )}
          {catalog.hasNextPage ? (
            <div style={{ display: "flex", justifyContent: "center", padding: "24px 0 4px" }}>
              <Hov
                as="button"
                onClick={() => void catalog.fetchNextPage()}
                disabled={catalog.isFetchingNextPage || adding}
                style={{ appearance: "none", cursor: catalog.isFetchingNextPage || adding ? "wait" : "pointer", fontFamily: "var(--sans)", fontSize: 13, padding: "10px 18px", borderRadius: 99, border: "1px solid var(--line-2)", background: "var(--surface)", color: "var(--ink)" }}
                hover={{ borderColor: "var(--accent-line)", color: "var(--accent)" }}
              >
                {catalog.isFetchingNextPage ? "Loading..." : "Load more"}
              </Hov>
            </div>
          ) : null}
        </div>

        <div style={{ flex: "none", padding: "16px 26px", borderTop: "1px solid var(--line)", display: "flex", alignItems: "center", justifyContent: "space-between", gap: 18 }}>
          <div style={{ fontFamily: "var(--sans)", fontSize: 13, color: "var(--muted)" }}>
            {hasSelection ? `${selected.size.toLocaleString()} selected` : "Select pieces to add"}
          </div>
          <Hov
            as="button"
            onClick={() => void addSelected()}
            disabled={!hasSelection || adding}
            style={{ appearance: "none", cursor: !hasSelection || adding ? "not-allowed" : "pointer", fontFamily: "var(--sans)", fontSize: 13.5, fontWeight: 500, padding: "11px 22px", borderRadius: 11, border: 0, background: hasSelection ? "var(--accent)" : "var(--line)", color: hasSelection ? "var(--on-accent)" : "var(--muted)", flex: "none" }}
            hover={hasSelection && !adding ? { filter: "brightness(1.06)" } : undefined}
          >
            {confirmLabel}
          </Hov>
        </div>
      </div>
    </div>
  );
}

function MobileDetailTile({ piece }: { piece: PieceVM }) {
  const { openPiece } = useFolio();
  return (
    <button
      type="button"
      onClick={() => openPiece(piece.id)}
      aria-label={piece.t}
      style={{
        position: "relative",
        aspectRatio: "1 / 1",
        border: 0,
        borderRadius: 3,
        padding: 0,
        overflow: "hidden",
        background: "var(--wall)",
      }}
    >
      <OkfImage
        src={piece.thumb}
        alt={piece.t}
        title={piece.t}
        artist={piece.a}
        loading="eager"
        imgStyle={{ position: "absolute", inset: 0, width: "100%", height: "100%", objectFit: "cover", zIndex: 1 }}
        matteStyle={TILE_MATTE}
        matteTitleStyle={{ fontFamily: "var(--serif)", fontSize: 12.5, lineHeight: 1.12, color: "var(--ink)" }}
        matteArtistStyle={{ fontFamily: "var(--sans)", fontSize: 9, color: "var(--muted)" }}
      />
    </button>
  );
}

function MobileDetailMenu({
  folio,
  firstPieceId,
  onClose,
}: {
  folio: Folio;
  firstPieceId: number | null;
  onClose: () => void;
}) {
  const { renameFolioAction, changeFolioCoverAction, deleteFolioAction } = useFolio();
  const navigate = useNavigate();
  const [name, setName] = useState(folio.name);
  const [renaming, setRenaming] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);

  const submitRename = () => {
    const trimmed = name.trim();
    if (trimmed && trimmed !== folio.name) {
      renameFolioAction(folio.id, trimmed);
    }
    onClose();
  };

  return (
    <div
      role="dialog"
      aria-modal="true"
      onClick={onClose}
      style={{ position: "fixed", inset: 0, zIndex: 120, background: "rgba(20,14,10,.5)", display: "flex", alignItems: "flex-end", padding: "0 12px calc(12px + var(--safe-bottom))" }}
    >
      <div onClick={(event) => event.stopPropagation()} style={{ width: "100%" }}>
        <div style={{ borderRadius: 24, overflow: "hidden", background: "var(--surface)", boxShadow: "0 -18px 40px rgba(0,0,0,.25)" }}>
          <div style={{ padding: "16px 18px 14px", textAlign: "center" }}>
            <div style={{ width: 36, height: 4, borderRadius: 99, background: "var(--line-2)", margin: "0 auto 13px" }} />
            <div style={{ fontFamily: "var(--serif)", fontSize: 20, lineHeight: 1.15, color: "var(--ink)" }}>{folio.name}</div>
            <div style={{ fontFamily: "var(--sans)", fontSize: 12, color: "var(--muted)", marginTop: 4 }}>{pieceCountLabel(folio.piece_count)}</div>
          </div>
          {renaming ? (
            <div style={{ padding: "0 18px 18px" }}>
              <input
                autoFocus
                value={name}
                onChange={(event) => setName(event.target.value)}
                style={{ width: "100%", height: 50, borderRadius: 11, border: "1px solid var(--line-2)", background: "var(--surface-2)", color: "var(--ink)", outline: "none", padding: "0 13px", fontFamily: "var(--sans)", fontSize: 15 }}
              />
              <button type="button" onClick={submitRename} style={{ marginTop: 12, width: "100%", height: 52, borderRadius: 13, border: 0, background: "var(--accent)", color: "var(--on-accent)", fontFamily: "var(--sans)", fontSize: 15, fontWeight: 700 }}>
                Rename
              </button>
            </div>
          ) : (
            <>
              <button type="button" onClick={() => setRenaming(true)} style={{ width: "100%", minHeight: 52, border: 0, borderTop: "1px solid var(--line)", background: "transparent", color: "var(--ink)", fontFamily: "var(--sans)", fontSize: 15, fontWeight: 600, textAlign: "left", padding: "0 18px" }}>
                Rename
              </button>
              <button
                type="button"
                disabled={!firstPieceId}
                onClick={() => {
                  if (!firstPieceId) return;
                  changeFolioCoverAction(folio.id, firstPieceId);
                  onClose();
                }}
                style={{ width: "100%", minHeight: 52, border: 0, borderTop: "1px solid var(--line)", background: "transparent", color: firstPieceId ? "var(--ink)" : "var(--muted)", fontFamily: "var(--sans)", fontSize: 15, fontWeight: 600, textAlign: "left", padding: "0 18px" }}
              >
                Change cover
              </button>
              <button
                type="button"
                onClick={() => {
                  if (!confirmDelete) {
                    setConfirmDelete(true);
                    return;
                  }
                  void deleteFolioAction(folio.id).then((deleted) => {
                    if (deleted) {
                      onClose();
                      navigate("/folios");
                    }
                  });
                }}
                style={{ width: "100%", minHeight: 52, border: 0, borderTop: "1px solid var(--line)", background: "transparent", color: "var(--danger, #C0392B)", fontFamily: "var(--sans)", fontSize: 15, fontWeight: 700, textAlign: "left", padding: "0 18px" }}
              >
                {confirmDelete ? "Confirm delete" : "Delete folio"}
              </button>
            </>
          )}
        </div>
        <button type="button" onClick={onClose} style={{ marginTop: 8, width: "100%", height: 54, borderRadius: 18, border: 0, background: "var(--surface)", color: "var(--accent)", fontFamily: "var(--sans)", fontSize: 15, fontWeight: 700 }}>
          Cancel
        </button>
      </div>
    </div>
  );
}

function MobileFolioDetail({
  folio,
  folioId,
  pieces,
  total,
  piecesQuery,
  existingIds,
  pickerOpen,
  setPickerOpen,
  selectionMode,
  selectedIds,
  toggleSelected,
  toggleSelectionMode,
  clearSelection,
}: {
  folio: Folio | undefined;
  folioId: number;
  pieces: PieceVM[];
  total: number;
  piecesQuery: {
    isError: boolean;
    isLoading: boolean;
    hasNextPage: boolean;
    isFetchingNextPage: boolean;
    fetchNextPage: () => unknown;
  };
  existingIds: Set<number>;
  pickerOpen: boolean;
  setPickerOpen: (open: boolean) => void;
  selectionMode: boolean;
  selectedIds: Set<number>;
  toggleSelected: (id: number) => void;
  toggleSelectionMode: () => void;
  clearSelection: () => void;
}) {
  const [menuOpen, setMenuOpen] = useState(false);
  const firstPieceId = pieces[0]?.id ?? null;

  return (
    <div style={{ paddingBottom: 18 }}>
      <header style={{ minHeight: 82, paddingTop: 2 }}>
        <div style={{ height: 42, display: "flex", alignItems: "center", justifyContent: "space-between", gap: 10 }}>
          <Link to="/folios" aria-label="Back to folios" style={{ width: 40, height: 40, borderRadius: 99, color: "var(--ink)", display: "grid", placeItems: "center", textDecoration: "none" }}>
            <ChevronIcon dir="left" />
          </Link>
          <button type="button" aria-label={selectionMode ? "Finish selecting" : "Select pieces"} onClick={toggleSelectionMode} style={{ minWidth: 72, height: 40, borderRadius: 99, border: "1px solid var(--line)", background: selectionMode ? "var(--accent)" : "var(--surface)", color: selectionMode ? "var(--on-accent)" : "var(--ink)", fontFamily: "var(--sans)", fontSize: 13, fontWeight: 800 }}>
            {selectionMode ? "Done" : "Select"}
          </button>
          <button type="button" aria-label="Folio actions" onClick={() => setMenuOpen(true)} style={{ width: 40, height: 40, borderRadius: 99, border: "1px solid var(--line)", background: "var(--surface)", color: "var(--ink)", display: "grid", placeItems: "center" }}>
            <DotsIcon />
          </button>
        </div>
        <div style={{ padding: "1px 0 10px" }}>
          <h1 style={{ margin: 0, fontFamily: "var(--serif)", fontSize: 26, fontWeight: 500, lineHeight: 1.04, color: "var(--ink)", overflowWrap: "anywhere" }}>{folio?.name ?? "Loading folio"}</h1>
          <div style={{ marginTop: 5, fontFamily: "var(--sans)", fontSize: 12, color: "var(--muted)" }}>
            {piecesQuery.isLoading ? "Loading pieces..." : `${pieceCountLabel(total)} · ${updatedLabel(folio?.updated_at)}`}
          </div>
        </div>
      </header>

      {piecesQuery.isError ? (
        <div style={{ padding: "54px 8px", textAlign: "center", fontFamily: "var(--serif)", fontSize: 21, color: "var(--ink)" }}>This folio could not be reached.</div>
      ) : piecesQuery.isLoading ? (
        <section style={{ display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 8 }}>
          {Array.from({ length: 15 }, (_, index) => (
            <div key={index} className="okf-shimmer" style={{ aspectRatio: "1 / 1", borderRadius: 3, background: "var(--wall)" }} />
          ))}
        </section>
      ) : pieces.length === 0 ? (
        <div style={{ padding: "56px 8px 0", textAlign: "center" }}>
          <div style={{ fontFamily: "var(--serif)", fontSize: 22, fontWeight: 500, color: "var(--ink)" }}>No pieces yet</div>
          <div style={{ marginTop: 7, fontFamily: "var(--sans)", fontSize: 14, color: "var(--muted)" }}>Add pieces from your gallery to start shaping it.</div>
        </div>
      ) : (
        <>
          <section style={{ display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 8 }}>
            {pieces.map((piece) => (
              selectionMode ? (
                <SelectPieceTile key={piece.id} piece={piece} selected={selectedIds.has(piece.id)} onToggle={() => toggleSelected(piece.id)} />
              ) : (
                <MobileDetailTile key={piece.id} piece={piece} />
              )
            ))}
          </section>
          {piecesQuery.hasNextPage ? (
            <div style={{ display: "flex", justifyContent: "center", padding: "18px 0 0" }}>
              <button type="button" onClick={() => void piecesQuery.fetchNextPage()} disabled={piecesQuery.isFetchingNextPage} style={{ height: 40, borderRadius: 99, border: "1px solid var(--line-2)", background: "var(--surface)", color: "var(--ink)", fontFamily: "var(--sans)", fontSize: 13, fontWeight: 600, padding: "0 16px" }}>
                {piecesQuery.isFetchingNextPage ? "Loading..." : "Load more"}
              </button>
            </div>
          ) : null}
        </>
      )}

      <div style={{ position: "fixed", left: 0, right: 0, bottom: "calc(var(--mobile-tab-height) + var(--safe-bottom))", zIndex: 10, padding: "28px 20px 12px", pointerEvents: "none", background: "linear-gradient(to top, var(--bg), rgba(243,239,231,0))" }}>
        <button type="button" onClick={() => setPickerOpen(true)} style={{ pointerEvents: "auto", height: 48, borderRadius: 99, border: 0, background: "var(--accent)", color: "var(--on-accent)", boxShadow: "0 8px 20px rgba(124,36,32,.3)", display: "inline-flex", alignItems: "center", justifyContent: "center", gap: 8, padding: "0 18px", fontFamily: "var(--sans)", fontSize: 14, fontWeight: 800 }}>
          <PlusIcon size={15} />
          Add pieces
        </button>
      </div>

      {menuOpen && folio ? <MobileDetailMenu folio={folio} firstPieceId={firstPieceId} onClose={() => setMenuOpen(false)} /> : null}
      {pickerOpen ? <AddPiecesPicker folioId={folioId} folioName={folio?.name} existingIds={existingIds} onClose={() => setPickerOpen(false)} /> : null}
      {selectionMode ? <BulkEditBar selectedIds={Array.from(selectedIds)} onClear={clearSelection} /> : null}
    </div>
  );
}

const SHEET_SCRIM: CSSProperties = {
  position: "fixed",
  inset: 0,
  zIndex: 100,
  background: "rgba(12,9,6,0.62)",
  backdropFilter: "blur(6px)",
  WebkitBackdropFilter: "blur(6px)",
  display: "grid",
  placeItems: "center",
  padding: 34,
  animation: "okf-fade .2s ease",
};

const SHEET_CARD: CSSProperties = {
  width: "min(380px, 94vw)",
  borderRadius: 16,
  background: "var(--surface)",
  color: "var(--ink)",
  boxShadow: "0 40px 110px rgba(0,0,0,0.4)",
  overflow: "hidden",
  animation: "okf-rise .3s cubic-bezier(0.22,1,0.36,1)",
};

function PencilIcon({ size = 19 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M14.5 5.5l4 4M4 20l1-4 11.5-11.5a2 2 0 0 1 3 0l1 1a2 2 0 0 1 0 3L9 17l-5 3z" />
    </svg>
  );
}

function ImageIcon({ size = 19 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <rect x="3.5" y="5" width="17" height="14" rx="2" />
      <circle cx="8.5" cy="10" r="1.4" />
      <path d="M5 17l4.5-4 3 3 3-3 4 4" />
    </svg>
  );
}

function TrashIcon({ size = 19 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M4 7h16M9 7V5a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2M6 7l1 13a1 1 0 0 0 1 1h8a1 1 0 0 0 1-1l1-13" />
    </svg>
  );
}

function FolioCoverThumb({ folio, size = 56 }: { folio: Folio; size?: number }) {
  if (folio.cover_photo_id) {
    return (
      <span style={{ display: "block", width: size, height: size, borderRadius: 6, overflow: "hidden", background: "var(--surface-2)", flex: "none", boxShadow: "0 2px 6px var(--shadow)" }}>
        <OkfImage
          src={getPhotoThumbnailUrl(folio.cover_photo_id, 160)}
          alt={folio.name}
          title={folio.name}
          imgStyle={{ width: "100%", height: "100%", objectFit: "cover", display: "block" }}
          matteStyle={{ width: "100%", height: "100%", display: "grid", placeItems: "center", background: "var(--surface-2)" }}
        />
      </span>
    );
  }
  return (
    <span style={{ display: "grid", placeItems: "center", width: size, height: size, borderRadius: 6, background: "var(--surface-2)", border: "1px dashed var(--line-2)", flex: "none" }}>
      <BrandMark width={22} height={25} />
    </span>
  );
}

function DesktopFolioActionsSheet({
  folio,
  onClose,
  onRename,
  onCover,
  onDelete,
}: {
  folio: Folio;
  onClose: () => void;
  onRename: () => void;
  onCover: () => void;
  onDelete: () => void;
}) {
  const menuRow = (icon: ReactNode, label: string, onClick: () => void, danger = false) => (
    <Hov
      as="button"
      onClick={onClick}
      style={{
        appearance: "none",
        cursor: "pointer",
        width: "100%",
        display: "flex",
        alignItems: "center",
        gap: 15,
        padding: "15px 22px",
        border: 0,
        background: "transparent",
        color: danger ? "var(--danger)" : "var(--ink)",
        fontFamily: "var(--sans)",
        fontSize: 15.5,
        textAlign: "left",
      }}
      hover={{ background: "var(--bg)" }}
    >
      {icon}
      <span>{label}</span>
    </Hov>
  );

  return (
    <div
      role="presentation"
      onClick={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
      style={SHEET_SCRIM}
    >
      <div role="dialog" aria-modal="true" aria-label={`Folio actions for ${folio.name}`} style={SHEET_CARD}>
        <div style={{ display: "flex", alignItems: "center", gap: 14, padding: "20px 22px" }}>
          <FolioCoverThumb folio={folio} />
          <div style={{ minWidth: 0 }}>
            <div style={{ fontFamily: "var(--serif)", fontSize: 20, lineHeight: 1.15, color: "var(--ink)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{folio.name}</div>
            <div style={{ fontFamily: "var(--sans)", fontSize: 12.5, color: "var(--muted)", marginTop: 3 }}>{pieceCountLabel(folio.piece_count)}</div>
          </div>
        </div>
        <div style={{ borderTop: "1px solid var(--line)" }}>
          {menuRow(<PencilIcon />, "Rename", onRename)}
          {menuRow(<ImageIcon />, "Change cover", onCover)}
          {menuRow(<TrashIcon />, "Delete folio", onDelete, true)}
        </div>
      </div>
    </div>
  );
}

function FolioCoverSheet({ folio, onClose }: { folio: Folio; onClose: () => void }) {
  const { changeFolioCoverAction } = useFolio();
  const coverPieces = useQuery({
    queryKey: ["folio-cover-sheet-pieces", folio.id],
    queryFn: () => fetchFolioPieces(folio.id, 24, 0),
    staleTime: 15000,
  });

  return (
    <div
      role="presentation"
      onClick={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
      style={SHEET_SCRIM}
    >
      <div role="dialog" aria-modal="true" aria-label="Change cover" style={SHEET_CARD}>
        <div style={{ padding: "20px 22px 6px" }}>
          <div style={{ fontFamily: "var(--sans)", fontSize: 11, letterSpacing: "0.16em", textTransform: "uppercase", color: "var(--accent)" }}>Change cover</div>
          <h2 style={{ margin: "8px 0 0", fontFamily: "var(--serif)", fontWeight: 300, fontSize: 20, lineHeight: 1.15, color: "var(--ink)" }}>Choose a piece for the cover</h2>
        </div>
        <div style={{ maxHeight: 330, overflow: "auto", padding: "14px 18px 18px" }}>
          {coverPieces.isLoading ? (
            <div style={{ padding: "24px 0", textAlign: "center", fontFamily: "var(--sans)", fontSize: 13, color: "var(--muted)" }}>Loading pieces...</div>
          ) : coverPieces.data?.photos.length ? (
            <div style={{ display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 9 }}>
              {coverPieces.data.photos.map((photo) => {
                const piece = mapPhoto(photo);
                const isCurrent = photo.ID === folio.cover_photo_id;
                return (
                  <button
                    key={photo.ID}
                    type="button"
                    aria-label={`Use ${piece.t} as cover`}
                    onClick={() => {
                      changeFolioCoverAction(folio.id, photo.ID);
                      onClose();
                    }}
                    style={{ position: "relative", aspectRatio: "1 / 1", border: 0, borderRadius: 4, padding: 0, overflow: "hidden", background: "var(--wall)", cursor: "pointer", boxShadow: isCurrent ? "0 0 0 3px var(--accent)" : "0 1px 5px var(--shadow)", transition: "transform .12s ease" }}
                    onMouseEnter={(event) => { event.currentTarget.style.transform = "scale(0.97)"; }}
                    onMouseLeave={(event) => { event.currentTarget.style.transform = "none"; }}
                  >
                    <OkfImage src={getPhotoThumbnailUrl(photo.ID, 400)} alt={piece.t} title={piece.t} imgStyle={{ width: "100%", height: "100%", objectFit: "cover", display: "block" }} matteStyle={TILE_MATTE} />
                    {isCurrent ? (
                      <span aria-hidden="true" style={{ position: "absolute", top: 6, right: 6, zIndex: 3, width: 20, height: 20, borderRadius: 99, background: "var(--accent)", color: "var(--on-accent)", display: "grid", placeItems: "center", boxShadow: "0 1px 4px rgba(0,0,0,.3)" }}>
                        <svg width="12" height="12" viewBox="0 0 16 16" fill="none">
                          <path d="M3.2 8.4l3 3 6.6-7" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
                        </svg>
                      </span>
                    ) : null}
                  </button>
                );
              })}
            </div>
          ) : (
            <div style={{ padding: "24px 0", textAlign: "center", fontFamily: "var(--sans)", fontSize: 13, color: "var(--muted)" }}>Add pieces before choosing a cover.</div>
          )}
        </div>
      </div>
    </div>
  );
}

export default function FolioDetail() {
  const params = useParams();
  const folioId = Number(params.id);
  const { setViewerPieces, removePieceFromFolioAction, renameFolioAction, deleteFolioAction } = useFolio();
  const navigate = useNavigate();
  const { isMobile } = useViewport();
  const [pickerOpen, setPickerOpen] = useState(false);
  const [selectionMode, setSelectionMode] = useState(false);
  const [selectedIds, setSelectedIds] = useState<Set<number>>(() => new Set());
  const [actionsOpen, setActionsOpen] = useState(false);
  const [coverOpen, setCoverOpen] = useState(false);
  const [renameOpen, setRenameOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const folios = useQuery({ queryKey: ["folios"], queryFn: fetchFolios });
  const folio = folios.data?.folios.find((item) => item.id === folioId);
  const piecesQuery = useInfiniteQuery({
    queryKey: ["folio-pieces", folioId],
    queryFn: ({ pageParam }) => fetchFolioPieces(folioId, PAGE_SIZE, pageParam as number),
    enabled: Number.isFinite(folioId) && folioId > 0,
    initialPageParam: 0,
    getNextPageParam: (lastPage, allPages) => {
      const loaded = allPages.reduce((n, pg) => n + pg.photos.length, 0);
      return loaded < lastPage.total ? loaded : undefined;
    },
  });
  const photos = useMemo(() => piecesQuery.data?.pages.flatMap((page) => page.photos) ?? [], [piecesQuery.data]);
  const pieces = useMemo(() => photos.map(mapPhoto), [photos]);
  const existingIds = useMemo(() => new Set(photos.map((photo) => photo.ID)), [photos]);
  const total = piecesQuery.data?.pages[0]?.total ?? folio?.piece_count ?? pieces.length;
  const toggleSelected = (id: number) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };
  const clearSelection = () => {
    setSelectedIds(new Set());
    setSelectionMode(false);
  };
  const toggleSelectionMode = () => {
    setSelectionMode((enabled) => !enabled);
    setSelectedIds(new Set());
  };
  const removeSelectedFromFolio = () => {
    const ids = Array.from(selectedIds);
    if (ids.length === 0) return;
    for (const id of ids) {
      removePieceFromFolioAction(folioId, id);
    }
    clearSelection();
  };

  useEffect(() => {
    setViewerPieces(pieces);
    return () => setViewerPieces([]);
  }, [pieces, setViewerPieces]);

  if (!Number.isFinite(folioId) || folioId <= 0) {
    return (
      <div>
        <PageHeader eyebrow="Folios" title="Folio not found" subcopy={<Link to="/folios" style={{ color: "var(--accent)" }}>Back to folios</Link>} />
      </div>
    );
  }

  if (isMobile) {
    return (
      <MobileFolioDetail
        folio={folio}
        folioId={folioId}
        pieces={pieces}
        total={total}
        piecesQuery={piecesQuery}
        existingIds={existingIds}
        pickerOpen={pickerOpen}
        setPickerOpen={setPickerOpen}
        selectionMode={selectionMode}
        selectedIds={selectedIds}
        toggleSelected={toggleSelected}
        toggleSelectionMode={toggleSelectionMode}
        clearSelection={clearSelection}
      />
    );
  }

  return (
    <div>
      <Hov
        as={Link}
        to="/folios"
        style={{
          appearance: "none",
          cursor: "pointer",
          display: "inline-flex",
          alignItems: "center",
          gap: 8,
          margin: "34px 0 0",
          padding: "7px 4px",
          border: 0,
          background: "transparent",
          fontFamily: "var(--sans)",
          fontSize: 13.5,
          color: "var(--graphite)",
          textDecoration: "none",
        }}
        hover={{ color: "var(--accent)" }}
      >
        <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
          <path d="M15 5l-7 7 7 7" />
        </svg>
        All folios
      </Hov>

      <header
        style={{
          padding: "14px 0 24px",
          display: "flex",
          alignItems: "flex-end",
          justifyContent: "space-between",
          gap: 20,
          flexWrap: "wrap",
          borderBottom: "1px solid var(--line)",
        }}
      >
        <div style={{ minWidth: 0 }}>
          <h1 style={{ margin: 0, fontFamily: "var(--serif)", fontWeight: 300, fontSize: 46, lineHeight: 1.0, letterSpacing: "-0.012em", color: "var(--ink)", overflowWrap: "anywhere" }}>{folio?.name ?? "Loading folio"}</h1>
          <div style={{ marginTop: 12, fontFamily: "var(--sans)", fontSize: 13.5, color: "var(--muted)" }}>
            {piecesQuery.isLoading ? "Loading pieces..." : `${pieceCountLabel(total)} · ${updatedLabel(folio?.updated_at)}`}
          </div>
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 11 }}>
          <Hov
            as="button"
            onClick={toggleSelectionMode}
            style={{
              appearance: "none",
              cursor: "pointer",
              fontFamily: "var(--sans)",
              fontSize: 13.5,
              fontWeight: 500,
              padding: "11px 18px",
              borderRadius: 99,
              border: "1px solid var(--line-2)",
              background: selectionMode ? "var(--accent)" : "var(--surface)",
              color: selectionMode ? "var(--on-accent)" : "var(--ink)",
            }}
            hover={selectionMode ? undefined : { borderColor: "var(--accent-line)", color: "var(--accent)" }}
          >
            {selectionMode ? "Done" : "Select"}
          </Hov>
          <Hov
            as="button"
            onClick={() => setPickerOpen(true)}
            style={{
              appearance: "none",
              cursor: "pointer",
              display: "inline-flex",
              alignItems: "center",
              gap: 8,
              fontFamily: "var(--sans)",
              fontSize: 13.5,
              fontWeight: 500,
              padding: "11px 18px",
              borderRadius: 99,
              border: 0,
              background: "var(--accent)",
              color: "var(--on-accent)",
            }}
            hover={{ filter: "brightness(1.07)" }}
          >
            <PlusIcon size={14} strokeWidth={2.1} />
            Add pieces
          </Hov>
          <Hov
            as="button"
            aria-label={`Actions for ${folio?.name ?? "folio"}`}
            onClick={() => setActionsOpen(true)}
            style={{
              appearance: "none",
              cursor: "pointer",
              width: 42,
              height: 42,
              borderRadius: 99,
              border: "1px solid var(--line-2)",
              background: "var(--surface)",
              color: "var(--graphite)",
              display: "grid",
              placeItems: "center",
              flex: "none",
            }}
            hover={{ borderColor: "var(--accent-line)", color: "var(--accent)" }}
          >
            <DotsIcon size={18} />
          </Hov>
        </div>
      </header>

      {piecesQuery.isError ? (
        <div style={{ padding: "90px 0", textAlign: "center", fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 22, color: "var(--graphite)" }}>
          This folio could not be reached.
        </div>
      ) : piecesQuery.isLoading ? (
        <div style={{ padding: "90px 0", textAlign: "center", fontFamily: "var(--sans)", fontSize: 14, color: "var(--muted)" }}>Loading pieces...</div>
      ) : pieces.length === 0 ? (
        <div style={{ display: "flex", flexDirection: "column", alignItems: "center", textAlign: "center", padding: "104px 0 0" }}>
          <span style={{ opacity: 0.6, display: "inline-flex" }}>
            <BrandMark width={58} height={64} tone="muted" />
          </span>
          <h2 style={{ margin: "26px 0 0", fontFamily: "var(--serif)", fontWeight: 300, fontSize: 27, lineHeight: 1.15, color: "var(--ink)" }}>Nothing here yet</h2>
          <div style={{ margin: "11px 0 0", fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 16, color: "var(--graphite)" }}>Add a few pieces to begin this folio.</div>
        </div>
      ) : (
        <section style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(166px, 1fr))", gap: 13, padding: "30px 0 0" }}>
          {pieces.map((piece) => (
            selectionMode ? (
              <SelectPieceTile key={piece.id} piece={piece} selected={selectedIds.has(piece.id)} onToggle={() => toggleSelected(piece.id)} />
            ) : (
              <PieceTile key={piece.id} piece={piece} />
            )
          ))}
          {piecesQuery.hasNextPage ? (
            <div style={{ display: "flex", justifyContent: "center", padding: "36px 0 0", gridColumn: "1 / -1" }}>
              <Hov
                as="button"
                onClick={() => void piecesQuery.fetchNextPage()}
                disabled={piecesQuery.isFetchingNextPage}
                style={{ appearance: "none", cursor: piecesQuery.isFetchingNextPage ? "wait" : "pointer", fontFamily: "var(--sans)", fontSize: 13, padding: "10px 18px", borderRadius: 99, border: "1px solid var(--line-2)", background: "var(--surface)", color: "var(--ink)" }}
                hover={{ borderColor: "var(--accent-line)", color: "var(--accent)" }}
              >
                {piecesQuery.isFetchingNextPage ? "Loading..." : "Load more"}
              </Hov>
            </div>
          ) : null}
        </section>
      )}

      {pickerOpen ? <AddPiecesPicker folioId={folioId} folioName={folio?.name} existingIds={existingIds} onClose={() => setPickerOpen(false)} /> : null}
      {selectionMode ? <BulkEditBar selectedIds={Array.from(selectedIds)} onClear={clearSelection} onRemoveFromFolio={removeSelectedFromFolio} /> : null}
      {actionsOpen && folio ? (
        <DesktopFolioActionsSheet
          folio={folio}
          onClose={() => setActionsOpen(false)}
          onRename={() => {
            setActionsOpen(false);
            setRenameOpen(true);
          }}
          onCover={() => {
            setActionsOpen(false);
            setCoverOpen(true);
          }}
          onDelete={() => {
            setActionsOpen(false);
            setDeleteOpen(true);
          }}
        />
      ) : null}
      {coverOpen && folio ? <FolioCoverSheet folio={folio} onClose={() => setCoverOpen(false)} /> : null}
      {renameOpen && folio ? (
        <FolioNameModal
          mode="rename"
          initialName={folio.name}
          onClose={() => setRenameOpen(false)}
          onSubmitName={(name) => renameFolioAction(folio.id, name)}
        />
      ) : null}
      {deleteOpen && folio ? (
        <ConfirmationDialog
          eyebrow="Delete folio"
          title={`Delete "${folio.name}"?`}
          description={`This removes the folio. The ${folio.piece_count.toLocaleString()} ${folio.piece_count === 1 ? "piece" : "pieces"} stay in your gallery.`}
          confirmLabel="Delete"
          busyLabel="Deleting"
          destructive
          onCancel={() => setDeleteOpen(false)}
          onConfirm={async () => {
            setDeleteOpen(false);
            const deleted = await deleteFolioAction(folio.id);
            if (deleted) navigate("/folios");
          }}
        />
      ) : null}
    </div>
  );
}
