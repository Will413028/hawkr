// Package coupon 是 coupon 模組的 Connect handler：買家的券與可領的活動、店家發的活動（hawkr.coupon.v1alpha1.CouponAppService）。
// 這裡只做欄位的轉換、身分的推導與顯示資料的組裝（店家卡片來自 seller、買家暱稱來自 buyer）；規則在 internal/modules/coupon。
// 顯示資料讀取失敗整個請求失敗，不降級成空值（計畫 D3）。
package coupon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"
	"uuid"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/coupon/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/coupon/v1alpha1/couponv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// Module 是 coupon 模組（*coupon.Module）。
type Module interface {
	ListVouchers(ctx context.Context, in coupon.VoucherListInput) (coupon.VoucherPage, error)
	ListClaimable(ctx context.Context, in coupon.ClaimableInput) (coupon.CampaignPage, error)
	GetVoucher(ctx context.Context, buyerID int64, id uuid.UUID) (coupon.VoucherView, error)
	Claim(ctx context.Context, campaignID uuid.UUID, buyerID int64) (coupon.ClaimResult, error)
	CreateCampaign(ctx context.Context, in coupon.CreateCampaignInput) (coupon.Campaign, error)
	ListSellerCampaigns(ctx context.Context, in coupon.SellerListInput) (coupon.SellerCampaignPage, error)
	GetSellerCampaign(ctx context.Context, storeID int64, id uuid.UUID) (coupon.CampaignDetail, error)
	EndCampaign(ctx context.Context, in coupon.EndCampaignInput) (coupon.EndCampaignResult, error)
}

// Buyers 是買家身分與顯示資料的來源（*buyer.Module）。
type Buyers interface {
	CurrentBuyerID(ctx context.Context, accountID int64) (int64, error)
	BatchDisplay(ctx context.Context, ids []int64) ([]buyer.Display, error)
}

// Customers 是店家的客戶（*crm.Module）：指定發放的名單只收這家店的客戶。
type Customers interface {
	CustomersAmong(ctx context.Context, storeID int64, buyerIDs []int64) (map[int64]bool, error)
}

// Stores 是店家的來源（*seller.Module）：賣家方法的店家與能力、券上顯示的店家卡片。
type Stores interface {
	ActingStore(ctx context.Context, accountID int64, c seller.Capability) (seller.Store, error)
	StoreCards(ctx context.Context, storeIDs []int64) ([]seller.StoreCard, error)
}

// App 實作 CouponAppService。
type App struct {
	module    Module
	buyers    Buyers
	customers Customers
	stores    Stores
	images    rpc.Images
	log       *slog.Logger
	reasonOf func(error) coupon.Reason
	table    map[string]declaration
}

var _ couponv1alpha1connect.CouponAppServiceHandler = (*App)(nil)

// NewApp 建立券的 handler。能力宣告表（declarations）與 CouponAppService 的方法對不上時回錯誤，不能啟動。
func NewApp(m Module, buyers Buyers, customers Customers, stores Stores, images rpc.Images, log *slog.Logger) (*App, error) {
	return newApp(declarations, m, buyers, customers, stores, images, log)
}

func newApp(table map[string]declaration, m Module, buyers Buyers, customers Customers, stores Stores, images rpc.Images, log *slog.Logger) (*App, error) {
	switch {
	case log == nil:
		return nil, fmt.Errorf("httpapi/coupon：需要 logger")
	case m == nil:
		return nil, fmt.Errorf("httpapi/coupon：需要 coupon 模組")
	case buyers == nil:
		return nil, fmt.Errorf("httpapi/coupon：需要 Buyers")
	case customers == nil:
		return nil, fmt.Errorf("httpapi/coupon：需要 Customers")
	case stores == nil:
		return nil, fmt.Errorf("httpapi/coupon：需要 Stores")
	}
	if err := checkDeclarations(table, serviceDescriptor()); err != nil {
		return nil, err
	}
	return &App{module: m, buyers: buyers, customers: customers, stores: stores, images: images, log: log, reasonOf: coupon.ReasonOf, table: table}, nil
}

func (h *App) buyerID(ctx context.Context) (int64, error) {
	p, err := rpc.RequireAccount(ctx)
	if err != nil {
		return 0, err
	}
	id, err := h.buyers.CurrentBuyerID(ctx, p.ID)
	if err != nil {
		return 0, rpc.BuyerInvariant(p.ID, err)
	}
	return id, nil
}

