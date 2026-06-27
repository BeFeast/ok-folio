import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  triggerExtraction,
  triggerPagesExtraction,
  triggerPhotoprismIndex,
} from "../api";

export default function Controls() {
  const [message, setMessage] = useState<{
    type: "success" | "error";
    text: string;
  } | null>(null);
  const [pageCount, setPageCount] = useState(5);
  const queryClient = useQueryClient();

  const extractMutation = useMutation({
    mutationFn: triggerExtraction,
    onSuccess: () => {
      setMessage({ type: "success", text: "Provider extraction started." });
      queryClient.invalidateQueries({ queryKey: ["runs"] });
      queryClient.invalidateQueries({ queryKey: ["stats"] });
      const pollInterval = setInterval(() => {
        queryClient.invalidateQueries({ queryKey: ["runs"] });
        queryClient.invalidateQueries({ queryKey: ["stats"] });
      }, 3000);
      setTimeout(() => {
        clearInterval(pollInterval);
        queryClient.invalidateQueries({ queryKey: ["runs"] });
        queryClient.invalidateQueries({ queryKey: ["stats"] });
      }, 60000);
      setTimeout(() => setMessage(null), 5000);
    },
    onError: (error: Error) => {
      setMessage({
        type: "error",
        text: `Failed to start extraction: ${error.message}`,
      });
      setTimeout(() => setMessage(null), 5000);
    },
  });

  const indexMutation = useMutation({
    mutationFn: triggerPhotoprismIndex,
    onSuccess: () => {
      setMessage({
        type: "success",
        text: "Gallery index request sent.",
      });
      setTimeout(() => setMessage(null), 5000);
    },
    onError: (error: Error) => {
      setMessage({
        type: "error",
        text: `Failed to trigger indexing: ${error.message}`,
      });
      setTimeout(() => setMessage(null), 5000);
    },
  });

  const pagesExtractMutation = useMutation({
    mutationFn: (count: number) => triggerPagesExtraction(count),
    onSuccess: () => {
      setMessage({
        type: "success",
        text: `Extraction of ${pageCount} pages started.`,
      });
      queryClient.invalidateQueries({ queryKey: ["runs"] });
      queryClient.invalidateQueries({ queryKey: ["stats"] });
      const pollInterval = setInterval(() => {
        queryClient.invalidateQueries({ queryKey: ["runs"] });
        queryClient.invalidateQueries({ queryKey: ["stats"] });
      }, 3000);
      setTimeout(() => {
        clearInterval(pollInterval);
        queryClient.invalidateQueries({ queryKey: ["runs"] });
        queryClient.invalidateQueries({ queryKey: ["stats"] });
      }, 120000);
      setTimeout(() => setMessage(null), 5000);
    },
    onError: (error: Error) => {
      setMessage({
        type: "error",
        text: `Failed to start pages extraction: ${error.message}`,
      });
      setTimeout(() => setMessage(null), 5000);
    },
  });

  return (
    <section className="rounded-lg border border-gray-200 bg-white p-5 shadow-xs">
      <div className="flex flex-col gap-1">
        <p className="text-xs font-semibold uppercase tracking-wide text-gray-500">
          Retry and run controls
        </p>
        <h2 className="text-xl font-semibold text-gray-950">
          Provider actions
        </h2>
      </div>
      <div className="mt-5 space-y-4">
        <div className="flex flex-wrap items-center gap-3">
          <button
            onClick={() => extractMutation.mutate()}
            disabled={extractMutation.isPending}
            className={`rounded-md px-4 py-2 text-sm font-medium transition-colors ${
              extractMutation.isPending
                ? "bg-gray-300 text-gray-500 cursor-not-allowed"
                : "bg-gray-950 text-white hover:bg-gray-800"
            }`}
          >
            {extractMutation.isPending ? "Starting" : "Start default run"}
          </button>
          <button
            onClick={() => indexMutation.mutate()}
            disabled={indexMutation.isPending}
            className={`rounded-md border px-4 py-2 text-sm font-medium transition-colors ${
              indexMutation.isPending
                ? "border-gray-200 bg-gray-100 text-gray-500 cursor-not-allowed"
                : "border-gray-300 bg-white text-gray-700 hover:bg-gray-50"
            }`}
          >
            {indexMutation.isPending ? "Indexing" : "Request gallery index"}
          </button>
          {message && (
            <div
              className={`rounded-md px-4 py-2 text-sm ${
                message.type === "success"
                  ? "bg-emerald-50 text-emerald-800"
                  : "bg-red-100 text-red-800"
              }`}
            >
              {message.text}
            </div>
          )}
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <label className="text-sm font-medium text-gray-700">Pages:</label>
          <input
            type="number"
            min={1}
            max={20}
            value={pageCount}
            onChange={(e) =>
              setPageCount(
                Math.min(20, Math.max(1, parseInt(e.target.value) || 1)),
              )
            }
            className="w-20 rounded-md border border-gray-300 px-3 py-2 text-center focus:outline-hidden focus:ring-2 focus:ring-gray-500"
          />
          <button
            onClick={() => pagesExtractMutation.mutate(pageCount)}
            disabled={pagesExtractMutation.isPending}
            className={`rounded-md px-4 py-2 text-sm font-medium transition-colors ${
              pagesExtractMutation.isPending
                ? "bg-gray-300 text-gray-500 cursor-not-allowed"
                : "bg-amber-600 text-white hover:bg-amber-700"
            }`}
          >
            {pagesExtractMutation.isPending ? "Starting" : "Extract pages"}
          </button>
        </div>
        <div className="rounded-md border border-gray-200 bg-gray-50 p-3 text-sm text-gray-600">
          Default runs use the provider settings from the extractor service.
          Custom page runs enqueue pages 1 through N, up to 20.
        </div>
      </div>
    </section>
  );
}
