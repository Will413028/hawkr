//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres"
)

// buyer 7 與 store 7 是兩個不同的人：讚以 (kind, id) 為鍵，清理只動自己那一種身分。
func TestLikesAndCleanup_AreKeyedByActorKind(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	mustExec(t, ctx, pool, `INSERT INTO cms_post (id, seller_id, content) VALUES (1, 900, 'p')`)
	mustExec(t, ctx, pool, `INSERT INTO cms_comment (id, post_id, user_id, role) VALUES (5001, 1, 500, 1)`)
	mustExec(t, ctx, pool, `INSERT INTO cms_comment (id, post_id, user_id, role) VALUES (5002, 1, 7, 1)`)
	mustExec(t, ctx, pool, `INSERT INTO cms_comment (id, post_id, user_id, role) VALUES (5003, 1, 7, 2)`)
	mustExec(t, ctx, pool, `INSERT INTO cms_seller_forum (id, seller_id, user_id, content, role) VALUES (6001, 900, 7, 'f', 1)`)
	mustExec(t, ctx, pool, `INSERT INTO cms_seller_forum (id, seller_id, user_id, content, role) VALUES (6002, 900, 7, 'f', 2)`)

	comments := postgres.NewCommentRepository(pool)
	posts := postgres.NewPostRepository(pool)
	forum := postgres.NewForumRepository(pool)

	require.NoError(t, comments.CreateLike(ctx, 5001, domain.ActorKindBuyer, 7))
	require.NoError(t, comments.CreateLike(ctx, 5001, domain.ActorKindStore, 7))
	require.NoError(t, posts.CreateLike(ctx, 1, domain.ActorKindBuyer, 7))

	// 同一個 (comment, kind, id) 重複寫入是 no-op，不是另一列。
	require.NoError(t, comments.CreateLike(ctx, 5001, domain.ActorKindStore, 7))
	assertColumnValue(t, ctx, pool, `SELECT COUNT(*) FROM cms_comment_like WHERE comment_id = 5001`, 2)

	_, err := comments.GetLike(ctx, 5001, domain.ActorKindStore, 7)
	require.NoError(t, err)
	liked, err := comments.BatchGetLikedCommentIDs(ctx, []int64{5001}, domain.ActorKindStore, 8)
	require.NoError(t, err)
	require.Empty(t, liked, "store 8 沒有按過讚")

	// buyer.deleted：只清 buyer 7 的讚與留言，store 7 的不動。
	require.NoError(t, comments.DeleteLikesByActor(ctx, domain.ActorKindBuyer, 7))
	require.NoError(t, posts.DeleteLikesByActor(ctx, domain.ActorKindBuyer, 7))
	require.NoError(t, comments.SoftDeleteByActor(ctx, domain.ActorKindBuyer, 7))
	require.NoError(t, forum.SoftDeleteByActor(ctx, domain.ActorKindBuyer, 7))

	_, err = comments.GetLike(ctx, 5001, domain.ActorKindBuyer, 7)
	require.ErrorIs(t, err, postgres.ErrNotFound)
	_, err = comments.GetLike(ctx, 5001, domain.ActorKindStore, 7)
	require.NoError(t, err, "store 7 的讚不屬於被刪除的 buyer 7")
	assertColumnValue(t, ctx, pool, `SELECT COUNT(*) FROM cms_post_like`, 0)
	assertColumnValue(t, ctx, pool, `SELECT COUNT(*) FROM cms_comment WHERE id = 5002 AND deleted_at IS NOT NULL`, 1)
	assertColumnValue(t, ctx, pool, `SELECT COUNT(*) FROM cms_comment WHERE id = 5003 AND deleted_at IS NULL`, 1)
	assertColumnValue(t, ctx, pool, `SELECT COUNT(*) FROM cms_seller_forum WHERE id = 6001 AND deleted_at IS NOT NULL`, 1)
	assertColumnValue(t, ctx, pool, `SELECT COUNT(*) FROM cms_seller_forum WHERE id = 6002 AND deleted_at IS NULL`, 1)

	// seller.deleted：只清 store 7 的留言讚。
	require.NoError(t, comments.DeleteLikesByActor(ctx, domain.ActorKindStore, 7))
	assertColumnValue(t, ctx, pool, `SELECT COUNT(*) FROM cms_comment_like`, 0)
}

// 不合法的身分種類在資料庫層就寫不進去：role 沒有預設值、只收 1／2；貼文讚只收 buyer。
func TestActorKindConstraints_RejectInvalidStates(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	mustExec(t, ctx, pool, `INSERT INTO cms_post (id, seller_id, content) VALUES (1, 900, 'p')`)
	mustExec(t, ctx, pool, `INSERT INTO cms_comment (id, post_id, user_id, role) VALUES (5001, 1, 7, 1)`)

	for _, sql := range []string{
		`INSERT INTO cms_comment (id, post_id, user_id) VALUES (5002, 1, 7)`,
		`INSERT INTO cms_comment (id, post_id, user_id, role) VALUES (5003, 1, 7, 0)`,
		`INSERT INTO cms_comment (id, post_id, user_id, role) VALUES (5004, 1, 7, 3)`,
		`INSERT INTO cms_seller_forum (id, seller_id, user_id, content) VALUES (6001, 900, 7, 'f')`,
		`INSERT INTO cms_seller_forum (id, seller_id, user_id, content, role) VALUES (6002, 900, 7, 'f', 3)`,
		`INSERT INTO cms_post_like (post_id, user_id, actor_kind) VALUES (1, 7, 2)`,
		`INSERT INTO cms_comment_like (comment_id, user_id, actor_kind) VALUES (5001, 7, 3)`,
	} {
		_, err := pool.Exec(ctx, sql)
		require.Error(t, err, "must be rejected: %s", sql)
	}
	mustExec(t, ctx, pool, `INSERT INTO cms_post_like (post_id, user_id, actor_kind) VALUES (1, 7, 1)`)
}
