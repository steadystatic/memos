package test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/usememos/memos/store"
)

// TestMigrationSpaceHandleBackfill upgrades a database that already has Spaces and memos: keys
// come from short uids, numbers follow creation order, and comments are left out.
func TestMigrationSpaceHandleBackfill(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	t.Cleanup(func() { require.NoError(t, ts.Close()) })
	driver := getDriverFromEnv()
	user, err := createTestingHostUser(ctx, ts)
	require.NoError(t, err)

	dropSpaceHandleSchema(ctx, t, ts, driver)
	longUID := "7dda9b2b-ae3a-4692-bf5c-887a5a4f2b10"
	exec := func(statement string) {
		t.Helper()
		_, err := ts.GetDriver().GetDB().ExecContext(ctx, statement)
		require.NoError(t, err, statement)
	}
	exec(fmt.Sprintf("INSERT INTO space (id, uid, title, description, payload) VALUES (901, 'Haymakr', 'Haymakr', '', '{}'), (902, '%s', 'Songs', '', '{}')", longUID))
	memo := func(id int, uid string, spaceID string, createdTs int) {
		exec(fmt.Sprintf(
			"INSERT INTO memo (id, uid, creator_id, created_ts, content, visibility, payload, space_id) VALUES (%d, '%s', %d, %d, '%s', 'PRIVATE', '{}', %s)",
			id, uid, user.ID, createdTs, uid, spaceID))
	}
	if driver == "mysql" {
		memo = func(id int, uid string, spaceID string, createdTs int) {
			exec(fmt.Sprintf(
				"INSERT INTO memo (id, uid, creator_id, created_ts, content, visibility, payload, space_id) VALUES (%d, '%s', %d, FROM_UNIXTIME(%d), '%s', 'PRIVATE', '{}', %s)",
				id, uid, user.ID, createdTs, uid, spaceID))
		}
	}
	memo(9001, "later", "901", 200)
	memo(9002, "earlier", "901", 100)
	memo(9003, "a-comment", "901", 150)
	memo(9004, "song", "902", 300)
	memo(9005, "unplaced", "NULL", 400)
	exec("INSERT INTO memo_relation (memo_id, related_memo_id, type) VALUES (9003, 9002, 'COMMENT')")

	setSchemaVersion(ctx, t, ts, "26.11.1")
	require.NoError(t, ts.Migrate(ctx))

	spaces, err := ts.ListSpaces(ctx, &store.FindSpace{})
	require.NoError(t, err)
	keys := map[string]string{}
	for _, space := range spaces {
		keys[space.UID] = space.Key
	}
	require.Equal(t, map[string]string{"Haymakr": "haymakr", longUID: "s902"}, keys, "a short uid is lowercased; a long one falls back to s<id>")

	seq := func(uid string) *int32 {
		found, err := ts.GetMemo(ctx, &store.FindMemo{UID: &uid})
		require.NoError(t, err)
		return found.SpaceSeq
	}
	require.Equal(t, int32(1), *seq("earlier"), "numbered by creation time, not id")
	require.Equal(t, int32(2), *seq("later"))
	require.Nil(t, seq("a-comment"), "comments do not take a memo space_seq")
	require.Equal(t, int32(1), *seq("song"))
	require.Nil(t, seq("unplaced"))

	comment, err := ts.GetMemo(ctx, &store.FindMemo{UID: new("a-comment")})
	require.NoError(t, err)
	require.NotNil(t, comment.CommentSeq)
	require.Equal(t, int32(1), *comment.CommentSeq, "migration 26.11/02 numbers existing comments")

	// The counter continues after the backfill.
	hay, err := ts.GetSpace(ctx, &store.FindSpace{UID: new("Haymakr")})
	require.NoError(t, err)
	created, err := ts.CreateMemo(ctx, &store.Memo{UID: "after-backfill", CreatorID: user.ID, Content: "x", Visibility: store.Private, SpaceID: &hay.ID})
	require.NoError(t, err)
	require.Equal(t, int32(3), *created.SpaceSeq)
}

