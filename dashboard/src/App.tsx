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

  const isActive = (path: string) => {
    if (path === "/") {
      return location.pathname === "/";
    }
    return location.pathname.startsWith(path);
  };

  const navLinkClass = (path: string) =>
    `${
      isActive(path)
        ? "border-gray-900 text-gray-950"
        : "border-transparent text-gray-500 hover:text-gray-800 hover:border-gray-300"
    } whitespace-nowrap py-4 px-1 border-b-2 font-medium text-sm`;

  return (
    <div className="mt-4 border-b border-gray-200">
      <nav className="-mb-px flex space-x-8">
        <Link to="/" className={navLinkClass("/")}>
          Gallery
        </Link>
        <Link to="/operations" className={navLinkClass("/operations")}>
          Streams
        </Link>
        <Link to="/today" className={navLinkClass("/today")}>
          Today
        </Link>
        <Link to="/week" className={navLinkClass("/week")}>
          This Week
        </Link>
        <Link to="/analytics" className={navLinkClass("/analytics")}>
          Analytics
        </Link>
        <Link to="/failed" className={navLinkClass("/failed")}>
          Failed
        </Link>
        <Link to="/search" className={navLinkClass("/search")}>
          Search
        </Link>
        <Link to="/artists" className={navLinkClass("/artists")}>
          Artists
        </Link>
      </nav>
    </div>
  );
}

function AppContent() {
  return (
    <div className="min-h-screen bg-gray-50">
      <header className="border-b border-gray-200 bg-white">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4">
          <div className="flex items-center justify-between">
            <div>
              <h1 className="text-3xl font-semibold text-gray-950">
                OK Folio
              </h1>
              <p className="text-sm text-gray-600 mt-1">
                A beautiful folio for visual discoveries.
              </p>
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
          <Route path="/runs/:runId" element={<RunDetail />} />
        </Routes>
      </main>

      <footer className="bg-white border-t border-gray-200 mt-12">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4">
          <p className="text-center text-sm text-gray-500">
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
