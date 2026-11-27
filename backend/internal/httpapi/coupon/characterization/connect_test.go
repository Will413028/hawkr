//go:build integration

package characterization

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/protobuf/types/known/timestamppb"

	ch "github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/coupon/v1alpha1"
	admincouponconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/coupon/v1alpha1/couponv1alpha1connect"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/coupon/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/coupon/v1alpha1/couponv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/modules/crm"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

type who struct{ Account, Buyer, Store string }

type connectDriver struct {
	clients map[ch.Caller]couponv1alpha1connect.CouponAppServiceClient
	admins  map[ch.Caller]admincouponconnect.CouponAdminServiceClient
	ids     map[ch.Caller]who
	buyerOf map[string]string
	crm     *crm.Module
	db      *db.DB
}

var _ Driver = (*connectDriver)(nil)

func newConnectDriver(httpClient *http.Client, base string, tokens map[ch.Caller]string, ids map[ch.Caller]who, customers *crm.Module, d *db.DB) *connectDriver {
	drv := &connectDriver{
		clients: map[ch.Caller]couponv1alpha1connect.CouponAppServiceClient{}, admins: map[ch.Caller]admincouponconnect.CouponAdminServiceClient{},
		ids: ids, buyerOf: map[string]string{}, crm: customers, db: d,
	}
	for _, w := range ids {
		drv.buyerOf[w.Account] = w.Buyer
	}
	for _, c := range []ch.Caller{ch.Anonymous, ch.Buyer, ch.Buyer2, ch.Seller, ch.Seller2, ch.Admin} {
		token := tokens[c]
		auth := connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
			return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
				if token != "" {
					req.Header().Set("Authorization", "Bearer "+token)
				}
				return next(ctx, req)
			}
		}))
		drv.clients[c] = couponv1alpha1connect.NewCouponAppServiceClient(httpClient, base, auth)
		drv.admins[c] = admincouponconnect.NewCouponAdminServiceClient(httpClient, base, auth)
	}
	return drv
}

func (d *connectDriver) Legacy() bool   { return false }
func (d *connectDriver) AdminRPC() bool { return true }

func (d *connectDriver) StoreOf(c ch.Caller) string      { return d.ids[c].Store }
func (d *connectDriver) BuyerAccount(c ch.Caller) string { return d.ids[c].Account }
func (d *connectDriver) BuyerID(c ch.Caller) string      { return d.ids[c].Buyer }
func (d *connectDriver) Unknown() string                 { return "00000000-0000-7000-8000-000000000001" }
func (d *connectDriver) Malformed() string               { return "not-a-number" }

var reasonErrors = map[string]error{
	"COUPON_QUOTA_EXCEEDED":      ErrQuotaExceeded,
	"COUPON_ISSUE_LIMIT_REACHED": ErrSoldOut,
	"COUPON_CAMPAIGN_NOT_ACTIVE": ErrNotActive,
	"COUPON_OUTSIDE_WINDOW":      ErrOutOfWindow,
	"COUPON_NOT_ELIGIBLE":        ErrNotEligible,
}

func reasonKind(err error) (error, bool) {
	var ce *connect.Error
	if !errors.As(err, &ce) {
		return nil, false
	}
	for _, detail := range ce.Details() {
		v, e := detail.Value()
		if e != nil {
			continue
		}
		if info, ok := v.(*errdetails.ErrorInfo); ok {
			if kind, ok := reasonErrors[info.GetReason()]; ok {
				return apperr.WithCause(kind, err), true
			}
		}
	}
	return nil, false
}

func kindOf(err error) error {
	if err == nil {
		return nil
	}
	if kind, ok := reasonKind(err); ok {
		return kind
	}
	return ch.ConnectKind(err)
}

func adminKind(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := reasonKind(err); !ok && connect.CodeOf(err) == connect.CodeFailedPrecondition {
		return apperr.WithCause(ErrWrongStatus, err)
	}
	return kindOf(err)
}

