package coupon

import (
	"context"
	"strconv"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
)

// 後台的寫入：領域變更與 adminaudit.Record 在同一個 transaction，紀錄寫不進去整筆回滾（ADR 2026-11-14-platform-admin-audit-log）。
// 管理員身分來自 actor，不接受輸入帶；沒有管理員身分的 actor 由領域檢查與 adminaudit 擋下。

const (
	auditCampaign    = "coupon_campaign"
	auditVoucher     = "coupon_voucher"
	auditSellerQuota = "coupon_seller_quota"
)

func (m *Module) record(ctx context.Context, actor adminaudit.Actor, targetType, targetID string, detail map[string]any) error {
	return m.audit.Record(ctx, adminaudit.Entry{Actor: actor, TargetType: targetType, TargetID: targetID, Detail: detail})
}

// AdminCreateCampaign 以 actor 為發行者建立平台活動（DRAFT）；in.Issuer 被覆蓋。
func (m *Module) AdminCreateCampaign(ctx context.Context, actor adminaudit.Actor, in CreateCampaignInput) (Campaign, error) {
	in.Issuer = Issuer{AdminID: actor.AdminID}
	var out Campaign
	err := m.db.InTx(ctx, func(ctx context.Context) error {
		c, err := m.campaign.Create(ctx, in)
		if err != nil {
			return err
		}
		out = c
		return m.record(ctx, actor, auditCampaign, c.ID.String(), map[string]any{
			"title": c.Title, "target_type": string(c.TargetType), "store_id": c.StoreID, "total_limit": c.TotalLimit,
		})
	})
	return out, err
}

// AdminPublishCampaign 把 DRAFT 的活動發布成 ACTIVE。
func (m *Module) AdminPublishCampaign(ctx context.Context, actor adminaudit.Actor, id uuid.UUID) error {
	return m.db.InTx(ctx, func(ctx context.Context) error {
		if err := m.campaign.Publish(ctx, id); err != nil {
			return err
		}
		return m.record(ctx, actor, auditCampaign, id.String(), nil)
	})
}

// AdminEndCampaign 提前結束任何發行者的活動；in.AdminID 與 in.StoreID 被覆蓋（後台不受店家範圍限制）。
func (m *Module) AdminEndCampaign(ctx context.Context, actor adminaudit.Actor, in EndCampaignInput) (EndCampaignResult, error) {
	in.AdminID, in.StoreID = actor.AdminID, 0
	var out EndCampaignResult
	err := m.db.InTx(ctx, func(ctx context.Context) error {
		res, err := m.campaign.End(ctx, in)
		if err != nil {
			return err
		}
		out = res
		return m.record(ctx, actor, auditCampaign, in.CampaignID.String(), map[string]any{"voided_vouchers": res.Voided, "reason": in.Reason})
	})
	return out, err
}

// AdminIssue 對 DESIGNATED 活動指定發放；in.AdminID 被覆蓋。
func (m *Module) AdminIssue(ctx context.Context, actor adminaudit.Actor, in IssueInput) (IssueResult, error) {
	in.AdminID = actor.AdminID
	var out IssueResult
	err := m.db.InTx(ctx, func(ctx context.Context) error {
		res, err := m.voucher.Issue(ctx, in)
		if err != nil {
			return err
		}
		out = res
		return m.record(ctx, actor, auditCampaign, in.CampaignID.String(), map[string]any{
			"granted_buyers": len(in.BuyerIDs), "issued": res.Issued, "already_held": res.AlreadyHeld,
		})
	})
	return out, err
}

// AdminVoidVoucher 作廢單張券；in.AdminID 被覆蓋。
func (m *Module) AdminVoidVoucher(ctx context.Context, actor adminaudit.Actor, in VoidVoucherInput) error {
	in.AdminID = actor.AdminID
	return m.db.InTx(ctx, func(ctx context.Context) error {
		if err := m.voucher.Void(ctx, in); err != nil {
			return err
		}
		return m.record(ctx, actor, auditVoucher, in.VoucherID.String(), map[string]any{"reason": in.Reason})
	})
}

// AdminSetSellerQuota 設定店家同時進行的活動數上限。
func (m *Module) AdminSetSellerQuota(ctx context.Context, actor adminaudit.Actor, storeID int64, maxCount int32) error {
	return m.db.InTx(ctx, func(ctx context.Context) error {
		if err := m.campaign.SetQuota(ctx, storeID, maxCount); err != nil {
			return err
		}
		return m.record(ctx, actor, auditSellerQuota, strconv.FormatInt(storeID, 10), map[string]any{"max_count": maxCount})
	})
}

// AdminListCampaigns 列出平台與店家的活動（後台），不含補償活動。
func (m *Module) AdminListCampaigns(ctx context.Context, in AdminListInput) (AdminCampaignPage, error) {
	return m.read.ListCampaigns(ctx, in)
}

// AdminGetCampaign 讀任一個活動的明細（後台），補償活動也讀得到。
func (m *Module) AdminGetCampaign(ctx context.Context, id uuid.UUID) (CampaignDetail, error) {
	return m.read.GetCampaign(ctx, id)
}
