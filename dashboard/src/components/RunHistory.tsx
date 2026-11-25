import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { fetchRuns } from "../api";
import { formatDate, formatDuration } from "../utils";
import type { ExtractionRun } from "../types";

export default function RunHistory() {
  const { data, isLoading, error } = useQuery({
    queryKey: ["runs"],
    queryFn: () => fetchRuns(20),
    refetchInterval: (query) => {
      // Poll every 3 seconds if there's a running extraction
      const runs = query.state.data?.runs || [];
      const hasRunning = runs.some(
        (run: ExtractionRun) => run.Status === "running",
      );
      return hasRunning ? 3000 : 30000;
    },
  });

  if (isLoading) {
    return (
      <section className="rounded-lg border border-gray-200 bg-white p-5 shadow-xs">
        <h2 className="text-xl font-semibold text-gray-950">Run history</h2>
        <div className="animate-pulse space-y-3">
          {[...Array(5)].map((_, i) => (
            <div key={i} className="h-12 bg-gray-200 rounded-sm"></div>
          ))}
        </div>
      </section>
    );
  }

  if (error) {
    return (
      <section className="rounded-lg border border-gray-200 bg-white p-5 shadow-xs">
        <h2 className="text-xl font-semibold text-gray-950">Run history</h2>
        <div className="bg-red-50 border border-red-200 rounded-sm p-4">
          <p className="text-red-800">Error loading runs: {error.message}</p>
        </div>
      </section>
    );
  }

  const runs = data?.runs || [];

  return (
    <section className="rounded-lg border border-gray-200 bg-white shadow-xs">
      <div className="border-b border-gray-200 p-5">
        <p className="text-xs font-semibold uppercase tracking-wide text-gray-500">
          Verification trail
        </p>
        <h2 className="mt-2 text-xl font-semibold text-gray-950">
          Run history
        </h2>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full">
          <thead className="bg-gray-50">
            <tr>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Run
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Started
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Duration
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Status
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Pages
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Downloaded
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Skipped
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Failed
              </th>
            </tr>
          </thead>
          <tbody className="bg-white divide-y divide-gray-200">
            {runs.length === 0 ? (
              <tr>
                <td colSpan={8} className="px-6 py-4 text-center text-gray-500">
                  No extraction runs reported by the API
                </td>
              </tr>
            ) : (
              runs.map((run) => (
                <tr key={run.ID} className="hover:bg-gray-50">
                  <td className="px-6 py-4 whitespace-nowrap text-sm font-medium">
                    <Link
                      to={`/runs/${run.ID}`}
                      className="text-blue-600 hover:text-blue-800 hover:underline"
                    >
                      #{run.ID}
                    </Link>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    {formatDate(run.StartTime)}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    {formatDuration(run.StartTime, run.EndTime)}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <span
                      className={`inline-flex rounded-full border px-2 py-1 text-xs font-semibold ${
                        run.Status === "completed"
                          ? "bg-emerald-50 text-emerald-800 border-emerald-200"
                          : run.Status === "failed"
                            ? "bg-red-50 text-red-800 border-red-200"
                            : "bg-amber-50 text-amber-800 border-amber-200"
                      }`}
                    >
                      {run.Status}
                    </span>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    {run.PagesProcessed}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    <span className="text-emerald-700 font-medium">
                      {run.PhotosDownloaded}
                    </span>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    {run.PhotosSkipped}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    {run.PhotosFailed > 0 ? (
                      <span className="text-red-600 font-medium">
                        {run.PhotosFailed}
                      </span>
                    ) : (
                      run.PhotosFailed
                    )}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </section>
  );
}
