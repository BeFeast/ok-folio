import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import {
  fetchConnectorStatus,
  fetchRuns,
  fetchTodayPhotos,
  fetchWeekPhotos,
  fetchWorkerStatus,
} from "../api";
import Controls from "../components/Controls";
import FailedPhotos from "../components/FailedPhotos";
import ImageThumbnail from "../components/ImageThumbnail";
import RunHistory from "../components/RunHistory";
import StatsCards from "../components/StatsCards";
import WorkerStatus from "../components/WorkerStatus";
import { formatDate, formatDuration, formatNumber } from "../utils";
import type { ConnectorStatus, Photo } from "../types";

function statusClass(status: string) {
  switch (status) {
    case "completed":
      return "border-emerald-200 bg-emerald-50 text-emerald-800";
    case "failed":
      return "border-red-200 bg-red-50 text-red-800";
    case "running":
      return "border-amber-200 bg-amber-50 text-amber-800";
    default:
      return "border-gray-200 bg-gray-50 text-gray-700";
  }
}

function connectorTone(connector?: ConnectorStatus) {
  return connector?.health === "syncing"
    ? "border-amber-200 bg-amber-50 text-amber-900"
    : connector?.health === "error" || connector?.health === "degraded"
      ? "border-red-200 bg-red-50 text-red-900"
      : connector?.health === "healthy"
        ? "border-emerald-200 bg-emerald-50 text-emerald-900"
        : "border-gray-200 bg-gray-50 text-gray-700";
}