func (h *App) actingStore(ctx context.Context, procedure string) (seller.Store, error) {
	d, ok := h.table[procedure]
	if !ok || d.capability == "" {
		return seller.Store{}, fmt.Errorf("httpapi/coupon：%s 沒有宣告賣家能力", procedure)
	}
	p, err := rpc.RequireAccount(ctx)
	if err != nil {
		return seller.Store{}, err
	}
	return h.stores.ActingStore(ctx, p.ID, d.capability)
}

var reasons = map[coupon.Reason]apperr.Reason{
	coupon.ReasonCampaignNotActive: apperr.ReasonCouponCampaignNotActive,
	coupon.ReasonOutsideWindow:     apperr.ReasonCouponOutsideWindow,
	coupon.ReasonNotEligible:       apperr.ReasonCouponNotEligible,
	coupon.ReasonStoreSuspended:    apperr.ReasonCouponStoreSuspended,
	coupon.ReasonIssueLimitReached: apperr.ReasonCouponIssueLimitReached,
	coupon.ReasonQuotaExceeded:     apperr.ReasonCouponQuotaExceeded,
}

func (h *App) failure(err error, paths rpc.FieldPaths) error {
	return failure(h.reasonOf, err, paths)
}

func failure(reasonOf func(error) coupon.Reason, err error, paths rpc.FieldPaths) error {
	if err == nil {
		return nil
	}
	var ae *apperr.Error
	if r, ok := reasons[reasonOf(err)]; ok && errors.As(err, &ae) {
		ae.Reason = r
	}
	return rpc.WithFieldPaths(err, paths)
}

var (
	noFieldPaths = rpc.FieldPaths{}
	createPaths  = rpc.FieldPaths{
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
		"StoreID":           "campaign.target_type",
		"TargetBuyers":      "campaign.designated_buyer_ids",
		"TargetBuyers[]":    "campaign.designated_buyer_ids[]",
	}
)

func (h *App) requireCustomers(ctx context.Context, storeID int64, raw []string, ids []int64) error {
	have, err := h.customers.CustomersAmong(ctx, storeID, ids)
	if err != nil {
		return err
	}
	for i, id := range ids {
		if !have[id] {
			return rpc.WithFieldPaths(apperr.InvalidInputOn([]string{apperr.IndexedField("TargetBuyers", i)}, "名單只能是這家店的客戶：%s", raw[i]), createPaths)
		}
	}
	return nil
}

type cursor struct {
	At time.Time `json:"a"`
	ID uuid.UUID `json:"i"`
}

func (c cursor) Valid() bool { return c.ID != (uuid.UUID{}) && !c.At.IsZero() }

func decodeCursor(scope, token string) (*coupon.Cursor, error) {
	c, err := rpc.ValidCursor[cursor](scope, token)
	if err != nil || c == nil {
		return nil, err
	}
	return &coupon.Cursor{At: c.At, ID: c.ID}, nil
}

func nextToken(scope string, next *coupon.Cursor) string {
	if next == nil {
		return ""
	}
	return rpc.TokenFor(scope, cursor{At: next.At, ID: next.ID})
}

func pageSize(n int32) (int32, error) {
	if n < 0 {
		return 0, rpc.RequiredField("page_size", "page_size 不能是負數")
	}
	return n, nil
}

func (h *App) ListMyVouchers(ctx context.Context, req *connect.Request[pb.ListMyVouchersRequest]) (*connect.Response[pb.ListMyVouchersResponse], error) {
	msg := req.Msg
	const procedure = couponv1alpha1connect.CouponAppServiceListMyVouchersProcedure
	size, err := pageSize(msg.GetPageSize())
	if err != nil {
		return nil, err
	}
	in := coupon.VoucherListInput{Size: size}
	statusKeys := make([]string, 0, len(msg.GetStatuses()))
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
	for _, s := range in.Statuses {
		statusKeys = append(statusKeys, string(s))
	}
	checkoutKey := ""
	if msg.Checkout != nil {
		co := msg.GetCheckout()
		storeID, err := rpc.ParseID("checkout.store_id", co.GetStoreId())
		if err != nil {
			return nil, err
		}
		if co.GetOrderAmount() < 1 || co.GetOrderAmount() > coupon.MaxOrderAmount {
			return nil, rpc.RequiredField("checkout.order_amount", "order_amount 必須大於 0，而且不能超過上限")
		}
		if len(in.Statuses) > 1 || (len(in.Statuses) == 1 && in.Statuses[0] != coupon.VoucherAvailable) {
			return nil, rpc.RequiredField("statuses", "帶 checkout 時 statuses 必須為空或只含 AVAILABLE")
		}
		in.Checkout = &coupon.Checkout{StoreID: storeID, OrderAmount: co.GetOrderAmount()}
		checkoutKey = fmt.Sprintf("%d/%d", storeID, co.GetOrderAmount())
	}
	if in.BuyerID, err = h.buyerID(ctx); err != nil {
		return nil, err
	}
	scope := rpc.FilterScope(procedure, rpc.FormatID(in.BuyerID), fmt.Sprint(statusKeys), checkoutKey)
	if in.After, err = decodeCursor(scope, msg.GetPageToken()); err != nil {
		return nil, err
	}
	page, err := h.module.ListVouchers(ctx, in)
	if err != nil {
		return nil, h.failure(err, noFieldPaths)
	}
	cards, err := h.storeCards(ctx, voucherStoreIDs(page.Vouchers))
	if err != nil {
		return nil, err
	}
	out := make([]*pb.Voucher, len(page.Vouchers))
	for i, v := range page.Vouchers {
		out[i] = toVoucher(v, cards, in.Checkout != nil)
	}
	return connect.NewResponse(&pb.ListMyVouchersResponse{Vouchers: out, NextPageToken: nextToken(scope, page.Next)}), nil
}

