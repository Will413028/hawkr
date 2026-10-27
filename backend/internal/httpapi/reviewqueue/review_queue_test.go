package reviewqueue

import (
	"cmp"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/reviewqueue/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/reviewqueue/v1alpha1/reviewqueuev1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	rq "github.com/Will413028/hawkr/backend/internal/modules/reviewqueue"
	"github.com/Will413028/hawkr/backend/internal/platform/adminperm"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

var now = time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)

// fakeLister 依模組的契約實作 List：只回 kinds 內的、全序 (期限, 開始, 種類, ID) 在 after 之後的、最多 limit 件；並記下每次的參數。
type fakeLister struct {
	items []rq.Item
	calls []call
	err   error
}

type call struct {
	now   time.Time
	kinds []rq.Kind
	after *rq.Cursor
	limit int32
}

func order(a, b rq.Item) int {
	return cmp.Or(a.Due.Compare(b.Due), a.Start.Compare(b.Start), strings.Compare(string(a.Kind), string(b.Kind)), strings.Compare(a.ItemID, b.ItemID))
}

func (f *fakeLister) List(_ context.Context, at time.Time, kinds []rq.Kind, after *rq.Cursor, limit int32) ([]rq.Item, error) {
	f.calls = append(f.calls, call{at, slices.Clone(kinds), after, limit})
	if f.err != nil {
		return nil, f.err
	}
	var out []rq.Item
	for _, it := range slices.SortedFunc(slices.Values(f.items), order) {
		if !slices.Contains(kinds, it.Kind) {
			continue
		}
		if after != nil && order(it, rq.Item{Due: after.Due, Start: after.Start, Kind: after.Kind, ItemID: after.ID}) <= 0 {
			continue
		}
		if int32(len(out)) < limit {
			out = append(out, it)
		}
	}
	return out, nil
}

// reportItem 是一件待分案的通報（只有 report_id）。
func reportItem(id string, due time.Time) rq.Item {
	r := uuid.NewV7()
	return rq.Item{Kind: rq.ReportTriage, ItemID: id, ReportID: &r, Start: due.Add(-72 * time.Hour), Due: due}
}

func newAdmin(f *fakeLister) *Admin { return NewAdmin(f, func() time.Time { return now }) }

// 呼叫者：以 header 選擇放進 ctx 的 principal（真實的入口是 withSession），其餘走真正的 interceptor 與 Connect 的 wire，錯誤碼因此是 client 看到的。
const (
	asOwner   = "owner"   // 管理員、OWNER 角色
	asNoRole  = "norole"  // 管理員、沒有任何角色
	asAccount = "account" // 一般帳號（不是管理員）
	asNobody  = ""        // 沒有認證
)

func list(t *testing.T, h *Admin, who string, req *adminpb.ListReviewQueueRequest) (*adminpb.ListReviewQueueResponse, error) {
	t.Helper()
	mux := http.NewServeMux()
	path, handler := reviewqueuev1alpha1connect.NewReviewQueueAdminServiceHandler(h, connect.WithInterceptors(rpc.NewInterceptor(slog.New(slog.DiscardHandler))))
	mux.Handle(path, handler)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		switch r.Header.Get("X-Test-Caller") {
		case asOwner:
			ctx = rpc.WithPrincipal(ctx, identity.Principal{Kind: identity.PrincipalAdmin, ID: 7, AdminRoles: []adminperm.Role{adminperm.RoleOwner}})
		case asNoRole:
			ctx = rpc.WithPrincipal(ctx, identity.Principal{Kind: identity.PrincipalAdmin, ID: 8})
		case asAccount:
			ctx = rpc.WithPrincipal(ctx, identity.Principal{Kind: identity.PrincipalAccount, ID: 1})
		}
		mux.ServeHTTP(w, r.WithContext(ctx))
	}))
	defer srv.Close()
	client := reviewqueuev1alpha1connect.NewReviewQueueAdminServiceClient(srv.Client(), srv.URL)
	r := connect.NewRequest(req)
	r.Header().Set("X-Test-Caller", who)
	resp, err := client.ListReviewQueue(context.Background(), r)
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

func wantInvalid(t *testing.T, label string, err error) {
	t.Helper()
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("%s：%v 不是 INVALID_ARGUMENT", label, err)
	}
}

