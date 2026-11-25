import { useEffect, useMemo, useRef, useState, type CSSProperties, type FormEvent, type MouseEvent, type ReactNode } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { fetchFolioPieces, fetchFolios, getPhotoThumbnailUrl } from "../api";
import type { Folio, Photo } from "../types";
import { mapPhoto, useFolio } from "./context";
import { BrandMark, ConfirmationDialog, DotsIcon, Hov, OkfImage, OutlineButton, PageHeader, PictureFrameIcon, PlusIcon } from "./ui";
import { useViewport } from "./useViewport";

const TILE_MATTE: CSSProperties = {
  position: "absolute",
  inset: 0,
  flexDirection: "column",
  alignItems: "center",
  justifyContent: "center",
  gap: 7,
  padding: 18,
  textAlign: "center",
  background: "linear-gradient(155deg, var(--surface-2), var(--surface))",
};

function folioPiecesLabel(count: number): string {
  return `${count.toLocaleString()} ${count === 1 ? "piece" : "pieces"}`;
}

type FolioSheetState =
  | { mode: "create" }
  | { mode: "actions"; folio: Folio }
  | { mode: "rename"; folio: Folio }
  | { mode: "cover"; folio: Folio };

const MOBILE_SHEET_BUTTON: CSSProperties = {
  width: "100%",
  minHeight: 52,
  appearance: "none",
  border: 0,
  borderTop: "1px solid var(--line)",
  background: "transparent",
  color: "var(--ink)",
  fontFamily: "var(--sans)",
  fontSize: 15,
  fontWeight: 600,
  textAlign: "left",
  padding: "0 18px",
};

function coverIds(folio: Folio, photos?: Photo[]): number[] {
  const ids = photos?.map((photo) => photo.ID).filter((id) => id !== folio.cover_photo_id) ?? [];
  if (folio.cover_photo_id) ids.unshift(folio.cover_photo_id);
  return ids.slice(0, 3);
}

/**
 * The single hero cover for the Web (desktop) folio tile: the user-chosen cover
 * if it is still one of the folio's pieces, else the first piece, else nothing
 * (empty folio → matte). `photos` is the cover-preview window; when it is the
 * complete piece list (`photos.length >= piece_count`) and the stored cover is
 * absent, that piece was removed from the folio, so we fall back to the first
 * piece. For larger folios the window cannot disprove membership, so we honour
 * the stored cover rather than risk hiding a valid, older cover choice.
 */
function heroCoverId(folio: Folio, photos?: Photo[]): number | undefined {
  const chosen = folio.cover_photo_id ?? undefined;
  if (chosen) {
    const complete = photos !== undefined && photos.length >= folio.piece_count;
    if (complete && !photos.some((photo) => photo.ID === chosen)) return photos[0]?.ID;
    return chosen;
  }
  return photos?.[0]?.ID;
}

function FolioCoverObject({
  folio,
  ids,
  selected = false,
  eager = false,
}: {
  folio: Folio;
  ids: number[];
  selected?: boolean;
  eager?: boolean;
}) {
  const layers = [
    { key: "back", id: ids[2], left: "15%", top: "15%", zIndex: 1, size: 520, filter: "brightness(0.8) saturate(0.9)", shadow: "0 8px 16px var(--shadow)" },
    { key: "mid", id: ids[1], left: "7.5%", top: "7.5%", zIndex: 2, size: 600, filter: "brightness(0.92)", shadow: "0 9px 18px var(--shadow)" },
    { key: "hero", id: ids[0], left: 0, top: 0, zIndex: 3, size: 720, filter: undefined, shadow: "0 14px 28px var(--shadow-2), 0 2px 6px var(--shadow)" },
  ].filter((layer) => layer.id);

  return (
    <span style={{ display: "block", position: "relative", width: "100%", aspectRatio: "1 / 1", fontFamily: "var(--sans)" }}>
      {layers.length === 0 ? (
        <span
          style={{
            position: "absolute",
            inset: 0,
            border: "1.5px dashed var(--line-2)",
            borderRadius: 3,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            background: "var(--surface)",
            boxShadow: selected ? "0 0 0 3px var(--accent)" : undefined,
          }}
        >
          <span style={{ opacity: 0.55 }}>
            <BrandMark width={34} height={39} />
          </span>
        </span>
      ) : (
        layers.map((layer) => (
          <span
            key={layer.key}
            style={{
              position: "absolute",
              left: layer.left,
              top: layer.top,
              width: "80%",
              height: "80%",
              zIndex: layer.zIndex,
              borderRadius: 3,
              overflow: "hidden",
              background: "var(--surface-2)",
              boxShadow: layer.key === "hero" && selected ? `0 0 0 3px var(--accent), ${layer.shadow}` : layer.shadow,
              border: "1px solid var(--line)",
            }}
          >
            <OkfImage
              src={getPhotoThumbnailUrl(layer.id!, layer.size)}
              alt={layer.key === "hero" ? folio.name : ""}
              title={folio.name}
              artist={layer.key === "hero" ? folioPiecesLabel(folio.piece_count) : undefined}
              loading={eager && layer.key === "hero" ? "eager" : "lazy"}
              imgStyle={{ width: "100%", height: "100%", objectFit: "cover", display: "block", filter: layer.filter }}
              matteStyle={{ ...TILE_MATTE, borderRadius: 3 }}
              matteTitleStyle={{ fontFamily: "var(--serif)", fontSize: 16, lineHeight: 1.12, color: "var(--ink)" }}
              matteArtistStyle={{ fontFamily: "var(--sans)", fontSize: 11, color: "var(--muted)" }}
            />
          </span>
        ))
      )}
    </span>
  );
}

