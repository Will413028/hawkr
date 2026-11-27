package coupon

import (
	"context"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
)

// AdminCreateCampaign 以 actor 為發行者建立平台活動（DRAFT）；in.Issuer 被覆蓋。
func (m *Module) AdminCreateCampaign(ctx context.Context, actor adminaudit.Actor, in CreateCampaignInput) (Campaign, error) {
	return m.admin.CreateCampaign(ctx, actor, in)
}

// AdminPublishCampaign 把 DRAFT 的活動發布成 ACTIVE。
func (m *Module) AdminPublishCampaign(ctx context.Context, actor adminaudit.Actor, id uuid.UUID) error {
	return m.admin.PublishCampaign(ctx, actor, id)
}

// AdminEndCampaign 提前結束平台發的非補償活動；店家的與補償活動回 NotFound。
func (m *Module) AdminEndCampaign(ctx context.Context, actor adminaudit.Actor, in EndCampaignInput) (EndCampaignResult, error) {
	return m.admin.EndCampaign(ctx, actor, in)
}

// AdminIssue 對平台的 DESIGNATED 活動指定發放；店家的活動回 NotFound。
func (m *Module) AdminIssue(ctx context.Context, actor adminaudit.Actor, in IssueInput) (IssueResult, error) {
	return m.admin.Issue(ctx, actor, in)
}

// AdminVoidVoucher 作廢單張券。
func (m *Module) AdminVoidVoucher(ctx context.Context, actor adminaudit.Actor, in VoidVoucherInput) error {
	return m.admin.VoidVoucher(ctx, actor, in)
}

// AdminSetSellerQuota 設定店家同時進行的活動數上限。
func (m *Module) AdminSetSellerQuota(ctx context.Context, actor adminaudit.Actor, storeID int64, maxCount int32) error {
	return m.admin.SetSellerQuota(ctx, actor, storeID, maxCount)
}

// AdminListCampaigns 列出平台與店家的活動（後台），不含補償活動。
func (m *Module) AdminListCampaigns(ctx context.Context, in AdminListInput) (AdminCampaignPage, error) {
	return m.read.ListCampaigns(ctx, in)
}

// AdminGetCampaign 讀任一個活動的明細（後台），補償活動也讀得到。
func (m *Module) AdminGetCampaign(ctx context.Context, id uuid.UUID) (CampaignDetail, error) {
	return m.read.GetCampaign(ctx, id)
}