// 種類 → 權限的表、模組的種類與 proto 的 enum 一對一：漏一個要失敗。
func TestKindsAreOneToOneWithTheContract(t *testing.T) {
	if got := rpc.EnumMismatches[adminpb.ReviewQueueItemKind](rq.Kinds()); len(got) != 0 {
		t.Errorf("模組的種類與 ReviewQueueItemKind 對不上：%v", got)
	}
	var fromProto []rq.Kind
	vs := adminpb.ReviewQueueItemKind(0).Descriptor().Values()
	for i := range vs.Len() {
		if k := rpc.EnumFromProto[rq.Kind](adminpb.ReviewQueueItemKind(vs.Get(i).Number())); k != "" {
			fromProto = append(fromProto, k)
		}
	}
	slices.Sort(fromProto)
	all := rq.Kinds()
	slices.Sort(all)
	if !slices.Equal(fromProto, all) {
		t.Errorf("proto %v，模組 %v", fromProto, all)
	}
	if len(all) != 10 {
		t.Errorf("種類 %d 個，預期 10 個", len(all))
	}
	// 權限表：逐種類寫死期望的來源讀取權限（E13），每個權限都在目錄裡。
	wantPerm := map[rq.Kind]adminperm.Permission{
		rq.ReportTriage:          adminperm.SellerReportsList,
		rq.CaseDecision:          adminperm.SellerCasesList,
		rq.CaseInquiryOverdue:    adminperm.SellerCaseInquiriesList,
		rq.AuthorityOrder:        adminperm.SellerAuthorityOrdersList,
		rq.AuthorityTargetNotice: adminperm.SellerAuthorityOrdersList,
		rq.MeasureAppeal:         adminperm.SellerMeasuresList,
		rq.MeasureLiftRequest:    adminperm.SellerMeasuresList,
		rq.MeasureReview:         adminperm.SellerMeasuresList,
		rq.KYCApplication:        adminperm.SellerKYCApplicationsList,
		// PayUni 註冊只有 get 讀取權（沒有 list RPC）。
		rq.PayUniBindRetry: adminperm.SellerPayUniRegistrationsGet,
	}
	for _, k := range rq.Kinds() {
		perm, ok := kindPermissions[k]
		if !ok {
			t.Errorf("%s 沒有對應的讀取權限", k)
			continue
		}
		if !adminperm.InCatalog(perm) {
			t.Errorf("%s 對應 %q：不在目錄裡", k, perm)
		}
		if perm != wantPerm[k] {
			t.Errorf("%s 對應 %q，期望來源的讀取權限 %q", k, perm, wantPerm[k])
		}
	}
	if len(kindPermissions) != len(all) {
		t.Errorf("權限表 %d 項，種類 %d 個", len(kindPermissions), len(all))
	}
}

// 翻頁：取 N＋1 才知道有沒有下一頁。mutation：只取 N 筆（limit 不加一），第一頁剛好 N 筆時不會有 token，後面的頁讀不到。
func TestPagesWalkAllItemsAndRequestOneExtraRow(t *testing.T) {
	f := &fakeLister{}
	for i := range 5 {
		f.items = append(f.items, reportItem(string(rune('a'+i)), now.Add(time.Duration(i)*time.Hour)))
	}
	h := newAdmin(f)
	who := asOwner
	var ids []string
	token := ""
	pages := 0
	for {
		resp, err := list(t, h, who, &adminpb.ListReviewQueueRequest{PageSize: 2, PageToken: token})
		if err != nil {
			t.Fatal(err)
		}
		pages++
		for _, it := range resp.Items {
			ids = append(ids, it.GetReportId())
		}
		if last := f.calls[len(f.calls)-1]; last.limit != 3 {
			t.Errorf("page_size 2 向模組要 %d 筆，預期 3（N＋1）", last.limit)
		}
		if resp.NextPageToken == "" {
			break
		}
		if len(resp.Items) != 2 {
			t.Errorf("有下一頁卻只有 %d 筆", len(resp.Items))
		}
		token = resp.NextPageToken
	}
	var want []string
	for _, it := range slices.SortedFunc(slices.Values(f.items), order) {
		want = append(want, it.ReportID.String())
	}
	if pages != 3 || !slices.Equal(ids, want) {
		t.Errorf("%d 頁，讀到 %v，預期 3 頁 %v", pages, ids, want)
	}
	// 剛好一頁：N 筆、沒有 N＋1，沒有 token。
	one, err := list(t, h, who, &adminpb.ListReviewQueueRequest{PageSize: 5})
	if err != nil || len(one.Items) != 5 || one.NextPageToken != "" {
		t.Errorf("剛好 5 筆：%v %d 筆 token=%q", err, len(one.GetItems()), one.GetNextPageToken())
	}
	if f.calls[len(f.calls)-1].now != now {
		t.Errorf("now 不是注入的時鐘：%v", f.calls[len(f.calls)-1].now)
	}
}

