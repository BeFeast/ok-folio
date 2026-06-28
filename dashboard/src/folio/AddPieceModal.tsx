import { useRef, useState, type CSSProperties, type DragEvent } from "react";
import { useFolio } from "./context";
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
  const fileRef = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [title, setTitle] = useState("");
  const [source, setSource] = useState("");
  const [artist, setArtist] = useState("");
  const [date, setDate] = useState("");
  const [notes, setNotes] = useState("");
  const [error, setError] = useState("");

  if (!addOpen) return null;

  const resetAndClose = () => {
    setFile(null);
    setTitle("");
    setSource("");
    setArtist("");
    setDate("");
    setNotes("");
    setError("");
    if (fileRef.current) fileRef.current.value = "";
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

  return (
    <div
      onClick={resetAndClose}
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
        style={{ width: "min(780px, 95vw)", maxHeight: "90vh", overflow: "auto", background: "var(--surface)", boxShadow: "0 50px 130px rgba(0,0,0,0.5)" }}
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