func (h *App) ListClaimableCampaigns(ctx context.Context, req *connect.Request[pb.ListClaimableCampaignsRequest]) (*connect.Response[pb.ListClaimableCampaignsResponse], error) {
	msg := req.Msg
	const procedure = couponv1alpha1connect.CouponAppServiceListClaimableCampaignsProcedure
	size, err := pageSize(msg.GetPageSize())
	if err != nil {
		return nil, err
	}
	in := coupon.ClaimableInput{Size: size}
	storeKey := ""
	if msg.StoreId != nil {
		if in.StoreID, err = rpc.ParseID("store_id", msg.GetStoreId()); err != nil {
			return nil, err
		}
		storeKey = rpc.FormatID(in.StoreID)
	}
	if in.BuyerID, err = h.buyerID(ctx); err != nil {
		return nil, err
	}
	scope := rpc.FilterScope(procedure, rpc.FormatID(in.BuyerID), storeKey)
	if in.After, err = decodeCursor(scope, msg.GetPageToken()); err != nil {
		return nil, err
	}
	page, err := h.module.ListClaimable(ctx, in)
	if err != nil {
		return nil, h.failure(err, noFieldPaths)
	}
	cards, err := h.storeCards(ctx, campaignStoreIDs(page.Campaigns))
	if err != nil {
		return nil, err
	}
	out := make([]*pb.Campaign, len(page.Campaigns))
	for i, c := range page.Campaigns {
		out[i] = toCampaign(c, cards)
	}
	return connect.NewResponse(&pb.ListClaimableCampaignsResponse{Campaigns: out, NextPageToken: nextToken(scope, page.Next)}), nil
}

func (h *App) ClaimCoupon(ctx context.Context, req *connect.Request[pb.ClaimCouponRequest]) (*connect.Response[pb.ClaimCouponResponse], error) {
	id, err := rpc.ParseUUID("campaign_id", req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	buyerID, err := h.buyerID(ctx)
	if err != nil {
		return nil, err
	}
	res, err := h.module.Claim(ctx, id, buyerID)
	if err != nil {
		return nil, h.failure(err, noFieldPaths)
	}
	view, err := h.module.GetVoucher(ctx, buyerID, res.Voucher.ID)
	if err != nil {
		return nil, h.failure(err, noFieldPaths)
	}
	cards, err := h.storeCards(ctx, voucherStoreIDs([]coupon.VoucherView{view}))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.ClaimCouponResponse{Voucher: toVoucher(view, cards, false)}), nil
}

func (h *App) CreateCampaign(ctx context.Context, req *connect.Request[pb.CreateCampaignRequest]) (*connect.Response[pb.CreateCampaignResponse], error) {
	const procedure = couponv1alpha1connect.CouponAppServiceCreateCampaignProcedure
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
	spec, err := specOf(in)
	if err != nil {
		return nil, err
	}
	st, err := h.actingStore(ctx, procedure)
	if err != nil {
		return nil, err
	}
	if len(buyers) > 0 {
		own, err := h.buyerID(ctx)
		if err != nil {
			return nil, err
		}
		if slices.Contains(buyers, own) {
			return nil, rpc.RequiredField("campaign.designated_buyer_ids", "不能指定自己的買家身分")
		}
		if err := h.requireCustomers(ctx, st.StoreID, in.GetDesignatedBuyerIds(), buyers); err != nil {
			return nil, err
		}
	}
	create := coupon.CreateCampaignInput{
		CampaignSpec: spec, TargetType: rpc.EnumFromProto[coupon.TargetType](in.GetTargetType()),
		Issuer: coupon.Issuer{StoreID: st.StoreID}, TargetBuyers: buyers,
	}
	if in.TotalIssueLimit != nil {
		if in.GetTotalIssueLimit() < 1 {
			return nil, rpc.RequiredField("campaign.total_issue_limit", "total_issue_limit 至少是 1；不設表示不限量")
		}
		create.TotalLimit = in.GetTotalIssueLimit()
	}
	c, err := h.module.CreateCampaign(ctx, create)
	if err != nil {
		return nil, h.failure(err, createPaths)
	}
	return connect.NewResponse(&pb.CreateCampaignResponse{Campaign: toSellerCampaign(c, false)}), nil
}