func TestPageSizeDefaultsCapsAndRejectsNegative(t *testing.T) {
	for _, tt := range []struct {
		in   int32
		want int32 // 向模組要的筆數（page_size＋1）
		bad  bool
	}{{0, 51, false}, {1, 2, false}, {100, 101, false}, {101, 101, false}, {100000, 101, false}, {-1, 0, true}} {
		f := &fakeLister{}
		_, err := list(t, newAdmin(f), asOwner, &adminpb.ListReviewQueueRequest{PageSize: tt.in})
		if tt.bad {
			wantInvalid(t, "page_size 負數", err)
			if len(f.calls) != 0 {
				t.Errorf("page_size %d 仍然呼叫了模組", tt.in)
			}
			continue
		}
		if err != nil || len(f.calls) != 1 || f.calls[0].limit != tt.want {
			t.Errorf("page_size %d：%v，向模組要 %v，預期 %d", tt.in, err, f.calls, tt.want)
		}
	}
}

// kinds：UNSPECIFIED 與不認得的數字是 INVALID_ARGUMENT（不論夾在有效的種類之間），BadRequest 指到出錯的元素 kinds[i]（AIP-193）。
func TestKindsRejectUnspecifiedAndUnknown(t *testing.T) {
	for name, tc := range map[string]struct {
		kinds []adminpb.ReviewQueueItemKind
		field string
	}{
		"UNSPECIFIED": {[]adminpb.ReviewQueueItemKind{adminpb.ReviewQueueItemKind_REVIEW_QUEUE_ITEM_KIND_UNSPECIFIED}, "kinds[0]"},
		"未知數字":        {[]adminpb.ReviewQueueItemKind{adminpb.ReviewQueueItemKind(99)}, "kinds[0]"},
		"負數":          {[]adminpb.ReviewQueueItemKind{adminpb.ReviewQueueItemKind(-1)}, "kinds[0]"},
		"有效的種類之間夾著未知的值": {[]adminpb.ReviewQueueItemKind{adminpb.ReviewQueueItemKind_REVIEW_QUEUE_ITEM_KIND_REPORT_TRIAGE, adminpb.ReviewQueueItemKind(13)}, "kinds[1]"},
		"有效的種類後面接著未指定":  {[]adminpb.ReviewQueueItemKind{adminpb.ReviewQueueItemKind_REVIEW_QUEUE_ITEM_KIND_KYC_APPLICATION, 0}, "kinds[1]"},
	} {
		f := &fakeLister{}
		_, err := list(t, newAdmin(f), asOwner, &adminpb.ListReviewQueueRequest{Kinds: tc.kinds})
		wantInvalid(t, name, err)
		if got := badRequestFields(err); !slices.Equal(got, []string{tc.field}) {
			t.Errorf("%s：BadRequest 的欄位 %v，要 [%s]", name, got, tc.field)
		}
		if len(f.calls) != 0 {
			t.Errorf("%s：仍然呼叫了模組", name)
		}
	}
	if missing := rpc.CheckFieldPaths((&adminpb.ListReviewQueueRequest{}).ProtoReflect().Descriptor(), kindsPaths); len(missing) > 0 {
		t.Errorf("kindsPaths 在請求訊息裡找不到：%v", missing)
	}
}

// badRequestFields 取出 Connect 錯誤的 BadRequest 違規欄位。
func badRequestFields(err error) []string {
	var out []string
	var ce *connect.Error
	if !errors.As(err, &ce) {
		return nil
	}
	for _, d := range ce.Details() {
		v, verr := d.Value()
		if verr != nil {
			continue
		}
		if br, ok := v.(*errdetails.BadRequest); ok {
			for _, fv := range br.GetFieldViolations() {
				out = append(out, fv.GetField())
			}
		}
	}
	return out
}

