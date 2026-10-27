package seller

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1/sellerv1alpha1connect"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
)

type fakeAdminModule struct {
	AdminModule
	stores   model.Store
	list     model.StoreListInput
	listPage model.StorePage
}

func (f *fakeAdminModule) Store(context.Context, int64) (model.Store, error) { return f.stores, nil }

func (f *fakeAdminModule) AdminStores(_ context.Context, in model.StoreListInput) (model.StorePage, error) {
	f.list = in
	return f.listPage, nil
}

func adminCtx(id int64) context.Context {
	return rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAdmin, ID: id})
}

func newTestAdmin(m AdminModule) *Admin { return NewAdmin(m, rpc.Images{}) }

// wantInvalidArgument 確認請求被當成 client 的錯拒絕：handler 回的是模組的 invalid_input，或傳輸層的 invalid_argument（interceptor
// 之後都是 invalid_argument）；沒帶或讀不懂的 etag 是 etag 的規則（API 設計規範（Connect）§七），傳輸層以 version_conflict 回。
func wantInvalidArgument(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("要被拒絕")
	}
	for _, prefix := range []string{"invalid_argument", "invalid_input", "version_conflict"} {
		if strings.HasPrefix(err.Error(), prefix) {
			return
		}
	}
	t.Fatalf("err=%v，要被當成 client 的錯拒絕", err)
}

func TestAdminMethodsRejectMalformedIDs(t *testing.T) {
	h := newTestAdmin(&fakeAdminModule{})
	ctx := adminCtx(1)
	for name, call := range map[string]func() error{
		"get": func() error {
			_, err := h.GetStore(ctx, connect.NewRequest(&adminpb.GetStoreRequest{Id: "x"}))
			return err
		},
	} {
		t.Run(name, func(t *testing.T) { wantInvalidArgument(t, call()) })
	}
}

// 篩選條件換了就不能沿用上一頁的 token（API 設計規範（Connect）§六）。
func TestListStoresPageTokenIsBoundToItsFilters(t *testing.T) {
	cursor := &model.StoreCursor{CreatedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), StoreID: 5}
	m := &fakeAdminModule{listPage: model.StorePage{Items: []model.StoreListItem{{Store: currentStore()}}, Next: cursor}}
	h := newTestAdmin(m)
	first, err := h.ListStores(adminCtx(1), connect.NewRequest(&adminpb.ListStoresRequest{Query: "小農", PageSize: 1}))
	if err != nil {
		t.Fatal(err)
	}
	token := first.Msg.GetNextPageToken()
	if token == "" || m.list.Query != "小農" || m.list.PageSize != 1 || m.list.After != nil {
		t.Fatalf("token=%q list=%+v", token, m.list)
	}
	if _, err := h.ListStores(adminCtx(1), connect.NewRequest(&adminpb.ListStoresRequest{Query: "小農", PageToken: token})); err != nil {
		t.Fatal(err)
	}
	if m.list.After == nil || *m.list.After != *cursor {
		t.Fatalf("after=%+v", m.list.After)
	}
	for name, req := range map[string]*adminpb.ListStoresRequest{
		"other query":   {Query: "別的", PageToken: token},
		"other status":  {Query: "小農", Status: pb.StoreStatus_STORE_STATUS_CLOSED, PageToken: token},
		"other kyc":     {Query: "小農", KycStatus: pb.KYCStatus_KYC_STATUS_APPROVED, PageToken: token},
		"other account": {Query: "小農", AccountId: "7", PageToken: token},
		"other bind":    {Query: "小農", PayuniBindStatus: pb.PayUniBindStatus_PAY_UNI_BIND_STATUS_FAILED, PageToken: token},
		"suspended":     {Query: "小農", Suspended: ptr(false), PageToken: token},
	} {
		_, err := h.ListStores(adminCtx(1), connect.NewRequest(req))
		if err == nil {
			t.Fatalf("%s: token 不該跨篩選使用", name)
		}
		wantInvalidArgument(t, err)
	}
	// 同一組篩選、但解得開卻不完整的位置也不能用。
	scope := sellerv1alpha1connect.SellerAdminServiceListStoresProcedure + "\x00x\x00\x00" + pb.KYCStatus_KYC_STATUS_UNSPECIFIED.String()
	_, err = h.ListStores(adminCtx(1), connect.NewRequest(&adminpb.ListStoresRequest{Query: "x", PageToken: rpc.TokenFor(scope, &model.StoreCursor{})}))
	wantInvalidArgument(t, err)
}

