import { useEffect, useRef } from "react";
import useCurrentUser from "@/hooks/useCurrentUser";
import { useSpaceMembers } from "@/hooks/useSpaceQueries";
import { Visibility } from "@/types/proto/api/memo_service_pb";
import { useEditorContext } from "../state";

/** Fork: the user whose presence in a Space makes new notes there default to the Members audience. */
export const AGENT_USER_NAME = "users/agent";

/**
 * Defaults a new memo to the Members (`SPACE`) audience when its Space is shared with the Agent user.
 * Applied once per Space as the placement settles, so a later manual audience choice sticks.
 */
export function useAgentSpaceVisibility({ space, enabled }: { space?: string; enabled: boolean }) {
  const currentUser = useCurrentUser();
  const { actions, dispatch } = useEditorContext();
  const { data: members } = useSpaceMembers(currentUser?.name, space, { enabled });
  const appliedFor = useRef<string | undefined>(undefined);
  const shared = members?.some((member) => member.user === AGENT_USER_NAME) ?? false;

  useEffect(() => {
    if (!enabled || !space || !shared || appliedFor.current === space) return;
    appliedFor.current = space;
    dispatch(actions.setMetadata({ visibility: Visibility.SPACE }));
  }, [enabled, space, shared, actions, dispatch]);
}
