import { useQuery } from "@tanstack/react-query";
import { fetchStats } from "../api";
import { useStatsSSE } from "../hooks/useSSE";
import { formatBytes, formatNumber, getRelativeTime } from "../utils";

export default function StatsCards() {
  const { stats: liveStats, isConnected } = useStatsSSE();
  const { data: polledStats, isLoading, error } = useQuery({
    queryKey: ["stats"],
    queryFn: fetchStats,
    refetchInterval: 30000,
  });
  const stats = liveStats || polledStats;

  if (isLoading) {
    return (
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-4">
        {[...Array(4)].map((_, i) => (
          <div
            key={i}
            className="animate-pulse rounded-lg border border-gray-200 bg-white p-5 shadow-sm"
          >
            <div className="h-4 bg-gray-200 rounded w-1/2 mb-4"></div>
            <div className="h-8 bg-gray-200 rounded w-3/4"></div>
          </div>
        ))}
      </div>
    );
  }

  if (error) {
    return (
      <div className="rounded-lg border border-red-200 bg-red-50 p-4">
        <p className="text-red-800">Error loading stats: {error.message}</p>
      </div>
    );
  }

  if (!stats) return null;

  const cards = [
    {
      title: "Library media",
      value: formatNumber(stats.total_photos),
      detail: "Downloaded files tracked by the extractor",
    },
    {
      title: "Artists",
      value: formatNumber(stats.unique_artists),
      detail: "Distinct artists in extracted metadata",
    },
    {
      title: "Stored size",
      value: formatBytes(stats.total_size_bytes),
      detail: "Media bytes reported by the service",
    },
    {
      title: "Last download",
      value: getRelativeTime(stats.last_download),
      detail: isConnected ? "Live via SSE" : "Polled from API",
    },
  ];

  return (
    <section className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-4">
      {cards.map((card) => (
        <div
          key={card.title}
          className="rounded-lg border border-gray-200 bg-white p-5 shadow-sm"
        >
          <h3 className="text-sm font-medium text-gray-600">{card.title}</h3>
          <p className="mt-2 text-2xl font-semibold text-gray-950">
            {card.value}
          </p>
          <p className="mt-2 text-sm text-gray-500">{card.detail}</p>
        </div>
      ))}
    </section>
  );
}
