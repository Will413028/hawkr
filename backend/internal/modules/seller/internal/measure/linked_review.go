package measure

import (
	"context"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// MarkLinkedAccountReviewed 把措施的一個關聯帳號標為已檢視：這一對（措施、帳號）此後不再出現在後台待辦 SHARED_BANK_ACCOUNT。
// 這一對必須是銀行帳號 fingerprint 命中的關聯帳號（與處分頁的關聯帳號同一份命中定義，BankFingerprintHitExists），措施或命中不存在回 NotFound。
// 冪等：已標過的不改原來的時間與管理員。不碰 measures，所以措施的版本不變。
func (s *Service) MarkLinkedAccountReviewed(ctx context.Context, in model.MarkLinkedAccountReviewedInput) error {
	if err := model.Positive(in.AdminID); err != nil {
		return err
	}
	if err := model.Positive(in.AccountID); err != nil {
		return err
	}
	q := store.New(s.d.DB.Conn(ctx))
	hit, err := q.BankFingerprintHitExists(ctx, store.BankFingerprintHitExistsParams{MeasureID: in.MeasureID, AccountID: in.AccountID})
	if err != nil {
		return err
	}
	if !hit {
		return apperr.NotFound("找不到這筆措施的這個關聯帳號")
	}
	_, err = q.MarkLinkedAccountReviewed(ctx, store.MarkLinkedAccountReviewedParams{
		MeasureID: in.MeasureID, AccountID: in.AccountID, Now: s.now(), AdminID: in.AdminID,
	})
	return err
}
