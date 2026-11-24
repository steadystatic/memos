package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/usememos/memos/store"
)

func TestSpaceHandleNumbering(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	t.Cleanup(func() { require.NoError(t, ts.Close()) })

	owner, err := ts.CreateUser(ctx, &store.User{Username: "handle-owner", Role: store.RoleUser})
	require.NoError(t, err)
	hay, err := ts.CreateSpace(ctx, &store.Space{UID: "haymakr", Title: "Haymakr"}, owner.ID)
	require.NoError(t, err)
	home, err := ts.CreateSpace(ctx, &store.Space{UID: "homelab", Title: "Homelab", Key: "lab"}, owner.ID)
	require.NoError(t, err)
	require.Equal(t, "haymakr", hay.Key, "a short uid is its own key")
	require.Equal(t, "lab", home.Key, "an explicit key wins")

	create := func(uid string, spaceID *int32) *store.Memo {
		memo, err := ts.CreateMemo(ctx, &store.Memo{UID: uid, CreatorID: owner.ID, Content: uid, Visibility: store.Private, SpaceID: spaceID})
		require.NoError(t, err)
		return memo
	}
	listByHandle := func(handle string) []*store.Memo {
		memos, err := ts.ListMemos(ctx, &store.FindMemo{Filters: []string{`handle == "` + handle + `"`}})
		require.NoError(t, err)
		return memos
	}

	a := create("handle-a", &hay.ID)
	b := create("handle-b", &hay.ID)
	other := create("handle-other", &home.ID)
	loose := create("handle-loose", nil)
	require.Equal(t, int32(1), *a.SpaceSeq)
	require.Equal(t, int32(2), *b.SpaceSeq)
	require.Equal(t, int32(1), *other.SpaceSeq, "each Space counts on its own")
	require.Nil(t, loose.SpaceSeq, "a memo outside any Space has no number")

	t.Run("comments get per-parent numbers without taking a memo space_seq", func(t *testing.T) {
		comment1, err := ts.CreateMemoComment(ctx, &store.Memo{UID: "handle-comment-1", CreatorID: owner.ID, Content: "c1", Visibility: store.Private, SpaceID: &hay.ID}, a.ID, owner.ID)
		require.NoError(t, err)
		require.Nil(t, comment1.SpaceSeq)
		require.NotNil(t, comment1.CommentSeq)
		require.Equal(t, int32(1), *comment1.CommentSeq)

		comment2, err := ts.CreateMemoComment(ctx, &store.Memo{UID: "handle-comment-2", CreatorID: owner.ID, Content: "c2", Visibility: store.Private}, a.ID, owner.ID)
		require.NoError(t, err)
		require.Nil(t, comment2.SpaceSeq)
		require.Equal(t, int32(2), *comment2.CommentSeq)

		// Moving a comment into a Space must not allocate a memo space_seq.
		require.NoError(t, ts.UpdateMemo(ctx, &store.UpdateMemo{ID: comment2.ID, SpaceID: &hay.ID}))
		reloaded, err := ts.GetMemo(ctx, &store.FindMemo{ID: &comment2.ID})
		require.NoError(t, err)
		require.Nil(t, reloaded.SpaceSeq)
		require.Equal(t, int32(2), *reloaded.CommentSeq)

		// Deleting a comment leaves a gap; numbers are never reused.
		require.NoError(t, ts.DeleteMemo(ctx, &store.DeleteMemo{ID: comment1.ID}))
		comment3, err := ts.CreateMemoComment(ctx, &store.Memo{UID: "handle-comment-3", CreatorID: owner.ID, Content: "c3", Visibility: store.Private, SpaceID: &hay.ID}, a.ID, owner.ID)
		require.NoError(t, err)
		require.Equal(t, int32(3), *comment3.CommentSeq)

		// Comment handles resolve through the filter; deleted ones do not.
		require.Empty(t, listByHandle("haymakr-1-1"))
		foundComment := listByHandle("haymakr-1-2")
		require.Len(t, foundComment, 1)
		require.Equal(t, "handle-comment-2", foundComment[0].UID)
		require.Equal(t, "handle-comment-3", listByHandle("haymakr-1-3")[0].UID)

		// Comments on an unplaced memo have no handle until the parent enters a Space.
		looseComment, err := ts.CreateMemoComment(ctx, &store.Memo{UID: "handle-loose-comment", CreatorID: owner.ID, Content: "lc", Visibility: store.Private}, loose.ID, owner.ID)
		require.NoError(t, err)
		require.Equal(t, int32(1), *looseComment.CommentSeq)

		// Replies to comments (depth > 1) have no handle.
		reply, err := ts.CreateMemoComment(ctx, &store.Memo{UID: "handle-reply", CreatorID: owner.ID, Content: "r", Visibility: store.Private, SpaceID: &hay.ID}, comment2.ID, owner.ID)
		require.NoError(t, err)
		require.Equal(t, int32(1), *reply.CommentSeq)
		require.Empty(t, listByHandle("haymakr-1-2-1"))

		next := create("handle-c", &hay.ID)
		require.Equal(t, int32(3), *next.SpaceSeq, "comments did not take a memo space_seq")
	})

	t.Run("the handle resolves through the filter", func(t *testing.T) {
		found := listByHandle("haymakr-2")
		require.Len(t, found, 1)
		require.Equal(t, "handle-b", found[0].UID)
		require.Equal(t, "haymakr", found[0].SpaceKey)
		require.Equal(t, "handle-other", listByHandle("lab-1")[0].UID)
		require.Empty(t, listByHandle("haymakr-99"))
		require.Empty(t, listByHandle("nokey-1"))
	})

	t.Run("a move renumbers in the new Space", func(t *testing.T) {
		require.NoError(t, ts.UpdateMemo(ctx, &store.UpdateMemo{ID: a.ID, SpaceID: &home.ID}))
		moved, err := ts.GetMemo(ctx, &store.FindMemo{ID: &a.ID})
		require.NoError(t, err)
		require.Equal(t, int32(2), *moved.SpaceSeq)
		require.Equal(t, "lab", moved.SpaceKey)
		require.Empty(t, listByHandle("haymakr-1"), "the old handle no longer resolves")
		require.Equal(t, "handle-a", listByHandle("lab-2")[0].UID)
		require.Empty(t, listByHandle("haymakr-1-2"), "old comment handles stop resolving when parent moves")
		require.Equal(t, "handle-comment-2", listByHandle("lab-2-2")[0].UID, "comment handles follow parent memo")
	})

	t.Run("staying put keeps the number", func(t *testing.T) {
		require.NoError(t, ts.UpdateMemo(ctx, &store.UpdateMemo{ID: b.ID, SpaceID: &hay.ID, Content: new("edited")}))
		same, err := ts.GetMemo(ctx, &store.FindMemo{ID: &b.ID})
		require.NoError(t, err)
		require.Equal(t, int32(2), *same.SpaceSeq)
	})

	t.Run("withdrawing clears the number and numbers are never reused", func(t *testing.T) {
		require.NoError(t, ts.UpdateMemo(ctx, &store.UpdateMemo{ID: b.ID, ClearSpace: true}))
		cleared, err := ts.GetMemo(ctx, &store.FindMemo{ID: &b.ID})
		require.NoError(t, err)
		require.Nil(t, cleared.SpaceSeq)
		require.NoError(t, ts.UpdateMemo(ctx, &store.UpdateMemo{ID: b.ID, SpaceID: &hay.ID}))
		again, err := ts.GetMemo(ctx, &store.FindMemo{ID: &b.ID})
		require.NoError(t, err)
		require.Equal(t, int32(4), *again.SpaceSeq, "haymakr 1 to 3 were taken")
	})
}