function ProviderStatus() {
  const { data: statusData } = useQuery({
    queryKey: ["connector-status"],
    queryFn: fetchConnectorStatus,
    refetchInterval: (query) => {
      const connectors = query.state.data?.connectors || [];
      return connectors.some((connector: ConnectorStatus) => connector.health === "syncing")
        ? 3000
        : 30000;
    },
  });
  const { data: workerStatus } = useQuery({
    queryKey: ["worker-status"],
    queryFn: fetchWorkerStatus,
    refetchInterval: 5000,
  });

  const connectors = statusData?.connectors || [];
  const connector = connectors[0];
  const latestRun = connector?.recent_runs?.[0];
  const primarySource = connector?.sources?.[0];
  const queuePercent = workerStatus
    ? Math.min(workerStatus.queue_utilization, 100)
    : 0;

  return (
    <section className="grid gap-4 lg:grid-cols-[1.2fr_1fr]">
      <div className="rounded-lg border border-gray-200 bg-white p-5 shadow-sm">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <p className="text-xs font-semibold uppercase tracking-wide text-gray-500">
              Provider
            </p>
            <div className="mt-2 flex flex-wrap items-center gap-3">
              <h2 className="text-2xl font-semibold text-gray-950">
                {connector?.display_name ?? "Web Gallery"}
              </h2>
              <span
                className={`rounded-full border px-3 py-1 text-sm font-medium ${connectorTone(connector)}`}
              >
                {connector?.state ?? "Unavailable"}
              </span>
            </div>
            <p className="mt-2 max-w-2xl text-sm text-gray-600">
              Connector status from persisted source counts, last sync telemetry,
              health, and recent extraction errors.
            </p>
          </div>
          <Link
            to={latestRun ? `/runs/${latestRun.id}` : "/"}
            className="inline-flex items-center justify-center rounded-md border border-gray-300 px-3 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
          >
            Latest run
          </Link>
        </div>

        <div className="mt-6 grid gap-3 sm:grid-cols-3">
          <div className="rounded-md border border-gray-200 p-4">
            <p className="text-xs font-medium uppercase tracking-wide text-gray-500">
              Cataloged
            </p>
            <p className="mt-2 text-xl font-semibold text-gray-950">
              {formatNumber(connector?.counts.downloaded ?? 0)}
            </p>
            <p className="mt-1 text-sm text-gray-600">
              {primarySource
                ? `${primarySource.display_name}`
                : "No source media recorded"}
            </p>
          </div>
          <div className="rounded-md border border-gray-200 p-4">
            <p className="text-xs font-medium uppercase tracking-wide text-gray-500">
              Last sync
            </p>
            <p className="mt-2 text-xl font-semibold text-gray-950">
              {connector?.last_sync ? formatDate(connector.last_sync) : "Never"}
            </p>
            <p className="mt-1 text-sm text-gray-600">
              {latestRun
                ? `Run #${latestRun.id} ${latestRun.status}`
                : "Trigger a run to create history"}
            </p>
          </div>
          <div className="rounded-md border border-gray-200 p-4">
            <p className="text-xs font-medium uppercase tracking-wide text-gray-500">
              Recent errors
            </p>
            <p className="mt-2 text-xl font-semibold text-gray-950">
              {connector?.recent_errors.length ?? 0}
            </p>
            <p className="mt-1 text-sm text-gray-600">
              {(connector?.counts.failed ?? 0) > 0
                ? `${formatNumber(connector?.counts.failed ?? 0)} failed items need review`
                : "No failed downloads"}
            </p>
          </div>
        </div>

        {connectors.length > 1 ? (
          <div className="mt-6 grid gap-3 md:grid-cols-2">
            {connectors.map((item) => (
              <div key={item.id} className="rounded-md border border-gray-200 p-4">
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <p className="truncate text-sm font-semibold text-gray-950">
                      {item.display_name}
                    </p>
                    <p className="mt-1 truncate text-xs text-gray-500">
                      {item.sources[0]?.display_name ?? "No source media recorded"}
                    </p>
                  </div>
                  <span
                    className={`shrink-0 rounded-full border px-2 py-1 text-xs font-medium ${connectorTone(item)}`}
                  >
                    {item.state}
                  </span>
                </div>
                <div className="mt-4 grid grid-cols-3 gap-2 text-sm">
                  <div>
                    <p className="text-xs text-gray-500">Kept</p>
                    <p className="font-semibold text-gray-950">
                      {formatNumber(item.counts.downloaded)}
                    </p>
                  </div>
                  <div>
                    <p className="text-xs text-gray-500">Failed</p>
                    <p className="font-semibold text-gray-950">
                      {formatNumber(item.counts.failed)}
                    </p>
                  </div>
                  <div>
                    <p className="text-xs text-gray-500">Errors</p>
                    <p className="font-semibold text-gray-950">
                      {formatNumber(item.recent_errors.length)}
                    </p>
                  </div>
                </div>
                <p className="mt-3 truncate text-xs text-gray-500">
                  Last sync: {item.last_sync ? formatDate(item.last_sync) : "Never"}
                </p>
              </div>
            ))}
          </div>
        ) : null}

        {connector?.sources?.length ? (
          <div className="mt-6 overflow-hidden rounded-md border border-gray-200">
            <div className="grid grid-cols-[minmax(0,1fr)_auto_auto_auto] gap-3 bg-gray-50 px-4 py-2 text-xs font-semibold uppercase tracking-wide text-gray-500">
              <span>Source</span>
              <span className="text-right">Kept</span>
              <span className="text-right">Failed</span>
              <span className="text-right">Last sync</span>
            </div>
            {connector.sources.slice(0, 4).map((source) => (
              <div
                key={source.id || source.display_name}
                className="grid grid-cols-[minmax(0,1fr)_auto_auto_auto] gap-3 border-t border-gray-200 px-4 py-3 text-sm"
              >
                <span className="min-w-0 truncate font-medium text-gray-900">
                  {source.display_name}
                </span>
                <span className="text-right text-gray-700">
                  {formatNumber(source.counts.downloaded)}
                </span>
                <span className="text-right text-gray-700">
                  {formatNumber(source.counts.failed)}
                </span>
                <span className="text-right text-gray-600">
                  {source.last_sync ? formatDate(source.last_sync) : "Never"}
                </span>
              </div>
            ))}
          </div>
        ) : null}
      </div>

      <div className="rounded-lg border border-gray-200 bg-white p-5 shadow-sm">
        <div className="flex items-center justify-between">
          <div>
            <p className="text-xs font-semibold uppercase tracking-wide text-gray-500">
              Live queue
            </p>
            <h2 className="mt-2 text-xl font-semibold text-gray-950">
              Worker capacity
            </h2>
          </div>
          <span className="text-sm font-medium text-gray-600">
            {workerStatus
              ? `${workerStatus.queue_size}/${workerStatus.queue_capacity}`
              : "Unavailable"}
          </span>
        </div>
        <div className="mt-5 h-3 overflow-hidden rounded-full bg-gray-100">
          <div
            className={`h-full rounded-full ${
              queuePercent > 80
                ? "bg-red-500"
                : queuePercent > 50
                  ? "bg-amber-500"
                  : "bg-emerald-500"
            }`}
            style={{ width: `${queuePercent}%` }}
          />
        </div>
        <div className="mt-5 grid grid-cols-3 gap-3 text-sm">
          <div>
            <p className="text-gray-500">Workers</p>
            <p className="text-lg font-semibold text-gray-950">
              {workerStatus?.total_workers ?? "-"}
            </p>
          </div>
          <div>
            <p className="text-gray-500">Busy</p>
            <p className="text-lg font-semibold text-gray-950">
              {workerStatus?.workers_busy ?? "-"}
            </p>
          </div>
          <div>
            <p className="text-gray-500">Idle</p>
            <p className="text-lg font-semibold text-gray-950">
              {workerStatus?.workers_idle ?? "-"}
            </p>
          </div>
        </div>
      </div>
    </section>
  );
}

