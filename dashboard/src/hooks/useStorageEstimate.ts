import { useEffect, useState } from "react";

export interface StorageEstimateState {
  usage: number | null;
  quota: number | null;
  supported: boolean;
}

export function useStorageEstimate(refreshKey = 0): StorageEstimateState {
  const [estimate, setEstimate] = useState<StorageEstimateState>({
    usage: null,
    quota: null,
    supported: false,
  });

  useEffect(() => {
    let active = true;
    if (typeof navigator === "undefined" || !navigator.storage?.estimate) {
      setEstimate({ usage: null, quota: null, supported: false });
      return;
    }

    navigator.storage
      .estimate()
      .then((result) => {
        if (!active) return;
        setEstimate({
          usage: result.usage ?? null,
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
