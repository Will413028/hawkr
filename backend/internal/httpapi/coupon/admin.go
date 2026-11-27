package coupon

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/coupon/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/coupon/v1alpha1/couponv1alpha1connect"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/coupon/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
)

// AdminModule 是後台用的 coupon 模組（*coupon.Module）。寫入方法在領域變更的同一個 transaction 內寫後台動作紀錄，所以呼叫者帶 actor。
type AdminModule interface {
	AdminCreateCampaign(ctx context.Context, actor adminaudit.Actor, in coupon.CreateCampaignInput) (coupon.Campaign, error)
	AdminPublishCampaign(ctx context.Context, actor adminaudit.Actor, id uuid.UUID) error
	AdminEndCampaign(ctx context.Context, actor adminaudit.Actor, in coupon.EndCampaignInput) (coupon.EndCampaignResult, error)
	AdminIssue(ctx context.Context, actor adminaudit.Actor, in coupon.IssueInput) (coupon.IssueResult, error)
	AdminVoidVoucher(ctx context.Context, actor adminaudit.Actor, in coupon.VoidVoucherInput) error
	AdminSetSellerQuota(ctx context.Context, actor adminaudit.Actor, storeID int64, maxCount int32) error
	AdminListCampaigns(ctx context.Context, in coupon.AdminListInput) (coupon.AdminCampaignPage, error)
	AdminGetCampaign(ctx context.Context, id uuid.UUID) (coupon.CampaignDetail, error)
	ListVouchers(ctx context.Context, in coupon.VoucherListInput) (coupon.VoucherPage, error)
}

// Admin 實作 CouponAdminService。授權（只有管理員、逐方法的權限、audited）由 rpc 的 interceptor 與 adminauth 的宣告表檢查；
// 這裡只做欄位的轉換與錯誤對應，規則、存在檢查與稽核紀錄在 internal/modules/coupon。
type Admin struct {
	m        AdminModule
	reasonOf func(error) coupon.Reason
}

var _ couponv1alpha1connect.CouponAdminServiceHandler = (*Admin)(nil)

// NewAdmin 建立後台的 handler。
func NewAdmin(m AdminModule) *Admin {
	return &Admin{m: m, reasonOf: coupon.ReasonOf}
}

var (
	adminCreatePaths = rpc.FieldPaths{
		"Title":             "campaign.title",
		"DiscountAmount":    "campaign.cash.amount",
		"DiscountPercent":   "campaign.percent.percent",
		"MaxDiscountAmount": "campaign.percent.max_amount",
		"DiscountType":      "campaign.cash",
		"ThresholdAmount":   "campaign.threshold_amount",
		"TargetType":        "campaign.target_type",
		"StartAt":           "campaign.start_time",
		"EndAt":             "campaign.end_time",
		"TotalLimit":        "campaign.total_issue_limit",
		"StoreID":           "campaign.store_id",
		"TargetBuyers":      "campaign.designated_buyer_ids",
		"TargetBuyers[]":    "campaign.designated_buyer_ids[]",
	}
	adminEndPaths   = rpc.FieldPaths{"Reason": "reason"}
	adminGrantPaths = rpc.FieldPaths{"BuyerIDs": "buyer_ids", "BuyerIDs[]": "buyer_ids[]"}
	adminVoidPaths  = rpc.FieldPaths{"Reason": "reason"}
	adminQuotaPaths = rpc.FieldPaths{"StoreID": "store_id", "MaxCount": "max_active_campaigns"}
	adminListPaths  = rpc.FieldPaths{"StoreID": "issuer.store_id"}
)

func (h *Admin) failure(err error, paths rpc.FieldPaths) error {
	return failure(h.reasonOf, err, paths)
}

func (h *Admin) actor(ctx context.Context, procedure string) (adminaudit.Actor, error) {
	admin, err := rpc.RequireAdmin(ctx)
	if err != nil {
		return adminaudit.Actor{}, err
	}
	return adminaudit.Actor{AdminID: admin, Action: procedure}, nil
}