var (
	targetEnum = map[Target]pb.TargetType{
		TargetAll: pb.TargetType_TARGET_TYPE_ALL, TargetMember: pb.TargetType_TARGET_TYPE_MEMBER,
		TargetDesignated: pb.TargetType_TARGET_TYPE_DESIGNATED, TargetCompensation: pb.TargetType_TARGET_TYPE_COMPENSATION,
	}
	statusEnum = map[Status]pb.CampaignStatus{
		StatusDraft: pb.CampaignStatus_CAMPAIGN_STATUS_DRAFT, StatusActive: pb.CampaignStatus_CAMPAIGN_STATUS_ACTIVE, StatusEnded: pb.CampaignStatus_CAMPAIGN_STATUS_ENDED,
	}
)

func targetOf(t Target) pb.TargetType {
	if v, ok := targetEnum[t]; ok {
		return v
	}
	return pb.TargetType(99)
}

func (d *connectDriver) buyerIDs(accounts []string) []string {
	out := make([]string, len(accounts))
	for i, a := range accounts {
		out[i] = a
		if b, ok := d.buyerOf[a]; ok {
			out[i] = b
		}
	}
	return out
}

func (d *connectDriver) campaignInput(in TemplateInput) *pb.CampaignInput {
	out := &pb.CampaignInput{
		Title: in.Title, ThresholdAmount: in.Threshold, TargetType: targetOf(in.Target),
		StartTime: timestamppb.New(in.StartAt), EndTime: timestamppb.New(in.EndAt), DesignatedBuyerIds: d.buyerIDs(in.BuyerAccounts),
	}
	switch in.Type {
	case TypeCash:
		out.Discount = &pb.CampaignInput_Cash{Cash: &pb.CashDiscount{Amount: in.DiscountAmount}}
	case TypePercent:
		out.Discount = &pb.CampaignInput_Percent{Percent: &pb.PercentDiscount{Percent: int32(in.DiscountPercent), MaxAmount: in.MaxDiscount}} //nolint:gosec // 情境的百分比很小
	}
	if in.TotalIssueLimit > 0 {
		limit := int32(in.TotalIssueLimit) //nolint:gosec // 情境的上限很小
		out.TotalIssueLimit = &limit
	}
	return out
}

func (d *connectDriver) templateOf(ctx context.Context, c *pb.Campaign) Template {
	t := Template{
		ID: c.GetId(), Title: c.GetTitle(), Target: targetFromProto(c.GetTargetType()), Status: StatusActive, Threshold: c.GetThresholdAmount(),
		StoreID: c.GetStoreId(),
	}
	if cash := c.GetCash(); cash != nil {
		t.Type, t.DiscountAmount = TypeCash, cash.GetAmount()
	}
	if p := c.GetPercent(); p != nil {
		t.Type, t.DiscountPercent, t.MaxDiscount = TypePercent, int(p.GetPercent()), p.GetMaxAmount()
	}
	t.IssuedCount, t.TotalIssueLimit = d.counts(ctx, c.GetId())
	return t
}

func targetFromProto(t pb.TargetType) Target {
	for k, v := range targetEnum {
		if v == t {
			return k
		}
	}
	return Target("?" + t.String())
}

func statusFromProto(s pb.CampaignStatus) Status {
	for k, v := range statusEnum {
		if v == s {
			return k
		}
	}
	return Status("?" + s.String())
}

func (d *connectDriver) counts(ctx context.Context, id string) (issued, limit int) {
	u, err := uuid.Parse(id)
	if err != nil {
		return 0, -1
	}
	if err := d.db.Conn(ctx).QueryRow(ctx, `SELECT issued_count, COALESCE(total_limit, -1) FROM coupon.coupon_campaigns WHERE id = $1`, u).Scan(&issued, &limit); err != nil {
		return 0, -1
	}
	return issued, limit
}

