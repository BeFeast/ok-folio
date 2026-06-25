import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  BrowserRouter,
  Routes,
  Route,
  Link,
  useLocation,
} from "react-router-dom";
import HealthIndicator from "./components/HealthIndicator";
import TimelineChart from "./components/TimelineChart";
import TopArtistsChart from "./components/TopArtistsChart";
import FailedPhotos from "./components/FailedPhotos";
import Search from "./components/Search";
import ArtistList from "./components/ArtistList";
import ArtistDetail from "./components/ArtistDetail";
import ExtractorOperations from "./pages/ExtractorOperations";
import Gallery from "./pages/Gallery";
import TodayImages from "./pages/TodayImages";
import WeeklyImages from "./pages/WeeklyImages";
import RunDetail from "./pages/RunDetail";
import PieceDetail from "./pages/PieceDetail";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 5000,
      retry: 1,
    },
  },
});

function Navigation() {
  const location = useLocation();
  const navItems = [
    { label: "Gallery", path: "/" },
    { label: "Folios", path: "/folios" },
    { label: "Inbox", path: "/inbox" },
    { label: "Streams", path: "/streams" },
    { label: "Settings", path: "/settings" },
  ];

  const isActive = (path: string) => {
    if (path === "/") {
      return location.pathname === "/";
    }
    return location.pathname.startsWith(path);
  };

  const navLinkClass = (path: string) =>
    `${
      isActive(path)
        ? "border-[color:var(--folio-accent)] text-[color:var(--folio-ink)]"
        : "border-transparent text-[color:var(--folio-graphite)] hover:border-[color:var(--folio-line)] hover:text-[color:var(--folio-ink)]"
    } whitespace-nowrap py-3 px-1 border-b-2 text-sm font-medium transition-colors`;

  return (
    <div className="mt-5 overflow-x-auto border-b border-[color:var(--folio-line)]">
      <nav className="-mb-px flex gap-8">
        {navItems.map((item) => (
          <Link key={item.path} to={item.path} className={navLinkClass(item.path)}>
            {item.label}
          </Link>
        ))}
      </nav>
    </div>
  );
}

function BrandMark() {
  return (
    <div
      aria-hidden="true"
      className="relative h-10 w-10 shrink-0 border border-[color:var(--folio-accent)] bg-[color:var(--folio-surface)]"
    >
      <div className="absolute inset-1 border border-[color:var(--folio-line)]" />
      <div className="absolute bottom-0 right-0 h-5 w-4 border-l border-t border-[color:var(--folio-accent)] bg-[color:var(--folio-paper)]" />
    </div>
  );
}

function PlaceholderSurface({ title, copy }: { title: string; copy: string }) {
  return (
    <section className="border border-[color:var(--folio-line)] bg-[color:var(--folio-surface)] p-8 shadow-[var(--folio-shadow)]">
      <p className="text-xs font-medium uppercase tracking-[0.18em] text-[color:var(--folio-accent)]">
        OK Folio
      </p>
      <h2 className="mt-3 font-serif text-3xl text-[color:var(--folio-ink)]">
        {title}
      </h2>
      <p className="mt-3 max-w-2xl text-sm leading-6 text-[color:var(--folio-graphite)]">
        {copy}
      </p>
    </section>
  );
}

function AppContent() {
  return (
    <div className="min-h-screen bg-[color:var(--folio-paper)] text-[color:var(--folio-ink)]">
      <header className="border-b border-[color:var(--folio-line)] bg-[color:var(--folio-surface)]">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-4">
              <BrandMark />
              <div>
                <h1 className="font-serif text-3xl text-[color:var(--folio-ink)]">
                  OK Folio
                </h1>
                <p className="text-sm text-[color:var(--folio-graphite)] mt-1">
                  A beautiful folio for visual discoveries.
                </p>
              </div>
            </div>
            <HealthIndicator />
          </div>

          <Navigation />
        </div>
      </header>

      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <Routes>
          <Route
            path="/"
            element={<Gallery />}
          />
          <Route
            path="/folios"
            element={
              <PlaceholderSurface
                title="Folios"
                copy="Curated groups will live here. Gallery remains the primary aggregator surface while folio workflows are built out."
              />
            }
          />
          <Route
            path="/inbox"
            element={
              <PlaceholderSurface
                title="Inbox"
                copy="Exceptions, duplicates, and ambiguous arrivals will be reviewed here without turning every incoming piece into manual work."
              />
            }
          />
          <Route
            path="/streams"
            element={<ExtractorOperations />}
          />
          <Route
            path="/settings"
            element={
              <PlaceholderSurface
                title="Settings"
                copy="Product and connector settings will collect here as OK Folio moves beyond the legacy operations dashboard."
              />
            }
          />
          <Route
            path="/operations"
            element={<ExtractorOperations />}
          />
          <Route path="/today" element={<TodayImages />} />
          <Route path="/week" element={<WeeklyImages />} />
          <Route
            path="/analytics"
            element={
              <div className="space-y-8">
                <TimelineChart days={14} />
                <TopArtistsChart limit={15} />
              </div>
            }
          />
          <Route path="/failed" element={<FailedPhotos />} />
          <Route path="/search" element={<Search />} />
          <Route path="/artists" element={<ArtistList />} />
          <Route path="/artists/:artistName" element={<ArtistDetail />} />
          <Route path="/pieces/:photoId" element={<PieceDetail />} />
          <Route path="/runs/:runId" element={<RunDetail />} />
        </Routes>
      </main>

      <footer className="bg-[color:var(--folio-surface)] border-t border-[color:var(--folio-line)] mt-12">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4">
          <p className="text-center text-sm text-[color:var(--folio-graphite)]">
            OK Folio
          </p>
        </div>
      </footer>
    </div>
  );
}

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <AppContent />
      </BrowserRouter>
    </QueryClientProvider>
  );
}

export default App;
