package post

import (
	"context"
	"fmt"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/model"
)

// Mine 是店主讀自己的貼文（任何狀態，不含刪除）：店家要有 model.StoreReadCapability；別家的貼文與不存在一樣回 NotFound。
func (s *Service) Mine(ctx context.Context, st seller.Store, id uuid.UUID) (model.Post, error) {
	if err := seller.Authorize(st, model.StoreReadCapability); err != nil {
		return model.Post{}, err
	}
	return s.Own(ctx, st.StoreID, id)
}

// ListMine 是店主自己的貼文（含草稿與下架）：店家要有 model.StoreReadCapability；status 為空字串不篩選。
func (s *Service) ListMine(ctx context.Context, st seller.Store, status model.PostStatus, after *model.OwnPostCursor, size int32) (model.OwnPostPage, error) {
	if err := seller.Authorize(st, model.StoreReadCapability); err != nil {
		return model.OwnPostPage{}, err
	}
	return s.ListOwn(ctx, st.StoreID, status, after, size)
}

// Summary 是店主的動態數：貼文依狀態的篇數與待發布排程的篇數，由 DB 計數，兩個計數在同一個讀取快照；店家要有 model.StoreReadCapability。
func (s *Service) Summary(ctx context.Context, st seller.Store) (model.StoreSummary, error) {
	if err := seller.Authorize(st, model.StoreReadCapability); err != nil {
		return model.StoreSummary{}, err
	}
	out := model.StoreSummary{Posts: map[model.PostStatus]int64{}}
	for _, status := range model.PostStatusValues() {
		out.Posts[status] = 0
	}
	err := s.d.DB.InReadSnapshot(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		rows, err := q.CountStorePostsByStatus(ctx, st.StoreID)
		if err != nil {
			return fmt.Errorf("social：計算貼文篇數：%w", err)
		}
		for _, r := range rows {
			status := model.PostStatus(r.Status)
			if !validStatus(status) {
				return fmt.Errorf("social：貼文 status %q 不在值域內", r.Status)
			}
			out.Posts[status] = r.Posts
		}
		if out.Scheduled, err = q.CountPendingSchedulesOfStore(ctx, st.StoreID); err != nil {
			return fmt.Errorf("social：計算待發布排程：%w", err)
		}
		return nil
	})
	return out, err
}