func (d *connectDriver) sellerTemplate(ctx context.Context, c *pb.SellerCampaign) Template {
	t := d.templateOf(ctx, c.GetCampaign())
	t.Status, t.Issuer, t.IssuerStoreID = statusFromProto(c.GetStatus()), IssuerSeller, c.GetCampaign().GetStoreId()
	t.IssuedCount, t.TotalIssueLimit = int(c.GetIssuedCount()), -1
	if c.TotalIssueLimit != nil {
		t.TotalIssueLimit = int(c.GetTotalIssueLimit())
	}
	return t
}

func pageOf[T any](all []T, p Page) []T {
	if p.Size <= 0 {
		return all
	}
	n := max(p.Number, 1)
	start := (n - 1) * p.Size
	if start >= len(all) {
		return []T{}
	}
	return all[start:min(start+p.Size, len(all))]
}

func (d *connectDriver) ListMyCoupons(ctx context.Context, c ch.Caller, q ListQuery) (MyCoupons, error) {
	client := d.clients[c]
	out := MyCoupons{Available: []Template{}, Used: []Instance{}}
	wantAvailable, wantUsed := q.Status == "" || q.Status == "UNUSED", q.Status == "" || q.Status == "USED"
	if !wantAvailable && !wantUsed {
		_, err := client.ListMyVouchers(ctx, connect.NewRequest(&pb.ListMyVouchersRequest{Statuses: []pb.VoucherStatus{pb.VoucherStatus_VOUCHER_STATUS_UNSPECIFIED}}))
		return out, kindOf(err)
	}
	if wantAvailable {
		for token := ""; ; {
			res, err := client.ListClaimableCampaigns(ctx, connect.NewRequest(&pb.ListClaimableCampaignsRequest{PageSize: 100, PageToken: token}))
			if err != nil {
				return out, kindOf(err)
			}
			for _, camp := range res.Msg.GetCampaigns() {
				out.Available = append(out.Available, d.templateOf(ctx, camp))
			}
			if token = res.Msg.GetNextPageToken(); token == "" {
				break
			}
		}
	}
	statuses := []pb.VoucherStatus{}
	if wantAvailable {
		statuses = append(statuses, pb.VoucherStatus_VOUCHER_STATUS_AVAILABLE)
	}
	if wantUsed {
		statuses = append(statuses, pb.VoucherStatus_VOUCHER_STATUS_REDEEMED)
	}
	for token := ""; ; {
		res, err := client.ListMyVouchers(ctx, connect.NewRequest(&pb.ListMyVouchersRequest{PageSize: 100, PageToken: token, Statuses: statuses}))
		if err != nil {
			return out, kindOf(err)
		}
		for _, v := range res.Msg.GetVouchers() {
			if v.GetStatus() == pb.VoucherStatus_VOUCHER_STATUS_REDEEMED {
				out.Used = append(out.Used, Instance{ID: v.GetId(), TemplateID: v.GetCampaign().GetId(), Status: "USED"})
				continue
			}
			t := d.templateOf(ctx, v.GetCampaign())
			t.Claimed = true
			out.Available = append(out.Available, t)
		}
		if token = res.Msg.GetNextPageToken(); token == "" {
			break
		}
	}
	out.Total = len(out.Available)
	if q.Page.Number > 1 {
		out.Available = []Template{}
	}
	return out, nil
}

func (d *connectDriver) ListAvailable(ctx context.Context, c ch.Caller, q AvailableQuery) (AvailableList, error) {
	client := d.clients[c]
	co := &pb.Checkout{StoreId: q.StoreID, OrderAmount: q.OrderAmount}
	var all []AvailableCoupon
	for token := ""; ; {
		res, err := client.ListMyVouchers(ctx, connect.NewRequest(&pb.ListMyVouchersRequest{PageSize: int32(max(q.Page.Size, 1)), PageToken: token, Checkout: co})) //nolint:gosec // 情境的頁大小很小
		if err != nil {
			return AvailableList{}, kindOf(err)
		}
		for _, v := range res.Msg.GetVouchers() {
			t := d.templateOf(ctx, v.GetCampaign())
			t.Claimed = true
			all = append(all, AvailableCoupon{Template: t, EstimatedDiscount: v.GetEstimatedDiscount()})
		}
		if token = res.Msg.GetNextPageToken(); token == "" {
			break
		}
	}
	return AvailableList{Records: pageOf(all, q.Page), Total: len(all)}, nil
}