// token 綁請求的 kinds：不同的集合是 INVALID_ARGUMENT，順序不同但集合相同（含重複）通過，壞 token 是 INVALID_ARGUMENT。
func TestPageTokenIsBoundToTheRequestedKinds(t *testing.T) {
	f := &fakeLister{}
	for i := range 4 {
		it := reportItem(string(rune('a'+i)), now.Add(time.Duration(i)*time.Hour))
		f.items = append(f.items, it)
	}
	h := newAdmin(f)
	who := asOwner
	k := adminpb.ReviewQueueItemKind_REVIEW_QUEUE_ITEM_KIND_REPORT_TRIAGE
	m := adminpb.ReviewQueueItemKind_REVIEW_QUEUE_ITEM_KIND_MEASURE_REVIEW
	first, err := list(t, h, who, &adminpb.ListReviewQueueRequest{PageSize: 2, Kinds: []adminpb.ReviewQueueItemKind{k, m}})
	if err != nil || first.NextPageToken == "" {
		t.Fatalf("第一頁：%v token=%q", err, first.GetNextPageToken())
	}
	tok := first.NextPageToken
	for name, kinds := range map[string][]adminpb.ReviewQueueItemKind{
		"少一種": {k}, "多一種": {k, m, adminpb.ReviewQueueItemKind_REVIEW_QUEUE_ITEM_KIND_KYC_APPLICATION}, "沒有篩選": nil, "換一種": {k, adminpb.ReviewQueueItemKind_REVIEW_QUEUE_ITEM_KIND_KYC_APPLICATION},
	} {
		_, err := list(t, h, who, &adminpb.ListReviewQueueRequest{PageSize: 2, PageToken: tok, Kinds: kinds})
		wantInvalid(t, "kinds "+name, err)
	}
	for name, kinds := range map[string][]adminpb.ReviewQueueItemKind{
		"順序不同": {m, k}, "重複": {m, k, k, m},
	} {
		if _, err := list(t, h, who, &adminpb.ListReviewQueueRequest{PageSize: 2, PageToken: tok, Kinds: kinds}); err != nil {
			t.Errorf("kinds %s：集合相同卻被拒絕：%v", name, err)
		}
	}
	for name, bad := range map[string]string{"亂碼": "not-a-token!", "截斷": tok[:len(tok)-6], "空白以外的短字串": "AAAA"} {
		_, err := list(t, h, who, &adminpb.ListReviewQueueRequest{PageToken: bad, Kinds: []adminpb.ReviewQueueItemKind{k, m}})
		wantInvalid(t, "壞 token "+name, err)
	}
	// 其他 procedure 的 token（同樣的 kinds）也不能用。
	foreign := rpc.TokenFor(rpc.FilterScope("/other.Service/List", "MEASURE_REVIEW", "REPORT_TRIAGE"), rq.CursorOf(f.items[0]))
	_, err = list(t, h, who, &adminpb.ListReviewQueueRequest{PageToken: foreign, Kinds: []adminpb.ReviewQueueItemKind{k, m}})
	wantInvalid(t, "別的 procedure 的 token", err)
	// 內容不合法的游標（種類空）。
	invalid := rpc.TokenFor(rpc.FilterScope("/hawkr.admin.reviewqueue.v1alpha1.ReviewQueueAdminService/ListReviewQueue"), rq.Cursor{ID: "x"})
	_, err = list(t, h, who, &adminpb.ListReviewQueueRequest{PageToken: invalid})
	wantInvalid(t, "游標內容不合法", err)
}