function LiveProgress() {
  const { data } = useQuery({
    queryKey: ["runs"],
    queryFn: () => fetchRuns(20),
    refetchInterval: 3000,
  });
  const runs = data?.runs || [];
  const activeRun = runs.find((run) => run.Status === "running");
  const latestRun = activeRun || runs[0];

  if (!latestRun) {
    return (
      <section className="rounded-lg border border-gray-200 bg-white p-5 shadow-sm">
        <h2 className="text-xl font-semibold text-gray-950">Live progress</h2>
        <p className="mt-2 text-sm text-gray-600">
          No provider runs have been recorded yet.
        </p>
      </section>
    );
  }

  const totalSeen =
    latestRun.PhotosDownloaded + latestRun.PhotosSkipped + latestRun.PhotosFailed;
  const progressTotal = Math.max(totalSeen, latestRun.PhotosFound, 1);
  const downloadedWidth = (latestRun.PhotosDownloaded / progressTotal) * 100;
  const skippedWidth = (latestRun.PhotosSkipped / progressTotal) * 100;
  const failedWidth = (latestRun.PhotosFailed / progressTotal) * 100;

  return (
    <section className="rounded-lg border border-gray-200 bg-white p-5 shadow-sm">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <p className="text-xs font-semibold uppercase tracking-wide text-gray-500">
            Live progress
          </p>
          <h2 className="mt-2 text-xl font-semibold text-gray-950">
            Run #{latestRun.ID}
          </h2>
          <p className="mt-1 text-sm text-gray-600">
            {formatDuration(latestRun.StartTime, latestRun.EndTime)} elapsed,
            {` ${latestRun.PagesProcessed}`} pages processed
          </p>
        </div>
        <span
          className={`w-fit rounded-full border px-3 py-1 text-sm font-medium ${statusClass(latestRun.Status)}`}
        >
          {latestRun.Status}
        </span>
      </div>

      <div className="mt-5 flex h-3 overflow-hidden rounded-full bg-gray-100">
        <div className="bg-emerald-500" style={{ width: `${downloadedWidth}%` }} />
        <div className="bg-gray-400" style={{ width: `${skippedWidth}%` }} />
        <div className="bg-red-500" style={{ width: `${failedWidth}%` }} />
      </div>

      <div className="mt-5 grid gap-3 sm:grid-cols-4">
        <div>
          <p className="text-xs font-medium uppercase tracking-wide text-gray-500">
            Found
          </p>
          <p className="text-2xl font-semibold text-gray-950">
            {latestRun.PhotosFound}
          </p>
        </div>
        <div>
          <p className="text-xs font-medium uppercase tracking-wide text-gray-500">
            Downloaded
          </p>
          <p className="text-2xl font-semibold text-emerald-700">
            {latestRun.PhotosDownloaded}
          </p>
        </div>
        <div>
          <p className="text-xs font-medium uppercase tracking-wide text-gray-500">
            Skipped
          </p>
          <p className="text-2xl font-semibold text-gray-700">
            {latestRun.PhotosSkipped}
          </p>
        </div>
        <div>
          <p className="text-xs font-medium uppercase tracking-wide text-gray-500">
            Failed
          </p>
          <p className="text-2xl font-semibold text-red-700">
            {latestRun.PhotosFailed}
          </p>
        </div>
      </div>
    </section>
  );
}