func (d *connectDriver) Claim(ctx context.Context, c ch.Caller, id string) (ClaimResult, error) {
	res, err := d.clients[c].ClaimCoupon(ctx, connect.NewRequest(&pb.ClaimCouponRequest{CampaignId: id}))
	if err != nil {
		return ClaimResult{}, kindOf(err)
	}
	v := res.Msg.GetVoucher()
	return ClaimResult{ClaimedAt: v.GetCreateTime().AsTime().Format(time.RFC3339Nano), VoucherID: v.GetId()}, nil
}

func (d *connectDriver) ensureCustomers(ctx context.Context, c ch.Caller, accounts []string) error {
	if len(accounts) == 0 || d.ids[c].Store == "" {
		return nil
	}
	store, err := strconv.ParseInt(d.ids[c].Store, 10, 64)
	if err != nil {
		return err
	}
	for _, b := range d.buyerIDs(accounts) {
		buyerID, err := strconv.ParseInt(b, 10, 64)
		if err != nil {
			continue
		}
		key := "seed-" + d.ids[c].Store + "-" + b
		if _, err := d.crm.RecordOrderCompleted(ctx, crm.OrderCompleted{EventID: key, OrderNo: key, StoreID: store, BuyerID: buyerID, Amount: 100}); err != nil {
			return err
		}
	}
	return nil
}

func (d *connectDriver) SellerCreate(ctx context.Context, c ch.Caller, in TemplateInput) (Template, error) {
	if err := d.ensureCustomers(ctx, c, in.BuyerAccounts); err != nil {
		return Template{}, err
	}
	res, err := d.clients[c].CreateCampaign(ctx, connect.NewRequest(&pb.CreateCampaignRequest{Campaign: d.campaignInput(in)}))
	if err != nil {
		return Template{}, kindOf(err)
	}
	return d.sellerTemplate(ctx, res.Msg.GetCampaign()), nil
}

func (d *connectDriver) SellerList(ctx context.Context, c ch.Caller, status Status, p Page) (TemplateList, error) {
	req := &pb.ListCampaignsRequest{PageSize: int32(max(p.Size, 1))} //nolint:gosec // 情境的頁大小很小
	if status != "" {
		s, ok := statusEnum[status]
		if !ok {
			s = pb.CampaignStatus_CAMPAIGN_STATUS_UNSPECIFIED
		}
		req.Status = &s
	}
	var all []Template
	for {
		res, err := d.clients[c].ListCampaigns(ctx, connect.NewRequest(req))
		if err != nil {
			return TemplateList{}, kindOf(err)
		}
		for _, camp := range res.Msg.GetCampaigns() {
			all = append(all, d.sellerTemplate(ctx, camp))
		}
		if res.Msg.GetNextPageToken() == "" {
			break
		}
		req.PageToken = res.Msg.GetNextPageToken()
	}
	return TemplateList{Records: pageOf(all, p), Total: len(all)}, nil
}

func (d *connectDriver) SellerDetail(ctx context.Context, c ch.Caller, id string) (SellerDetail, error) {
	res, err := d.clients[c].GetCampaign(ctx, connect.NewRequest(&pb.GetCampaignRequest{CampaignId: id}))
	if err != nil {
		return SellerDetail{}, kindOf(err)
	}
	t := d.sellerTemplate(ctx, res.Msg.GetCampaign())
	for _, b := range res.Msg.GetDesignatedBuyers() {
		t.TargetBuyers = append(t.TargetBuyers, b.GetBuyerId())
	}
	return SellerDetail{Template: t, Used: int(res.Msg.GetRedeemedCount()), Reserved: int(res.Msg.GetLockedCount())}, nil
}

// SellerRevoke 是 EndCampaign；契約沒有理由欄位。
func (d *connectDriver) SellerRevoke(ctx context.Context, c ch.Caller, id, _ string) error {
	_, err := d.clients[c].EndCampaign(ctx, connect.NewRequest(&pb.EndCampaignRequest{CampaignId: id}))
	return kindOf(err)
}