func TestListStoresMapsFiltersAndLeavesUnspecifiedUnfiltered(t *testing.T) {
	m := &fakeAdminModule{}
	h := newTestAdmin(m)
	if _, err := h.ListStores(adminCtx(1), connect.NewRequest(&adminpb.ListStoresRequest{})); err != nil {
		t.Fatal(err)
	}
	if len(m.list.Statuses) != 0 || m.list.KYCStatus != nil {
		t.Fatalf("UNSPECIFIED 不是篩選值：%+v", m.list)
	}
	if _, err := h.ListStores(adminCtx(1), connect.NewRequest(&adminpb.ListStoresRequest{
		Status: pb.StoreStatus_STORE_STATUS_CLOSED, KycStatus: pb.KYCStatus_KYC_STATUS_SUBMITTED,
	})); err != nil {
		t.Fatal(err)
	}
	if len(m.list.Statuses) != 1 || m.list.Statuses[0] != model.Closed || m.list.KYCStatus == nil || *m.list.KYCStatus != model.KYCStatusSubmitted {
		t.Fatalf("list=%+v", m.list)
	}
	if m.list.AccountID != nil || m.list.PayUniBindStatus != nil || m.list.Suspended != nil {
		t.Fatalf("沒有設定的篩選不能出現在模組的輸入：%+v", m.list)
	}
	if _, err := h.ListStores(adminCtx(1), connect.NewRequest(&adminpb.ListStoresRequest{
		AccountId: "7", PayuniBindStatus: pb.PayUniBindStatus_PAY_UNI_BIND_STATUS_FAILED, Suspended: ptr(false),
	})); err != nil {
		t.Fatal(err)
	}
	if m.list.AccountID == nil || *m.list.AccountID != 7 || m.list.PayUniBindStatus == nil || *m.list.PayUniBindStatus != model.BindStatusFailed ||
		m.list.Suspended == nil || *m.list.Suspended {
		t.Fatalf("list=%+v", m.list)
	}
	for _, bad := range []string{"abc", "0", "-3"} {
		_, err := h.ListStores(adminCtx(1), connect.NewRequest(&adminpb.ListStoresRequest{AccountId: bad}))
		wantInvalidArgument(t, err)
	}
}

func TestAdminEnumsMatchTheModule(t *testing.T) {
	if got := rpc.EnumMismatches[adminpb.StoreCloser](model.ClosedByValues()); len(got) != 0 {
		t.Fatalf("StoreCloser：%v", got)
	}
	if got := rpc.EnumMismatches[adminpb.FoodRegistrationInbox](model.FoodRegInboxValues()); len(got) != 0 {
		t.Fatalf("FoodRegistrationInbox：%v", got)
	}
	if got := rpc.EnumMismatches[adminpb.FoodRegistrationState](model.FoodRegStateValues()); len(got) != 0 {
		t.Fatalf("FoodRegistrationState：%v", got)
	}
	// 賣家的 FoodRegistrationState 只有賣家看得到的狀態：撤回的（WITHDRAWN）賣家看不到，所以沒有這個值。
	sellerVisible := slices.DeleteFunc(model.FoodRegStateValues(), func(v model.FoodRegState) bool { return v == model.FoodRegWithdrawn })
	if got := rpc.EnumMismatches[pb.FoodRegistrationState](sellerVisible); len(got) != 0 {
		t.Fatalf("賣家的 FoodRegistrationState：%v", got)
	}
	if got := rpc.EnumMismatches[adminpb.FoodRegistrationEndCause](model.FoodRegEndCauseValues()); len(got) != 0 {
		t.Fatalf("FoodRegistrationEndCause：%v", got)
	}
}
