import { useEffect, useState } from "react";

const MOBILE_QUERY = "(max-width: 640px)";
const TABLET_QUERY = "(min-width: 641px) and (max-width: 1024px)";

interface ViewportState {
  isMobile: boolean;
  isTablet: boolean;
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
    mobile.addEventListener("change", update);
    tablet.addEventListener("change", update);
    return () => {
      mobile.removeEventListener("change", update);
      tablet.removeEventListener("change", update);
    };
  }, []);

  return viewport;
}
