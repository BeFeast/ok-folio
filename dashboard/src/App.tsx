import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, Routes, Route } from "react-router-dom";

import { FolioProvider } from "./folio/context";
import Nav from "./folio/Nav";
import Gallery from "./folio/Gallery";
import Folios from "./folio/Folios";
import FolioDetail from "./folio/FolioDetail";
import Inbox from "./folio/Inbox";
import Streams from "./folio/Streams";
import Settings from "./folio/Settings";
import PieceViewer from "./folio/PieceViewer";
import AddPieceModal from "./folio/AddPieceModal";
import Toaster from "./folio/Toaster";

// Legacy operations surfaces — kept reachable by direct URL (not in the
// primary navigation) so existing deep links and tooling still work.
import ExtractorOperations from "./pages/ExtractorOperations";
import TodayImages from "./pages/TodayImages";
import WeeklyImages from "./pages/WeeklyImages";
import RunDetail from "./pages/RunDetail";
import PieceDetail from "./pages/PieceDetail";
import TimelineChart from "./components/TimelineChart";
import TopArtistsChart from "./components/TopArtistsChart";
import FailedPhotos from "./components/FailedPhotos";
import Search from "./components/Search";
import ArtistList from "./components/ArtistList";
import ArtistDetail from "./components/ArtistDetail";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 5000,
      retry: 1,
    },
  },
});

function FolioShell() {
  return (
    <div
      style={{
        minHeight: "100vh",
        background: "var(--bg)",
        color: "var(--ink)",
        fontFamily: "var(--sans)",
        WebkitFontSmoothing: "antialiased",
        textRendering: "optimizeLegibility",
      }}
    >
      <Nav />
      <main style={{ maxWidth: 1340, margin: "0 auto", padding: "0 30px 110px" }}>
        <Routes>
          <Route path="/" element={<Gallery />} />
          <Route path="/folios" element={<Folios />} />
          <Route path="/folios/:id" element={<FolioDetail />} />
          <Route path="/inbox" element={<Inbox />} />
          <Route path="/streams" element={<Streams />} />
          <Route path="/settings" element={<Settings />} />

          {/* Legacy / deep-link routes */}
          <Route path="/operations" element={<ExtractorOperations />} />
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

      <PieceViewer />
      <AddPieceModal />
      <Toaster />
    </div>
  );
}

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <FolioProvider>
          <FolioShell />
        </FolioProvider>
      </BrowserRouter>
    </QueryClientProvider>
  );
}

export default App;
