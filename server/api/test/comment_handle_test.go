package test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	apipb "github.com/usememos/memos/proto/gen/api"
	"github.com/usememos/memos/store"
)

func TestCommentHandles(t *testing.T) {
	ctx := context.Background()
	ts := NewTestService(t)
	defer ts.Cleanup()

	author, err := ts.CreateRegularUser(ctx, "ch-author")
	require.NoError(t, err)
	member, err := ts.CreateRegularUser(ctx, "ch-member")
	require.NoError(t, err)
	outsider, err := ts.CreateRegularUser(ctx, "ch-outsider")
	require.NoError(t, err)

	authorCtx := ts.CreateUserContext(ctx, author.ID)
	memberCtx := ts.CreateUserContext(ctx, member.ID)
	outsiderCtx := ts.CreateUserContext(ctx, outsider.ID)

	hay, err := ts.Store.CreateSpace(ctx, &store.Space{UID: "haymakr", Title: "Haymakr"}, author.ID)
	require.NoError(t, err)
	target, err := ts.Store.CreateSpace(ctx, &store.Space{UID: "target", Title: "Target"}, author.ID)
	require.NoError(t, err)
	_, err = ts.InviteAndAcceptSpaceMember(ctx, &store.SpaceMember{
		SpaceID: hay.ID, UserID: member.ID, Role: store.SpaceMemberRoleUser,
	}, author.ID)
	require.NoError(t, err)

	hayName := "spaces/" + hay.UID
	targetName := "spaces/" + target.UID

	parent, err := ts.Service.CreateMemo(authorCtx, &apipb.CreateMemoRequest{
		Memo: &apipb.Memo{Content: "parent note", Space: &hayName, Visibility: apipb.Visibility_PUBLIC},
	})
	require.NoError(t, err)
	require.Equal(t, "haymakr-1", parent.Handle)

	// Comment 1 placed in the same Space.
	c1, err := ts.Service.CreateMemoComment(memberCtx, &apipb.CreateMemoCommentRequest{
		Parent:  parent.Name,
		Comment: &apipb.Memo{Content: "first comment", Space: &hayName, Visibility: apipb.Visibility_PUBLIC},
	})
	require.NoError(t, err)
	require.Equal(t, "haymakr-1-1", c1.Handle)

	// Comment 2 without explicit space placement still gets a handle from its parent.
	c2, err := ts.Service.CreateMemoComment(memberCtx, &apipb.CreateMemoCommentRequest{
		Parent:  parent.Name,
		Comment: &apipb.Memo{Content: "second comment", Visibility: apipb.Visibility_PUBLIC},
	})
	require.NoError(t, err)
	require.Equal(t, "haymakr-1-2", c2.Handle)

	// ListMemoComments returns comment handles for Space members.
	listed, err := ts.Service.ListMemoComments(memberCtx, &apipb.ListMemoCommentsRequest{Parent: parent.Name})
	require.NoError(t, err)
	require.Len(t, listed.Comments, 2)
	handlesByName := map[string]string{}
	for _, c := range listed.Comments {
		handlesByName[c.Name] = c.Handle
	}
	require.Equal(t, map[string]string{c1.Name: "haymakr-1-1", c2.Name: "haymakr-1-2"}, handlesByName)

	// GetMemo returns the comment's handle.
	gotC2, err := ts.Service.GetMemo(authorCtx, &apipb.GetMemoRequest{Name: c2.Name})
	require.NoError(t, err)
	require.Equal(t, "haymakr-1-2", gotC2.Handle)

	// ListMemos resolves comment handles via the handle filter while still excluding comments otherwise.
	byHandle, err := ts.Service.ListMemos(memberCtx, &apipb.ListMemosRequest{Filter: `handle == "haymakr-1-2"`})
	require.NoError(t, err)
	require.Len(t, byHandle.Memos, 1)
	require.Equal(t, c2.Name, byHandle.Memos[0].Name)
	require.Equal(t, "haymakr-1-2", byHandle.Memos[0].Handle)

	byMemoHandle, err := ts.Service.ListMemos(memberCtx, &apipb.ListMemosRequest{Filter: `handle == "haymakr-1"`})
	require.NoError(t, err)
	require.Len(t, byMemoHandle.Memos, 1)
	require.Equal(t, parent.Name, byMemoHandle.Memos[0].Name)

	ordinaryList, err := ts.Service.ListMemos(memberCtx, &apipb.ListMemosRequest{})
	require.NoError(t, err)
	require.Len(t, ordinaryList.Memos, 1, "ordinary list still excludes comments")
	require.Equal(t, parent.Name, ordinaryList.Memos[0].Name)

	// Outsiders reading PUBLIC comments must not see the Space key or probe it via handle filter.
	outsiderGet, err := ts.Service.GetMemo(outsiderCtx, &apipb.GetMemoRequest{Name: c2.Name})
	require.NoError(t, err)
	require.Empty(t, outsiderGet.Handle)

	outsiderComments, err := ts.Service.ListMemoComments(outsiderCtx, &apipb.ListMemoCommentsRequest{Parent: parent.Name})
	require.NoError(t, err)
	require.Len(t, outsiderComments.Comments, 2)
	for _, c := range outsiderComments.Comments {
		require.Empty(t, c.Handle)
	}

	outsiderProbe, err := ts.Service.ListMemos(outsiderCtx, &apipb.ListMemosRequest{Filter: `handle == "haymakr-1-2"`})
	require.NoError(t, err)
	require.Empty(t, outsiderProbe.Memos, "handle filter must not confirm a hidden Space key to non-members")

	// Bearer-shared comments omit handle alongside parent.
	share, err := ts.Service.CreateMemoShare(memberCtx, &apipb.CreateMemoShareRequest{Parent: c2.Name, MemoShare: &apipb.MemoShare{}})
	require.NoError(t, err)
	shareToken := share.Name[strings.LastIndex(share.Name, "/")+1:]
	sharedComment, err := ts.Service.GetSharedMemo(memberCtx, &apipb.GetSharedMemoRequest{ShareToken: shareToken})
	require.NoError(t, err)
	require.Empty(t, sharedComment.GetParent())
	require.Empty(t, sharedComment.Handle)

	// Deleting comment 1 leaves a gap; the next comment is -3.
	_, err = ts.Service.DeleteMemo(memberCtx, &apipb.DeleteMemoRequest{Name: c1.Name})
	require.NoError(t, err)
	c3, err := ts.Service.CreateMemoComment(authorCtx, &apipb.CreateMemoCommentRequest{
		Parent:  parent.Name,
		Comment: &apipb.Memo{Content: "third comment", Space: &hayName, Visibility: apipb.Visibility_PUBLIC},
	})
	require.NoError(t, err)
	require.Equal(t, "haymakr-1-3", c3.Handle)

	// Renaming the Space key updates memo and comment handles immediately.
	_, err = ts.Service.UpdateSpace(authorCtx, &apipb.UpdateSpaceRequest{
		Space:      &apipb.Space{Name: hayName, Key: "hm"},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"key"}},
	})
	require.NoError(t, err)
	renamedC2, err := ts.Service.ListMemos(memberCtx, &apipb.ListMemosRequest{Filter: `handle == "hm-1-2"`})
	require.NoError(t, err)
	require.Len(t, renamedC2.Memos, 1)
	require.Equal(t, "hm-1-2", renamedC2.Memos[0].Handle)

	// Moving the parent memo to a Space where member does not belong hides the new handle from member
	// while exposing it to author.
	movedParent, err := ts.Service.UpdateMemo(authorCtx, &apipb.UpdateMemoRequest{
		Memo:       &apipb.Memo{Name: parent.Name, Space: &targetName, Visibility: apipb.Visibility_SPACE},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"space", "visibility"}},
	})
	require.NoError(t, err)
	require.Equal(t, "target-1", movedParent.Handle)

	authorMovedC2, err := ts.Service.GetMemo(authorCtx, &apipb.GetMemoRequest{Name: c2.Name})
	require.NoError(t, err)
	require.Equal(t, "target-1-2", authorMovedC2.Handle)

	memberMovedC2, err := ts.Service.GetMemo(memberCtx, &apipb.GetMemoRequest{Name: c2.Name})
	require.NoError(t, err)
	require.Empty(t, memberMovedC2.Handle, "comment author must not learn the target Space key after parent moves")

	memberProbeMoved, err := ts.Service.ListMemos(memberCtx, &apipb.ListMemosRequest{Filter: `handle == "target-1-2"`})
	require.NoError(t, err)
	require.Empty(t, memberProbeMoved.Memos)
}
