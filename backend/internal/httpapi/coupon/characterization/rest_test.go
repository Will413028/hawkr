//go:build gateway

package characterization

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	ch "github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
)

// restDriver 以舊 gateway 的 REST API（統一 envelope）操作舊的 Java coupon-service：路徑與 wire 型別照
// gateway/internal/handler/coupon.go，r.rest.Do 送出請求並以 ch.RESTKind 分類錯誤。Java 的業務錯誤是 HTTP 200 加
// envelope code（7xxx 是 coupon、400 參數、403 權限），先在 restBusinessKind 對應。coupon-service 刪除時一起刪除。
type restDriver struct {
	rest ch.REST
}

var _ Driver = (*restDriver)(nil)

func newRESTDriver(base string, tokens map[ch.Caller]string, client *http.Client) *restDriver {
	return &restDriver{rest: ch.REST{Base: base, Tokens: tokens, HTTP: client, Business: restBusinessKind}}
}

// restBusinessKind 對應 GrpcResponseCode 的 coupon 業務碼；7015–7017、7019 是建立時的欄位規則。
func restBusinessKind(code int32) error {
	switch code {
	case 7001:
		return ch.ErrNotFound
	case 7005:
		return ErrQuotaExceeded
	case 7006:
		return ErrSoldOut
	case 7009, 7013, 7014:
		return ErrWrongStatus
	case 7012:
		return ErrNotClaimable
	case 7015, 7016, 7017, 7019:
		return ch.ErrInvalid
	case 7020:
		return ErrNotActive
	case 7021:
		return ErrOutOfWindow
	}
	return nil
}

func (d *restDriver) Legacy() bool { return true }

const (
	appCoupons    = "/api/v1/app/coupons"
	appTemplates  = "/api/v1/app/coupon_templates"
	adminTemplate = "/api/v1/admin/coupon_templates"
	adminCoupons  = "/api/v1/admin/coupons"
	adminQuotas   = "/api/v1/admin/coupon_quotas"
)

var (
	typeWire   = map[CouponType]string{TypeCash: "1", TypePercent: "2"}
	targetWire = map[Target]string{TargetAll: "1", TargetMember: "2", TargetDesignated: "3", TargetCompensation: "4"}
	statusWire = map[Status]string{StatusDraft: "0", StatusActive: "1", StatusInactive: "2", StatusExpired: "3", StatusRevoked: "4"}
)

// enumWire 取列舉的 wire 值；不認得的值原樣送出，讓情境能送非法值。
func enumWire[K ~string](m map[K]string, k K) string {
	if v, ok := m[k]; ok {
		return v
	}
	return string(k)
}

func enumFromWire[K ~string](m map[K]string, wire string) K {
	for k, v := range m {
		if v == wire {
			return k
		}
	}
	return K("?" + wire)
}

func issuerFromWire(wire string) Issuer {
	switch wire {
	case "1":
		return IssuerAdmin
	case "2":
		return IssuerSeller
	}
	return Issuer("?" + wire)
}

type wireTemplate struct {
	TemplateID        string `json:"templateId"`
	Title             string `json:"title"`
	Type              string `json:"type"`
	TargetType        string `json:"targetType"`
	Status            string `json:"status"`
	IssuerType        string `json:"issuerType"`
	ApplicableStoreID string `json:"applicableStoreId"`
	IssuerStoreID     string `json:"issuerStoreId"`
	ThresholdAmount   string `json:"thresholdAmount"`
	DiscountAmount    string `json:"discountAmount"`
	DiscountPercent   string `json:"discountPercent"`
	MaxDiscountAmount string `json:"maxDiscountAmount"`
	TotalIssueLimit   string `json:"totalIssueLimit"`
	IssuedCount       string `json:"issuedCount"`
	ClaimRequired     bool   `json:"claimRequired"`
	Claimed           bool   `json:"claimed"`
	TargetAccounts    []struct {
		BuyerID string `json:"buyerId"`
	} `json:"targetAccounts"`
}

// numReader 讀 proto JSON 的 int64 字串；空字串與 "null"（舊 gateway 對未設定數字的編碼）是 0，其餘解析失敗記下第一個錯誤。
type numReader struct{ err error }

