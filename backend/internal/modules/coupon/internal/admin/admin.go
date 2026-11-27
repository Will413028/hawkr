// Package admin 是後台對優惠券的寫入：領域變更與 adminaudit.Record 在同一個 transaction，紀錄寫不進去整筆回滾（ADR 2026-11-14-platform-admin-audit-log）。
// 管理員身分來自 actor，不接受輸入帶；沒有管理員身分的 actor 由領域檢查與 adminaudit 擋下。
package admin

import (
	"context"
	"strconv"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/campaign"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/voucher"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

const (
	auditCampaign    = "coupon_campaign"
	auditVoucher     = "coupon_voucher"
	auditSellerQuota = "coupon_seller_quota"
)

// Audit 是後台動作紀錄（*adminaudit.Log）：Record 必須在呼叫方的 transaction 內。
type Audit interface {
	Record(ctx context.Context, e adminaudit.Entry) error
}

// Buyers 是買家的來源：指定名單與發放對象必須是存在的買家。Existing 回傳 ids 之中存在的（存活的 profile）；讀取失敗回錯誤，不當成不存在。
type Buyers interface {
	Existing(ctx context.Context, ids []int64) ([]int64, error)
}

// StoreState 是店家在建立限店活動時需要的狀態；停權不在這裡（停權只在領券與鎖券時看）。
type StoreState struct {
	Exists bool
	Closed bool
}

// StoreDirectory 是店家的來源：限店的平台活動的店家必須存在而且沒有關閉。店家不存在回 Exists=false；讀取失敗回錯誤。
type StoreDirectory interface {
	StoreState(ctx context.Context, storeID int64) (StoreState, error)
}

// Deps 是 Service 的依賴。
type Deps struct {
	DB        *db.DB
	Audit     Audit
	Campaign  *campaign.Service
	Voucher   *voucher.Service
	Buyers    Buyers
	Directory StoreDirectory
}

// Service 是後台的寫入。
type Service struct{ d Deps }

// New 建立 Service。
func New(d Deps) *Service { return &Service{d: d} }

func (s *Service) record(ctx context.Context, actor adminaudit.Actor, targetType, targetID string, detail map[string]any) error {
	return s.d.Audit.Record(ctx, adminaudit.Entry{Actor: actor, TargetType: targetType, TargetID: targetID, Detail: detail})
}

func uniqueBuyers(ctx context.Context, field string, raw []int64, limit int) (out []int64, index []int, err error) {
	if len(raw) > limit {
		return nil, nil, model.Invalid(ctx, field, model.InvalidBuyerCountInvalid)
	}
	seen := make(map[int64]struct{}, len(raw))
	out, index = make([]int64, 0, len(raw)), make([]int, 0, len(raw))
	for i, id := range raw {
		if id <= 0 {
			return nil, nil, model.Invalid(ctx, apperr.IndexedField(field, i), model.InvalidIDPositive)
		}
		if _, dup := seen[id]; !dup {
			seen[id] = struct{}{}
			out, index = append(out, id), append(index, i)
		}
	}
	return out, index, nil
}

func (s *Service) requireBuyers(ctx context.Context, field string, ids []int64, index []int) error {
	if len(ids) == 0 {
		return nil
	}
	existing, err := s.d.Buyers.Existing(ctx, ids)
	if err != nil {
		return err
	}
	have := make(map[int64]struct{}, len(existing))
	for _, id := range existing {
		have[id] = struct{}{}
	}
	for i, id := range ids {
		if _, ok := have[id]; !ok {
			return model.Invalid(ctx, apperr.IndexedField(field, index[i]), model.InvalidBuyerNotFound)
		}
	}
	return nil
}

func (s *Service) requireOpenStore(ctx context.Context, storeID int64) error {
	st, err := s.d.Directory.StoreState(ctx, storeID)
	if err != nil {
		return err
	}
	if !st.Exists {
		return model.Invalid(ctx, "StoreID", model.InvalidStoreNotFound)
	}
	if st.Closed {
		return model.Invalid(ctx, "StoreID", model.InvalidStoreClosed)
	}
	return nil
}

// CreateCampaign 以 actor 為發行者建立平台活動（DRAFT）；in.Issuer 被覆蓋。
// 寫入前檢查：限店的店家存在且沒有關閉；DESIGNATED 的名單都是存在的買家（其他資格帶名單由 campaign.Create 以自己的錯誤拒絕，不查買家）。
func (s *Service) CreateCampaign(ctx context.Context, actor adminaudit.Actor, in campaign.CreateInput) (model.Campaign, error) {
	in.Issuer = model.Issuer{AdminID: actor.AdminID}
	if in.StoreID > 0 {
		if err := s.requireOpenStore(ctx, in.StoreID); err != nil {
			return model.Campaign{}, err
		}
	}
	if in.TargetType == model.TargetDesignated && len(in.TargetBuyers) > 0 && len(in.TargetBuyers) <= model.MaxTargetBuyers {
		ids, index, err := uniqueBuyers(ctx, "TargetBuyers", in.TargetBuyers, model.MaxTargetBuyers)
		if err != nil {
			return model.Campaign{}, err
		}
		if err := s.requireBuyers(ctx, "TargetBuyers", ids, index); err != nil {
			return model.Campaign{}, err
		}
	}
	var out model.Campaign
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		c, err := s.d.Campaign.Create(ctx, in)
		if err != nil {
			return err
		}
		out = c
		return s.record(ctx, actor, auditCampaign, c.ID.String(), map[string]any{
			"title": c.Title, "target_type": string(c.TargetType), "store_id": c.StoreID, "total_limit": c.TotalLimit,
		})
	})
	return out, err
}