func (d *connectDriver) AdminCreate(ctx context.Context, c ch.Caller, in TemplateInput) (Template, error) {
	campaign := &adminpb.AdminCampaignInput{
		Title: in.Title, ThresholdAmount: in.Threshold, TargetType: targetOf(in.Target),
		StartTime: timestamppb.New(in.StartAt), EndTime: timestamppb.New(in.EndAt), DesignatedBuyerIds: d.buyerIDs(in.BuyerAccounts),
	}
	switch in.Type {
	case TypeCash:
		campaign.Discount = &adminpb.AdminCampaignInput_Cash{Cash: &pb.CashDiscount{Amount: in.DiscountAmount}}
	case TypePercent:
		campaign.Discount = &adminpb.AdminCampaignInput_Percent{Percent: &pb.PercentDiscount{Percent: int32(in.DiscountPercent), MaxAmount: in.MaxDiscount}} //nolint:gosec // 情境的百分比很小
	}
	if in.StoreID != "" {
		campaign.StoreId = &in.StoreID
	}
	if in.TotalIssueLimit > 0 {
		limit := int32(in.TotalIssueLimit) //nolint:gosec // 情境的上限很小
		campaign.TotalIssueLimit = &limit
	}
	res, err := d.admins[c].CreateCampaign(ctx, connect.NewRequest(&adminpb.CreateCampaignRequest{Campaign: campaign}))
	if err != nil {
		return Template{}, adminKind(err)
	}
	return d.adminTemplate(ctx, res.Msg.GetCampaign()), nil
}

func (d *connectDriver) adminTemplate(ctx context.Context, c *adminpb.AdminCampaign) Template {
	t := d.sellerTemplate(ctx, c.GetCampaign())
	t.Issuer, t.IssuerStoreID = IssuerAdmin, ""
	if c.GetIssuerStoreId() != "" {
		t.Issuer, t.IssuerStoreID = IssuerSeller, c.GetIssuerStoreId()
	}
	return t
}

func (d *connectDriver) AdminPublish(ctx context.Context, c ch.Caller, id string) error {
	_, err := d.admins[c].PublishCampaign(ctx, connect.NewRequest(&adminpb.PublishCampaignRequest{CampaignId: id}))
	return adminKind(err)
}

func (d *connectDriver) AdminDisable(ctx context.Context, c ch.Caller, id string) error {
	_, err := d.admins[c].EndCampaign(ctx, connect.NewRequest(&adminpb.EndCampaignRequest{CampaignId: id}))
	return adminKind(err)
}

// AdminIssue 是 GrantVouchers；immediate（舊的代領）不再保留（D6），Distributed 恆為 0。
func (d *connectDriver) AdminIssue(ctx context.Context, c ch.Caller, id string, accounts []string, _ bool) (IssueResult, error) {
	res, err := d.admins[c].GrantVouchers(ctx, connect.NewRequest(&adminpb.GrantVouchersRequest{CampaignId: id, BuyerIds: d.buyerIDs(accounts)}))
	if err != nil {
		return IssueResult{}, adminKind(err)
	}
	return IssueResult{Issued: int(res.Msg.GetIssued())}, nil
}

func (d *connectDriver) AdminVoid(ctx context.Context, c ch.Caller, id, reason string) error {
	_, err := d.admins[c].VoidVoucher(ctx, connect.NewRequest(&adminpb.VoidVoucherRequest{VoucherId: id, Reason: reason}))
	return adminKind(err)
}

func (d *connectDriver) AdminSetQuota(ctx context.Context, c ch.Caller, storeID string, maxCount int) error {
	_, err := d.admins[c].SetSellerQuota(ctx, connect.NewRequest(&adminpb.SetSellerQuotaRequest{StoreId: storeID, MaxActiveCampaigns: int32(maxCount)})) //nolint:gosec // 情境的配額很小
	return adminKind(err)
}