func (h *App) ListCampaigns(ctx context.Context, req *connect.Request[pb.ListCampaignsRequest]) (*connect.Response[pb.ListCampaignsResponse], error) {
	msg := req.Msg
	const procedure = couponv1alpha1connect.CouponAppServiceListCampaignsProcedure
	size, err := pageSize(msg.GetPageSize())
	if err != nil {
		return nil, err
	}
	in := coupon.SellerListInput{Size: size}
	statusKey := ""
	if msg.Status != nil {
		in.Status = rpc.EnumFromProto[coupon.CampaignStatus](msg.GetStatus())
		if in.Status != coupon.StatusDraft && in.Status != coupon.StatusActive && in.Status != coupon.StatusEnded {
			return nil, rpc.RequiredField("status", "status 必須是已知的活動狀態")
		}
		statusKey = string(in.Status)
	}
	st, err := h.actingStore(ctx, procedure)
	if err != nil {
		return nil, err
	}
	in.StoreID = st.StoreID
	scope := rpc.FilterScope(procedure, rpc.FormatID(st.StoreID), statusKey)
	if in.After, err = decodeCursor(scope, msg.GetPageToken()); err != nil {
		return nil, err
	}
	page, err := h.module.ListSellerCampaigns(ctx, in)
	if err != nil {
		return nil, h.failure(err, noFieldPaths)
	}
	out := make([]*pb.SellerCampaign, len(page.Campaigns))
	for i, c := range page.Campaigns {
		out[i] = toSellerCampaign(c.Campaign, c.Expired)
	}
	return connect.NewResponse(&pb.ListCampaignsResponse{
		Campaigns: out, NextPageToken: nextToken(scope, page.Next), MaxActiveCampaigns: page.MaxActive, ActiveCampaigns: page.CurrentActive,
	}), nil
}

func (h *App) GetCampaign(ctx context.Context, req *connect.Request[pb.GetCampaignRequest]) (*connect.Response[pb.GetCampaignResponse], error) {
	const procedure = couponv1alpha1connect.CouponAppServiceGetCampaignProcedure
	id, err := rpc.ParseUUID("campaign_id", req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	st, err := h.actingStore(ctx, procedure)
	if err != nil {
		return nil, err
	}
	d, err := h.module.GetSellerCampaign(ctx, st.StoreID, id)
	if err != nil {
		return nil, h.failure(err, noFieldPaths)
	}
	buyers, err := h.designated(ctx, d.TargetBuyers)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.GetCampaignResponse{
		Campaign: toSellerCampaign(d.Campaign, d.Expired), DesignatedBuyers: buyers,
		AvailableCount: d.Available, LockedCount: d.Locked, RedeemedCount: d.Redeemed, VoidedCount: d.Voided,
	}), nil
}

func (h *App) EndCampaign(ctx context.Context, req *connect.Request[pb.EndCampaignRequest]) (*connect.Response[pb.EndCampaignResponse], error) {
	const procedure = couponv1alpha1connect.CouponAppServiceEndCampaignProcedure
	id, err := rpc.ParseUUID("campaign_id", req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	st, err := h.actingStore(ctx, procedure)
	if err != nil {
		return nil, err
	}
	res, err := h.module.EndCampaign(ctx, coupon.EndCampaignInput{CampaignID: id, StoreID: st.StoreID})
	if err != nil {
		return nil, h.failure(err, noFieldPaths)
	}
	d, err := h.module.GetSellerCampaign(ctx, st.StoreID, id)
	if err != nil {
		return nil, h.failure(err, noFieldPaths)
	}
	return connect.NewResponse(&pb.EndCampaignResponse{Campaign: toSellerCampaign(d.Campaign, d.Expired), VoidedVouchers: int32(min(res.Voided, 1<<31-1))}), nil //nolint:gosec // 夾在 int32 範圍內
}
