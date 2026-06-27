import { OutlineButton, PageHeader } from "./ui";

export default function Folios() {
  return (
    <div>
      <PageHeader
        eyebrow="Folios"
        title="Curated groups"
        subcopy="Folios gather pieces by a thread you choose. Covers chosen for you, yours to change."
        action={<OutlineButton>New folio</OutlineButton>}
      />
      <section style={{ padding: "46px 0 0" }}>
        <div style={{ textAlign: "center", padding: "80px 0", color: "var(--muted)" }}>
          <div style={{ fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 24, color: "var(--graphite)" }}>No folios yet.</div>
          <div style={{ fontFamily: "var(--sans)", fontSize: 14, marginTop: 10, maxWidth: 420, marginLeft: "auto", marginRight: "auto", lineHeight: 1.6 }}>
            Group pieces into folios to keep a theme together. They will appear here once you make one.
          </div>
        </div>
      </section>
    </div>
  );
}
