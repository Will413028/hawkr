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
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/coupon/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/coupon/v1alpha1/couponv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon"
	"github.com/Will413028/hawkr/backend/internal/modules/crm"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// who 是一個呼叫者在單體裡的身分（十進位字串）。Store 只有賣家有。
type who struct{ Account, Buyer, Store string }

// connectDriver 以單體的 CouponAppService 操作 coupon。每個呼叫者一個帶著自己 Bearer token 的 client；Connect 錯誤依 ErrorInfo.reason 對應到 coupon 的錯誤，
// 其餘以 ch.ConnectKind 分類。
//
// 與 REST 的 wire 不同之處由 driver 吸收，情境只看業務結果：
//   - 舊的列表有 page、total；Connect 是 keyset，driver 翻完所有頁後切出第 Number 頁、Total 是全部的筆數（頁大小與 token 由伺服器守住）。
//   - 舊的買家「我的券」是可領模板加已使用的券；單體是可領的活動（ListClaimableCampaigns）加持有的券（ListMyVouchers）。
//   - 發行張數與上限不在買家的契約裡，driver 直接讀資料庫（只用在驗證計數）。
//
// 還沒有 admin 的 Connect API：Admin* 直接呼叫 coupon 模組準備資料，錯誤依同一套對應分類。
type connectDriver struct {
	clients map[ch.Caller]couponv1alpha1connect.CouponAppServiceClient
	ids     map[ch.Caller]who
	// buyerOf 是帳號 ID 對買家 ID（情境以帳號 ID 指定買家）。
	buyerOf map[string]string
	module  *coupon.Module
	crm     *crm.Module
	db      *db.DB
	adminID int64
}

var _ Driver = (*connectDriver)(nil)

func newConnectDriver(httpClient *http.Client, base string, tokens map[ch.Caller]string, ids map[ch.Caller]who, m *coupon.Module, customers *crm.Module, d *db.DB) *connectDriver {
	drv := &connectDriver{
		clients: map[ch.Caller]couponv1alpha1connect.CouponAppServiceClient{}, ids: ids, buyerOf: map[string]string{}, module: m, crm: customers, db: d, adminID: 7,
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
	}
	return drv
}

func (d *connectDriver) Legacy() bool   { return false }
func (d *connectDriver) AdminRPC() bool { return false }

func (d *connectDriver) StoreOf(c ch.Caller) string      { return d.ids[c].Store }
func (d *connectDriver) BuyerAccount(c ch.Caller) string { return d.ids[c].Account }
func (d *connectDriver) BuyerID(c ch.Caller) string      { return d.ids[c].Buyer }
func (d *connectDriver) Unknown() string                 { return "00000000-0000-7000-8000-000000000001" }
func (d *connectDriver) Malformed() string               { return "not-a-number" }

// reasonErrors 是 ErrorInfo.reason 對到情境的 coupon 錯誤。
var reasonErrors = map[string]error{
	"COUPON_QUOTA_EXCEEDED":      ErrQuotaExceeded,
	"COUPON_ISSUE_LIMIT_REACHED": ErrSoldOut,
	"COUPON_CAMPAIGN_NOT_ACTIVE": ErrNotActive,
	"COUPON_OUTSIDE_WINDOW":      ErrOutOfWindow,
	"COUPON_NOT_ELIGIBLE":        ErrNotEligible,
}

// kindOf 把 Connect 錯誤分類：有 coupon 的 ErrorInfo.reason 就是對應的 coupon 錯誤，否則依 code。
func kindOf(err error) error {
	if err == nil {
		return nil
	}
	var ce *connect.Error
	if errors.As(err, &ce) {
		for _, detail := range ce.Details() {
			v, e := detail.Value()
			if e != nil {
				continue
			}
			if info, ok := v.(*errdetails.ErrorInfo); ok {
				if kind, ok := reasonErrors[info.GetReason()]; ok {
					return apperr.WithCause(kind, err)
				}
			}
		}
	}
	return ch.ConnectKind(err)
}

