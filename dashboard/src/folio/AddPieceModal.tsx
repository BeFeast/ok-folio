import { useEffect, useRef, useState, type CSSProperties, type DragEvent } from "react";
import { useFolio } from "./context";
import { useViewport } from "./useViewport";
import { BrandMark, CloseIcon, Hov } from "./ui";

const FIELD_LABEL: CSSProperties = {
  fontFamily: "var(--sans)",
  fontSize: 11,
  letterSpacing: "0.1em",
  textTransform: "uppercase",
  color: "var(--muted)",
  marginBottom: 7,
};
const FIELD_BASE: CSSProperties = {
  width: "100%",
  appearance: "none",
  border: 0,
  borderBottom: "1px solid var(--line-2)",
  background: "transparent",
  outline: "none",
  padding: "6px 0",
  color: "var(--ink)",
};
const ACCEPTED_IMAGE_TYPES = new Set([
  "image/jpeg",
  "image/png",
  "image/tiff",
  "image/webp",
]);
const ACCEPTED_IMAGE_EXTENSIONS = [".jpg", ".jpeg", ".png", ".tif", ".tiff", ".webp"];

function isAcceptedImage(file: File): boolean {
  if (ACCEPTED_IMAGE_TYPES.has(file.type)) return true;
  const name = file.name.toLowerCase();
  return ACCEPTED_IMAGE_EXTENSIONS.some((ext) => name.endsWith(ext));
}

function Field({
  label,
  placeholder,
  value,
  onChange,
  serif,
  textarea,
}: {
  label: string;
  placeholder: string;
  value: string;
  onChange: (v: string) => void;
  serif?: boolean;
  textarea?: boolean;
}) {
  const style: CSSProperties = {
    ...FIELD_BASE,
    fontFamily: serif ? "var(--serif)" : "var(--sans)",
    fontStyle: textarea ? "italic" : undefined,
    fontSize: serif ? (textarea ? 15 : 17) : 14,
    resize: textarea ? "none" : undefined,
  };
  return (
    <label style={{ display: "block" }}>
      <div style={FIELD_LABEL}>{label}</div>
      {textarea ? (
        <Hov as="textarea" rows={2} placeholder={placeholder} value={value} onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) => onChange(e.target.value)} style={style} focus={{ borderColor: "var(--accent)" }} />
      ) : (
        <Hov as="input" placeholder={placeholder} value={value} onChange={(e: React.ChangeEvent<HTMLInputElement>) => onChange(e.target.value)} style={style} focus={{ borderColor: "var(--accent)" }} />
      )}
    </label>
  );
}