func (h *Admin) CreateCampaign(ctx context.Context, req *connect.Request[adminpb.CreateCampaignRequest]) (*connect.Response[adminpb.CreateCampaignResponse], error) {
	actor, err := h.actor(ctx, req.Spec().Procedure)
	if err != nil {
		return nil, err
	}
	in := req.Msg.GetCampaign()
	if in == nil {
		return nil, rpc.RequiredField("campaign", "campaign 必填")
	}
	if len(in.GetDesignatedBuyerIds()) > coupon.MaxTargetBuyers {
		return nil, rpc.RequiredField("campaign.designated_buyer_ids", fmt.Sprintf("指定名單最多 %d 位買家", coupon.MaxTargetBuyers))
	}
	buyers, err := parseBuyerIDs(in.GetDesignatedBuyerIds())
	if err != nil {
		return nil, err
	}
	spec, err := adminSpecOf(in)
	if err != nil {
		return nil, err
	}
	create := coupon.CreateCampaignInput{
		CampaignSpec: spec, TargetType: rpc.EnumFromProto[coupon.TargetType](in.GetTargetType()), TargetBuyers: buyers,
	}
	if in.StoreId != nil {
		if create.StoreID, err = rpc.ParseID("campaign.store_id", in.GetStoreId()); err != nil {
			return nil, err
		}
	}
	if in.TotalIssueLimit != nil {
		if in.GetTotalIssueLimit() < 1 {
			return nil, rpc.RequiredField("campaign.total_issue_limit", "total_issue_limit 至少是 1；不設表示不限量")
		}
		create.TotalLimit = in.GetTotalIssueLimit()
	}
	c, err := h.m.AdminCreateCampaign(ctx, actor, create)
	if err != nil {
		return nil, h.failure(err, adminCreatePaths)
	}
	return connect.NewResponse(&adminpb.CreateCampaignResponse{Campaign: toAdminCampaign(c, false)}), nil
}

func adminSpecOf(in *adminpb.AdminCampaignInput) (coupon.CampaignSpec, error) {
	spec := coupon.CampaignSpec{Title: in.GetTitle(), ThresholdAmount: in.GetThresholdAmount()}
	switch d := in.GetDiscount().(type) {
	case *adminpb.AdminCampaignInput_Cash:
		spec.DiscountType, spec.DiscountAmount = coupon.DiscountCash, d.Cash.GetAmount()
	case *adminpb.AdminCampaignInput_Percent:
		spec.DiscountType, spec.DiscountPercent, spec.MaxDiscountAmount = coupon.DiscountPercent, d.Percent.GetPercent(), d.Percent.GetMaxAmount()
	default:
		return spec, rpc.RequiredField("campaign.cash", "cash 或 percent 必須設定其中一個")
	}
	if in.GetStartTime() == nil {
		return spec, rpc.RequiredField("campaign.start_time", "start_time 必填")
	}
	if in.GetEndTime() == nil {
		return spec, rpc.RequiredField("campaign.end_time", "end_time 必填")
	}
	spec.StartAt, spec.EndAt = in.GetStartTime().AsTime(), in.GetEndTime().AsTime()
	return spec, nil
}

