import { useEffect, useState } from "react";

export interface StorageEstimateState {
  usage: number | null;
  quota: number | null;
  supported: boolean;
}

const OFFLINE_CACHE_PREFIXES = ["ok-folio-", "workbox-precache"];

async function estimateCacheUsage(cacheName: string): Promise<number> {
  const cache = await caches.open(cacheName);
  const requests = await cache.keys();
  const sizes = await Promise.all(
    requests.map(async (request) => {
      const response = await cache.match(request);
      if (!response) {
        return 0;
      }

      const contentLength = response.headers.get("content-length");
      if (contentLength) {
        const parsed = Number.parseInt(contentLength, 10);
        if (Number.isFinite(parsed)) {
          return parsed;
        }
      }

      try {
        return (await response.clone().blob()).size;
      } catch {
        return 0;
      }
    }),
  );

  return sizes.reduce((total, size) => total + size, 0);
}

export function useStorageEstimate(refreshKey = 0): StorageEstimateState {
  const [estimate, setEstimate] = useState<StorageEstimateState>({
    usage: null,
    quota: null,
    supported: false,
  });

  useEffect(() => {
    let active = true;
    if (
      typeof caches === "undefined" ||
      typeof navigator === "undefined" ||
      !navigator.storage?.estimate
    ) {
      setEstimate({ usage: null, quota: null, supported: false });
      return;
    }

    Promise.all([caches.keys(), navigator.storage.estimate()])
      .then(async ([cacheNames, result]) => {
        const offlineCacheNames = cacheNames.filter((cacheName) =>
          OFFLINE_CACHE_PREFIXES.some((prefix) => cacheName.startsWith(prefix)),
        );
        const usage = (
          await Promise.all(offlineCacheNames.map((cacheName) => estimateCacheUsage(cacheName)))
        ).reduce((total, size) => total + size, 0);

        if (!active) return;
        setEstimate({
          usage,
          quota: result.quota ?? null,
          supported: true,
        });
      })
      .catch(() => {
        if (!active) return;
        setEstimate({ usage: null, quota: null, supported: false });
      });

    return () => {
      active = false;
    };
  }, [refreshKey]);

  return estimate;
}