// PublishCampaign 把平台發的 DRAFT 活動發布成 ACTIVE（店家的與補償活動回 NotFound）。
func (s *Service) PublishCampaign(ctx context.Context, actor adminaudit.Actor, id uuid.UUID) error {
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		if err := s.d.Campaign.PublishPlatform(ctx, id); err != nil {
			return err
		}
		return s.record(ctx, actor, auditCampaign, id.String(), nil)
	})
}

// EndCampaign 提前結束平台發的非補償活動（店家的與補償活動回 NotFound）；in.AdminID、in.StoreID 與 in.PlatformOnly 被覆蓋。
// 稽核紀錄的 reason 與活動操作紀錄同一個（去空白後）。
func (s *Service) EndCampaign(ctx context.Context, actor adminaudit.Actor, in campaign.EndInput) (campaign.EndResult, error) {
	reason, err := model.NormalizeReason(ctx, in.Reason)
	if err != nil {
		return campaign.EndResult{}, err
	}
	in.AdminID, in.StoreID, in.PlatformOnly, in.Reason = actor.AdminID, 0, true, reason
	var out campaign.EndResult
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		res, err := s.d.Campaign.End(ctx, in)
		if err != nil {
			return err
		}
		out = res
		return s.record(ctx, actor, auditCampaign, in.CampaignID.String(), map[string]any{"voided_vouchers": res.Voided, "reason": reason})
	})
	return out, err
}

// Issue 對平台的 DESIGNATED 活動指定發放；in.AdminID 被覆蓋。in.BuyerIDs 是請求的原始名單：先限制原始個數，去重後檢查 1 到 MaxIssueBatch 位，
// 再確認每位都是存在的買家（指出第一個不存在的在請求裡的位置）；稽核紀錄的 granted_buyers 是去重後的人數。
func (s *Service) Issue(ctx context.Context, actor adminaudit.Actor, in voucher.IssueInput) (voucher.IssueResult, error) {
	in.AdminID = actor.AdminID
	buyers, index, err := uniqueBuyers(ctx, "BuyerIDs", in.BuyerIDs, model.MaxIssueRequestBuyers)
	if err != nil {
		return voucher.IssueResult{}, err
	}
	if len(buyers) == 0 || len(buyers) > model.MaxIssueBatch {
		return voucher.IssueResult{}, model.Invalid(ctx, "BuyerIDs", model.InvalidBuyerCountInvalid)
	}
	if err := s.requireBuyers(ctx, "BuyerIDs", buyers, index); err != nil {
		return voucher.IssueResult{}, err
	}
	in.BuyerIDs = buyers
	var out voucher.IssueResult
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		res, err := s.d.Voucher.Issue(ctx, in)
		if err != nil {
			return err
		}
		out = res
		return s.record(ctx, actor, auditCampaign, in.CampaignID.String(), map[string]any{
			"granted_buyers": len(in.BuyerIDs), "issued": res.Issued, "already_held": res.AlreadyHeld,
		})
	})
	return out, err
}

// VoidVoucher 作廢單張券；in.AdminID 被覆蓋。
func (s *Service) VoidVoucher(ctx context.Context, actor adminaudit.Actor, in voucher.VoidInput) error {
	in.AdminID = actor.AdminID
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		if err := s.d.Voucher.Void(ctx, in); err != nil {
			return err
		}
		return s.record(ctx, actor, auditVoucher, in.VoucherID.String(), map[string]any{"reason": in.Reason})
	})
}

// SetSellerQuota 設定店家同時進行的活動數上限。
func (s *Service) SetSellerQuota(ctx context.Context, actor adminaudit.Actor, storeID int64, maxCount int32) error {
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		if err := s.d.Campaign.SetQuota(ctx, storeID, maxCount); err != nil {
			return err
		}
		return s.record(ctx, actor, auditSellerQuota, strconv.FormatInt(storeID, 10), map[string]any{"max_count": maxCount})
	})
}
