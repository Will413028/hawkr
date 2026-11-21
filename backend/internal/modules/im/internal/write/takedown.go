package write

import (
	"context"
	"fmt"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/live"
	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/contentkind"
)

// TakedownContentError 是內容下架的對象不成立：訊息存在但不是措施的店家寄的（買家寄的訊息走隱藏）。訂閱者回這個錯誤讓建立措施的 transaction 回滾。
type TakedownContentError struct{ ID uuid.UUID }

func (e *TakedownContentError) Error() string {
	return fmt.Sprintf("im：內容下架的對象不是措施的店家寄的訊息：%s", e.ID)
}

func chatMessageIDs(refs []seller.ContentRef) []uuid.UUID {
	var out []uuid.UUID
	for _, r := range refs {
		if r.Kind == contentkind.ChatMessage {
			out = append(out, r.ID)
		}
	}
	return out
}

// TakedownCreated 是 seller 建立內容下架措施時同 transaction 的訂閱者：鎖住措施裡的訊息列（依 id 升序）、核對存在的訊息是措施的店家寄的，
// 對每則發可見度改變的 NOTIFY（可見性由讀取述詞 join seller.pub_content_takedowns() 推導，im 不存複本）。已不存在的訊息成立、沒有東西可鎖；
// 已抹除的訊息仍在列上，照常核對與通知。必須在呼叫方的 transaction 內。
func (s *Service) TakedownCreated(ctx context.Context, e seller.MeasureCreated) error {
	ids := chatMessageIDs(e.Contents)
	if len(ids) == 0 {
		return nil
	}
	return s.lockAndNotify(ctx, ids, e.StoreID)
}

// TakedownRevoked 是 seller 撤銷措施時同 transaction 的訂閱者：鎖住訊息列並發 NOTIFY，讓訊息回到可見（同一則訊息被另一個生效中的措施下架時，讀取述詞仍遮住它）。
func (s *Service) TakedownRevoked(ctx context.Context, e seller.MeasureRevoked) error {
	ids := chatMessageIDs(e.Contents)
	if len(ids) == 0 {
		return nil
	}
	return s.lockAndNotify(ctx, ids, 0)
}

// lockAndNotify 的 storeID 為 0 時不核對歸屬。
func (s *Service) lockAndNotify(ctx context.Context, ids []uuid.UUID, storeID int64) error {
	q := s.q(ctx)
	rows, err := q.LockMessagesForTakedown(ctx, ids)
	if err != nil {
		return fmt.Errorf("im：鎖定被下架的訊息：%w", err)
	}
	if storeID > 0 {
		for _, r := range rows {
			if model.PartyKind(r.SenderKind) != model.PartyStore || r.StoreID != storeID {
				return &TakedownContentError{ID: r.ID}
			}
		}
	}
	for _, r := range rows {
		if err := q.Notify(ctx, live.Visible(r.ConversationID, r.BuyerID, r.StoreID, r.Seq).Encode()); err != nil {
			return err
		}
	}
	return nil
}