func TestSpaceKeys(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	t.Cleanup(func() { require.NoError(t, ts.Close()) })
	owner, err := ts.CreateUser(ctx, &store.User{Username: "key-owner", Role: store.RoleUser})
	require.NoError(t, err)

	first, err := ts.CreateSpace(ctx, &store.Space{UID: "7dda9b2b-ae3a-4692-bf5c-887a5a4f2b10", Title: "Weekly Song Journal"}, owner.ID)
	require.NoError(t, err)
	require.Equal(t, "weekly-song-jour", first.Key, "a long uid falls back to the title")
	second, err := ts.CreateSpace(ctx, &store.Space{UID: "11111111-ae3a-4692-bf5c-887a5a4f2b10", Title: "Weekly Song Journal"}, owner.ID)
	require.NoError(t, err)
	require.Equal(t, "weekly-song-jo-2", second.Key, "a taken derived key gets a suffix within the length limit")

	_, err = ts.CreateSpace(ctx, &store.Space{UID: "key-explicit", Title: "x", Key: first.Key}, owner.ID)
	require.ErrorIs(t, err, store.ErrSpaceKeyTaken, "an explicit key that is taken is refused")
	_, err = ts.CreateSpace(ctx, &store.Space{UID: "key-bad", Title: "x", Key: "Bad Key"}, owner.ID)
	require.ErrorIs(t, err, store.ErrInvalidSpaceKey)

	renamed, err := ts.UpdateSpace(ctx, &store.UpdateSpace{ID: second.ID, Key: new("Songs")}, owner.ID)
	require.NoError(t, err)
	require.Equal(t, "songs", renamed.Key, "keys are lowercased")
	_, err = ts.UpdateSpace(ctx, &store.UpdateSpace{ID: second.ID, Key: &first.Key}, owner.ID)
	require.ErrorIs(t, err, store.ErrSpaceKeyTaken)
	_, err = ts.UpdateSpace(ctx, &store.UpdateSpace{ID: second.ID, Key: new("")}, owner.ID)
	require.ErrorIs(t, err, store.ErrInvalidSpaceKey)

	listed, err := ts.GetSpace(ctx, &store.FindSpace{ID: &second.ID})
	require.NoError(t, err)
	require.Equal(t, "songs", listed.Key)
}