func (h *Admin) PublishCampaign(ctx context.Context, req *connect.Request[adminpb.PublishCampaignRequest]) (*connect.Response[adminpb.PublishCampaignResponse], error) {
	actor, err := h.actor(ctx, req.Spec().Procedure)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseUUID("campaign_id", req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	if err := h.m.AdminPublishCampaign(ctx, actor, id); err != nil {
		return nil, h.failure(err, noFieldPaths)
	}
	d, err := h.m.AdminGetCampaign(ctx, id)
	if err != nil {
		return nil, h.failure(err, noFieldPaths)
	}
	return connect.NewResponse(&adminpb.PublishCampaignResponse{Campaign: toAdminCampaign(d.Campaign, d.Expired)}), nil
}

func (h *Admin) EndCampaign(ctx context.Context, req *connect.Request[adminpb.EndCampaignRequest]) (*connect.Response[adminpb.EndCampaignResponse], error) {
	actor, err := h.actor(ctx, req.Spec().Procedure)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseUUID("campaign_id", req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	res, err := h.m.AdminEndCampaign(ctx, actor, coupon.EndCampaignInput{CampaignID: id, Reason: req.Msg.GetReason()})
	if err != nil {
		return nil, h.failure(err, adminEndPaths)
	}
	d, err := h.m.AdminGetCampaign(ctx, id)
	if err != nil {
		return nil, h.failure(err, noFieldPaths)
	}
	return connect.NewResponse(&adminpb.EndCampaignResponse{
		Campaign: toAdminCampaign(d.Campaign, d.Expired), VoidedVouchers: int32(min(res.Voided, 1<<31-1)), //nolint:gosec // 夾在 int32 範圍內
	}), nil
}

func (h *Admin) GrantVouchers(ctx context.Context, req *connect.Request[adminpb.GrantVouchersRequest]) (*connect.Response[adminpb.GrantVouchersResponse], error) {
	actor, err := h.actor(ctx, req.Spec().Procedure)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseUUID("campaign_id", req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	raw := req.Msg.GetBuyerIds()
	if len(raw) == 0 || len(raw) > coupon.MaxIssueRequestBuyers {
		return nil, rpc.RequiredField("buyer_ids", fmt.Sprintf("buyer_ids 必須是 1 到 %d 個（去重後最多 %d 位買家）", coupon.MaxIssueRequestBuyers, coupon.MaxIssueBatch))
	}
	buyers := make([]int64, len(raw))
	for i, r := range raw {
		if buyers[i], err = rpc.ParseID("buyer_ids["+strconv.Itoa(i)+"]", r); err != nil {
			return nil, err
		}
	}
	res, err := h.m.AdminIssue(ctx, actor, coupon.IssueInput{CampaignID: id, BuyerIDs: buyers})
	if err != nil {
		return nil, h.failure(err, adminGrantPaths)
	}
	return connect.NewResponse(&adminpb.GrantVouchersResponse{Issued: int32(min(res.Issued, coupon.MaxIssueBatch)), AlreadyHeld: int32(min(res.AlreadyHeld, coupon.MaxIssueBatch))}), nil //nolint:gosec // 一次最多 MaxIssueBatch
}

func (h *Admin) VoidVoucher(ctx context.Context, req *connect.Request[adminpb.VoidVoucherRequest]) (*connect.Response[adminpb.VoidVoucherResponse], error) {
	actor, err := h.actor(ctx, req.Spec().Procedure)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseUUID("voucher_id", req.Msg.GetVoucherId())
	if err != nil {
		return nil, err
	}
	reason := trimtext.Trim(req.Msg.GetReason())
	if reason == "" {
		return nil, rpc.RequiredField("reason", "reason 必填")
	}
	if err := h.m.AdminVoidVoucher(ctx, actor, coupon.VoidVoucherInput{VoucherID: id, Reason: reason}); err != nil {
		return nil, h.failure(err, adminVoidPaths)
	}
	return connect.NewResponse(&adminpb.VoidVoucherResponse{}), nil
}

func (h *Admin) SetSellerQuota(ctx context.Context, req *connect.Request[adminpb.SetSellerQuotaRequest]) (*connect.Response[adminpb.SetSellerQuotaResponse], error) {
	actor, err := h.actor(ctx, req.Spec().Procedure)
	if err != nil {
		return nil, err
	}
	storeID, err := rpc.ParseID("store_id", req.Msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	if req.Msg.GetMaxActiveCampaigns() < 1 {
		return nil, rpc.RequiredField("max_active_campaigns", "max_active_campaigns 至少是 1")
	}
	if err := h.m.AdminSetSellerQuota(ctx, actor, storeID, req.Msg.GetMaxActiveCampaigns()); err != nil {
		return nil, h.failure(err, adminQuotaPaths)
	}
	return connect.NewResponse(&adminpb.SetSellerQuotaResponse{}), nil
}

func (h *Admin) ListCampaigns(ctx context.Context, req *connect.Request[adminpb.ListCampaignsRequest]) (*connect.Response[adminpb.ListCampaignsResponse], error) {
	msg := req.Msg
	const procedure = couponv1alpha1connect.CouponAdminServiceListCampaignsProcedure
	size, err := pageSize(msg.GetPageSize())
	if err != nil {
		return nil, err
	}
	in := coupon.AdminListInput{Size: size}
	statusKey, issuerKey := "", ""
	if msg.Status != nil {
		in.Status = rpc.EnumFromProto[coupon.CampaignStatus](msg.GetStatus())
		if in.Status != coupon.StatusDraft && in.Status != coupon.StatusActive && in.Status != coupon.StatusEnded {
			return nil, rpc.RequiredField("status", "status 必須是已知的活動狀態")
		}
		statusKey = string(in.Status)
	}
	if msg.Issuer != nil {
		switch i := msg.GetIssuer().GetIssuer().(type) {
		case *adminpb.IssuerFilter_Platform:
			in.PlatformOnly, issuerKey = true, "platform"
		case *adminpb.IssuerFilter_StoreId:
			if in.StoreID, err = rpc.ParseID("issuer.store_id", i.StoreId); err != nil {
				return nil, err
			}
			issuerKey = "store:" + rpc.FormatID(in.StoreID)
		default:
			return nil, rpc.RequiredField("issuer", "issuer 必須設定 platform 或 store_id 其中一個")
		}
	}
	scope := rpc.FilterScope(procedure, statusKey, issuerKey)
	if in.After, err = decodeCursor(scope, msg.GetPageToken()); err != nil {
		return nil, err
	}
	page, err := h.m.AdminListCampaigns(ctx, in)
	if err != nil {
		return nil, h.failure(err, adminListPaths)
	}
	out := make([]*adminpb.AdminCampaign, len(page.Campaigns))
	for i, c := range page.Campaigns {
		out[i] = toAdminCampaign(c.Campaign, c.Expired)
	}
	return connect.NewResponse(&adminpb.ListCampaignsResponse{Campaigns: out, NextPageToken: nextToken(scope, page.Next)}), nil
}

func (h *Admin) GetCampaign(ctx context.Context, req *connect.Request[adminpb.GetCampaignRequest]) (*connect.Response[adminpb.GetCampaignResponse], error) {
	id, err := rpc.ParseUUID("campaign_id", req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	d, err := h.m.AdminGetCampaign(ctx, id)
	if err != nil {
		return nil, h.failure(err, noFieldPaths)
	}
	buyers := make([]string, len(d.TargetBuyers))
	for i, b := range d.TargetBuyers {
		buyers[i] = rpc.FormatID(b)
	}
	return connect.NewResponse(&adminpb.GetCampaignResponse{
		Campaign: toAdminCampaign(d.Campaign, d.Expired), DesignatedBuyerIds: buyers,
		AvailableCount: d.Available, LockedCount: d.Locked, RedeemedCount: d.Redeemed, VoidedCount: d.Voided,
	}), nil
}

func (h *Admin) ListBuyerVouchers(ctx context.Context, req *connect.Request[adminpb.ListBuyerVouchersRequest]) (*connect.Response[adminpb.ListBuyerVouchersResponse], error) {
	msg := req.Msg
	const procedure = couponv1alpha1connect.CouponAdminServiceListBuyerVouchersProcedure
	buyerID, err := rpc.ParseID("buyer_id", msg.GetBuyerId())
	if err != nil {
		return nil, err
	}
	size, err := pageSize(msg.GetPageSize())
	if err != nil {
		return nil, err
	}
	in := coupon.VoucherListInput{BuyerID: buyerID, Size: size}
	for _, s := range msg.GetStatuses() {
		v := rpc.EnumFromProto[coupon.VoucherStatus](s)
		if v != coupon.VoucherAvailable && v != coupon.VoucherLocked && v != coupon.VoucherRedeemed && v != coupon.VoucherVoided {
			return nil, rpc.RequiredField("statuses", "statuses 的值必須是已知的券狀態")
		}
		if !slices.Contains(in.Statuses, v) {
			in.Statuses = append(in.Statuses, v)
		}
	}
	slices.Sort(in.Statuses)
	statusKeys := make([]string, len(in.Statuses))
	for i, s := range in.Statuses {
		statusKeys[i] = string(s)
	}
	scope := rpc.FilterScope(procedure, rpc.FormatID(buyerID), fmt.Sprint(statusKeys))
	if in.After, err = decodeCursor(scope, msg.GetPageToken()); err != nil {
		return nil, err
	}
	page, err := h.m.ListVouchers(ctx, in)
	if err != nil {
		return nil, h.failure(err, noFieldPaths)
	}
	out := make([]*adminpb.AdminVoucher, len(page.Vouchers))
	for i, v := range page.Vouchers {
		out[i] = toAdminVoucher(v)
	}
	return connect.NewResponse(&adminpb.ListBuyerVouchersResponse{Vouchers: out, NextPageToken: nextToken(scope, page.Next)}), nil
}

func toAdminCampaign(c coupon.Campaign, expired bool) *adminpb.AdminCampaign {
	out := &adminpb.AdminCampaign{Campaign: toSellerCampaign(c, expired)}
	switch {
	case c.IssuerAdminID != nil:
		out.Issuer = &adminpb.AdminCampaign_IssuerAdminId{IssuerAdminId: rpc.FormatID(*c.IssuerAdminID)}
	case c.IssuerStoreID != nil:
		out.Issuer = &adminpb.AdminCampaign_IssuerStoreId{IssuerStoreId: rpc.FormatID(*c.IssuerStoreID)}
	}
	return out
}

func toAdminVoucher(v coupon.VoucherView) *adminpb.AdminVoucher {
	out := &adminpb.AdminVoucher{
		Id: v.Voucher.ID.String(), Campaign: toCampaign(v.Campaign, nil), BuyerId: rpc.FormatID(v.Voucher.BuyerID),
		Status: rpc.EnumToProto[pb.VoucherStatus](v.Voucher.Status), Origin: rpc.EnumToProto[pb.VoucherOrigin](v.Voucher.Origin),
		Expired: v.Expired, CreateTime: timestamppb.New(v.Voucher.CreatedAt),
	}
	if v.Voucher.OrderNo != "" {
		out.OrderNo = &v.Voucher.OrderNo
	}
	if v.Voucher.LockedAt != nil {
		out.LockedTime = timestamppb.New(*v.Voucher.LockedAt)
	}
	if v.Voucher.RedeemedAt != nil {
		out.RedeemedTime = timestamppb.New(*v.Voucher.RedeemedAt)
	}
	if v.Voucher.VoidedAt != nil {
		out.VoidedTime = timestamppb.New(*v.Voucher.VoidedAt)
	}
	return out
}
