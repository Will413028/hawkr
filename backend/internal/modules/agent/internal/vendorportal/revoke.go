package vendorportal

import (
	"context"
	"errors"

	"github.com/Will413028/hawkr/backend/internal/modules/agent/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/agent/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// RevokeProspect 撤銷一個 prospect 的全部邀請（記下原因與時間）與它們底下的全部 session。它是 seller 的封存事件的 handler
// 內容，必須在發布方的 transaction 內執行（transaction 外呼叫會讓邀請與 prospect 的狀態不一致，直接拒絕）。沒有邀請時是
// no-op；重送是冪等的（已撤銷的邀請不改原因與時間）。
func (s *Service) RevokeProspect(ctx context.Context, prospectID int64, reason model.RevokeReason) error {
	if _, ok := db.TxFrom(ctx); !ok {
		return errors.New("agent: 撤銷邀請必須在 transaction 內")
	}
	if err := requirePositive("ProspectID", prospectID); err != nil {
		return err
	}
	q := store.New(s.d.DB.Conn(ctx))
	now := s.d.Now().UTC()
	r := string(reason)
	if _, err := q.RevokeInvitesByProspect(ctx, store.RevokeInvitesByProspectParams{Now: &now, Reason: &r, ProspectID: prospectID}); err != nil {
		return err
	}
	_, err := q.RevokeSessionsByProspect(ctx, store.RevokeSessionsByProspectParams{Now: &now, ProspectID: prospectID})
	return err
}

// RepointProspect 把輸家（source）prospect 的全部邀請改掛贏家（target）：MDM 的 survivorship，引用一律解析到保留的那筆。
// email 快照、secret、session 與驗證紀錄都不動，也不撤銷；rfq_recipient_id 是 UNIQUE，不會與贏家既有的邀請衝突。之後贏家
// 封存才撤銷。seller 的 Merge 已把輸家封存，所以輸家身上不會再有新的邀請（Ensure 對非進行中的 prospect 一律拒絕）。必須在
// 發布方的 transaction 內執行；沒有邀請是 no-op，重送冪等。
func (s *Service) RepointProspect(ctx context.Context, sourceID, targetID int64) error {
	if _, ok := db.TxFrom(ctx); !ok {
		return errors.New("agent: 改掛邀請必須在 transaction 內")
	}
	if err := requirePositive("SourceProspectID", sourceID); err != nil {
		return err
	}
	if err := requirePositive("TargetProspectID", targetID); err != nil {
		return err
	}
	if sourceID == targetID {
		return nil
	}
	_, err := store.New(s.d.DB.Conn(ctx)).RepointInvitesToProspect(ctx, store.RepointInvitesToProspectParams{
		TargetProspectID: targetID, SourceProspectID: sourceID, Now: s.d.Now().UTC(),
	})
	return err
}
