import { useQuery } from "@tanstack/react-query";
import { fetchWorkerStatus } from "../api";

export default function WorkerStatus() {
  const { data: status } = useQuery({
    queryKey: ["worker-status"],
    queryFn: fetchWorkerStatus,
    refetchInterval: 5000,
  });

  if (!status) {
    return null;
  }

  return (
    <section className="rounded-lg border border-gray-200 bg-white p-5 shadow-xs">
      <div className="flex flex-col gap-1 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <p className="text-xs font-semibold uppercase tracking-wide text-gray-500">
            Queue state
          </p>
          <h2 className="mt-2 text-xl font-semibold text-gray-950">
            Worker pool
          </h2>
        </div>
        <p className="text-sm text-gray-600">Refreshes every 5 seconds</p>
      </div>

      <div className="mt-5 grid grid-cols-2 gap-4 md:grid-cols-4">
        <div>
          <p className="text-sm text-gray-600">Total workers</p>
          <p className="text-2xl font-semibold text-gray-950">
            {status.total_workers}
          </p>
        </div>
        <div>
          <p className="text-sm text-gray-600">Busy</p>
          <p className="text-2xl font-semibold text-amber-700">
            {status.workers_busy}
          </p>
        </div>
        <div>
          <p className="text-sm text-gray-600">Idle</p>
          <p className="text-2xl font-semibold text-emerald-700">
            {status.workers_idle}
          </p>
        </div>
        <div>
          <p className="text-sm text-gray-600">Queue utilization</p>
          <p className="text-2xl font-semibold text-gray-950">
            {status.queue_utilization.toFixed(1)}%
          </p>
        </div>
      </div>

      <div className="mt-4">
        <div className="flex items-center justify-between text-sm text-gray-600 mb-2">
          <span>Extraction jobs queued</span>
          <span>
            {status.queue_size} / {status.queue_capacity}
          </span>
        </div>
        <div className="h-3 w-full rounded-full bg-gray-100">
          <div
            className={`h-3 rounded-full transition-all ${
              status.queue_utilization > 80
                ? "bg-red-500"
                : status.queue_utilization > 50
                  ? "bg-amber-500"
                  : "bg-emerald-500"
            }`}
            style={{ width: `${Math.min(status.queue_utilization, 100)}%` }}
          ></div>
        </div>
      </div>
    </section>
  );
}
