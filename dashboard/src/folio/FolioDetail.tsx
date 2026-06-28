import { useEffect, useMemo, useState, type CSSProperties, type MouseEvent } from "react";
import { Link, useParams } from "react-router-dom";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { fetchFolioPieces, fetchFolios, fetchGalleryCatalog, getPhotoThumbnailUrl } from "../api";
import type { Photo } from "../types";
import { mapPhoto, useFolio, type PieceVM } from "./context";
import { CloseIcon, Hov, OkfImage, OutlineButton, PageHeader } from "./ui";

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

function AddPiecesPicker({
  folioId,
  existingIds,
  onClose,
}: {
  folioId: number;
  existingIds: Set<number>;
  onClose: () => void;
}) {
  const { addPieceToFolioAction } = useFolio();
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

export default function FolioDetail() {
  const params = useParams();
  const folioId = Number(params.id);
  const { setViewerPieces } = useFolio();
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

      {pickerOpen ? <AddPiecesPicker folioId={folioId} existingIds={existingIds} onClose={() => setPickerOpen(false)} /> : null}
    </div>
  );
}
