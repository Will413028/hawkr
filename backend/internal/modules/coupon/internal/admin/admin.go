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

// Deps 是 Service 的依賴。
type Deps struct {
	DB       *db.DB
	Audit    Audit
	Campaign *campaign.Service
	Voucher  *voucher.Service
}

// Service 是後台的寫入。
type Service struct{ d Deps }

// New 建立 Service。
func New(d Deps) *Service { return &Service{d: d} }

func (s *Service) record(ctx context.Context, actor adminaudit.Actor, targetType, targetID string, detail map[string]any) error {
	return s.d.Audit.Record(ctx, adminaudit.Entry{Actor: actor, TargetType: targetType, TargetID: targetID, Detail: detail})
}

// CreateCampaign 以 actor 為發行者建立平台活動（DRAFT）；in.Issuer 被覆蓋。
func (s *Service) CreateCampaign(ctx context.Context, actor adminaudit.Actor, in campaign.CreateInput) (model.Campaign, error) {
	in.Issuer = model.Issuer{AdminID: actor.AdminID}
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

// PublishCampaign 把 DRAFT 的活動發布成 ACTIVE。
func (s *Service) PublishCampaign(ctx context.Context, actor adminaudit.Actor, id uuid.UUID) error {
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		if err := s.d.Campaign.Publish(ctx, id); err != nil {
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

// Issue 對平台的 DESIGNATED 活動指定發放；in.AdminID 被覆蓋。
func (s *Service) Issue(ctx context.Context, actor adminaudit.Actor, in voucher.IssueInput) (voucher.IssueResult, error) {
	in.AdminID = actor.AdminID
	var out voucher.IssueResult
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
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
