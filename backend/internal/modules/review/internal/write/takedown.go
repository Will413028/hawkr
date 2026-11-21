package write

import (
	"context"
	"fmt"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/contentkind"
)

// TakedownContentError 是內容下架的對象不成立：評價存在但不是措施的店家的（回覆的作者是評價所屬的店家）。訂閱者回這個錯誤讓建立措施的 transaction 回滾。
type TakedownContentError struct{ ID uuid.UUID }

func (e *TakedownContentError) Error() string {
	return fmt.Sprintf("review：內容下架的對象不是措施的店家的評價回覆：%s", e.ID)
}

func replyReviewIDs(refs []seller.ContentRef) []uuid.UUID {
	var out []uuid.UUID
	for _, r := range refs {
		if r.Kind == contentkind.ReviewReply {
			out = append(out, r.ID)
		}
	}
	return out
}

// TakedownCreated 是 seller 建立內容下架措施時同 transaction 的訂閱者：鎖住措施裡的評價列（依 id 升序）並核對評價屬於措施的店家。
// 可見性由讀取（TakenDownReplyIDs）join seller.pub_content_takedowns() 推導，review 不存複本；回覆不進評分彙總，沒有通知要發。
// 已不存在的評價成立、沒有東西可鎖。必須在呼叫方的 transaction 內。
func (s *Service) TakedownCreated(ctx context.Context, e seller.MeasureCreated) error {
	ids := replyReviewIDs(e.Contents)
	if len(ids) == 0 {
		return nil
	}
	rows, err := s.q(ctx).LockReviewsForTakedown(ctx, ids)
	if err != nil {
		return fmt.Errorf("review：鎖定被下架回覆的評價：%w", err)
	}
	for _, r := range rows {
		if r.StoreID != e.StoreID {
			return &TakedownContentError{ID: r.ID}
		}
	}
	return nil
}

// TakedownRevoked 是 seller 撤銷措施時同 transaction 的訂閱者：鎖住評價列，與建立措施、隱藏與還原用同一個鎖順序；撤銷後讀取立即不再遮回覆。
func (s *Service) TakedownRevoked(ctx context.Context, e seller.MeasureRevoked) error {
	ids := replyReviewIDs(e.Contents)
	if len(ids) == 0 {
		return nil
	}
	if _, err := s.q(ctx).LockReviewsForTakedown(ctx, ids); err != nil {
		return fmt.Errorf("review：鎖定被撤銷下架回覆的評價：%w", err)
	}
	return nil
}
