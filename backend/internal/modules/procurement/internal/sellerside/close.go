package sellerside

import (
	"context"
	"fmt"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/quote"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// Withdraw 撤回指定版本的正式回價（目標狀態的方法，不建收據）。reasonCode 去頭尾空白後最多 model.MaxReasonCodeRunes 個字，空字串表示沒填，只記錄、不解析。
// 該版已撤回時回目前的結案狀態、不改動（先於停止寫入與版本的檢查，重送不會被判成已提交）；回價不是這家店對這筆詢價的回價回 NOT_FOUND；
// 停止寫入（見套件說明）回 FAILED_PRECONDITION／RFQ_RECIPIENT_CLOSED；expectedVersion 不是目前的版本、或還沒有正式回價回 ABORTED／VERSION_CONFLICT（conflictTarget 是 seller_response）。
// expectedVersion 必填。
func (s Service) Withdraw(ctx context.Context, storeID int64, publicationID, responseID uuid.UUID, expectedVersion, reasonCodeRaw string) (model.ResponseClosure, error) {
	if err := s.check(); err != nil {
		return model.ResponseClosure{}, err
	}
	expected, err := ParseVersion("expected_seller_response_version", expectedVersion)
	if err != nil {
		return model.ResponseClosure{}, err
	}
	if expected == 0 {
		return model.ResponseClosure{}, apperr.InvalidInputOn([]string{"expected_seller_response_version"}, "expected_seller_response_version 不能是空的")
	}
	reason, err := reasonCode(reasonCodeRaw)
	if err != nil {
		return model.ResponseClosure{}, err
	}
	var out model.ResponseClosure
	err = s.DB.InTx(ctx, func(ctx context.Context) error {
		if err := s.guardWrite(ctx, storeID); err != nil {
			return err
		}
		q := store.New(s.DB.Conn(ctx))
		v, err := loadView(ctx, q, storeID, publicationID)
		if err != nil {
			return err
		}
		resp, err := s.lock(ctx, q, storeID, publicationID)
		if err != nil {
			return err
		}
		if resp.ID != responseID {
			return notFound()
		}
		if resp.WithdrawnVersion != nil && *resp.WithdrawnVersion == expected {
			out = closureOf(resp)
			return nil
		}
		now := s.Now()
		if err := checkWritable(ctx, q, resp, v, now); err != nil {
			return err
		}
		if resp.LatestVersion != expected {
			return quote.VersionConflict("你的報價已經變動，請重新載入", quote.TargetSellerResponse)
		}
		n, err := q.WithdrawSellerResponse(ctx, store.WithdrawSellerResponseParams{ID: resp.ID, Version: expected, ReasonCode: reason, Now: now})
		if err != nil {
			return fmt.Errorf("sellerside：撤回回價 %s：%w", resp.ID, err)
		}
		if n != 1 {
			return fmt.Errorf("sellerside：回價 %s 的版本在鎖內被改動", resp.ID)
		}
		resp.WithdrawnVersion = &expected
		out = closureOf(resp)
		return nil
	})
	return out, err
}

// Decline 婉拒一筆詢價，之後這家店不能再寫入（目標狀態的方法，不建收據）。reasonCode 同 Withdraw。已婉拒時回第一次的結案狀態、不改動（保留第一次的時間與原因）。
// 婉拒不受「買家已刪除、已有成交、回價期限已過」限制；已送出的正式回價保留，買家端不能再接受它。詢價不是發給這家店的回 NOT_FOUND。
func (s Service) Decline(ctx context.Context, storeID int64, publicationID uuid.UUID, reasonCodeRaw string) (model.ResponseClosure, error) {
	if err := s.check(); err != nil {
		return model.ResponseClosure{}, err
	}
	reason, err := reasonCode(reasonCodeRaw)
	if err != nil {
		return model.ResponseClosure{}, err
	}
	var out model.ResponseClosure
	err = s.DB.InTx(ctx, func(ctx context.Context) error {
		if err := s.guardWrite(ctx, storeID); err != nil {
			return err
		}
		q := store.New(s.DB.Conn(ctx))
		if _, err := loadView(ctx, q, storeID, publicationID); err != nil {
			return err
		}
		resp, err := s.ensureAndLock(ctx, q, storeID, publicationID)
		if err != nil {
			return err
		}
		if resp.WriteClosedAt == nil {
			now := s.Now()
			n, err := q.DeclineSellerResponse(ctx, store.DeclineSellerResponseParams{ID: resp.ID, ReasonCode: reason, Now: now})
			if err != nil {
				return fmt.Errorf("sellerside：婉拒詢價 %s：%w", publicationID, err)
			}
			if n != 1 {
				return fmt.Errorf("sellerside：回價 %s 的狀態在鎖內被改動", resp.ID)
			}
			declined := model.WriteClosedDeclined
			resp.WriteClosedAt, resp.WriteClosedReason = &now, &declined
		}
		out = closureOf(resp)
		return nil
	})
	return out, err
}
