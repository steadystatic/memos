import { Menu } from "@usememos/ui";
import { AstroidIcon, ChevronDownIcon } from "lucide-react";
import { useLocation, useNavigate } from "react-router-dom";
import SpaceMark from "@/components/SpaceMark";
import { useSpaceContext } from "@/contexts/SpaceContext";
import useCurrentUser from "@/hooks/useCurrentUser";
import { extractSpaceUidFromName } from "@/lib/space-display";
import { cn } from "@/lib/utils";
import { getSpaceSwitchPath } from "@/router/routes";
import { useTranslate } from "@/utils/i18n";

/** Fork: a flat, one-click Space switcher for the page header; the sidebar switcher nests it under a menu. */
const SpaceQuickSwitcher = ({ className }: { className?: string }) => {
  const t = useTranslate();
  const location = useLocation();
  const navigate = useNavigate();
  const currentUser = useCurrentUser();
  const { spaces, duplicateSpaceTitles, selectedSpace, selectedSpaceName } = useSpaceContext();
  if (!currentUser || spaces.length === 0) return null;

  const label = (title: string, name: string) => (duplicateSpaceTitles.has(title) ? `${title} (${extractSpaceUidFromName(name)})` : title);
  const current = selectedSpaceName ? (selectedSpace?.title ?? extractSpaceUidFromName(selectedSpaceName)) : t("space.all-spaces");

  return (
    <Menu.Root modal={false} size="md">
      <Menu.Trigger
        className={cn(
          "flex h-8 max-w-56 items-center gap-1.5 rounded-mu-sm px-2 mu-label text-mu-text hover:bg-mu-hover/65 border border-transparent mu-focus focus-visible:border-mu-focus",
          className,
        )}
        aria-label={`${t("space.switch")}: ${current}`}
        title={current}
      >
        {selectedSpaceName ? <SpaceMark icon={selectedSpace?.icon} size="sm" /> : <AstroidIcon className="mu-icon" />}
        <span className="min-w-0 truncate">{current}</span>
        <ChevronDownIcon className="mu-icon-sm shrink-0 text-mu-text-secondary/70" />
      </Menu.Trigger>
      <Menu.Popup align="end" className="w-56 max-h-[min(27rem,calc(100dvh-5rem))] overflow-y-auto" aria-label={t("space.switch")}>
        <Menu.RadioGroup
          value={selectedSpaceName ?? "all"}
          onValueChange={(value) => navigate(getSpaceSwitchPath(location, value === "all" ? undefined : value))}
        >
          <Menu.RadioItem value="all" closeOnClick icon={<AstroidIcon />}>
            {t("space.all-spaces")}
          </Menu.RadioItem>
          {spaces.map((space) => (
            <Menu.RadioItem key={space.name} value={space.name} closeOnClick icon={<SpaceMark icon={space.icon} size="sm" />}>
              {label(space.title, space.name)}
            </Menu.RadioItem>
          ))}
        </Menu.RadioGroup>
      </Menu.Popup>
    </Menu.Root>
  );
};

export default SpaceQuickSwitcher;