/**
 * Web (desktop) folio cover: a single hero image over the tile matte, per the
 * binding Web design. The 3-layer {@link FolioCoverObject} stack stays
 * Mobile-only. Empty folios (no hero) render the gradient matte with a
 * picture-frame icon, the folio name and its piece count.
 */
function WebFolioCover({ folio, heroId }: { folio: Folio; heroId?: number }) {
  const count = folioPiecesLabel(folio.piece_count);
  if (heroId) {
    return (
      <OkfImage
        src={getPhotoThumbnailUrl(heroId, 720)}
        alt={folio.name}
        title={folio.name}
        artist={count}
        imgStyle={{ position: "absolute", inset: 0, width: "100%", height: "100%", objectFit: "cover", display: "block" }}
        matteStyle={{ position: "absolute", inset: 0, gap: 9, padding: 18 }}
        matteTitleStyle={{ fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 17, lineHeight: 1.15 }}
        matteArtistStyle={{ fontFamily: "var(--sans)", fontSize: 10, letterSpacing: "0.12em", textTransform: "uppercase" }}
      />
    );
  }
  return (
    <span
      style={{
        position: "absolute",
        inset: 0,
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        gap: 9,
        padding: 18,
        textAlign: "center",
        background: "linear-gradient(155deg, var(--surface-2), var(--surface))",
      }}
    >
      <span style={{ color: "color-mix(in srgb, var(--ink) 42%, transparent)" }}>
        <PictureFrameIcon />
      </span>
      <span style={{ fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 17, lineHeight: 1.15, color: "color-mix(in srgb, var(--ink) 62%, transparent)" }}>
        {folio.name}
      </span>
      <span style={{ fontFamily: "var(--sans)", fontSize: 10, letterSpacing: "0.12em", textTransform: "uppercase", color: "color-mix(in srgb, var(--ink) 48%, transparent)" }}>
        {count}
      </span>
    </span>
  );
}

