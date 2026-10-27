//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres"
	"github.com/stretchr/testify/require"
)

// #2757：SoftDelete 的 affected rows 是授權判定的一部分，不是統計值。
// 述詞 `WHERE id = $1 AND seller_id = $2 AND deleted_at IS NULL` 的三種落空
// 各有不同的業務意義，但對呼叫端是同一個結論：這次刪除沒有發生，副作用一個都不能做。
func TestSoftDeleteAffectedRows(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewPostRepository(pool)

	const ownerID int64 = 2685914682
	const strangerID int64 = 2685914683
	const postID int64 = 9100000000000001

	_, err := repo.Create(ctx, postID, ownerID, "owner 的貼文", "", nil, nil,
		domain.PostPublishStatusPublished, nil)
	require.NoError(t, err)

	t.Run("wrong seller affects zero rows", func(t *testing.T) {
		rows, err := repo.SoftDelete(ctx, postID, strangerID)
		require.NoError(t, err, "跨店刪除不是錯誤，是 0 列——錯誤與 0 列必須分得開")
		require.EqualValues(t, 0, rows)

		post, err := repo.GetByID(ctx, postID)
		require.NoError(t, err, "受害貼文必須還在")
		require.Nil(t, post.DeletedAt)
	})

	t.Run("owner affects exactly one row", func(t *testing.T) {
		rows, err := repo.SoftDelete(ctx, postID, ownerID)
		require.NoError(t, err)
		require.EqualValues(t, 1, rows)
	})

	t.Run("second delete affects zero rows", func(t *testing.T) {
		rows, err := repo.SoftDelete(ctx, postID, ownerID)
		require.NoError(t, err,
			"重複刪除不是錯誤，是 0 列——並發雙擊會走到這裡，呼叫端靠 0 判斷不要重扣 counter")
		require.EqualValues(t, 0, rows)
	})

	t.Run("missing post affects zero rows", func(t *testing.T) {
		rows, err := repo.SoftDelete(ctx, 9100000000000999, ownerID)
		require.NoError(t, err)
		require.EqualValues(t, 0, rows)
	})
}
