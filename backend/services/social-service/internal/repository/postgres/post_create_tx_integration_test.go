//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
)

// #2773：hashtag 失敗時貼文必須一起 rollback。
// 這條只有真 DB 證得了——mock 無法證明交易邊界。改動前的行為是「API 回失敗，
// 但 cms_post 已提交」，使用者看到建立失敗卻在列表裡看到那篇貼文。
func TestCreatePostRollsBackWithSameTx(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewPostRepository(pool)
	const postID int64 = 9400000000000001
	imageAssetID := int64(555)

	_, err := repo.Create(ctx, postID, 2685914682, "會失敗的貼文", "posts/x.jpg",
		&imageAssetID, nil, domain.PostPublishStatusPublished,
		func(_ context.Context, _ gen.DBTX) error { return errors.New("hashtag boom") })
	require.ErrorContains(t, err, "hashtag boom", "Create 必須把 sameTx 的錯誤原樣回傳，而不是吞掉或換成別的錯誤")

	_, err = repo.GetByID(ctx, postID)
	require.ErrorIs(t, err, pgx.ErrNoRows, "sameTx 失敗時貼文列必須不存在——否則 API 回失敗但貼文已建立")
}