// 權限：請求的 kinds（空的是全部）與呼叫者有權的種類取交集。
// mutation：拿掉交集（直接把請求的 kinds 傳給模組），沒有權限的種類會出現，這裡失敗。
func TestKindsAreIntersectedWithThePermissionsOfTheCaller(t *testing.T) {
	allowed := map[adminperm.Permission]bool{adminperm.SellerMeasuresList: true, adminperm.SellerKYCApplicationsList: true}
	f := &fakeLister{}
	h := newAdmin(f)
	h.allows = func(_ identity.Principal, p adminperm.Permission) bool { return allowed[p] }
	who := asOwner
	kinds := func(ks ...adminpb.ReviewQueueItemKind) *adminpb.ListReviewQueueRequest {
		return &adminpb.ListReviewQueueRequest{Kinds: ks}
	}
	enum := func(k rq.Kind) adminpb.ReviewQueueItemKind { return rpc.EnumToProto[adminpb.ReviewQueueItemKind](k) }
	sorted := func(ks []rq.Kind) []rq.Kind { slices.Sort(ks); return ks }

	// 空的 kinds 展開成有權的全部。
	if _, err := list(t, h, who, kinds()); err != nil {
		t.Fatal(err)
	}
	want := []rq.Kind{rq.KYCApplication, rq.MeasureAppeal, rq.MeasureLiftRequest, rq.MeasureReview}
	if got := sorted(f.calls[0].kinds); !slices.Equal(got, want) {
		t.Errorf("空的 kinds 展開成 %v，預期有權的全部 %v", got, want)
	}
	// 請求的一部分有權：只傳有權的。
	f.calls = nil
	if _, err := list(t, h, who, kinds(enum(rq.ReportTriage), enum(rq.KYCApplication), enum(rq.CaseDecision))); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 1 || !slices.Equal(f.calls[0].kinds, []rq.Kind{rq.KYCApplication}) {
		t.Errorf("請求 3 種、有權 1 種，向模組要 %v", f.calls)
	}
	// 請求的全部沒有權限：空清單、不是錯誤、不呼叫模組。
	f.calls = nil
	resp, err := list(t, h, who, kinds(enum(rq.ReportTriage), enum(rq.AuthorityOrder)))
	if err != nil || len(resp.Items) != 0 || resp.NextPageToken != "" || len(f.calls) != 0 {
		t.Errorf("全部沒有權限：err=%v items=%d token=%q 呼叫=%v", err, len(resp.GetItems()), resp.GetNextPageToken(), f.calls)
	}
	// 呼叫者什麼都沒有（沒有任何角色）：任何請求都是空清單。
	none := newAdmin(f)
	f.calls = nil
	resp, err = list(t, none, asNoRole, kinds())
	if err != nil || len(resp.Items) != 0 || len(f.calls) != 0 {
		t.Errorf("沒有角色：err=%v items=%d 呼叫=%v", err, len(resp.GetItems()), f.calls)
	}
	// OWNER 是全部。
	f.calls = nil
	if _, err := list(t, newAdmin(f), asOwner, kinds()); err != nil || len(f.calls[0].kinds) != 10 {
		t.Errorf("OWNER：%v %v", err, f.calls)
	}
	// 呼叫者不是管理員、或沒有認證：interceptor 在 handler 之前擋下，不呼叫模組。
	f.calls = nil
	for name, who := range map[string]string{"沒有呼叫者": asNobody, "帳號": asAccount} {
		_, err := list(t, newAdmin(f), who, kinds())
		if c := connect.CodeOf(err); c != connect.CodeUnauthenticated && c != connect.CodePermissionDenied {
			t.Errorf("%s：err=%v", name, err)
		}
	}
	if len(f.calls) != 0 {
		t.Errorf("沒有管理員卻呼叫了模組")
	}
}

// 模組的錯誤：apperr 的分類原樣對應（INVALID_ARGUMENT），沒有分類的錯誤是 internal。
func TestModuleErrorsAreMappedByTheirCategory(t *testing.T) {
	_, err := list(t, newAdmin(&fakeLister{err: apperr.InvalidInput("x")}), asOwner, &adminpb.ListReviewQueueRequest{})
	wantInvalid(t, "模組的 InvalidInput", err)
	if _, err := list(t, newAdmin(&fakeLister{err: errors.New("boom")}), asOwner, &adminpb.ListReviewQueueRequest{}); connect.CodeOf(err) != connect.CodeInternal {
		t.Errorf("沒有分類的錯誤：%v", err)
	}
}

