import { RefreshCwIcon } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { cn } from "@/lib/utils";

const THRESHOLD = 72;
const MAX_PULL = 110;
const RESISTANCE = 0.5;
/** Places that own touch drags: text entry, dialogs and the maps. */
const IGNORED =
  'input, textarea, select, dialog, [role="dialog"], [contenteditable="true"], .leaflet-container, .maplibregl-map, [data-no-pull-refresh]';

/** A Home Screen web app has no browser chrome, so iOS gives it no pull-to-refresh and no reload button. */
const isStandalone = () =>
  typeof window !== "undefined" &&
  ((window.navigator as Navigator & { standalone?: boolean }).standalone === true ||
    (typeof matchMedia === "function" && matchMedia("(display-mode: standalone)").matches));

/** Whether the touch began inside something that is itself scrolled down, which a pull should scroll back first. */
const insideScrolledContainer = (target: Element | null) => {
  for (let el = target; el && el !== document.documentElement; el = el.parentElement) {
    if (el.scrollTop > 0) return true;
  }
  return false;
};

/**
 * Fork: swipe down from the top of the page to reload it, in a Home Screen web app.
 * A reload also picks up a newly deployed frontend, which is the other reason to want it.
 */
const PullToRefresh = () => {
  const [distance, setDistance] = useState(0);
  const [refreshing, setRefreshing] = useState(false);
  const startY = useRef<number | null>(null);
  const pulled = useRef(0);

  useEffect(() => {
    if (!isStandalone() || typeof matchMedia !== "function" || !matchMedia("(pointer: coarse)").matches) return;

    const reset = () => {
      startY.current = null;
      pulled.current = 0;
      setDistance(0);
    };

    const onStart = (event: TouchEvent) => {
      const target = event.target instanceof Element ? event.target : null;
      const atTop = (document.scrollingElement?.scrollTop ?? window.scrollY) <= 0;
      startY.current =
        event.touches.length === 1 && atTop && !target?.closest(IGNORED) && !insideScrolledContainer(target)
          ? event.touches[0].clientY
          : null;
    };

    const onMove = (event: TouchEvent) => {
      if (startY.current === null) return;
      const delta = event.touches[0].clientY - startY.current;
      // Scrolling up, or the page moved off the top: this is an ordinary scroll.
      if (delta <= 0 || (document.scrollingElement?.scrollTop ?? window.scrollY) > 0) {
        if (pulled.current > 0) reset();
        else startY.current = delta <= 0 ? event.touches[0].clientY : startY.current;
        return;
      }
      pulled.current = Math.min(delta * RESISTANCE, MAX_PULL);
      setDistance(pulled.current);
    };

    const onEnd = () => {
      if (startY.current === null) return;
      if (pulled.current >= THRESHOLD) {
        setRefreshing(true);
        setDistance(THRESHOLD);
        window.location.reload();
        return;
      }
      reset();
    };

    document.addEventListener("touchstart", onStart, { passive: true });
    document.addEventListener("touchmove", onMove, { passive: true });
    document.addEventListener("touchend", onEnd, { passive: true });
    document.addEventListener("touchcancel", reset, { passive: true });
    return () => {
      document.removeEventListener("touchstart", onStart);
      document.removeEventListener("touchmove", onMove);
      document.removeEventListener("touchend", onEnd);
      document.removeEventListener("touchcancel", reset);
    };
  }, []);

  if (distance <= 0 && !refreshing) return null;
  const progress = Math.min(distance / THRESHOLD, 1);

  return (
    <div
      aria-hidden="true"
      className="pointer-events-none fixed inset-x-0 top-0 z-50 flex justify-center"
      style={{ paddingTop: "env(safe-area-inset-top)", transform: `translateY(${distance - 36}px)` }}
    >
      <div className="flex size-9 items-center justify-center rounded-full border border-mu-line bg-mu-canvas text-mu-text-secondary shadow-sm">
        <RefreshCwIcon
          className={cn("size-4", refreshing && "animate-spin")}
          style={refreshing ? undefined : { transform: `rotate(${progress * 270}deg)`, opacity: 0.4 + progress * 0.6 }}
        />
      </div>
    </div>
  );
};

export default PullToRefresh;
