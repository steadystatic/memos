import { render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import PullToRefresh from "@/components/PullToRefresh";

const reload = vi.fn();

const touch = (type: string, y: number, target: Element = document.body) => {
  const event = new Event(type, { bubbles: true }) as Event & { touches: { clientY: number }[] };
  event.touches = type === "touchend" ? [] : [{ clientY: y }];
  target.dispatchEvent(event);
};

const setup = ({ standalone = true } = {}) => {
  Object.defineProperty(window.navigator, "standalone", { value: standalone, configurable: true });
  vi.stubGlobal(
    "matchMedia",
    vi.fn((query: string) => ({ matches: query === "(pointer: coarse)", addEventListener: vi.fn(), removeEventListener: vi.fn() })),
  );
  render(<PullToRefresh />);
};

describe("PullToRefresh", () => {
  beforeEach(() => {
    reload.mockClear();
    vi.stubGlobal("location", { ...window.location, reload });
  });
  afterEach(() => vi.unstubAllGlobals());

  it("reloads after a long pull down from the top", () => {
    setup();
    touch("touchstart", 100);
    touch("touchmove", 300);
    touch("touchend", 300);
    expect(reload).toHaveBeenCalledTimes(1);
  });

  it("ignores a short pull", () => {
    setup();
    touch("touchstart", 100);
    touch("touchmove", 150);
    touch("touchend", 150);
    expect(reload).not.toHaveBeenCalled();
  });

  it("ignores a pull that starts in a text field", () => {
    setup();
    const input = document.body.appendChild(document.createElement("textarea"));
    touch("touchstart", 100, input);
    touch("touchmove", 300, input);
    touch("touchend", 300, input);
    expect(reload).not.toHaveBeenCalled();
  });

  it("does nothing in a normal browser tab, which has its own pull-to-refresh", () => {
    setup({ standalone: false });
    touch("touchstart", 100);
    touch("touchmove", 300);
    touch("touchend", 300);
    expect(reload).not.toHaveBeenCalled();
  });
});
