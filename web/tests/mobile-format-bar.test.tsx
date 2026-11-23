import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import MobileFormatBar from "@/components/MemoEditor/edit/components/MobileFormatBar";

const fakeEditor = (focused: boolean) => {
  const handlers = new Map<string, () => void>();
  const commands = {
    toggleHeading: vi.fn(),
    toggleBold: vi.fn(),
    toggleItalic: vi.fn(),
    toggleBulletList: vi.fn(),
    toggleTaskList: vi.fn(),
    toggleBlockquote: vi.fn(),
  };
  const editor = {
    commands,
    hasFocus: () => focused,
    isActive: (name: string, attrs?: { level?: number }) => name === "heading" && attrs?.level === 2,
    on: (type: string, handler: () => void) => {
      handlers.set(type, handler);
      return () => handlers.delete(type);
    },
  };
  return { editor: editor as never, commands, handlers };
};

describe("MobileFormatBar", () => {
  it("renders nothing until the editor is focused", () => {
    const { editor } = fakeEditor(false);
    render(<MobileFormatBar editor={editor} />);
    expect(screen.queryByRole("toolbar")).toBeNull();
  });

  it("runs the heading command and reflects the active heading", () => {
    const { editor, commands } = fakeEditor(true);
    render(<MobileFormatBar editor={editor} />);
    expect(screen.getByRole("button", { name: "Heading 2" }).getAttribute("aria-pressed")).toBe("true");
    fireEvent.click(screen.getByRole("button", { name: "Heading 1" }));
    expect(commands.toggleHeading).toHaveBeenCalledWith({ level: 1 });
  });

  it("stays out of the way in landscape", () => {
    vi.stubGlobal(
      "matchMedia",
      vi.fn(() => ({ matches: false, addEventListener: vi.fn(), removeEventListener: vi.fn() })),
    );
    const { editor } = fakeEditor(true);
    render(<MobileFormatBar editor={editor} />);
    expect(screen.queryByRole("toolbar")).toBeNull();
    vi.unstubAllGlobals();
  });

  it("does not steal focus from the editor on press", () => {
    const { editor } = fakeEditor(true);
    render(<MobileFormatBar editor={editor} />);
    const notPrevented = fireEvent.mouseDown(screen.getByRole("button", { name: "Bold" }));
    expect(notPrevented).toBe(false);
  });
});
