import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import {
  fetchFailedPhotos,
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
import { formatDate, formatDuration } from "../utils";
import type { ExtractionRun, Photo } from "../types";

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

function ProviderStatus() {
  const { data: runsData } = useQuery({
    queryKey: ["runs"],
    queryFn: () => fetchRuns(20),
    refetchInterval: (query) => {
      const runs = query.state.data?.runs || [];
      return runs.some((run: ExtractionRun) => run.Status === "running")
        ? 3000
        : 30000;
    },
  });
  const { data: workerStatus } = useQuery({
    queryKey: ["worker-status"],
    queryFn: fetchWorkerStatus,
    refetchInterval: 5000,
  });
  const { data: failedData } = useQuery({
    queryKey: ["failed-photos"],
    queryFn: () => fetchFailedPhotos(50),
    refetchInterval: 30000,
  });

  const runs = runsData?.runs || [];
  const activeRun = runs.find((run) => run.Status === "running");
  const latestRun = runs[0];
  const providerState = activeRun
    ? "Running"
    : latestRun?.Status === "failed"
      ? "Needs review"
      : "Idle";
  const providerTone = activeRun
    ? "border-amber-200 bg-amber-50 text-amber-900"
    : latestRun?.Status === "failed"
      ? "border-red-200 bg-red-50 text-red-900"
      : "border-emerald-200 bg-emerald-50 text-emerald-900";
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
                Web Gallery
              </h2>
              <span
                className={`rounded-full border px-3 py-1 text-sm font-medium ${providerTone}`}
              >
                {providerState}
              </span>
            </div>
            <p className="mt-2 max-w-2xl text-sm text-gray-600">
              Operator surface for provider extraction runs, queue pressure,
              failures, retries, and verification from live service telemetry.
            </p>
          </div>
          <Link
            to={latestRun ? `/runs/${latestRun.ID}` : "/"}
            className="inline-flex items-center justify-center rounded-md border border-gray-300 px-3 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
          >
            Latest run
          </Link>
        </div>

        <div className="mt-6 grid gap-3 sm:grid-cols-3">
          <div className="rounded-md border border-gray-200 p-4">
            <p className="text-xs font-medium uppercase tracking-wide text-gray-500">
              Current run
            </p>
            <p className="mt-2 text-xl font-semibold text-gray-950">
              {activeRun ? `#${activeRun.ID}` : "None"}
            </p>
            <p className="mt-1 text-sm text-gray-600">
              {activeRun
                ? `${activeRun.PagesProcessed} pages processed`
                : "No run is active"}
            </p>
          </div>
          <div className="rounded-md border border-gray-200 p-4">
            <p className="text-xs font-medium uppercase tracking-wide text-gray-500">
              Last result
            </p>
            <p className="mt-2 text-xl font-semibold text-gray-950">
              {latestRun ? latestRun.Status : "No runs"}
            </p>
            <p className="mt-1 text-sm text-gray-600">
              {latestRun
                ? `${formatDate(latestRun.StartTime)}`
                : "Trigger a run to create history"}
            </p>
          </div>
          <div className="rounded-md border border-gray-200 p-4">
            <p className="text-xs font-medium uppercase tracking-wide text-gray-500">
              Failed media
            </p>
            <p className="mt-2 text-xl font-semibold text-gray-950">
              {failedData?.count ?? 0}
            </p>
            <p className="mt-1 text-sm text-gray-600">
              {failedData?.count ? "Retry queue needs attention" : "No failed downloads"}
            </p>
          </div>
        </div>
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