function MobileFolioTile({
  folio,
  selected,
  onActions,
}: {
  folio: Folio;
  selected: boolean;
  onActions: (folio: Folio) => void;
}) {
  const pieces = useQuery({
    queryKey: ["folio-cover-pieces", folio.id],
    queryFn: () => fetchFolioPieces(folio.id, 3, 0),
    staleTime: 15000,
  });

  return (
    <figure style={{ margin: 0, minWidth: 0 }}>
      <div style={{ position: "relative" }}>
        <Link
          to={`/folios/${folio.id}`}
          style={{
            display: "block",
            position: "relative",
            aspectRatio: "1 / 1",
            color: "inherit",
            textDecoration: "none",
          }}
        >
          <FolioCoverObject folio={folio} ids={coverIds(folio, pieces.data?.photos)} selected={selected} eager />
        </Link>
        <button
          type="button"
          aria-label={`Actions for ${folio.name}`}
          onClick={() => onActions(folio)}
          style={{
            position: "absolute",
            top: 8,
            right: 8,
            zIndex: 3,
            width: 36,
            height: 36,
            borderRadius: 99,
            border: "1px solid rgba(255,255,255,.34)",
            background: "rgba(12,10,7,.42)",
            color: "#FBF6EE",
            display: "grid",
            placeItems: "center",
            backdropFilter: "blur(10px)",
          }}
        >
          <DotsIcon />
        </button>
      </div>
      <figcaption style={{ padding: "8px 2px 0" }}>
        <Link to={`/folios/${folio.id}`} style={{ color: "inherit", textDecoration: "none" }}>
          <div style={{ fontFamily: "var(--serif)", fontWeight: 500, fontSize: 16.5, lineHeight: 1.12, color: "var(--ink)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{folio.name}</div>
        </Link>
        <div style={{ fontFamily: "var(--sans)", fontSize: 12, color: "var(--muted)", marginTop: 3 }}>{folioPiecesLabel(folio.piece_count)}</div>
      </figcaption>
    </figure>
  );
}

function MobileNewFolioTile({ onClick }: { onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      style={{
        appearance: "none",
        border: 0,
        background: "transparent",
        color: "inherit",
        padding: 0,
        textAlign: "left",
      }}
    >
      <span
        style={{
          aspectRatio: "1 / 1",
          border: "1.5px dashed var(--accent)",
          borderRadius: 3,
          background: "var(--accent-soft)",
          color: "var(--accent)",
          display: "grid",
          placeItems: "center",
        }}
      >
        <span style={{ width: 44, height: 44, borderRadius: 99, border: "1px solid var(--accent-line)", display: "grid", placeItems: "center", background: "var(--surface)" }}>
          <PlusIcon size={18} />
        </span>
      </span>
      <span style={{ display: "block", padding: "8px 2px 0" }}>
        <span style={{ display: "block", fontFamily: "var(--serif)", fontWeight: 500, fontSize: 16.5, lineHeight: 1.12, color: "var(--ink)" }}>New folio</span>
        <span style={{ display: "block", fontFamily: "var(--sans)", fontSize: 12, color: "var(--muted)", marginTop: 3 }}>Start a set</span>
      </span>
    </button>
  );
}

function MobileFolioSheet({
  state,
  onClose,
  onSwitch,
}: {
  state: FolioSheetState;
  onClose: () => void;
  onSwitch: (state: FolioSheetState) => void;
}) {
  const { createFolioAction, renameFolioAction, changeFolioCoverAction, deleteFolioAction } = useFolio();
  const navigate = useNavigate();
  const [name, setName] = useState(state.mode === "rename" ? state.folio.name : "");
  const [nameError, setNameError] = useState("");
  const [isSubmittingName, setIsSubmittingName] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const coverPieces = useQuery({
    queryKey: ["folio-cover-sheet-pieces", state.mode === "cover" ? state.folio.id : 0],
    queryFn: () => state.mode === "cover" ? fetchFolioPieces(state.folio.id, 24, 0) : Promise.resolve({ photos: [], total: 0, limit: 0, offset: 0 }),
    enabled: state.mode === "cover",
  });

  useEffect(() => {
    setName(state.mode === "rename" ? state.folio.name : "");
    setNameError("");
    setIsSubmittingName(false);
    setConfirmDelete(false);
  }, [state]);

  const submitName = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const trimmed = name.trim();
    if (!trimmed || isSubmittingName) return;
    setNameError("");
    setIsSubmittingName(true);

    if (state.mode === "create") {
      const created = await createFolioAction(trimmed);
      if (created) {
        onClose();
        navigate(`/folios/${created.id}`);
        return;
      }
      setNameError("Folio could not be created. Try again.");
    } else if (state.mode === "rename" && trimmed !== state.folio.name) {
      const renamed = await renameFolioAction(state.folio.id, trimmed);
      if (renamed) {
        onClose();
        return;
      }
      setNameError("Folio could not be renamed. Try again.");
    } else {
      onClose();
      return;
    }

    setIsSubmittingName(false);
  };

  return (
    <div
      role="dialog"
      aria-modal="true"
      onClick={onClose}
      style={{
        position: "fixed",
        inset: 0,
        zIndex: 120,
        background: "rgba(20,14,10,.5)",
        display: "flex",
        alignItems: "flex-end",
        padding: "0 12px calc(12px + var(--safe-bottom))",
      }}
    >
      <div onClick={(event) => event.stopPropagation()} style={{ width: "100%" }}>
        <div style={{ borderRadius: 24, overflow: "hidden", background: "var(--surface)", boxShadow: "0 -18px 40px rgba(0,0,0,.25)" }}>
          <div style={{ padding: "16px 18px 14px", textAlign: "center" }}>
            <div style={{ width: 36, height: 4, borderRadius: 99, background: "var(--line-2)", margin: "0 auto 13px" }} />
            <div style={{ fontFamily: "var(--serif)", fontSize: 20, lineHeight: 1.15, color: "var(--ink)" }}>
              {state.mode === "create" ? "New folio" : state.folio.name}
            </div>
            <div style={{ fontFamily: "var(--sans)", fontSize: 12, color: "var(--muted)", marginTop: 4 }}>
              {state.mode === "create" ? "Name the folio to create it." : folioPiecesLabel(state.folio.piece_count)}
            </div>
          </div>

          {state.mode === "create" || state.mode === "rename" ? (
            <form onSubmit={submitName} style={{ padding: "0 18px 18px" }}>
              <input
                value={name}
                onChange={(event) => setName(event.target.value)}
                autoFocus
                disabled={isSubmittingName}
                placeholder="Folio name"
                aria-describedby={nameError ? "mobile-folio-name-error" : undefined}
                style={{
                  width: "100%",
                  height: 50,
                  borderRadius: 11,
                  border: "1px solid var(--line-2)",
                  background: "var(--surface-2)",
                  color: "var(--ink)",
                  outline: "none",
                  padding: "0 13px",
                  fontFamily: "var(--sans)",
                  fontSize: 15,
                  opacity: isSubmittingName ? 0.72 : 1,
                }}
              />
              {nameError ? (
                <div id="mobile-folio-name-error" role="alert" style={{ marginTop: 9, fontFamily: "var(--sans)", fontSize: 12.5, color: "var(--danger, #C0392B)" }}>
                  {nameError}
                </div>
              ) : null}
              <button
                type="submit"
                disabled={!name.trim() || isSubmittingName}
                style={{ marginTop: 12, width: "100%", height: 52, borderRadius: 13, border: 0, background: "var(--accent)", color: "var(--on-accent)", fontFamily: "var(--sans)", fontSize: 15, fontWeight: 700, opacity: name.trim() && !isSubmittingName ? 1 : 0.55 }}
              >
                {isSubmittingName ? (state.mode === "create" ? "Creating" : "Renaming") : state.mode === "create" ? "Create folio" : "Rename"}
              </button>
            </form>
          ) : state.mode === "cover" ? (
            <div style={{ padding: "0 14px 18px" }}>
              {coverPieces.isLoading ? (
                <div style={{ padding: "28px 0", textAlign: "center", fontFamily: "var(--sans)", fontSize: 13, color: "var(--muted)" }}>Loading pieces...</div>
              ) : coverPieces.data?.photos.length ? (
                <div style={{ display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 8 }}>
                  {coverPieces.data.photos.map((photo) => {
                    const piece = mapPhoto(photo);
                    return (
                      <button
                        key={photo.ID}
                        type="button"
                        aria-label={`Use ${piece.t} as cover`}
                        onClick={() => {
                          changeFolioCoverAction(state.folio.id, photo.ID);
                          onClose();
                        }}
                        style={{ position: "relative", aspectRatio: "1 / 1", border: 0, borderRadius: 3, padding: 0, overflow: "hidden", background: "var(--wall)" }}
                      >
                        <OkfImage src={getPhotoThumbnailUrl(photo.ID, 400)} alt={piece.t} title={piece.t} imgStyle={{ width: "100%", height: "100%", objectFit: "cover", display: "block" }} matteStyle={TILE_MATTE} />
                      </button>
                    );
                  })}
                </div>
              ) : (
                <div style={{ padding: "28px 0", textAlign: "center", fontFamily: "var(--sans)", fontSize: 13, color: "var(--muted)" }}>Add pieces before choosing a cover.</div>
              )}
            </div>
          ) : null}

          {state.mode === "actions" ? (
            <div>
              <button type="button" style={MOBILE_SHEET_BUTTON} onClick={() => onSwitch({ mode: "rename", folio: state.folio })}>
                Rename
              </button>
              <button type="button" style={MOBILE_SHEET_BUTTON} onClick={() => onSwitch({ mode: "cover", folio: state.folio })}>
                Change cover
              </button>
              <button
                type="button"
                style={{ ...MOBILE_SHEET_BUTTON, color: "var(--danger, #C0392B)" }}
                onClick={() => {
                  if (!confirmDelete) {
                    setConfirmDelete(true);
                    return;
                  }
                  deleteFolioAction(state.folio.id);
                  onClose();
                }}
              >
                {confirmDelete ? "Confirm delete" : "Delete folio"}
              </button>
            </div>
          ) : null}
        </div>
        <button
          type="button"
          onClick={onClose}
          style={{
            marginTop: 8,
            width: "100%",
            height: 54,
            borderRadius: 18,
            border: 0,
            background: "var(--surface)",
            color: "var(--accent)",
            fontFamily: "var(--sans)",
            fontSize: 15,
            fontWeight: 700,
          }}
        >
          Cancel
        </button>
      </div>
    </div>
  );
}

function MobileFolios({
  folios,
  isLoading,
  isError,
}: {
  folios: Folio[];
  isLoading: boolean;
  isError: boolean;
}) {
  const [sheet, setSheet] = useState<FolioSheetState | null>(null);
  const skeletons = useMemo(() => Array.from({ length: 4 }, (_, index) => index), []);

  return (
    <div>
      <header style={{ height: 54, display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12 }}>
        <h1 style={{ margin: 0, fontFamily: "var(--serif)", fontSize: 26, fontWeight: 500, lineHeight: 1, color: "var(--ink)" }}>Folios</h1>
        <button
          type="button"
          aria-label="New folio"
          onClick={() => setSheet({ mode: "create" })}
          style={{ width: 40, height: 40, borderRadius: 99, border: 0, background: "var(--accent)", color: "var(--on-accent)", display: "grid", placeItems: "center", boxShadow: "0 8px 20px rgba(124,36,32,.3)" }}
        >
          <PlusIcon size={17} />
        </button>
      </header>

      {isError ? (
        <div style={{ padding: "52px 8px", textAlign: "center", fontFamily: "var(--serif)", fontSize: 21, color: "var(--ink)" }}>Folios could not be reached.</div>
      ) : isLoading ? (
        <section style={{ display: "grid", gridTemplateColumns: "repeat(2, minmax(0, 1fr))", gap: "18px 14px", paddingTop: 4 }}>
          {skeletons.map((item) => (
            <div key={item}>
              <div className="okf-shimmer" style={{ aspectRatio: "1 / 1", borderRadius: 3, background: "var(--wall)" }} />
              <div className="okf-shimmer" style={{ width: "70%", height: 16, marginTop: 10, borderRadius: 99, background: "var(--wall)" }} />
              <div className="okf-shimmer" style={{ width: "42%", height: 11, marginTop: 7, borderRadius: 99, background: "var(--wall)" }} />
            </div>
          ))}
        </section>
      ) : folios.length === 0 ? (
        <div style={{ padding: "56px 8px 0", textAlign: "center" }}>
          <div style={{ display: "flex", justifyContent: "center" }}>
            <BrandMark width={42} height={48} />
          </div>
          <div style={{ marginTop: 16, fontFamily: "var(--serif)", fontSize: 22, fontWeight: 500, color: "var(--ink)" }}>No folios yet</div>
          <button
            type="button"
            onClick={() => setSheet({ mode: "create" })}
            style={{ marginTop: 18, height: 48, padding: "0 18px", borderRadius: 13, border: 0, background: "var(--accent)", color: "var(--on-accent)", fontFamily: "var(--sans)", fontSize: 14, fontWeight: 700 }}
          >
            New folio
          </button>
        </div>
      ) : (
        <section style={{ display: "grid", gridTemplateColumns: "repeat(2, minmax(0, 1fr))", gap: "18px 14px", paddingTop: 4 }}>
          {folios.map((folio) => (
            <MobileFolioTile
              key={folio.id}
              folio={folio}
              selected={sheet?.mode !== "create" && sheet?.folio.id === folio.id}
              onActions={(item) => setSheet({ mode: "actions", folio: item })}
            />
          ))}
          <MobileNewFolioTile onClick={() => setSheet({ mode: "create" })} />
        </section>
      )}

      {sheet ? <MobileFolioSheet state={sheet} onClose={() => setSheet(null)} onSwitch={setSheet} /> : null}
    </div>
  );
}

function FolioTile({ folio, onRename }: { folio: Folio; onRename: (folio: Folio) => void }) {
  const { changeFolioCoverAction, deleteFolioAction } = useFolio();
  const [menuOpen, setMenuOpen] = useState<"actions" | "cover" | null>(null);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const pieces = useQuery({
    queryKey: ["folio-cover-pieces", folio.id],
    queryFn: () => fetchFolioPieces(folio.id, 3, 0),
    staleTime: 15000,
  });
  const coverChoices = useQuery({
    queryKey: ["folio-cover-menu-pieces", folio.id],
    queryFn: () => fetchFolioPieces(folio.id, 24, 0),
    enabled: menuOpen === "cover",
  });

  const rename = () => {
    setMenuOpen(null);
    onRename(folio);
  };

  const changeCover = (photoId: number) => {
    changeFolioCoverAction(folio.id, photoId);
    setMenuOpen(null);
  };

  const remove = () => {
    setMenuOpen(null);
    setDeleteOpen(true);
  };

  return (
    <figure style={{ margin: 0, position: "relative" }}>
      <Link
        to={`/folios/${folio.id}`}
        style={{
          display: "block",
          position: "relative",
          aspectRatio: "1 / 1",
          overflow: "hidden",
          background: "var(--surface)",
          boxShadow: "0 1px 8px var(--shadow)",
          cursor: "pointer",
          color: "inherit",
          textDecoration: "none",
        }}
      >
        <WebFolioCover folio={folio} heroId={heroCoverId(folio, pieces.data?.photos)} />
      </Link>

      <div style={{ position: "absolute", top: 9, right: 9, zIndex: 4 }}>
        <Hov
          as="button"
          aria-label={`Actions for ${folio.name}`}
          onClick={(event: MouseEvent<HTMLButtonElement>) => {
            event.preventDefault();
            event.stopPropagation();
            setMenuOpen((open) => (open ? null : "actions"));
          }}
          style={{
            appearance: "none",
            cursor: "pointer",
            width: 32,
            height: 32,
            borderRadius: 99,
            border: "1px solid rgba(255,255,255,0.28)",
            background: "rgba(12,10,7,0.42)",
            color: "#FBF6EE",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            backdropFilter: "blur(8px)",
          }}
          hover={{ background: "rgba(12,10,7,0.68)" }}
        >
          <DotsIcon />
        </Hov>
        {menuOpen === "actions" ? (
          <div
            style={{
              position: "absolute",
              right: 0,
              top: 38,
              minWidth: 150,
              padding: 6,
              border: "1px solid var(--line)",
              background: "var(--surface)",
              boxShadow: "0 18px 50px rgba(0,0,0,0.22)",
              zIndex: 5,
            }}
          >
            <MenuButton onClick={rename}>Rename</MenuButton>
            <MenuButton onClick={() => setMenuOpen("cover")}>Change cover</MenuButton>
            <MenuButton onClick={remove}>Delete</MenuButton>
          </div>
        ) : menuOpen === "cover" ? (
          <div
            style={{
              position: "absolute",
              right: 0,
              top: 38,
              width: 270,
              padding: 10,
              border: "1px solid var(--line)",
              background: "var(--surface)",
              boxShadow: "0 18px 50px rgba(0,0,0,0.22)",
              zIndex: 5,
            }}
          >
            <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, marginBottom: 10 }}>
              <div style={{ fontFamily: "var(--sans)", fontSize: 12, fontWeight: 700, color: "var(--graphite)" }}>Change cover</div>
              <button type="button" onClick={() => setMenuOpen("actions")} style={{ border: 0, background: "transparent", color: "var(--accent)", fontFamily: "var(--sans)", fontSize: 12, fontWeight: 700, cursor: "pointer" }}>
                Back
              </button>
            </div>
            {coverChoices.isLoading ? (
              <div style={{ padding: "22px 0", textAlign: "center", fontFamily: "var(--sans)", fontSize: 12, color: "var(--muted)" }}>Loading pieces...</div>
            ) : coverChoices.data?.photos.length ? (
              <div style={{ display: "grid", gridTemplateColumns: "repeat(4, 1fr)", gap: 7 }}>
                {coverChoices.data.photos.map((photo) => {
                  const piece = mapPhoto(photo);
                  return (
                    <button
                      key={photo.ID}
                      type="button"
                      aria-label={`Use ${piece.t} as cover`}
                      onClick={() => changeCover(photo.ID)}
                      style={{ position: "relative", aspectRatio: "1 / 1", border: 0, borderRadius: 3, padding: 0, overflow: "hidden", background: "var(--wall)", cursor: "pointer", boxShadow: photo.ID === folio.cover_photo_id ? "0 0 0 3px var(--accent)" : "0 1px 5px var(--shadow)" }}
                    >
                      <OkfImage src={getPhotoThumbnailUrl(photo.ID, 320)} alt={piece.t} title={piece.t} imgStyle={{ width: "100%", height: "100%", objectFit: "cover", display: "block" }} matteStyle={TILE_MATTE} />
                    </button>
                  );
                })}
              </div>
            ) : (
              <div style={{ padding: "22px 0", textAlign: "center", fontFamily: "var(--sans)", fontSize: 12, color: "var(--muted)" }}>Add pieces before choosing a cover.</div>
            )}
          </div>
        ) : null}
      </div>
      {deleteOpen ? (
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
            await deleteFolioAction(folio.id);
          }}
        />
      ) : null}

      <figcaption style={{ padding: "11px 2px 0" }}>
        <Link to={`/folios/${folio.id}`} style={{ color: "inherit", textDecoration: "none" }}>
          <div style={{ fontFamily: "var(--serif)", fontSize: 17, lineHeight: 1.2, color: "var(--ink)" }}>{folio.name}</div>
        </Link>
        <div style={{ fontFamily: "var(--sans)", fontSize: 12.5, color: "var(--muted)", marginTop: 4 }}>{folioPiecesLabel(folio.piece_count)}</div>
      </figcaption>
    </figure>
  );
}

