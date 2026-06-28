import { useEffect, useMemo, useState, type CSSProperties, type MouseEvent } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { fetchFolioPieces, fetchFolios, fetchGalleryCatalog, getPhotoThumbnailUrl } from "../api";
import type { Folio, Photo } from "../types";
import { mapPhoto, useFolio, type PieceVM } from "./context";
import { ChevronIcon, CloseIcon, DotsIcon, Hov, OkfImage, OutlineButton, PageHeader, PlusIcon } from "./ui";
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

function PieceTile({ piece, folioId }: { piece: PieceVM; folioId: number }) {
  const { openPiece, removePieceFromFolioAction } = useFolio();
  const [hover, setHover] = useState(false);
  return (
    <figure
      onClick={() => openPiece(piece.id)}
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
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
      <Hov
        as="button"
        aria-label={`Remove ${piece.t} from folio`}
        onClick={(event: MouseEvent<HTMLButtonElement>) => {
          event.stopPropagation();
          removePieceFromFolioAction(folioId, piece.id);
        }}
        style={{
          position: "absolute",
          top: 9,
          right: 9,
          zIndex: 4,
          appearance: "none",
          cursor: "pointer",
          border: "1px solid rgba(255,255,255,0.28)",
          background: "rgba(12,10,7,0.42)",
          color: "#FBF6EE",
          borderRadius: 99,
          height: 30,
          padding: "0 11px",
          fontFamily: "var(--sans)",
          fontSize: 12,
          backdropFilter: "blur(8px)",
        }}
        hover={{ background: "rgba(12,10,7,0.68)" }}
      >
        Remove
      </Hov>
      <figcaption
        style={{
          position: "absolute",
          left: 0,
          right: 0,
          bottom: 0,
          zIndex: 3,
          padding: "26px 12px 11px",
          opacity: hover ? 1 : 0,
          transition: "opacity .2s ease",
          background: "linear-gradient(to top, rgba(12,10,7,0.78), rgba(12,10,7,0))",
        }}
      >
        <div style={{ fontFamily: "var(--serif)", fontSize: 13.5, lineHeight: 1.2, color: "#FBF6EE" }}>{piece.t}</div>
        <div style={{ fontFamily: "var(--sans)", fontSize: 10.5, color: "rgba(251,246,238,0.7)", marginTop: 2 }}>{piece.a}</div>
      </figcaption>
    </figure>
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
    <Hov
      as="button"
      disabled={disabled}
      onClick={onToggle}
      style={{
        appearance: "none",
        cursor: disabled ? "not-allowed" : "pointer",
        opacity: disabled ? 0.45 : 1,
        border: selected ? "2px solid var(--accent)" : "1px solid var(--line)",
        background: "var(--surface)",
        padding: 0,
        textAlign: "left",
        color: "inherit",
      }}
      hover={disabled ? undefined : { borderColor: "var(--accent-line)" }}
    >
      <span style={{ display: "block", position: "relative", aspectRatio: "1 / 1", overflow: "hidden" }}>
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
        {selected ? (
          <span
            style={{
              position: "absolute",
              top: 8,
              right: 8,
              zIndex: 4,
              width: 24,
              height: 24,
              borderRadius: 99,
              background: "var(--accent)",
              color: "var(--on-accent)",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              fontFamily: "var(--sans)",
              fontSize: 13,
            }}
          >
            <svg width="13" height="13" viewBox="0 0 16 16" fill="none">
              <path d="M3.2 8.4l3 3 6.6-7" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          </span>
        ) : null}
      </span>
      <span style={{ display: "block", padding: "9px 10px 11px" }}>
        <span style={{ display: "block", fontFamily: "var(--serif)", fontSize: 14, lineHeight: 1.2, color: "var(--ink)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{piece.t}</span>
        <span style={{ display: "block", fontFamily: "var(--sans)", fontSize: 11.5, color: "var(--muted)", marginTop: 3, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{piece.a}</span>
      </span>
    </Hov>
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
  const { addPieceToFolioAction } = useFolio();
  const { isMobile } = useViewport();
  const [selected, setSelected] = useState<Set<number>>(() => new Set());
  const catalog = useInfiniteQuery({
    queryKey: ["folio-piece-picker", folioId],
    queryFn: ({ pageParam }) => fetchGalleryCatalog(PICKER_PAGE_SIZE, pageParam as number, {}),
    initialPageParam: 0,
    getNextPageParam: (lastPage, allPages) => {
      const loaded = allPages.reduce((n, pg) => n + pg.photos.length, 0);
      return loaded < lastPage.total ? loaded : undefined;
    },
  });
  const photos = catalog.data?.pages.flatMap((page) => page.photos) ?? [];

  const toggle = (photoId: number) => {
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

  const addSelected = () => {
    selected.forEach((photoId) => addPieceToFolioAction(folioId, photoId));
    onClose();
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
          <button type="button" onClick={onClose} style={{ border: 0, background: "transparent", color: "var(--accent)", fontFamily: "var(--sans)", fontSize: 14, fontWeight: 700, textAlign: "left", padding: 0 }}>
            Cancel
          </button>
          <div style={{ textAlign: "center", minWidth: 0 }}>
            <div style={{ fontFamily: "var(--serif)", fontSize: 20, lineHeight: 1.05, color: "var(--ink)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>Add to {folioName ?? "folio"}</div>
          </div>
          <button type="button" onClick={addSelected} disabled={selected.size === 0} style={{ border: 0, background: "transparent", color: selected.size ? "var(--accent)" : "var(--muted)", fontFamily: "var(--sans)", fontSize: 14, fontWeight: 700, textAlign: "right", padding: 0 }}>
            Done
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
                  disabled={existingIds.has(photo.ID)}
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
                disabled={catalog.isFetchingNextPage}
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
            onClick={addSelected}
            disabled={selected.size === 0}
            style={{ width: "100%", height: 52, borderRadius: 14, border: 0, background: "var(--accent)", color: "var(--on-accent)", fontFamily: "var(--sans)", fontSize: 15, fontWeight: 800, opacity: selected.size ? 1 : 0.55, boxShadow: selected.size ? "0 8px 20px rgba(124,36,32,.3)" : "none" }}
          >
            Add {selected.size.toLocaleString()} {selected.size === 1 ? "piece" : "pieces"}
          </button>
        </div>
      </div>
    );
  }

  return (
    <div
      onClick={onClose}
      style={{
        position: "fixed",
        inset: 0,
        zIndex: 100,
        background: "rgba(12,9,6,0.7)",
        backdropFilter: "blur(7px)",
        WebkitBackdropFilter: "blur(7px)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        padding: 34,
      }}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        style={{ width: "min(1040px, 95vw)", maxHeight: "90vh", overflow: "auto", background: "var(--surface)", boxShadow: "0 50px 130px rgba(0,0,0,0.5)" }}
      >
        <div style={{ padding: "24px 30px", borderBottom: "1px solid var(--line)", display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 20 }}>
          <div>
            <div style={{ fontFamily: "var(--sans)", fontSize: 11, letterSpacing: "0.2em", textTransform: "uppercase", color: "var(--accent)" }}>Add Pieces</div>
            <h2 style={{ margin: "9px 0 0", fontFamily: "var(--serif)", fontWeight: 300, fontSize: 27, color: "var(--ink)", letterSpacing: "-0.01em" }}>Choose from the gallery</h2>
          </div>
          <Hov
            as="button"
            onClick={onClose}
            aria-label="Close"
            style={{ appearance: "none", cursor: "pointer", width: 34, height: 34, borderRadius: 99, border: "1px solid var(--line)", background: "transparent", color: "var(--muted)", display: "flex", alignItems: "center", justifyContent: "center" }}
            hover={{ color: "var(--ink)", borderColor: "var(--line-2)" }}
          >
            <CloseIcon size={15} />
          </Hov>
        </div>

        <div style={{ padding: "24px 30px 30px" }}>
          {catalog.isError ? (
            <div style={{ padding: "50px 0", fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 20, color: "var(--graphite)", textAlign: "center" }}>
              The gallery could not be reached.
            </div>
          ) : catalog.isLoading ? (
            <div style={{ padding: "50px 0", fontFamily: "var(--sans)", fontSize: 14, color: "var(--muted)", textAlign: "center" }}>Loading pieces...</div>
          ) : (
            <section style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(142px, 1fr))", gap: 12 }}>
              {photos.map((photo) => (
                <PickerTile
                  key={photo.ID}
                  photo={photo}
                  selected={selected.has(photo.ID)}
                  disabled={existingIds.has(photo.ID)}
                  onToggle={() => toggle(photo.ID)}
                />
              ))}
            </section>
          )}
          {catalog.hasNextPage ? (
            <div style={{ display: "flex", justifyContent: "center", padding: "24px 0 0" }}>
              <Hov
                as="button"
                onClick={() => void catalog.fetchNextPage()}
                disabled={catalog.isFetchingNextPage}
                style={{ appearance: "none", cursor: catalog.isFetchingNextPage ? "wait" : "pointer", fontFamily: "var(--sans)", fontSize: 13, padding: "10px 18px", borderRadius: 99, border: "1px solid var(--line-2)", background: "var(--surface)", color: "var(--ink)" }}
                hover={{ borderColor: "var(--accent-line)", color: "var(--accent)" }}
              >
                {catalog.isFetchingNextPage ? "Loading..." : "Load more"}
              </Hov>
            </div>
          ) : null}
        </div>

        <div style={{ padding: "18px 30px", borderTop: "1px solid var(--line)", display: "flex", alignItems: "center", justifyContent: "space-between", gap: 18 }}>
          <div style={{ fontFamily: "var(--sans)", fontSize: 12, color: "var(--muted)" }}>
            {selected.size.toLocaleString()} selected
          </div>
          <div style={{ display: "flex", gap: 11, flex: "none" }}>
            <Hov
              as="button"
              onClick={onClose}
              style={{ appearance: "none", cursor: "pointer", fontFamily: "var(--sans)", fontSize: 13.5, padding: "10px 18px", borderRadius: 99, border: 0, background: "transparent", color: "var(--muted)" }}
              hover={{ color: "var(--ink)" }}
            >
              Cancel
            </Hov>
            <Hov
              as="button"
              onClick={addSelected}
              disabled={selected.size === 0}
              style={{ appearance: "none", cursor: selected.size === 0 ? "not-allowed" : "pointer", opacity: selected.size === 0 ? 0.6 : 1, fontFamily: "var(--sans)", fontSize: 13.5, fontWeight: 500, padding: "10px 22px", borderRadius: 99, border: 0, background: "var(--accent)", color: "var(--on-accent)" }}
              hover={{ filter: "brightness(1.06)" }}
            >
              Add pieces
            </Hov>
          </div>
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
              <MobileDetailTile key={piece.id} piece={piece} />
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
    </div>
  );
}

export default function FolioDetail() {
  const params = useParams();
  const folioId = Number(params.id);
  const { setViewerPieces } = useFolio();
  const { isMobile } = useViewport();
  const [pickerOpen, setPickerOpen] = useState(false);
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
  const photos = piecesQuery.data?.pages.flatMap((page) => page.photos) ?? [];
  const pieces = useMemo(() => photos.map(mapPhoto), [photos]);
  const existingIds = useMemo(() => new Set(photos.map((photo) => photo.ID)), [photos]);
  const total = piecesQuery.data?.pages[0]?.total ?? folio?.piece_count ?? pieces.length;

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
      />
    );
  }

  return (
    <div>
      <PageHeader
        eyebrow="Folio"
        title={folio?.name ?? "Loading folio"}
        subcopy={piecesQuery.isLoading ? "Gathering this folio..." : pieceCountLabel(total)}
        action={<OutlineButton onClick={() => setPickerOpen(true)}>Add pieces</OutlineButton>}
      />

      {piecesQuery.isError ? (
        <div style={{ padding: "90px 0", textAlign: "center", fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 22, color: "var(--graphite)" }}>
          This folio could not be reached.
        </div>
      ) : piecesQuery.isLoading ? (
        <div style={{ padding: "90px 0", textAlign: "center", fontFamily: "var(--sans)", fontSize: 14, color: "var(--muted)" }}>Loading pieces...</div>
      ) : pieces.length === 0 ? (
        <div style={{ textAlign: "center", padding: "80px 0", color: "var(--muted)" }}>
          <div style={{ fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 24, color: "var(--graphite)" }}>No pieces in this folio yet.</div>
          <div style={{ fontFamily: "var(--sans)", fontSize: 14, marginTop: 10 }}>Add pieces from your gallery to start shaping it.</div>
        </div>
      ) : (
        <>
          <div style={{ display: "flex", alignItems: "center", gap: 14, padding: "30px 0 22px", fontFamily: "var(--sans)", fontSize: 13, color: "var(--muted)" }}>
            <Link to="/folios" style={{ color: "var(--accent)", textDecoration: "none" }}>All folios</Link>
            <span style={{ opacity: 0.5 }}>·</span>
            <span>{pieceCountLabel(total)}</span>
            <span style={{ flex: 1 }} />
            <span style={{ color: "var(--faint)" }}>Click to open · remove pieces from the corner</span>
          </div>
          <section style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(166px, 1fr))", gap: 13 }}>
            {pieces.map((piece) => (
              <PieceTile key={piece.id} piece={piece} folioId={folioId} />
            ))}
          </section>
          {piecesQuery.hasNextPage ? (
            <div style={{ display: "flex", justifyContent: "center", padding: "36px 0 0" }}>
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
        </>
      )}

      {pickerOpen ? <AddPiecesPicker folioId={folioId} folioName={folio?.name} existingIds={existingIds} onClose={() => setPickerOpen(false)} /> : null}
    </div>
  );
}
