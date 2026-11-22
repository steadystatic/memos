import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { Visibility } from "@/types/proto/api/memo_service_pb";

const dispatch = vi.fn();
let members: { user: string }[] | undefined;

vi.mock("@/hooks/useCurrentUser", () => ({ default: () => ({ name: "users/mike" }) }));
vi.mock("@/hooks/useSpaceQueries", () => ({ useSpaceMembers: () => ({ data: members }) }));
vi.mock("@/components/MemoEditor/edit/state", () => ({
  useEditorContext: () => ({ dispatch, actions: { setMetadata: (metadata: unknown) => ({ type: "meta", metadata }) } }),
}));

import { useAgentSpaceVisibility } from "@/components/MemoEditor/edit/hooks/useAgentSpaceVisibility";

describe("useAgentSpaceVisibility", () => {
  beforeEach(() => {
    dispatch.mockClear();
    members = [{ user: "users/mike" }, { user: "users/agent" }];
  });

  it("defaults to Members when the Space is shared with the Agent user", () => {
    renderHook(() => useAgentSpaceVisibility({ space: "spaces/a", enabled: true }));
    expect(dispatch).toHaveBeenCalledWith({ type: "meta", metadata: { visibility: Visibility.SPACE } });
  });

  it("leaves other Spaces alone", () => {
    members = [{ user: "users/mike" }];
    renderHook(() => useAgentSpaceVisibility({ space: "spaces/a", enabled: true }));
    expect(dispatch).not.toHaveBeenCalled();
  });

  it("does nothing for edits and replies", () => {
    renderHook(() => useAgentSpaceVisibility({ space: "spaces/a", enabled: false }));
    expect(dispatch).not.toHaveBeenCalled();
  });

  it("applies once per Space so a manual choice sticks", () => {
    const { rerender } = renderHook(() => useAgentSpaceVisibility({ space: "spaces/a", enabled: true }));
    rerender();
    expect(dispatch).toHaveBeenCalledTimes(1);
  });
});
