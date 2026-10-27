// Package readcache 在可見性改變的寫入的同一個 transaction 排入 website 的快取失效（platform/revalidate，舊碼的 BustAsync）：發布、
// 下架、編輯、刪除、隱藏、還原。tag 是貼文（post:<id>，website 展開到貼文列表）加店家的兩個 tag（店面頁）。不送 hashtag: tag：
// website 把它展開成商品 hashtag 的選單快取，貼文的熱門話題在貼文列表的快取裡。
package readcache

import (
	"context"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/revalidate"
)

// Stores 是讀店家（取 store_code）的 seller 公開 API。
type Stores interface {
	Store(ctx context.Context, storeID int64) (seller.Store, error)
}

// maxPostsPerJob 是一個失效 job 帶的貼文 tag 數（website 的請求上限 64 KiB）。
const maxPostsPerJob = 200

// Cache 排入失效 job。
type Cache struct {
	Jobs   jobs.Enqueuer
	Stores Stores
}

// BustPost 排入一篇貼文的失效；必須在 transaction 內呼叫（job 與寫入一起 commit 或 rollback）。
func (c Cache) BustPost(ctx context.Context, postID uuid.UUID, storeID int64) error {
	tags := []string{revalidate.PostTag(postID)}
	store, err := c.Stores.Store(ctx, storeID)
	switch {
	case err == nil:
		tags = append(tags, revalidate.StoreTags(store.StoreID, store.StoreCode)...)
	case apperr.KindOf(err) == apperr.KindNotFound:
	default:
		return err
	}
	return c.Jobs.Enqueue(ctx, revalidate.Args{Tags: tags}, nil)
}

// BustStore 排入一家店的店面頁失效（討論區的隱藏與還原）；必須在 transaction 內呼叫。
func (c Cache) BustStore(ctx context.Context, storeID int64) error {
	store, err := c.Stores.Store(ctx, storeID)
	switch {
	case err == nil:
		return c.Jobs.Enqueue(ctx, revalidate.Args{Tags: revalidate.StoreTags(store.StoreID, store.StoreCode)}, nil)
	case apperr.KindOf(err) == apperr.KindNotFound:
		return nil
	default:
		return err
	}
}

// BustPosts 排入多篇貼文的失效（清除 job 影響的貼文），依 maxPostsPerJob 分成幾個 job。
func (c Cache) BustPosts(ctx context.Context, postIDs []uuid.UUID) error {
	for len(postIDs) > 0 {
		n := min(len(postIDs), maxPostsPerJob)
		tags := make([]string, n)
		for i, id := range postIDs[:n] {
			tags[i] = revalidate.PostTag(id)
		}
		if err := c.Jobs.Enqueue(ctx, revalidate.Args{Tags: tags}, nil); err != nil {
			return err
		}
		postIDs = postIDs[n:]
	}
	return nil
}
