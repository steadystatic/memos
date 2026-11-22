import type { Editor } from "@usememos/editor";
import { BoldIcon, Heading1Icon, Heading2Icon, Heading3Icon, ItalicIcon, ListChecksIcon, ListIcon, QuoteIcon } from "lucide-react";
import { type ReactNode, useEffect, useReducer, useState } from "react";
import { createPortal } from "react-dom";
import { cn } from "@/lib/utils";

/** Fork: touch devices have a native selection menu that covers the floating toolbar, so formatting is docked above the keyboard. */
export const isTouchDevice = () => typeof matchMedia === "function" && matchMedia("(pointer: coarse)").matches;

interface Tool {
  id: string;
  label: string;
  icon: ReactNode;
  active: (editor: Editor) => boolean;
  run: (editor: Editor) => void;
}

const heading = (level: 1 | 2 | 3): Pick<Tool, "active" | "run"> => ({
  active: (editor) => editor.isActive("heading", { level }),
  run: (editor) => void editor.commands.toggleHeading({ level }),
});

const TOOLS: Tool[] = [
  { id: "h1", label: "Heading 1", icon: <Heading1Icon />, ...heading(1) },
  { id: "h2", label: "Heading 2", icon: <Heading2Icon />, ...heading(2) },
  { id: "h3", label: "Heading 3", icon: <Heading3Icon />, ...heading(3) },
  { id: "bold", label: "Bold", icon: <BoldIcon />, active: (e) => e.isActive("bold"), run: (e) => void e.commands.toggleBold() },
  { id: "italic", label: "Italic", icon: <ItalicIcon />, active: (e) => e.isActive("italic"), run: (e) => void e.commands.toggleItalic() },
  {
    id: "bullet",
    label: "Bulleted list",
    icon: <ListIcon />,
    active: (e) => e.isActive("bulletList"),
    run: (e) => void e.commands.toggleBulletList(),
  },
  {
    id: "task",
    label: "To-do list",
    icon: <ListChecksIcon />,
    active: (e) => e.isActive("taskList"),
    run: (e) => void e.commands.toggleTaskList(),
  },
  {
    id: "quote",
    label: "Quote",
    icon: <QuoteIcon />,
    active: (e) => e.isActive("blockquote"),
    run: (e) => void e.commands.toggleBlockquote(),
  },
];

/** The gap between the layout viewport's bottom and the visual viewport's, which is the on-screen keyboard's height. */
const useKeyboardInset = () => {
  const [inset, setInset] = useState(0);
  useEffect(() => {
    const viewport = window.visualViewport;
    if (!viewport) return;
    const update = () => setInset(Math.max(0, Math.round(window.innerHeight - viewport.height - viewport.offsetTop)));
    update();
    viewport.addEventListener("resize", update);
    viewport.addEventListener("scroll", update);
    return () => {
      viewport.removeEventListener("resize", update);
      viewport.removeEventListener("scroll", update);
    };
  }, []);
  return inset;
};

const MobileFormatBar = ({ editor }: { editor: Editor | null }) => {
  const [focused, setFocused] = useState(false);
  const [, refresh] = useReducer((n: number) => n + 1, 0);
  const inset = useKeyboardInset();

  useEffect(() => {
    if (!editor) return;
    setFocused(editor.hasFocus());
    const offs = [
      editor.on("focus", () => setFocused(true)),
      // The bar's own buttons never take focus, so a blur is the editor really losing it.
      editor.on("blur", () => setFocused(false)),
      editor.on("selectionChange", refresh),
    ];
    return () => offs.forEach((off) => off());
  }, [editor]);

  if (!editor || !focused) return null;

  return createPortal(
    <div
      role="toolbar"
      aria-label="Formatting"
      style={{ bottom: inset }}
      className="fixed inset-x-0 z-50 flex items-center gap-1 overflow-x-auto border-t border-mu-line bg-mu-canvas px-2 py-1.5 [scrollbar-width:none]"
    >
      {TOOLS.map((tool) => {
        const active = tool.active(editor);
        return (
          <button
            key={tool.id}
            type="button"
            aria-label={tool.label}
            aria-pressed={active}
            // Keep the selection and the keyboard: the editor must not lose focus to a tap on the bar.
            onPointerDown={(event) => event.preventDefault()}
            onMouseDown={(event) => event.preventDefault()}
            onClick={() => {
              tool.run(editor);
              refresh();
            }}
            className={cn(
              "flex size-10 shrink-0 items-center justify-center rounded-mu-sm text-mu-text-secondary [&_svg]:size-5",
              active && "bg-mu-hover text-mu-text",
            )}
          >
            {tool.icon}
          </button>
        );
      })}
    </div>,
    document.body,
  );
};

export default MobileFormatBar;