// subject 依種類組：每一種的 oneof 與值、店家、期限；種類需要的對象鍵缺了是內部錯誤。
func TestToProtoBuildsTheSubjectOfEachKind(t *testing.T) {
	id := func() *uuid.UUID { u := uuid.NewV7(); return &u }
	pos, acc, store := int16(2), int64(901), int64(42)
	start, due := now.Add(-time.Hour), now.Add(time.Hour)
	for _, tt := range []struct {
		item rq.Item
		want func(*adminpb.ReviewQueueItem) string // 回傳 subject 的內容字串
		with string
	}{
		{rq.Item{Kind: rq.ReportTriage, ReportID: id()}, func(p *adminpb.ReviewQueueItem) string { return p.GetReportId() }, "report"},
		{rq.Item{Kind: rq.CaseDecision, CaseID: id()}, func(p *adminpb.ReviewQueueItem) string { return p.GetCaseId() }, "case"},
		{rq.Item{Kind: rq.CaseInquiryOverdue, CaseID: id(), InquiryID: id()}, func(p *adminpb.ReviewQueueItem) string {
			return p.GetCaseInquiry().GetCaseId() + "/" + p.GetCaseInquiry().GetInquiryId()
		}, "case_inquiry"},
		{rq.Item{Kind: rq.AuthorityOrder, AuthorityOrderID: id()}, func(p *adminpb.ReviewQueueItem) string { return p.GetAuthorityOrderId() }, "order"},
		{rq.Item{Kind: rq.AuthorityTargetNotice, AuthorityOrderID: id(), TargetPosition: &pos, AccountID: &acc}, func(p *adminpb.ReviewQueueItem) string {
			r := p.GetAuthorityTarget()
			return r.GetOrderId() + "/" + string(rune('0'+r.GetPosition())) + "/" + r.GetAccountId()
		}, "target"},
		{rq.Item{Kind: rq.MeasureAppeal, MeasureID: id(), RequestID: id()}, func(p *adminpb.ReviewQueueItem) string {
			return p.GetMeasureRequest().GetMeasureId() + "/" + p.GetMeasureRequest().GetRequestId()
		}, "measure_request"},
		{rq.Item{Kind: rq.MeasureLiftRequest, MeasureID: id(), RequestID: id()}, func(p *adminpb.ReviewQueueItem) string {
			return p.GetMeasureRequest().GetMeasureId() + "/" + p.GetMeasureRequest().GetRequestId()
		}, "measure_request"},
		{rq.Item{Kind: rq.MeasureReview, MeasureID: id()}, func(p *adminpb.ReviewQueueItem) string { return p.GetMeasureId() }, "measure"},
		{rq.Item{Kind: rq.KYCApplication, KYCApplicationID: id(), StoreID: &store}, func(p *adminpb.ReviewQueueItem) string {
			return p.GetKycApplication().GetStoreId() + "/" + p.GetKycApplication().GetApplicationId()
		}, "kyc"},
		{rq.Item{Kind: rq.PayUniBindRetry, KYCApplicationID: id(), StoreID: &store}, func(p *adminpb.ReviewQueueItem) string {
			return p.GetKycApplication().GetStoreId() + "/" + p.GetKycApplication().GetApplicationId()
		}, "payuni_bind_retry"},
	} {
		it := tt.item
		it.ItemID, it.Start, it.Due = "x", start, due
		if it.StoreID == nil {
			it.StoreID = &store
		}
		it.StoreName = "範例小農"
		p, err := toProto(it)
		if err != nil {
			t.Errorf("%s：%v", it.Kind, err)
			continue
		}
		var ids []string
		for _, u := range []*uuid.UUID{it.ReportID, it.CaseID, it.InquiryID, it.AuthorityOrderID, it.MeasureID, it.RequestID, it.KYCApplicationID} {
			if u != nil {
				ids = append(ids, u.String())
			}
		}
		got := tt.want(p)
		if got == "" || got == "/" {
			t.Errorf("%s：subject 是空的：%v", it.Kind, p)
		}
		for _, u := range ids {
			if !strings.Contains(got, u) {
				t.Errorf("%s：subject %q 少了 %s", it.Kind, got, u)
			}
		}
		if p.GetKind() == adminpb.ReviewQueueItemKind_REVIEW_QUEUE_ITEM_KIND_UNSPECIFIED || rpc.EnumFromProto[rq.Kind](p.GetKind()) != it.Kind {
			t.Errorf("%s：kind 轉成 %v", it.Kind, p.GetKind())
		}
		if p.GetStoreId() != "42" || p.GetStoreName() != "範例小農" {
			t.Errorf("%s：店家 %q/%q", it.Kind, p.GetStoreId(), p.GetStoreName())
		}
		if !p.GetDeadline().GetStartTime().AsTime().Equal(start) || !p.GetDeadline().GetDueTime().AsTime().Equal(due) {
			t.Errorf("%s：期限 %v", it.Kind, p.GetDeadline())
		}
		// 只設一支 subject：其他 oneof 的 getter 都是零值。
		set := 0
		for _, s := range []string{p.GetReportId(), p.GetCaseId(), p.GetAuthorityOrderId(), p.GetMeasureId()} {
			if s != "" {
				set++
			}
		}
		for _, present := range []bool{p.GetCaseInquiry() != nil, p.GetAuthorityTarget() != nil, p.GetMeasureRequest() != nil, p.GetKycApplication() != nil} {
			if present {
				set++
			}
		}
		if set != 1 {
			t.Errorf("%s：設了 %d 支 subject", it.Kind, set)
		}
		// 種類需要的對象鍵逐一缺少：不輸出半個 subject，是內部錯誤。
		for name, drop := range map[string]func(*rq.Item){
			"report_id": func(i *rq.Item) { i.ReportID = nil }, "case_id": func(i *rq.Item) { i.CaseID = nil }, "inquiry_id": func(i *rq.Item) { i.InquiryID = nil },
			"authority_order_id": func(i *rq.Item) { i.AuthorityOrderID = nil }, "position": func(i *rq.Item) { i.TargetPosition = nil },
			"account_id": func(i *rq.Item) { i.AccountID = nil }, "measure_id": func(i *rq.Item) { i.MeasureID = nil }, "request_id": func(i *rq.Item) { i.RequestID = nil },
			"kyc_application_id": func(i *rq.Item) { i.KYCApplicationID = nil },
		} {
			c := it
			drop(&c)
			needed := (name == "report_id" && c.ReportID == nil && it.ReportID != nil) || (name == "case_id" && it.CaseID != nil) || (name == "inquiry_id" && it.InquiryID != nil) ||
				(name == "authority_order_id" && it.AuthorityOrderID != nil) || (name == "position" && it.TargetPosition != nil) || (name == "account_id" && it.AccountID != nil) ||
				(name == "measure_id" && it.MeasureID != nil) || (name == "request_id" && it.RequestID != nil) ||
				(name == "kyc_application_id" && it.KYCApplicationID != nil)
			if _, err := toProto(c); needed && err == nil {
				t.Errorf("%s：缺 %s 卻沒有回錯", it.Kind, name)
			}
		}
	}
	// 沒有單一店家：store_id 空字串；種類空字串與未知的種類不是有效值。
	o := rq.Item{Kind: rq.AuthorityOrder, ItemID: "x", AuthorityOrderID: func() *uuid.UUID { u := uuid.NewV7(); return &u }(), Start: start, Due: due}
	if p, err := toProto(o); err != nil || p.GetStoreId() != "" || p.GetStoreName() != "" {
		t.Errorf("沒有店家的來函：%v %v", p, err)
	}
	for _, k := range []rq.Kind{"", "BOGUS"} {
		if _, err := toProto(rq.Item{Kind: k, ItemID: "x", Start: start, Due: due}); err == nil {
			t.Errorf("種類 %q 被轉成了 proto", k)
		}
	}
}

