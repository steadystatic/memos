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

	t.Run("comments are not numbered", func(t *testing.T) {
		comment, err := ts.CreateMemoComment(ctx, &store.Memo{UID: "handle-comment", CreatorID: owner.ID, Content: "c", Visibility: store.Private, SpaceID: &hay.ID}, a.ID, owner.ID)
		require.NoError(t, err)
		require.Nil(t, comment.SpaceSeq)
		next := create("handle-c", &hay.ID)
		require.Equal(t, int32(3), *next.SpaceSeq, "the comment did not take a number")
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
