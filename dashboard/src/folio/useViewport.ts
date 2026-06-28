import { useEffect, useState } from "react";

const MOBILE_QUERY = "(max-width: 640px)";
const TABLET_QUERY = "(min-width: 641px) and (max-width: 1024px)";

interface ViewportState {
  isMobile: boolean;
  isTablet: boolean;
}

function addQueryListener(query: MediaQueryList, update: () => void): () => void {
  if (typeof query.addEventListener === "function") {
    query.addEventListener("change", update);
    return () => query.removeEventListener("change", update);
  }

  query.addListener(update);
  return () => query.removeListener(update);
}

function getViewportState(): ViewportState {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
    return { isMobile: false, isTablet: false };
  }
  return {
    isMobile: window.matchMedia(MOBILE_QUERY).matches,
    isTablet: window.matchMedia(TABLET_QUERY).matches,
  };
}

export function useViewport(): ViewportState {
  const [viewport, setViewport] = useState<ViewportState>(() => getViewportState());

  useEffect(() => {
    if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
      return;
    }

    const mobile = window.matchMedia(MOBILE_QUERY);
    const tablet = window.matchMedia(TABLET_QUERY);
    const update = () => setViewport({ isMobile: mobile.matches, isTablet: tablet.matches });

    update();
    const removeMobileListener = addQueryListener(mobile, update);
    const removeTabletListener = addQueryListener(tablet, update);
    return () => {
      removeMobileListener();
      removeTabletListener();
    };
  }, []);

  return viewport;
}