function MenuButton({ children, onClick }: { children: ReactNode; onClick: () => void }) {
  return (
    <Hov
      as="button"
      onClick={onClick}
      style={{
        appearance: "none",
        cursor: "pointer",
        width: "100%",
        border: 0,
        background: "transparent",
        color: "var(--ink)",
        fontFamily: "var(--sans)",
        fontSize: 13,
        textAlign: "left",
        padding: "9px 10px",
      }}
      hover={{ background: "var(--surface-2)" }}
    >
      {children}
    </Hov>
  );
}

export function FolioNameModal({
  mode,
  initialName = "",
  onClose,
  onSubmitName,
}: {
  mode: "create" | "rename";
  initialName?: string;
  onClose: () => void;
  onSubmitName: (name: string) => Promise<boolean>;
}) {
  const [name, setName] = useState(initialName);
  const [error, setError] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [borderFocused, setBorderFocused] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const trimmed = name.trim();
  const isRename = mode === "rename";
  const unchanged = isRename && trimmed === initialName.trim();

  useEffect(() => {
    inputRef.current?.focus();
    inputRef.current?.select();

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        onClose();
      }
    };

    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [onClose]);

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!trimmed || isSubmitting) return;
    if (unchanged) {
      onClose();
      return;
    }

    setError("");
    setIsSubmitting(true);
    const saved = await onSubmitName(trimmed).catch(() => false);
    if (saved) {
      onClose();
      return;
    }

    setError(isRename ? "Folio could not be renamed. Try again." : "Folio could not be created. Try again.");
    setIsSubmitting(false);
    window.requestAnimationFrame(() => inputRef.current?.focus());
  };

  return (
    <div
      role="presentation"
      onClick={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
      style={{
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
      }}
    >
      <form
        role="dialog"
        aria-modal="true"
        aria-labelledby="new-folio-title"
        aria-describedby={error ? "folio-name-error" : undefined}
        onSubmit={submit}
        style={{
          width: "min(380px, 94vw)",
          borderRadius: 16,
          background: "var(--surface)",
          color: "var(--ink)",
          boxShadow: "0 40px 110px rgba(0,0,0,0.4)",
          padding: "26px 24px 22px",
          animation: "okf-rise .3s cubic-bezier(0.22,1,0.36,1)",
        }}
      >
        <div
          style={{
            fontFamily: "var(--sans)",
            fontSize: 11,
            letterSpacing: "0.16em",
            textTransform: "uppercase",
            color: "var(--accent)",
          }}
        >
          {isRename ? "RENAME FOLIO" : "NEW FOLIO"}
        </div>
        <h2
          id="new-folio-title"
          style={{
            fontFamily: "var(--serif)",
            fontWeight: 300,
            fontSize: 24,
            lineHeight: 1.12,
            color: "var(--ink)",
            margin: "8px 0 0",
          }}
        >
          {isRename ? "Give it a new name" : "Name your folio"}
        </h2>
        <input
          ref={inputRef}
          value={name}
          onChange={(event) => setName(event.target.value)}
          disabled={isSubmitting}
          placeholder="Folio name"
          aria-label="Folio name"
          onFocus={() => setBorderFocused(true)}
          onBlur={() => setBorderFocused(false)}
          style={{
            width: "100%",
            appearance: "none",
            fontFamily: "var(--serif)",
            fontSize: 22,
            color: "var(--ink)",
            border: 0,
            borderBottom: `1.5px solid ${borderFocused ? "var(--accent)" : "var(--line-2)"}`,
            background: "transparent",
            outline: "none",
            opacity: isSubmitting ? 0.72 : 1,
            padding: "14px 0 10px",
            marginTop: 14,
            transition: "border-color .15s ease",
          }}
        />
        {error ? (
          <div id="folio-name-error" role="alert" style={{ marginTop: 12, fontFamily: "var(--sans)", fontSize: 13, color: "var(--danger)" }}>
            {error}
          </div>
        ) : null}
        <div style={{ display: "flex", gap: 11, marginTop: 22 }}>
          <button
            type="button"
            onClick={onClose}
            disabled={isSubmitting}
            style={{
              flex: 1,
              appearance: "none",
              cursor: isSubmitting ? "default" : "pointer",
              height: 48,
              borderRadius: 12,
              border: "1px solid var(--line-2)",
              background: "transparent",
              color: isSubmitting ? "var(--muted)" : "var(--ink)",
              fontFamily: "var(--sans)",
              fontSize: 15,
            }}
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={!trimmed || isSubmitting}
            style={{
              flex: 1,
              appearance: "none",
              cursor: trimmed && !isSubmitting ? "pointer" : "default",
              height: 48,
              borderRadius: 12,
              border: 0,
              background: trimmed && !isSubmitting ? "var(--accent)" : "var(--line)",
              color: trimmed && !isSubmitting ? "var(--on-accent)" : "var(--muted)",
              fontFamily: "var(--sans)",
              fontSize: 15,
              fontWeight: 500,
            }}
          >
            {isSubmitting ? (isRename ? "Renaming" : "Creating") : isRename ? "Save" : "Create"}
          </button>
        </div>
      </form>
    </div>
  );
}