// token 綁的是請求的 kinds，不是與權限取交集之後的：翻頁途中呼叫者的權限改變，token 照常可用（後面的頁看得到的種類跟著權限變）。
// mutation：把 scope 綁在交集之後的 kinds，權限一變 token 就失效，這裡失敗。
func TestPageTokenSurvivesAPermissionChangeMidwayThroughThePages(t *testing.T) {
	f := &fakeLister{}
	for i := range 4 {
		f.items = append(f.items, reportItem(string(rune('a'+i)), now.Add(time.Duration(i)*time.Hour)))
	}
	allowed := map[adminperm.Permission]bool{adminperm.SellerReportsList: true}
	h := newAdmin(f)
	h.allows = func(_ identity.Principal, p adminperm.Permission) bool { return allowed[p] }
	first, err := list(t, h, asOwner, &adminpb.ListReviewQueueRequest{PageSize: 2})
	if err != nil || first.NextPageToken == "" {
		t.Fatalf("第一頁：%v", err)
	}
	allowed[adminperm.SellerKYCApplicationsList] = true // 權限變多了
	if _, err := list(t, h, asOwner, &adminpb.ListReviewQueueRequest{PageSize: 2, PageToken: first.NextPageToken}); err != nil {
		t.Errorf("權限改變之後 token 失效：%v", err)
	}
}
