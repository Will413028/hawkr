//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
)

// ListPosts 與 ListPostsByHashtagName 對真實 SQL 各驗一次整列內容。
//
// 這兩支長期沒有 integration 覆蓋（其餘四支整列查詢都有），是在 media_* 三欄的
// expand-contract 過程中補上的：那次它們的回傳型別被迫繞了一層獨立 Row struct，
// 而唯一會抓到「繞錯」的測試並不存在。三欄 DROP 之後那層已拆除，覆蓋留著——
// 缺口本來就與那次改動無關。
//
// 斷言刻意逐欄比對而不是只看 id：SELECT * 的展開少一欄、或日後有人把它改成明列
// 清單而漏一欄，回來的列 id 照樣是對的，錯的是內容——貼文的圖片、資產引用、發布
// 狀態、版本號其中之一變成零值，而清單畫面上沒有任何東西會說它不對。
func TestListPostsAndListByHashtagReturnEveryColumn(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewPostRepository(pool)
	tagRepo := postgres.NewHashtagRepository(pool)

	const (
		sellerID  int64 = 2685914683
		postID    int64 = 9310000000000001
		otherID   int64 = 9310000000000002
		hashtagID int64 = 9320000000000001
		assetID   int64 = 9330000000000001
		productID int64 = 9340000000000001
		tagName         = "生鮮"
	)

	assetIDVal, productIDVal := assetID, productID
	created, err := repo.Create(ctx, postID, sellerID, "帶標籤的貼文", "https://cdn.example/a.jpg",
		&assetIDVal, &productIDVal, domain.PostPublishStatusPublished,
		func(ctx context.Context, q gen.DBTX) error {
			hid, uerr := tagRepo.UpsertByName(ctx, q, hashtagID, tagName)
			if uerr != nil {
				return uerr
			}
			return tagRepo.InsertBinding(ctx, q, postID, hid, tagName, 0)
		})
	require.NoError(t, err)

	// 沒綁標籤的對照列：證明 ListByHashtag 的 JOIN 真的在篩，而不是兩支都回全表
	// ——那樣的話下面的逐欄比對會在一個恆真的集合上跑，看起來一樣綠。
	_, err = repo.Create(ctx, otherID, sellerID, "沒有標籤", "", nil, nil,
		domain.PostPublishStatusPublished, nil)
	require.NoError(t, err)

	requireSameRow := func(t *testing.T, what string, got *gen.CmsPost) {
		t.Helper()
		require.Equal(t, created.ID, got.ID, what)
		require.Equal(t, created.SellerID, got.SellerID, what)
		require.Equal(t, created.Content, got.Content, what)
		require.Equal(t, created.Image, got.Image, what)
		require.Equal(t, created.ProductID, got.ProductID, what)
		require.Equal(t, created.ImageAssetID, got.ImageAssetID,
			"%s：image_asset_id 是媒體引用的真相，掉了它 reconciler 會把這一列讀成沒有 desired", what)
		require.Equal(t, created.PublishStatus, got.PublishStatus, what)
		require.Equal(t, created.Version, got.Version, what)
		require.Equal(t, created.LikeCount, got.LikeCount, what)
		require.Equal(t, created.CommentCount, got.CommentCount, what)
		require.Equal(t, created.ViewCount, got.ViewCount, what)
		require.Equal(t, created.ShareCount, got.ShareCount, what)
		require.Equal(t, created.PublishedAt, got.PublishedAt, what)
		require.Equal(t, created.HiddenAt, got.HiddenAt, what)
		require.Equal(t, created.DeletedAt, got.DeletedAt, what)
		require.True(t, got.CreateTime.Valid, "%s：create_time 沒被讀回來", what)
		require.True(t, got.UpdateTime.Valid, "%s：update_time 沒被讀回來", what)
	}

	t.Run("ListPosts", func(t *testing.T) {
		rows, lerr := repo.List(ctx, sellerID, 50, 0)
		require.NoError(t, lerr)
		require.Len(t, rows, 2)

		var got *gen.CmsPost
		for _, r := range rows {
			if r.ID == postID {
				got = r
			}
		}
		require.NotNil(t, got, "ListPosts 沒回傳 id=%d", postID)
		requireSameRow(t, "ListPosts", got)
	})

	t.Run("ListPostsByHashtagName", func(t *testing.T) {
		rows, lerr := repo.ListByHashtag(ctx, tagName, 50, 0)
		require.NoError(t, lerr)
		require.Len(t, rows, 1, "只有一則貼文綁了 %s；回傳兩則表示 JOIN 沒在篩", tagName)
		requireSameRow(t, "ListPostsByHashtagName", rows[0])
	})
}
