import { useState, type CSSProperties, type MouseEvent, type ReactNode } from "react";
import { Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { fetchFolios, getPhotoThumbnailUrl } from "../api";
import type { Folio } from "../types";
import { useFolio } from "./context";
import { DotsIcon, Hov, OkfImage, OutlineButton, PageHeader } from "./ui";

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

function FolioTile({ folio }: { folio: Folio }) {
  const { renameFolioAction, deleteFolioAction } = useFolio();
  const [menuOpen, setMenuOpen] = useState(false);
  const coverSrc = folio.cover_photo_id == null ? `__missing-folio-cover-${folio.id}` : getPhotoThumbnailUrl(folio.cover_photo_id, 700);

  const rename = () => {
    const next = window.prompt("Rename folio", folio.name);
    if (!next) return;
    const name = next.trim();
    if (name && name !== folio.name) {
      renameFolioAction(folio.id, name);
    }
    setMenuOpen(false);
  };

  const remove = () => {
    if (window.confirm(`Delete "${folio.name}"? Pieces stay in your gallery.`)) {
      deleteFolioAction(folio.id);
    }
    setMenuOpen(false);
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
          color: "inherit",
          textDecoration: "none",
        }}
      >
        <OkfImage
          src={coverSrc}
          alt={folio.name}
          title={folio.name}
          artist={folioPiecesLabel(folio.piece_count)}
          imgStyle={{ position: "absolute", inset: 0, width: "100%", height: "100%", objectFit: "cover", zIndex: 1 }}
          matteStyle={TILE_MATTE}
          matteTitleStyle={{ fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 17, lineHeight: 1.2, color: "var(--ink)" }}
          matteArtistStyle={{ fontFamily: "var(--sans)", fontSize: 10, letterSpacing: "0.12em", textTransform: "uppercase", color: "var(--muted)" }}
        />
      </Link>

      <div style={{ position: "absolute", top: 9, right: 9, zIndex: 4 }}>
        <Hov
          as="button"
          aria-label={`Actions for ${folio.name}`}
          onClick={(event: MouseEvent<HTMLButtonElement>) => {
            event.preventDefault();
            event.stopPropagation();
            setMenuOpen((open) => !open);
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
        {menuOpen ? (
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
            <MenuButton onClick={remove}>Delete</MenuButton>
          </div>
        ) : null}
      </div>

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

export default function Folios() {
  const { createFolioAction } = useFolio();
  const { data, isLoading, isError } = useQuery({
    queryKey: ["folios"],
    queryFn: fetchFolios,
  });
  const folios = data?.folios ?? [];

  const create = () => {
    const next = window.prompt("Name this folio");
    if (!next) return;
    const name = next.trim();
    if (name) {
      createFolioAction(name);
    }
  };

  return (
    <div>
      <PageHeader
        eyebrow="Folios"
        title="Curated groups"
        subcopy="Folios gather pieces by a thread you choose. Covers chosen for you, yours to change."
        action={<OutlineButton onClick={create}>New folio</OutlineButton>}
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
              <FolioTile key={folio.id} folio={folio} />
            ))}
          </section>
        )}
      </section>
    </div>
  );
}