func (r *numReader) int(s string) int64 {
	if s == "" || s == "null" {
		return 0
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil && r.err == nil {
		r.err = fmt.Errorf("wire 的數字欄位不是整數：%q：%w", s, err)
	}
	return n
}

func (w wireTemplate) template() (Template, error) {
	var r numReader
	t := Template{
		ID: w.TemplateID, Title: w.Title,
		Type:   enumFromWire(typeWire, w.Type),
		Target: enumFromWire(targetWire, w.TargetType),
		Status: enumFromWire(statusWire, w.Status),
		Issuer: issuerFromWire(w.IssuerType), StoreID: w.ApplicableStoreID, IssuerStoreID: w.IssuerStoreID,
		Threshold: r.int(w.ThresholdAmount), DiscountAmount: r.int(w.DiscountAmount),
		DiscountPercent: int(r.int(w.DiscountPercent)), MaxDiscount: r.int(w.MaxDiscountAmount),
		TotalIssueLimit: int(r.int(w.TotalIssueLimit)), IssuedCount: int(r.int(w.IssuedCount)),
		ClaimRequired: w.ClaimRequired, Claimed: w.Claimed,
	}
	for _, a := range w.TargetAccounts {
		t.TargetBuyers = append(t.TargetBuyers, a.BuyerID)
	}
	return t, r.err
}

type wirePage[T any] struct {
	Records []T    `json:"records"`
	Total   string `json:"total"`
}

func templates(ws []wireTemplate) ([]Template, error) {
	out := make([]Template, 0, len(ws))
	for _, w := range ws {
		t, err := w.template()
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

func decimal(n int64) string {
	if n == 0 {
		return ""
	}
	return strconv.FormatInt(n, 10)
}

func isoTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05")
}

func pageQuery(q url.Values, p Page) {
	if p.Number > 0 {
		q.Set("page", strconv.Itoa(p.Number))
	}
	if p.Size > 0 {
		q.Set("page_size", strconv.Itoa(p.Size))
	}
}

func withQuery(path string, q url.Values) string {
	if len(q) == 0 {
		return path
	}
	return path + "?" + q.Encode()
}

func (d *restDriver) ListMyCoupons(ctx context.Context, c ch.Caller, in ListQuery) (MyCoupons, error) {
	q := url.Values{}
	switch in.Status {
	case "":
	case "UNUSED":
		q.Set("status", "0")
	case "USED":
		q.Set("status", "2")
	default:
		q.Set("status", in.Status)
	}
	pageQuery(q, in.Page)
	var out struct {
		wirePage[wireTemplate]
		UsedRecords []struct {
			InstanceID string `json:"instanceId"`
			TemplateID string `json:"templateId"`
			Status     string `json:"status"`
		} `json:"usedRecords"`
	}
	if err := d.rest.Do(ctx, c, http.MethodGet, withQuery(appCoupons, q), nil, &out); err != nil {
		return MyCoupons{}, err
	}
	used := make([]Instance, 0, len(out.UsedRecords))
	for _, u := range out.UsedRecords {
		used = append(used, Instance{ID: u.InstanceID, TemplateID: u.TemplateID, Status: u.Status})
	}
	avail, err := templates(out.Records)
	if err != nil {
		return MyCoupons{}, err
	}
	var r numReader
	res := MyCoupons{Available: avail, Used: used, Total: int(r.int(out.Total))}
	return res, r.err
}

func (d *restDriver) ListAvailable(ctx context.Context, c ch.Caller, in AvailableQuery) (AvailableList, error) {
	q := url.Values{}
	if in.StoreID != "" {
		q.Set("store_id", in.StoreID)
	}
	if in.OrderAmount != 0 {
		q.Set("order_amount", strconv.FormatInt(in.OrderAmount, 10))
	}
	pageQuery(q, in.Page)
	var out wirePage[struct {
		Template          wireTemplate `json:"template"`
		EstimatedDiscount string       `json:"estimatedDiscount"`
	}]
	if err := d.rest.Do(ctx, c, http.MethodGet, withQuery(appCoupons+"/available", q), nil, &out); err != nil {
		return AvailableList{}, err
	}
	var nr numReader
	list := AvailableList{Total: int(nr.int(out.Total))}
	for _, rec := range out.Records {
		t, err := rec.Template.template()
		if err != nil {
			return AvailableList{}, err
		}
		list.Records = append(list.Records, AvailableCoupon{Template: t, EstimatedDiscount: nr.int(rec.EstimatedDiscount)})
	}
	return list, nr.err
}

func (d *restDriver) Claim(ctx context.Context, c ch.Caller, templateID string) (ClaimResult, error) {
	var out struct {
		ClaimedAt string `json:"claimedAt"`
	}
	if err := d.rest.Do(ctx, c, http.MethodPost, appCoupons+"/"+url.PathEscape(templateID)+"/claim", map[string]any{}, &out); err != nil {
		return ClaimResult{}, err
	}
	return ClaimResult{ClaimedAt: out.ClaimedAt}, nil
}

func templateBody(in TemplateInput) map[string]any {
	body := map[string]any{
		"title": in.Title, "type": enumWire(typeWire, in.Type), "targetType": enumWire(targetWire, in.Target),
		"startAt": isoTime(in.StartAt), "endAt": isoTime(in.EndAt),
	}
	for k, v := range map[string]string{
		"thresholdAmount": decimal(in.Threshold), "discountAmount": decimal(in.DiscountAmount),
		"discountPercent": decimal(int64(in.DiscountPercent)), "maxDiscountAmount": decimal(in.MaxDiscount),
	} {
		if v != "" {
			body[k] = v
		}
	}
	if len(in.BuyerAccounts) > 0 {
		body["buyerAccountIds"] = in.BuyerAccounts
	}
	return body
}

func (d *restDriver) SellerCreate(ctx context.Context, c ch.Caller, in TemplateInput) (Template, error) {
	var out wireTemplate
	if err := d.rest.Do(ctx, c, http.MethodPost, appTemplates, templateBody(in), &out); err != nil {
		return Template{}, err
	}
	return out.template()
}

func (d *restDriver) SellerList(ctx context.Context, c ch.Caller, status Status, p Page) (TemplateList, error) {
	q := url.Values{}
	if status != "" {
		q.Set("status", enumWire(statusWire, status))
	}
	pageQuery(q, p)
	var out wirePage[wireTemplate]
	if err := d.rest.Do(ctx, c, http.MethodGet, withQuery(appTemplates, q), nil, &out); err != nil {
		return TemplateList{}, err
	}
	recs, err := templates(out.Records)
	if err != nil {
		return TemplateList{}, err
	}
	var r numReader
	res := TemplateList{Records: recs, Total: int(r.int(out.Total))}
	return res, r.err
}

func (d *restDriver) SellerDetail(ctx context.Context, c ch.Caller, templateID string) (SellerDetail, error) {
	var out struct {
		Template      wireTemplate `json:"template"`
		UsedCount     string       `json:"usedCount"`
		ReservedCount string       `json:"reservedCount"`
	}
	if err := d.rest.Do(ctx, c, http.MethodGet, appTemplates+"/"+url.PathEscape(templateID), nil, &out); err != nil {
		return SellerDetail{}, err
	}
	t, err := out.Template.template()
	if err != nil {
		return SellerDetail{}, err
	}
	var r numReader
	res := SellerDetail{Template: t, Used: int(r.int(out.UsedCount)), Reserved: int(r.int(out.ReservedCount))}
	return res, r.err
}

func (d *restDriver) SellerRevoke(ctx context.Context, c ch.Caller, templateID, reason string) error {
	return d.rest.Do(ctx, c, http.MethodPost, appTemplates+"/"+url.PathEscape(templateID)+"/revoke", map[string]any{"reason": reason}, nil)
}

func (d *restDriver) AdminCreate(ctx context.Context, c ch.Caller, in TemplateInput) (Template, error) {
	body := templateBody(in)
	if in.StoreID != "" {
		body["applicableStoreId"] = in.StoreID
	}
	if in.TotalIssueLimit != 0 {
		body["totalIssueLimit"] = strconv.Itoa(in.TotalIssueLimit)
	}
	if in.ClaimRequired != nil {
		body["claimRequired"] = *in.ClaimRequired
	}
	var out wireTemplate
	if err := d.rest.Do(ctx, c, http.MethodPost, adminTemplate, body, &out); err != nil {
		return Template{}, err
	}
	return out.template()
}

func (d *restDriver) AdminPublish(ctx context.Context, c ch.Caller, templateID string) error {
	return d.rest.Do(ctx, c, http.MethodPost, adminTemplate+"/"+url.PathEscape(templateID)+"/publish", map[string]any{}, nil)
}

func (d *restDriver) AdminDisable(ctx context.Context, c ch.Caller, templateID string) error {
	return d.rest.Do(ctx, c, http.MethodPost, adminTemplate+"/"+url.PathEscape(templateID)+"/disable", map[string]any{}, nil)
}

func (d *restDriver) AdminIssue(ctx context.Context, c ch.Caller, templateID string, buyerAccounts []string, immediate bool) (IssueResult, error) {
	var out struct {
		IssuedCount      string `json:"issuedCount"`
		DistributedCount string `json:"distributedCount"`
	}
	body := map[string]any{"buyerAccountIds": buyerAccounts, "immediateDistribute": immediate}
	if err := d.rest.Do(ctx, c, http.MethodPost, adminTemplate+"/"+url.PathEscape(templateID)+"/issue", body, &out); err != nil {
		return IssueResult{}, err
	}
	var r numReader
	res := IssueResult{Issued: int(r.int(out.IssuedCount)), Distributed: int(r.int(out.DistributedCount))}
	return res, r.err
}

func (d *restDriver) AdminVoid(ctx context.Context, c ch.Caller, voucherID, reason string) error {
	return d.rest.Do(ctx, c, http.MethodPost, adminCoupons+"/"+url.PathEscape(voucherID)+"/void", map[string]any{"reason": reason}, nil)
}

func (d *restDriver) AdminSetQuota(ctx context.Context, c ch.Caller, storeID string, max int) error {
	return d.rest.Do(ctx, c, http.MethodPut, adminQuotas+"/"+url.PathEscape(storeID), map[string]any{"maxTemplateCount": fmt.Sprint(max)}, nil)
}