export default function AddPieceModal() {
  const { addOpen, closeAdd, importPiece } = useFolio();
  const { isMobile } = useViewport();
  const fileRef = useRef<HTMLInputElement>(null);
  const cameraRef = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [sourceMode, setSourceMode] = useState<"library" | "camera">("library");
  const [title, setTitle] = useState("");
  const [source, setSource] = useState("");
  const [artist, setArtist] = useState("");
  const [date, setDate] = useState("");
  const [notes, setNotes] = useState("");
  const [error, setError] = useState("");
  const [previewUrl, setPreviewUrl] = useState("");

  useEffect(() => {
    if (!file) {
      setPreviewUrl("");
      return;
    }
    const url = URL.createObjectURL(file);
    setPreviewUrl(url);
    return () => URL.revokeObjectURL(url);
  }, [file]);

  if (!addOpen) return null;

  const resetAndClose = () => {
    setFile(null);
    setTitle("");
    setSource("");
    setArtist("");
    setDate("");
    setNotes("");
    setError("");
    setSourceMode("library");
    if (fileRef.current) fileRef.current.value = "";
    if (cameraRef.current) cameraRef.current.value = "";
    closeAdd();
  };
  const stageFile = (nextFile: File | undefined) => {
    if (!nextFile) return;
    if (!isAcceptedImage(nextFile)) {
      setError("Choose a JPEG, PNG, TIFF, or WebP image.");
      setFile(null);
      return;
    }
    setError("");
    setFile(nextFile);
  };
  const handleDrop = (event: DragEvent<HTMLDivElement>) => {
    event.preventDefault();
    event.stopPropagation();
    stageFile(event.dataTransfer.files?.[0]);
  };
  const handleAdd = () => {
    if (!file) {
      setError("Choose an image before adding the piece.");
      return;
    }
    // Hand the upload off to the context and close instantly — progress is shown
    // as a background toast, so the dialog never feels frozen behind the request.
    importPiece({ file, title, source, artist, date, notes });
    resetAndClose();
  };

  const mobileInputStyle: CSSProperties = {
    width: "100%",
    minHeight: 46,
    appearance: "none",
    border: "1px solid var(--line)",
    borderRadius: 11,
    background: "var(--surface)",
    color: "var(--ink)",
    outline: 0,
    padding: "11px 12px",
    fontFamily: "var(--sans)",
    fontSize: 15,
  };
  const mobileLabelStyle: CSSProperties = {
    fontFamily: "var(--sans)",
    fontSize: 11,
    fontWeight: 700,
    letterSpacing: "0.06em",
    textTransform: "uppercase",
    color: "var(--faint)",
    marginBottom: 7,
  };
  const sourceButton = (mode: "library" | "camera", label: string, glyph: string, onClick: () => void) => {
    const active = sourceMode === mode;
    return (
      <button
        type="button"
        onClick={onClick}
        style={{
          minWidth: 0,
          height: 74,
          border: `1px solid ${active ? "var(--accent-line)" : "var(--line)"}`,
          borderRadius: 14,
          background: active ? "var(--accent-soft)" : "var(--surface)",
          color: active ? "var(--accent)" : "var(--graphite)",
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          justifyContent: "center",
          gap: 6,
          fontFamily: "var(--sans)",
          fontSize: 12.5,
          fontWeight: 700,
        }}
      >
        <span style={{ fontSize: 20, lineHeight: 1 }}>{glyph}</span>
        {label}
      </button>
    );
  };

  if (isMobile) {
    return (
      <div
        style={{
          position: "fixed",
          inset: 0,
          zIndex: 100,
          background: "rgba(20,14,10,.5)",
          display: "flex",
          alignItems: "flex-end",
        }}
      >
        <section
          role="dialog"
          aria-modal="true"
          aria-label="Add a piece"
          style={{
            width: "100%",
            height: "calc(100dvh - var(--safe-top))",
            marginTop: "var(--safe-top)",
            borderRadius: "24px 24px 0 0",
            background: "var(--bg)",
            boxShadow: "0 -18px 40px rgba(0,0,0,.25)",
            display: "flex",
            flexDirection: "column",
            overflow: "hidden",
          }}
        >
          <input
            ref={fileRef}
            type="file"
            accept="image/jpeg,image/png,image/tiff,image/webp,.jpg,.jpeg,.png,.tif,.tiff,.webp"
            style={{ display: "none" }}
            onChange={(e) => stageFile(e.target.files?.[0])}
          />
          <input
            ref={cameraRef}
            type="file"
            accept="image/*"
            capture="environment"
            style={{ display: "none" }}
            onChange={(e) => stageFile(e.target.files?.[0])}
          />
          <div style={{ padding: "9px 20px 0", display: "flex", justifyContent: "center" }}>
            <div style={{ width: 42, height: 5, borderRadius: 99, background: "var(--line-2)" }} />
          </div>
          <header style={{ padding: "12px 20px 10px", display: "flex", alignItems: "center", justifyContent: "space-between" }}>
            <button type="button" onClick={resetAndClose} style={{ border: 0, background: "transparent", color: "var(--accent)", fontFamily: "var(--sans)", fontSize: 15, fontWeight: 700 }}>
              Cancel
            </button>
            <button type="button" onClick={handleAdd} style={{ border: 0, background: "transparent", color: "var(--accent)", fontFamily: "var(--sans)", fontSize: 15, fontWeight: 700 }}>
              Save
            </button>
          </header>
          <div style={{ flex: 1, overflow: "auto", padding: "0 20px calc(98px + var(--safe-bottom))" }}>
            <h2 style={{ margin: "8px 0 18px", fontFamily: "var(--serif)", fontSize: 28, fontWeight: 500, lineHeight: 1, color: "var(--ink)" }}>
              Add a piece
            </h2>
            <div style={{ display: "grid", gridTemplateColumns: "repeat(2, minmax(0, 1fr))", gap: 9 }}>
              {sourceButton("library", "Library", "▧", () => {
                setSourceMode("library");
                fileRef.current?.click();
              })}
              {sourceButton("camera", "Camera", "○", () => {
                setSourceMode("camera");
                cameraRef.current?.click();
              })}
            </div>
            <button
              type="button"
              onClick={() => fileRef.current?.click()}
              onDragOver={(e: DragEvent<HTMLButtonElement>) => {
                e.preventDefault();
              }}
              onDrop={(e: DragEvent<HTMLButtonElement>) => {
                e.preventDefault();
                stageFile(e.dataTransfer.files?.[0]);
              }}
              style={{
                width: "100%",
                height: 150,
                marginTop: 18,
                padding: 0,
                overflow: "hidden",
                border: "1px dashed var(--line-2)",
                borderRadius: 14,
                background: "var(--surface)",
                color: "var(--muted)",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
              }}
            >
              {previewUrl ? (
                <img src={previewUrl} alt="Selected piece preview" style={{ width: "100%", height: "100%", objectFit: "cover", display: "block" }} />
              ) : (
                <div style={{ display: "flex", flexDirection: "column", alignItems: "center", gap: 9 }}>
                  <BrandMark width={32} height={36} />
                  <span style={{ fontFamily: "var(--sans)", fontSize: 13, fontWeight: 700 }}>Choose an image</span>
                </div>
              )}
            </button>
            {error ? (
              <div role="alert" style={{ marginTop: 10, fontFamily: "var(--sans)", fontSize: 12.5, color: "var(--danger, #b42318)" }}>
                {error}
              </div>
            ) : null}
            <div style={{ marginTop: 20, display: "flex", flexDirection: "column", gap: 15 }}>
              <label>
                <div style={mobileLabelStyle}>Title</div>
                <input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Untitled piece" style={{ ...mobileInputStyle, fontFamily: "var(--serif)", fontSize: 17 }} />
              </label>
              <div style={{ display: "grid", gridTemplateColumns: "minmax(0, 1fr) 112px", gap: 10 }}>
                <label>
                  <div style={mobileLabelStyle}>Artist</div>
                  <input value={artist} onChange={(e) => setArtist(e.target.value)} placeholder="Unknown" style={mobileInputStyle} />
                </label>
                <label>
                  <div style={mobileLabelStyle}>Date</div>
                  <input value={date} onChange={(e) => setDate(e.target.value)} placeholder="Year" style={mobileInputStyle} />
                </label>
              </div>
              <label>
                <div style={mobileLabelStyle}>Source</div>
                <input id="okf-mobile-source" value={source} onChange={(e) => setSource(e.target.value)} placeholder="Where it came from" style={mobileInputStyle} />
              </label>
              <label>
                <div style={mobileLabelStyle}>Notes</div>
                <textarea value={notes} onChange={(e) => setNotes(e.target.value)} placeholder="Why you kept it." rows={3} style={{ ...mobileInputStyle, resize: "none", lineHeight: 1.35 }} />
              </label>
            </div>
          </div>
          <footer
            style={{
              position: "absolute",
              left: 0,
              right: 0,
              bottom: 0,
              padding: "14px 20px calc(14px + var(--safe-bottom))",
              background: "linear-gradient(180deg, color-mix(in srgb, var(--bg) 0%, transparent), var(--bg) 26%)",
            }}
          >
            <button
              type="button"
              onClick={handleAdd}
              disabled={!file}
              style={{
                width: "100%",
                height: 52,
                border: 0,
                borderRadius: 13,
                background: "var(--accent)",
                color: "var(--on-accent)",
                opacity: !file ? 0.62 : 1,
                fontFamily: "var(--sans)",
                fontSize: 15,
                fontWeight: 800,
                boxShadow: "0 8px 20px rgba(124,36,32,.3)",
              }}
            >
              Add to folio
            </button>
          </footer>
        </section>
      </div>
    );
  }

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Add a piece"
      onClick={resetAndClose}
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
        onClick={(e) => e.stopPropagation()}
        style={{ width: "min(780px, 95vw)", maxHeight: "90vh", overflow: "auto", background: "var(--surface)", boxShadow: "0 50px 130px rgba(0,0,0,0.5)", animation: "okf-rise .3s cubic-bezier(0.22,1,0.36,1)" }}
      >
        <div style={{ padding: "26px 32px", borderBottom: "1px solid var(--line)", display: "flex", alignItems: "flex-start", justifyContent: "space-between" }}>
          <div>
            <div style={{ fontFamily: "var(--sans)", fontSize: 11, letterSpacing: "0.2em", textTransform: "uppercase", color: "var(--accent)" }}>Add Piece</div>
            <h2 style={{ margin: "9px 0 0", fontFamily: "var(--serif)", fontWeight: 300, fontSize: 27, color: "var(--ink)", letterSpacing: "-0.01em" }}>Bring in a new piece</h2>
          </div>
          <Hov
            as="button"
            onClick={resetAndClose}
            aria-label="Close"
            style={{ appearance: "none", cursor: "pointer", width: 34, height: 34, borderRadius: 99, border: "1px solid var(--line)", background: "transparent", color: "var(--muted)", display: "flex", alignItems: "center", justifyContent: "center" }}
            hover={{ color: "var(--ink)", borderColor: "var(--line-2)" }}
          >
            <CloseIcon size={15} />
          </Hov>
        </div>

        <div style={{ display: "grid", gridTemplateColumns: "0.92fr 1.08fr", gap: 0 }}>
          <div style={{ padding: "26px 16px 26px 26px" }}>
            <input
              ref={fileRef}
              type="file"
              accept="image/jpeg,image/png,image/tiff,image/webp,.jpg,.jpeg,.png,.tif,.tiff,.webp"
              style={{ display: "none" }}
              onChange={(e) => stageFile(e.target.files?.[0])}
            />
            <Hov
              onClick={() => fileRef.current?.click()}
              onDragOver={(e: DragEvent<HTMLDivElement>) => {
                e.preventDefault();
                e.dataTransfer.dropEffect = "copy";
              }}
              onDrop={handleDrop}
              style={{ height: "100%", minHeight: 300, border: "1.5px dashed var(--line-2)", borderRadius: 8, display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 15, padding: "34px 22px", textAlign: "center", background: "var(--surface-2)", cursor: "pointer" }}
              hover={{ borderColor: "var(--accent-line)" }}
            >
              <BrandMark width={40} height={44} />
              <div style={{ fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 18, color: "var(--ink)" }}>
                {file?.name || "Drag an image here"}
              </div>
              <Hov
                as="button"
                onClick={(e: React.MouseEvent) => {
                  e.stopPropagation();
                  fileRef.current?.click();
                }}
                style={{ appearance: "none", cursor: "pointer", fontFamily: "var(--sans)", fontSize: 13, fontWeight: 500, padding: "9px 18px", borderRadius: 99, border: "1px solid var(--line-2)", background: "var(--surface)", color: "var(--ink)" }}
                hover={{ borderColor: "var(--accent-line)", color: "var(--accent)" }}
              >
                Choose a file
              </Hov>
              <div style={{ fontFamily: "var(--sans)", fontSize: 11, color: "var(--faint)", letterSpacing: "0.04em" }}>JPEG · PNG · TIFF · WebP</div>
            </Hov>
          </div>
          <div style={{ padding: "26px 28px 26px 16px", display: "flex", flexDirection: "column", gap: 18 }}>
            <Field label="Title" placeholder="Untitled piece" value={title} onChange={setTitle} serif />
            <Field label="Source" placeholder="Where it came from" value={source} onChange={setSource} />
            <Field label="Author / artist" placeholder="Unknown" value={artist} onChange={setArtist} />
            <Field label="Date" placeholder="When it was made" value={date} onChange={setDate} />
            <Field label="Notes" placeholder="Why you kept it." value={notes} onChange={setNotes} serif textarea />
          </div>
        </div>

        <div style={{ padding: "18px 32px", borderTop: "1px solid var(--line)", display: "flex", alignItems: "center", justifyContent: "space-between", gap: 18 }}>
          <div style={{ fontFamily: "var(--sans)", fontSize: 11.5, color: "var(--faint)", maxWidth: 300, lineHeight: 1.5 }}>
            Dimensions, colour, and similar pieces are filled in quietly after import.
          </div>
          <div style={{ display: "flex", gap: 11, flex: "none" }}>
            {error ? (
              <div role="alert" style={{ alignSelf: "center", maxWidth: 230, fontFamily: "var(--sans)", fontSize: 12, color: "var(--danger, #b42318)", lineHeight: 1.35 }}>
                {error}
              </div>
            ) : null}
            <Hov
              as="button"
              onClick={resetAndClose}
              style={{ appearance: "none", cursor: "pointer", fontFamily: "var(--sans)", fontSize: 13.5, padding: "10px 18px", borderRadius: 99, border: 0, background: "transparent", color: "var(--muted)" }}
              hover={{ color: "var(--ink)" }}
            >
              Cancel
            </Hov>
            <Hov
              as="button"
              onClick={handleAdd}
              disabled={!file}
              style={{ appearance: "none", cursor: !file ? "not-allowed" : "pointer", opacity: !file ? 0.6 : 1, fontFamily: "var(--sans)", fontSize: 13.5, fontWeight: 500, padding: "10px 22px", borderRadius: 99, border: 0, background: "var(--accent)", color: "var(--on-accent)" }}
              hover={{ filter: "brightness(1.06)" }}
            >
              Add Piece
            </Hov>
          </div>
        </div>
      </div>
    </div>
  );
}