export default function Folios() {
  const { createFolioAction, renameFolioAction } = useFolio();
  const { isMobile } = useViewport();
  const navigate = useNavigate();
  const [createOpen, setCreateOpen] = useState(false);
  const [renameTarget, setRenameTarget] = useState<Folio | null>(null);
  const { data, isLoading, isError } = useQuery({
    queryKey: ["folios"],
    queryFn: fetchFolios,
  });
  const folios = data?.folios ?? [];

  if (isMobile) {
    return <MobileFolios folios={folios} isLoading={isLoading} isError={isError} />;
  }

  return (
    <div>
      <PageHeader
        eyebrow="Folios"
        title="Curated groups"
        subcopy="Folios gather pieces by a thread you choose. Covers chosen for you, yours to change."
        action={<OutlineButton onClick={() => setCreateOpen(true)}>New folio</OutlineButton>}
      />
      <section style={{ padding: "46px 0 0" }}>
        {isError ? (
          <div style={{ padding: "90px 0", textAlign: "center", fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 22, color: "var(--graphite)" }}>
            Folios could not be reached.
          </div>
        ) : isLoading ? (
          <div style={{ padding: "90px 0", textAlign: "center", fontFamily: "var(--sans)", fontSize: 14, color: "var(--muted)" }}>Loading folios...</div>
        ) : folios.length === 0 ? (
          <div style={{ textAlign: "center", padding: "80px 0", color: "var(--muted)" }}>
            <div style={{ fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 24, color: "var(--graphite)" }}>No folios yet.</div>
            <div style={{ fontFamily: "var(--sans)", fontSize: 14, marginTop: 10, maxWidth: 420, marginLeft: "auto", marginRight: "auto", lineHeight: 1.6 }}>
              Group pieces into folios to keep a theme together. They will appear here once you make one.
            </div>
          </div>
        ) : (
          <section style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(166px, 1fr))", gap: 13 }}>
            {folios.map((folio) => (
              <FolioTile key={folio.id} folio={folio} onRename={setRenameTarget} />
            ))}
          </section>
        )}
      </section>
      {createOpen ? (
        <FolioNameModal
          mode="create"
          onClose={() => setCreateOpen(false)}
          onSubmitName={async (name) => {
            const folio = await createFolioAction(name);
            if (folio) {
              navigate(`/folios/${folio.id}`);
              return true;
            }
            return false;
          }}
        />
      ) : null}
      {renameTarget ? (
        <FolioNameModal
          mode="rename"
          initialName={renameTarget.name}
          onClose={() => setRenameTarget(null)}
          onSubmitName={(name) => {
            return renameFolioAction(renameTarget.id, name);
          }}
        />
      ) : null}
    </div>
  );
}