// TestMigrationCommentHandleBackfill upgrades a 26.11.2 database to 26.11.3: existing comments are
// numbered per parent memo by creation order and the per-parent counter continues afterwards.
func TestMigrationCommentHandleBackfill(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	t.Cleanup(func() { require.NoError(t, ts.Close()) })
	driver := getDriverFromEnv()
	user, err := createTestingHostUser(ctx, ts)
	require.NoError(t, err)

	dropCommentHandleSchema(ctx, t, ts)
	exec := func(statement string) {
		t.Helper()
		_, err := ts.GetDriver().GetDB().ExecContext(ctx, statement)
		require.NoError(t, err, statement)
	}
	exec("INSERT INTO space (id, uid, title, description, payload, handle_key, next_seq) VALUES (951, 'haymakr', 'Haymakr', '', '{}', 'haymakr', 3)")
	memo := func(id int, uid string, spaceID, spaceSeq string, createdTs int) {
		exec(fmt.Sprintf(
			"INSERT INTO memo (id, uid, creator_id, created_ts, content, visibility, payload, space_id, space_seq) VALUES (%d, '%s', %d, %d, '%s', 'PRIVATE', '{}', %s, %s)",
			id, uid, user.ID, createdTs, uid, spaceID, spaceSeq))
	}
	if driver == "mysql" {
		memo = func(id int, uid string, spaceID, spaceSeq string, createdTs int) {
			exec(fmt.Sprintf(
				"INSERT INTO memo (id, uid, creator_id, created_ts, content, visibility, payload, space_id, space_seq) VALUES (%d, '%s', %d, FROM_UNIXTIME(%d), '%s', 'PRIVATE', '{}', %s, %s)",
				id, uid, user.ID, createdTs, uid, spaceID, spaceSeq))
		}
	}
	memo(9501, "parent-a", "951", "1", 100)
	memo(9502, "parent-b", "951", "2", 110)
	memo(9503, "comment-later", "951", "NULL", 300)
	memo(9504, "comment-earlier", "951", "NULL", 200)
	memo(9505, "comment-on-b", "951", "NULL", 250)
	exec("INSERT INTO memo_relation (memo_id, related_memo_id, type) VALUES (9503, 9501, 'COMMENT'), (9504, 9501, 'COMMENT'), (9505, 9502, 'COMMENT')")

	setSchemaVersion(ctx, t, ts, "26.11.2")
	require.NoError(t, ts.Migrate(ctx))

	commentSeq := func(uid string) *int32 {
		found, err := ts.GetMemo(ctx, &store.FindMemo{UID: &uid})
		require.NoError(t, err)
		return found.CommentSeq
	}
	require.Nil(t, commentSeq("parent-a"))
	require.Equal(t, int32(1), *commentSeq("comment-earlier"), "numbered by creation time, not id")
	require.Equal(t, int32(2), *commentSeq("comment-later"))
	require.Equal(t, int32(1), *commentSeq("comment-on-b"), "each parent counts on its own")

	// The comment handle filter resolves backfilled comments.
	found, err := ts.ListMemos(ctx, &store.FindMemo{Filters: []string{`handle == "haymakr-1-2"`}})
	require.NoError(t, err)
	require.Len(t, found, 1)
	require.Equal(t, "comment-later", found[0].UID)

	// New comments continue the parent's counter after the backfill.
	parentA, err := ts.GetMemo(ctx, &store.FindMemo{UID: new("parent-a")})
	require.NoError(t, err)
	spaceID := int32(951)
	created, err := ts.CreateMemoComment(ctx, &store.Memo{
		UID: "comment-after-backfill", CreatorID: user.ID, Content: "x", Visibility: store.Private, SpaceID: &spaceID,
	}, parentA.ID, user.ID)
	require.NoError(t, err)
	require.NotNil(t, created.CommentSeq)
	require.Equal(t, int32(3), *created.CommentSeq)
}
