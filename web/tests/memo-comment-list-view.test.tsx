import { create } from "@bufbuild/protobuf";
import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import MemoCommentListView from "@/components/MemoView/components/MemoCommentListView";
import { MemoSchema } from "@/types/proto/api/memo_service_pb";

const mocks = vi.hoisted(() => ({
  copy: vi.fn(),
  toastAdd: vi.fn(),
  comments: [] as ReturnType<typeof create<typeof MemoSchema>>[],
}));

vi.mock("copy-to-clipboard", () => ({
  default: (text: string) => mocks.copy(text),
}));

vi.mock("@/lib/toast", () => ({
  toast: { add: (opts: unknown) => mocks.toastAdd(opts) },
}));

vi.mock("@/hooks/useNearViewport", () => ({
  useNearViewport: () => ({ ref: { current: null }, isNearViewport: true }),
}));

vi.mock("@/hooks/useMemoQueries", () => ({
  useMemoComments: () => ({ data: { comments: mocks.comments } }),
}));

vi.mock("@/hooks/useUserQueries", () => ({
  useUsersByNames: () => ({ data: new Map() }),
}));

vi.mock("@/components/MemoView/MemoViewContext", () => ({
  useMemoViewContext: () => ({
    memo: create(MemoSchema, { name: "memos/parent", handle: "haymakr-12" }),
    parentPage: "/",
  }),
  useMemoViewDerived: () => ({
    isInMemoDetailPage: false,
    commentAmount: mocks.comments.length,
  }),
}));

vi.mock("@/components/MemoPreview", () => ({
  MemoPreview: ({ content }: { content: string }) => <span>{content}</span>,
}));

vi.mock("@/utils/i18n", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/utils/i18n")>();
  return {
    ...actual,
    useTranslate: () => (key: string, params?: Record<string, string>) => (params?.handle ? `${key}:${params.handle}` : key),
  };
});

describe("MemoCommentListView", () => {
  beforeEach(() => {
    mocks.copy.mockReset();
    mocks.toastAdd.mockReset();
    mocks.comments = [
      create(MemoSchema, { name: "memos/c1", content: "First reply", handle: "haymakr-12-1" }),
      create(MemoSchema, { name: "memos/c2", content: "Second reply", handle: "" }),
    ];
  });

  it("renders a copyable handle beside each comment preview that has one", () => {
    render(
      <MemoryRouter>
        <MemoCommentListView />
      </MemoryRouter>,
    );

    const handleButton = screen.getByRole("button", { name: "memo.copy-handle: haymakr-12-1" });
    expect(handleButton).toBeInTheDocument();
    expect(handleButton.closest("a")).toBeNull();

    fireEvent.click(handleButton);
    expect(mocks.copy).toHaveBeenCalledWith("haymakr-12-1");
    expect(mocks.toastAdd).toHaveBeenCalledWith({
      title: "message.succeed-copy-handle:haymakr-12-1",
      type: "success",
    });

    expect(screen.getAllByRole("button", { name: /memo\.copy-handle/ })).toHaveLength(1);
  });
});