// moduleKind 對應 coupon 模組直接回的錯誤（Admin* 的準備動作）：業務原因先，其次分類。
func moduleKind(err error) error {
	if err == nil {
		return nil
	}
	reasons := map[coupon.Reason]error{
		coupon.ReasonQuotaExceeded:     ErrQuotaExceeded,
		coupon.ReasonIssueLimitReached: ErrSoldOut,
		coupon.ReasonInvalidStatus:     ErrWrongStatus,
		coupon.ReasonCampaignNotActive: ErrNotActive,
		coupon.ReasonOutsideWindow:     ErrOutOfWindow,
		coupon.ReasonNotEligible:       ErrNotEligible,
	}
	if kind, ok := reasons[coupon.ReasonOf(err)]; ok {
		return apperr.WithCause(kind, err)
	}
	if kind, ok := map[apperr.Kind]error{apperr.KindNotFound: ch.ErrNotFound, apperr.KindInvalidInput: ch.ErrInvalid}[apperr.KindOf(err)]; ok {
		return apperr.WithCause(kind, err)
	}
	return err
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

// targetOf 取情境的資格；不認得的值送一個契約沒有的數字，讓伺服器拒絕。
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

// templateOf 把契約的活動轉成情境的 Template。買家看得到的欄位來自 Campaign；發行張數與上限不在買家的契約裡，由 counts 補。
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

// counts 直接讀活動的已發行張數與上限（-1 是無上限）。
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

// pageOf 取翻完整份之後的第 p.Number 頁（Number 0 與 1 是第一頁；不給 Size 就是全部）。
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
		// 不認得的狀態：送一個 UNSPECIFIED，伺服器要拒絕。
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
	// 結帳情境一定送出（缺店家或金額由伺服器拒絕）。
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

// ensureCustomers 讓情境指定的買家成為呼叫者這家店的 CRM 客戶（單體只收店家自己的客戶為指定名單；舊服務沒有這個限制）：每位記一筆完成的訂單。
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

// 以下是 admin 的準備動作：沒有 admin 的 Connect API，直接呼叫 coupon 模組。

func parseID(id string) (uuid.UUID, error) {
	u, err := uuid.Parse(id)
	if err != nil {
		return uuid.UUID{}, apperr.WithCause(ch.ErrInvalid, err)
	}
	return u, nil
}

func (d *connectDriver) AdminCreate(ctx context.Context, _ ch.Caller, in TemplateInput) (Template, error) {
	spec := coupon.CampaignSpec{Title: in.Title, ThresholdAmount: in.Threshold, StartAt: in.StartAt, EndAt: in.EndAt}
	switch in.Type {
	case TypeCash:
		spec.DiscountType, spec.DiscountAmount = coupon.DiscountCash, in.DiscountAmount
	case TypePercent:
		spec.DiscountType, spec.DiscountPercent, spec.MaxDiscountAmount = coupon.DiscountPercent, int32(in.DiscountPercent), in.MaxDiscount //nolint:gosec // 情境的百分比很小
	}
	create := coupon.CreateCampaignInput{CampaignSpec: spec, TargetType: coupon.TargetType(in.Target), Issuer: coupon.Issuer{AdminID: d.adminID}, TotalLimit: int32(in.TotalIssueLimit)} //nolint:gosec // 情境的上限很小
	if in.StoreID != "" {
		store, err := strconv.ParseInt(in.StoreID, 10, 64)
		if err != nil {
			return Template{}, apperr.WithCause(ch.ErrInvalid, err)
		}
		create.StoreID = store
	}
	for _, b := range d.buyerIDs(in.BuyerAccounts) {
		id, err := strconv.ParseInt(b, 10, 64)
		if err != nil {
			return Template{}, apperr.WithCause(ch.ErrInvalid, err)
		}
		create.TargetBuyers = append(create.TargetBuyers, id)
	}
	camp, err := d.module.CreateCampaign(ctx, create)
	if err != nil {
		return Template{}, moduleKind(err)
	}
	t := Template{
		ID: camp.ID.String(), Title: camp.Title, Type: CouponType(camp.DiscountType), Target: Target(camp.TargetType), Status: Status(camp.Status), Issuer: IssuerAdmin,
		Threshold: camp.ThresholdAmount, DiscountAmount: camp.DiscountAmount, DiscountPercent: int(camp.DiscountPercent), MaxDiscount: camp.MaxDiscountAmount,
		TotalIssueLimit: -1, IssuedCount: int(camp.IssuedCount),
	}
	if camp.TotalLimit != nil {
		t.TotalIssueLimit = int(*camp.TotalLimit)
	}
	if camp.StoreID != nil {
		t.StoreID = strconv.FormatInt(*camp.StoreID, 10)
	}
	return t, nil
}

func (d *connectDriver) AdminPublish(ctx context.Context, _ ch.Caller, id string) error {
	u, err := parseID(id)
	if err != nil {
		return err
	}
	return moduleKind(d.module.PublishCampaign(ctx, u))
}

func (d *connectDriver) AdminDisable(ctx context.Context, _ ch.Caller, id string) error {
	u, err := parseID(id)
	if err != nil {
		return err
	}
	_, err = d.module.EndCampaign(ctx, coupon.EndCampaignInput{CampaignID: u, AdminID: d.adminID})
	return moduleKind(err)
}

func (d *connectDriver) AdminIssue(ctx context.Context, _ ch.Caller, id string, accounts []string, _ bool) (IssueResult, error) {
	u, err := parseID(id)
	if err != nil {
		return IssueResult{}, err
	}
	var buyers []int64
	for _, b := range d.buyerIDs(accounts) {
		n, err := strconv.ParseInt(b, 10, 64)
		if err != nil {
			return IssueResult{}, apperr.WithCause(ch.ErrInvalid, err)
		}
		buyers = append(buyers, n)
	}
	res, err := d.module.Issue(ctx, coupon.IssueInput{CampaignID: u, BuyerIDs: buyers, AdminID: d.adminID})
	return IssueResult{Issued: res.Issued, Distributed: res.Issued}, moduleKind(err)
}

func (d *connectDriver) AdminVoid(ctx context.Context, _ ch.Caller, id, reason string) error {
	u, err := parseID(id)
	if err != nil {
		return err
	}
	return moduleKind(d.module.VoidVoucher(ctx, coupon.VoidVoucherInput{VoucherID: u, AdminID: d.adminID, Reason: reason}))
}

func (d *connectDriver) AdminSetQuota(ctx context.Context, _ ch.Caller, storeID string, maxCount int) error {
	id, err := strconv.ParseInt(storeID, 10, 64)
	if err != nil {
		return apperr.WithCause(ch.ErrInvalid, err)
	}
	return moduleKind(d.module.SetSellerQuota(ctx, id, int32(maxCount))) //nolint:gosec // 情境的配額很小
}
