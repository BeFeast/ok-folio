import { PageHeader } from "./ui";

export default function Inbox() {
  return (
    <div>
      <PageHeader
        eyebrow="Inbox"
        title="To review"
        subcopy="New pieces gathered from your streams. Review at your pace — nothing is urgent."
      />
      <section style={{ maxWidth: 880, padding: "18px 0 0" }}>
        <div style={{ textAlign: "center", padding: "90px 0", color: "var(--muted)" }}>
          <div style={{ fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 24, color: "var(--graphite)" }}>All caught up.</div>
          <div style={{ fontFamily: "var(--sans)", fontSize: 14, marginTop: 10 }}>
            Nothing waiting. Pieces will appear here as your streams gather them.
          </div>
        </div>
      </section>
    </div>
  );
}
