import copy from "copy-to-clipboard";
import { toast } from "@/lib/toast";
import { useTranslate } from "@/utils/i18n";

/**
 * Fork: a memo's short handle ("haymakr-12"), small and muted beside its time. Tapping it copies the
 * handle, which names the memo to people and, through the list filter, to the Agent.
 */
const MemoHandle = ({ handle }: { handle: string }) => {
  const t = useTranslate();
  if (!handle) return null;
  return (
    <button
      type="button"
      title={t("memo.copy-handle")}
      aria-label={`${t("memo.copy-handle")}: ${handle}`}
      onClick={(event) => {
        // The header sits inside a card that opens on click.
        event.preventDefault();
        event.stopPropagation();
        copy(handle);
        toast.add({ title: t("message.succeed-copy-handle", { handle }), type: "success" });
      }}
      className="shrink-0 whitespace-nowrap rounded-mu-sm border border-transparent px-1 font-mono mu-caption text-mu-text-secondary/70 transition-colors hover:bg-mu-hover/65 hover:text-mu-text-secondary mu-focus focus-visible:border-mu-focus"
    >
      {handle}
    </button>
  );
};

export default MemoHandle;