function RecentMedia() {
  const { data: todayData, isLoading: todayLoading } = useQuery({
    queryKey: ["today-photos", 8, 0],
    queryFn: () => fetchTodayPhotos(8, 0),
    refetchInterval: 30000,
  });
  const { data: weekData } = useQuery({
    queryKey: ["week-photos", 8, 0],
    queryFn: () => fetchWeekPhotos(8, 0),
    refetchInterval: 60000,
  });

  const photos = useMemo<Photo[]>(() => {
    const byID = new Map<number, Photo>();
    [...(todayData?.photos || []), ...(weekData?.photos || [])].forEach(
      (photo) => byID.set(photo.ID, photo),
    );
    return Array.from(byID.values())
      .sort(
        (a, b) =>
          new Date(b.DownloadedAt).getTime() - new Date(a.DownloadedAt).getTime(),
      )
      .slice(0, 8);
  }, [todayData?.photos, weekData?.photos]);

  return (
    <section className="rounded-lg border border-gray-200 bg-white p-5 shadow-sm">
      <div className="flex items-start justify-between gap-4">
        <div>
          <p className="text-xs font-semibold uppercase tracking-wide text-gray-500">
            Recent media
          </p>
          <h2 className="mt-2 text-xl font-semibold text-gray-950">
            Latest extracted files
          </h2>
        </div>
        <Link
          to="/today"
          className="rounded-md border border-gray-300 px-3 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
        >
          View today
        </Link>
      </div>

      {todayLoading ? (
        <div className="mt-5 grid grid-cols-2 gap-4 sm:grid-cols-4 lg:grid-cols-8">
          {Array.from({ length: 8 }).map((_, index) => (
            <div key={index} className="h-36 animate-pulse rounded-md bg-gray-100" />
          ))}
        </div>
      ) : photos.length === 0 ? (
        <p className="mt-5 rounded-md border border-dashed border-gray-300 p-6 text-center text-sm text-gray-600">
          No extracted media is available from the current telemetry.
        </p>
      ) : (
        <div className="mt-5 grid grid-cols-2 gap-4 sm:grid-cols-4 lg:grid-cols-8">
          {photos.map((photo) => (
            <div key={photo.ID} className="min-w-0">
              <ImageThumbnail photoId={photo.ID} title={photo.Title} />
              <p className="mt-2 truncate text-sm font-medium text-gray-900">
                {photo.Title || "Untitled"}
              </p>
              <p className="truncate text-xs text-gray-500">{photo.Artist}</p>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}

export default function ExtractorOperations() {
  return (
    <div className="space-y-6">
      <ProviderStatus />
      <StatsCards />
      <div className="grid gap-6 xl:grid-cols-[1.2fr_0.8fr]">
        <LiveProgress />
        <Controls />
      </div>
      <WorkerStatus />
      <RecentMedia />
      <RunHistory />
      <FailedPhotos compact />
    </div>
  );
}
