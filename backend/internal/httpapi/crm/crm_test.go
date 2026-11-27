package crm

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/crm/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/crm/v1alpha1/crmv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/crm"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

const (
	callerAccount = 1
	callerStoreID = 11
)

var (
	at    = time.Date(2026, 11, 2, 3, 4, 5, 0, time.UTC)
	tagA  = uuid.UUID{1}
	tagB  = uuid.UUID{2}
	fresh = uuid.UUID{9}
)

type fakeReads struct {
	page     crm.CustomerPage
	customer crm.Customer
	tags     []crm.Tag
	err      error
	calls    []string
	listIn   crm.CustomerListInput
	gotStore int64
	gotBuyer int64
}

func (f *fakeReads) ListCustomers(_ context.Context, in crm.CustomerListInput) (crm.CustomerPage, error) {
	f.calls = append(f.calls, "ListCustomers")
	f.listIn = in
	return f.page, f.err
}

func (f *fakeReads) GetCustomer(_ context.Context, storeID, buyerID int64) (crm.Customer, error) {
	f.calls = append(f.calls, "GetCustomer")
	f.gotStore, f.gotBuyer = storeID, buyerID
	return f.customer, f.err
}

func (f *fakeReads) ListTags(_ context.Context, storeID int64) ([]crm.Tag, error) {
	f.calls = append(f.calls, "ListTags")
	f.gotStore = storeID
	return f.tags, f.err
}

type fakeWrites struct {
	err     error
	calls   []string
	storeID int64
	buyerID int64
	typ     crm.CustomerType
	tagID   uuid.UUID
	name    string
	names   []string
	created crm.Tag
	customer crm.Customer
	renamed  crm.Tag
}

func (f *fakeWrites) SetCustomerType(_ context.Context, storeID, buyerID int64, t crm.CustomerType) (crm.Customer, error) {
	f.calls = append(f.calls, "SetCustomerType")
	f.storeID, f.buyerID, f.typ = storeID, buyerID, t
	return f.customer, f.err
}

func (f *fakeWrites) CreateTag(_ context.Context, storeID int64, name string) (crm.Tag, error) {
	f.calls = append(f.calls, "CreateTag")
	f.storeID, f.name = storeID, name
	return f.created, f.err
}

func (f *fakeWrites) RenameTag(_ context.Context, storeID int64, id uuid.UUID, name string) (crm.Tag, error) {
	f.calls = append(f.calls, "RenameTag")
	f.storeID, f.tagID, f.name = storeID, id, name
	return f.renamed, f.err
}

func (f *fakeWrites) DeleteTag(_ context.Context, storeID int64, id uuid.UUID) error {
	f.calls = append(f.calls, "DeleteTag")
	f.storeID, f.tagID = storeID, id
	return f.err
}

func (f *fakeWrites) AddCustomerTags(_ context.Context, storeID, buyerID int64, names []string) (crm.Customer, error) {
	f.calls = append(f.calls, "AddCustomerTags")
	f.storeID, f.buyerID, f.names = storeID, buyerID, slices.Clone(names)
	return f.customer, f.err
}

func (f *fakeWrites) RemoveCustomerTag(_ context.Context, storeID, buyerID int64, id uuid.UUID) (crm.Customer, error) {
	f.calls = append(f.calls, "RemoveCustomerTag")
	f.storeID, f.buyerID, f.tagID = storeID, buyerID, id
	return f.customer, f.err
}

type fakeBuyers struct {
	displays []buyer.Display
	batches  [][]int64
}

func (f *fakeBuyers) BatchDisplay(_ context.Context, ids []int64) ([]buyer.Display, error) {
	f.batches = append(f.batches, slices.Clone(ids))
	return f.displays, nil
}

type fakeStores struct {
	own *seller.Store
}

func (f *fakeStores) CurrentStore(_ context.Context, accountID int64) (seller.Store, error) {
	if accountID != callerAccount || f.own == nil {
		return seller.Store{}, apperr.NotFound("no store")
	}
	return *f.own, nil
}

func images(t *testing.T) rpc.Images {
	t.Helper()
	im, err := rpc.NewImages("https://media.example.test")
	if err != nil {
		t.Fatal(err)
	}
	return im
}

func signedIn() context.Context {
	return rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAccount, ID: callerAccount})
}

func asAdmin() context.Context {
	return rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAdmin, ID: 9})
}

func codeOf(err error) connect.Code {
	return connect.CodeOf(rpc.ConnectError(context.Background(), err))
}

func detailsOf(err error) (reason string, fields []string) {
	for _, d := range rpc.ConnectError(context.Background(), err).Details() {
		v, e := d.Value()
		if e != nil {
			continue
		}
		switch x := v.(type) {
		case *errdetails.ErrorInfo:
			reason = x.GetReason()
		case *errdetails.BadRequest:
			for _, fv := range x.GetFieldViolations() {
				fields = append(fields, fv.GetField())
			}
		}
	}
	return reason, fields
}

type reasonedError struct {
	error
	reason crm.Reason
}

func (e reasonedError) Unwrap() error { return e.error }

func withReason(err error, r crm.Reason) error { return reasonedError{err, r} }

func testReasonOf(err error) crm.Reason {
	var r reasonedError
	if errors.As(err, &r) {
		return r.reason
	}
	return ""
}

type env struct {
	reads  *fakeReads
	writes *fakeWrites
	buyers *fakeBuyers
	stores *fakeStores
	app    *App
}

func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{
		reads: &fakeReads{}, writes: &fakeWrites{}, buyers: &fakeBuyers{},
		stores: &fakeStores{own: &seller.Store{StoreID: callerStoreID, AccountID: callerAccount}},
	}
	var err error
	if e.app, err = NewApp(e.reads, e.writes, e.buyers, e.stores, images(t), slog.New(slog.DiscardHandler)); err != nil {
		t.Fatal(err)
	}
	e.app.reasonOf = testReasonOf
	return e
}

func sampleCustomer(buyerID int64) crm.Customer {
	return crm.Customer{
		StoreID: callerStoreID, BuyerID: buyerID, Type: crm.TypeVIP, TotalSpent: 4200, OrderCount: 3, FirstOrderAt: at,
		Tags: []crm.Tag{{ID: tagA, Name: "常客"}, {ID: tagB, Name: "批發"}},
	}
}

func mask(paths ...string) *fieldmaskpb.FieldMask { return &fieldmaskpb.FieldMask{Paths: paths} }

func calls(e *env, storeID string) map[string]func(context.Context) error {
	return map[string]func(context.Context) error{
		crmv1alpha1connect.CrmAppServiceListCustomersProcedure: func(ctx context.Context) error {
			_, err := e.app.ListCustomers(ctx, connect.NewRequest(&pb.ListCustomersRequest{StoreId: storeID}))
			return err
		},
		crmv1alpha1connect.CrmAppServiceGetCustomerProcedure: func(ctx context.Context) error {
			_, err := e.app.GetCustomer(ctx, connect.NewRequest(&pb.GetCustomerRequest{StoreId: storeID, BuyerId: "70"}))
			return err
		},
		crmv1alpha1connect.CrmAppServiceUpdateCustomerProcedure: func(ctx context.Context) error {
			_, err := e.app.UpdateCustomer(ctx, connect.NewRequest(&pb.UpdateCustomerRequest{
				StoreId: storeID, Customer: &pb.Customer{BuyerId: "70", Type: pb.CustomerType_CUSTOMER_TYPE_VIP}, UpdateMask: mask("type"),
			}))
			return err
		},
		crmv1alpha1connect.CrmAppServiceCreateTagProcedure: func(ctx context.Context) error {
			_, err := e.app.CreateTag(ctx, connect.NewRequest(&pb.CreateTagRequest{StoreId: storeID, Name: "新標籤"}))
			return err
		},
		crmv1alpha1connect.CrmAppServiceListTagsProcedure: func(ctx context.Context) error {
			_, err := e.app.ListTags(ctx, connect.NewRequest(&pb.ListTagsRequest{StoreId: storeID}))
			return err
		},
		crmv1alpha1connect.CrmAppServiceUpdateTagProcedure: func(ctx context.Context) error {
			_, err := e.app.UpdateTag(ctx, connect.NewRequest(&pb.UpdateTagRequest{
				StoreId: storeID, Tag: &pb.Tag{Id: tagA.String(), Name: "改名"}, UpdateMask: mask("name"),
			}))
			return err
		},
		crmv1alpha1connect.CrmAppServiceDeleteTagProcedure: func(ctx context.Context) error {
			_, err := e.app.DeleteTag(ctx, connect.NewRequest(&pb.DeleteTagRequest{StoreId: storeID, TagId: tagA.String()}))
			return err
		},
		crmv1alpha1connect.CrmAppServiceAddCustomerTagsProcedure: func(ctx context.Context) error {
			_, err := e.app.AddCustomerTags(ctx, connect.NewRequest(&pb.AddCustomerTagsRequest{StoreId: storeID, BuyerId: "70", TagNames: []string{"a"}}))
			return err
		},
		crmv1alpha1connect.CrmAppServiceRemoveCustomerTagProcedure: func(ctx context.Context) error {
			_, err := e.app.RemoveCustomerTag(ctx, connect.NewRequest(&pb.RemoveCustomerTagRequest{StoreId: storeID, BuyerId: "70", TagId: tagA.String()}))
			return err
		},
	}
}

func seeded(e *env) {
	nick := "阿美"
	e.buyers.displays = []buyer.Display{{BuyerID: 70, DisplayName: &nick}}
	e.reads.customer = sampleCustomer(70)
	e.reads.tags = []crm.Tag{{ID: tagA, Name: "改名"}}
	e.writes.created = crm.Tag{ID: fresh, Name: "新標籤"}
	e.writes.customer = sampleCustomer(70)
	e.writes.renamed = crm.Tag{ID: tagA, Name: "改名"}
}

func TestEveryMethodIsCoveredByTheCallTable(t *testing.T) {
	e := newEnv(t)
	table := calls(e, "11")
	svc := serviceDescriptor()
	for i := range svc.Methods().Len() {
		procedure := "/" + string(svc.FullName()) + "/" + string(svc.Methods().Get(i).Name())
		if table[procedure] == nil {
			t.Errorf("%s 沒有在授權測試的呼叫表裡", procedure)
		}
	}
	if len(table) != svc.Methods().Len() {
		t.Errorf("呼叫表 %d 列，service %d 支方法", len(table), svc.Methods().Len())
	}
}

func TestEveryMethodRequiresTheCallersOwnStoreAndTouchesNothingOtherwise(t *testing.T) {
	for procedure := range calls(newEnv(t), "11") {
		t.Run(procedure, func(t *testing.T) {
			e := newEnv(t)
			seeded(e)
			if err := calls(e, "11")[procedure](signedIn()); err != nil {
				t.Fatalf("自己的店：%v", err)
			}

			cases := []struct {
				name  string
				store string
				ctx   context.Context
				setup func(*env)
				want  connect.Code
			}{
				{"別人的店", "12", signedIn(), nil, connect.CodePermissionDenied},
				{"沒有店", "11", signedIn(), func(e *env) { e.stores.own = nil }, connect.CodeNotFound},
				{"管理員", "11", asAdmin(), nil, connect.CodePermissionDenied},
				{"店家 ID 不是數字", "abc", signedIn(), nil, connect.CodeInvalidArgument},
				{"沒有店家 ID", "", signedIn(), nil, connect.CodeInvalidArgument},
				{"格式錯誤先於身分檢查", "abc", asAdmin(), nil, connect.CodeInvalidArgument},
			}
			for _, c := range cases {
				t.Run(c.name, func(t *testing.T) {
					e := newEnv(t)
					seeded(e)
					if c.setup != nil {
						c.setup(e)
					}
					err := calls(e, c.store)[procedure](c.ctx)
					if err == nil || codeOf(err) != c.want {
						t.Fatalf("code = %v（%v），預期 %v", codeOf(err), err, c.want)
					}
					if len(e.reads.calls)+len(e.writes.calls) != 0 {
						t.Errorf("被擋下的請求不得碰模組：reads=%v writes=%v", e.reads.calls, e.writes.calls)
					}
				})
			}
		})
	}
}

// 賣家能力由模組檢查（ADR 2026-09-29 的修訂）：handler 對停權的店照樣把請求交給模組，並原樣回報模組的拒絕。
// 停權與前置條件沒滿足是兩種拒絕，各自的狀態碼與原因分開斷言。
func TestSellerCapabilityRejectionsComeFromTheModuleAndKeepTheirReasons(t *testing.T) {
	ready := seller.Conditions{Derived: true, Active: true, NotPendingDeletion: true}
	suspended := seller.Authorize(seller.Store{StoreID: callerStoreID, Conditions: ready}, crm.RequiredCapability)
	pending := seller.Authorize(seller.Store{StoreID: callerStoreID, Sanctions: seller.NoSanctions(), Conditions: seller.Conditions{Derived: true, Active: true}}, crm.RequiredCapability)
	for _, c := range []struct {
		name   string
		err    error
		code   connect.Code
		reason string
	}{
		{"停權", suspended, connect.CodePermissionDenied, "SELLER_SUSPENDED"},
		{"前置條件沒滿足", pending, connect.CodeFailedPrecondition, "SELLER_REQUIREMENT_MISSING"},
	} {
		if c.err == nil {
			t.Fatalf("%s：seller.Authorize 沒有拒絕", c.name)
		}
		for procedure := range calls(newEnv(t), "11") {
			t.Run(c.name+procedure, func(t *testing.T) {
				e := newEnv(t)
				seeded(e)
				e.reads.err, e.writes.err = c.err, c.err
				err := calls(e, "11")[procedure](signedIn())
				if reason, _ := detailsOf(err); err == nil || codeOf(err) != c.code || reason != c.reason {
					t.Fatalf("code = %v reason = %q（%v），預期 %v %q", codeOf(err), reason, err, c.code, c.reason)
				}
				if len(e.reads.calls)+len(e.writes.calls) != 1 {
					t.Errorf("請求要交給模組檢查：reads=%v writes=%v", e.reads.calls, e.writes.calls)
				}
			})
		}
	}
}

// 每個方法的欄位格式檢查都先於身分檢查：管理員（會被 permission_denied 擋）帶壞掉的欄位，拿到的是 invalid_argument，模組沒有被碰。
func TestFieldFormatIsCheckedBeforeIdentityInEveryMethod(t *testing.T) {
	bad := map[string]func(*env) error{
		"ListCustomers 負的 page_size": func(e *env) error {
			_, err := e.app.ListCustomers(asAdmin(), connect.NewRequest(&pb.ListCustomersRequest{StoreId: "11", PageSize: -1}))
			return err
		},
		"ListCustomers 不認得的排序": func(e *env) error {
			_, err := e.app.ListCustomers(asAdmin(), connect.NewRequest(&pb.ListCustomersRequest{StoreId: "11", Sort: pb.CustomerSort(99)}))
			return err
		},
		"GetCustomer 買家 ID": func(e *env) error {
			_, err := e.app.GetCustomer(asAdmin(), connect.NewRequest(&pb.GetCustomerRequest{StoreId: "11", BuyerId: "x"}))
			return err
		},
		"UpdateCustomer 沒有 customer": func(e *env) error {
			_, err := e.app.UpdateCustomer(asAdmin(), connect.NewRequest(&pb.UpdateCustomerRequest{StoreId: "11", UpdateMask: mask("type")}))
			return err
		},
		"UpdateCustomer mask 沒有 type": func(e *env) error {
			_, err := e.app.UpdateCustomer(asAdmin(), connect.NewRequest(&pb.UpdateCustomerRequest{StoreId: "11", Customer: &pb.Customer{BuyerId: "70"}, UpdateMask: mask("display")}))
			return err
		},
		"UpdateTag 沒有 tag": func(e *env) error {
			_, err := e.app.UpdateTag(asAdmin(), connect.NewRequest(&pb.UpdateTagRequest{StoreId: "11", UpdateMask: mask("name")}))
			return err
		},
		"UpdateTag mask 沒有 name": func(e *env) error {
			_, err := e.app.UpdateTag(asAdmin(), connect.NewRequest(&pb.UpdateTagRequest{StoreId: "11", Tag: &pb.Tag{Id: tagA.String()}, UpdateMask: mask("id")}))
			return err
		},
		"DeleteTag 標籤 ID": func(e *env) error {
			_, err := e.app.DeleteTag(asAdmin(), connect.NewRequest(&pb.DeleteTagRequest{StoreId: "11", TagId: "x"}))
			return err
		},
		"AddCustomerTags 買家 ID": func(e *env) error {
			_, err := e.app.AddCustomerTags(asAdmin(), connect.NewRequest(&pb.AddCustomerTagsRequest{StoreId: "11", BuyerId: "x"}))
			return err
		},
		"RemoveCustomerTag 標籤 ID": func(e *env) error {
			_, err := e.app.RemoveCustomerTag(asAdmin(), connect.NewRequest(&pb.RemoveCustomerTagRequest{StoreId: "11", BuyerId: "70", TagId: "x"}))
			return err
		},
		"RemoveCustomerTag 買家 ID": func(e *env) error {
			_, err := e.app.RemoveCustomerTag(asAdmin(), connect.NewRequest(&pb.RemoveCustomerTagRequest{StoreId: "11", BuyerId: "x", TagId: tagA.String()}))
			return err
		},
	}
	for name, call := range bad {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			err := call(e)
			if err == nil || codeOf(err) != connect.CodeInvalidArgument {
				t.Fatalf("code = %v（%v），預期 invalid_argument", codeOf(err), err)
			}
			if len(e.reads.calls)+len(e.writes.calls) != 0 {
				t.Errorf("不得碰模組：reads=%v writes=%v", e.reads.calls, e.writes.calls)
			}
		})
	}
}

func TestStartupFailsWhenTheDeclarationTableDoesNotMatchTheService(t *testing.T) {
	e := newEnv(t)
	short := map[string]seller.Capability{}
	for k, v := range declarations {
		short[k] = v
	}
	delete(short, crmv1alpha1connect.CrmAppServiceDeleteTagProcedure)
	if _, err := newApp(short, e.reads, e.writes, e.buyers, e.stores, images(t), slog.New(slog.DiscardHandler)); err == nil {
		t.Error("缺一列的宣告表不能啟動")
	}
	extra := map[string]seller.Capability{"/hawkr.crm.v1alpha1.CrmAppService/Nope": seller.CapMarketing}
	for k, v := range declarations {
		extra[k] = v
	}
	if _, err := newApp(extra, e.reads, e.writes, e.buyers, e.stores, images(t), slog.New(slog.DiscardHandler)); err == nil {
		t.Error("多一列的宣告表不能啟動")
	}
	bad := map[string]seller.Capability{}
	for k := range declarations {
		bad[k] = "CAP_NOPE"
	}
	if _, err := newApp(bad, e.reads, e.writes, e.buyers, e.stores, images(t), slog.New(slog.DiscardHandler)); err == nil {
		t.Error("不存在的能力不能啟動")
	}
	if crm.RequiredCapability != seller.CapMarketing {
		t.Errorf("模組檢查的能力 %q 不是 CapMarketing", crm.RequiredCapability)
	}
	for _, c := range declarations {
		if c != seller.CapMarketing {
			t.Errorf("CRM 的每個方法都宣告 CapMarketing：%q", c)
		}
	}
}

// 宣告表與模組的檢查一致（對照 social 的 TestOwnerDeclarationsMatchTheModuleChecks）：模組對每個方法都檢查 crm.RequiredCapability
// （integration 的 TestSuspendedStoreIsRejectedByTheModuleForWritesAndReads 斷言九個方法在停權時全被擋）。這裡由每個方法宣告的能力推導「停權時擋或放」，
// 並要求九個方法全是「擋」且關閉的店也擋；宣告改成放行停權的能力（例如 CapContentContract）而模組沒跟著改，這裡就會失敗。
func TestDeclarationsMatchTheModuleChecks(t *testing.T) {
	suspended := seller.Store{StoreID: callerStoreID, Conditions: seller.Conditions{Derived: true, Active: true, NotPendingDeletion: true}}
	closed := seller.Store{StoreID: callerStoreID, Sanctions: seller.NoSanctions(), Conditions: seller.Conditions{Derived: true}}
	for procedure := range calls(newEnv(t), "11") {
		declared, ok := declarations[procedure]
		if !ok {
			t.Errorf("%s 沒有宣告", procedure)
			continue
		}
		if declared != crm.RequiredCapability {
			t.Errorf("%s 宣告 %q，模組檢查 %q", procedure, declared, crm.RequiredCapability)
		}
		if err := seller.Authorize(suspended, declared); err == nil {
			t.Errorf("%s：宣告的 %q 在停權時放行，但模組在停權時擋下全部方法（ADR 2026-09-29 的 CRM 分類）", procedure, declared)
		}
		if err := seller.Authorize(closed, declared); err == nil {
			t.Errorf("%s：關閉的店家必須被擋", procedure)
		}
	}
}

func TestListCustomersMapsEveryFieldAndTheDisplayFromBuyer(t *testing.T) {
	e := newEnv(t)
	name, avatar := "阿美", "avatars/70.jpg"
	e.buyers.displays = []buyer.Display{
		{BuyerID: 70, DisplayName: &name, Avatar: &avatar},
		{BuyerID: 71, Deleted: true},
	}
	e.reads.page = crm.CustomerPage{Customers: []crm.Customer{sampleCustomer(70), sampleCustomer(71), sampleCustomer(72)}}
	resp, err := e.app.ListCustomers(signedIn(), connect.NewRequest(&pb.ListCustomersRequest{StoreId: "11"}))
	if err != nil {
		t.Fatal(err)
	}
	got := resp.Msg.GetCustomers()
	if len(got) != 3 || resp.Msg.GetNextPageToken() != "" {
		t.Fatalf("customers = %d token = %q", len(got), resp.Msg.GetNextPageToken())
	}
	c := got[0]
	if c.GetBuyerId() != "70" || c.GetType() != pb.CustomerType_CUSTOMER_TYPE_VIP || c.GetTotalSpent() != 4200 || c.GetOrderCount() != 3 || !c.GetFirstOrderTime().AsTime().Equal(at) {
		t.Errorf("基本欄位：%v", c)
	}
	if len(c.GetTags()) != 2 || c.GetTags()[0].GetId() != tagA.String() || c.GetTags()[0].GetName() != "常客" || c.GetTags()[1].GetName() != "批發" {
		t.Errorf("標籤：%v", c.GetTags())
	}
	if d := c.GetDisplay(); d.GetNickname() != "阿美" || d.GetDeleted() || d.GetAvatar().GetUriTemplate() != "https://media.example.test/i/{preset}/avatars/70.jpg" {
		t.Errorf("顯示資料：%v", d)
	}
	if d := got[1].GetDisplay(); d.Nickname != nil || d.GetAvatar() != nil || !d.GetDeleted() {
		t.Errorf("已刪除的買家只有 deleted：%v", d)
	}
	if d := got[2].GetDisplay(); d.Nickname != nil || !d.GetDeleted() {
		t.Errorf("buyer 沒有回的買家當成已刪除：%v", d)
	}
	if len(e.buyers.batches) != 1 || !slices.Equal(e.buyers.batches[0], []int64{70, 71, 72}) {
		t.Errorf("顯示資料要一次批次查詢：%v", e.buyers.batches)
	}
}

func TestListCustomersPassesFilterSortAndPaging(t *testing.T) {
	e := newEnv(t)
	typ, tag := pb.CustomerType_CUSTOMER_TYPE_VIP, tagA.String()
	req := &pb.ListCustomersRequest{
		StoreId: "11", Type: &typ, TagId: &tag, Sort: pb.CustomerSort_CUSTOMER_SORT_TOTAL_SPENT, Descending: true, PageSize: 7,
	}
	if _, err := e.app.ListCustomers(signedIn(), connect.NewRequest(req)); err != nil {
		t.Fatal(err)
	}
	in := e.reads.listIn
	if in.StoreID != 11 || in.Type == nil || *in.Type != crm.TypeVIP || in.TagID == nil || *in.TagID != tagA || in.Sort != crm.SortTotalSpent || !in.Desc || in.Size != 7 || in.After != nil {
		t.Errorf("輸入：%+v", in)
	}
	for _, c := range []struct {
		sort pb.CustomerSort
		want crm.SortBy
	}{
		{pb.CustomerSort_CUSTOMER_SORT_UNSPECIFIED, crm.SortFirstOrderAt},
		{pb.CustomerSort_CUSTOMER_SORT_FIRST_ORDER_TIME, crm.SortFirstOrderAt},
		{pb.CustomerSort_CUSTOMER_SORT_TOTAL_SPENT, crm.SortTotalSpent},
		{pb.CustomerSort_CUSTOMER_SORT_ORDER_COUNT, crm.SortOrderCount},
	} {
		if _, err := e.app.ListCustomers(signedIn(), connect.NewRequest(&pb.ListCustomersRequest{StoreId: "11", Sort: c.sort})); err != nil || e.reads.listIn.Sort != c.want {
			t.Errorf("%v → %v（%v）", c.sort, e.reads.listIn.Sort, err)
		}
		if e.reads.listIn.Type != nil || e.reads.listIn.TagID != nil || e.reads.listIn.Desc {
			t.Errorf("沒有設定的篩選不傳：%+v", e.reads.listIn)
		}
	}
}

func TestListCustomersRejectsBadFilters(t *testing.T) {
	e := newEnv(t)
	unspecified, badTag := pb.CustomerType_CUSTOMER_TYPE_UNSPECIFIED, "not-a-uuid"
	unknownType := pb.CustomerType(99)
	for name, req := range map[string]*pb.ListCustomersRequest{
		"明確送出 UNSPECIFIED 的分類": {StoreId: "11", Type: &unspecified},
		"不認得的分類":               {StoreId: "11", Type: &unknownType},
		"標籤 ID 不是 UUID":        {StoreId: "11", TagId: &badTag},
		"不認得的排序":               {StoreId: "11", Sort: pb.CustomerSort(99)},
		"負的 page_size":         {StoreId: "11", PageSize: -1},
	} {
		_, err := e.app.ListCustomers(signedIn(), connect.NewRequest(req))
		if err == nil || codeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s：%v", name, err)
		}
		if name == "負的 page_size" {
			if _, fields := detailsOf(err); !slices.Equal(fields, []string{"page_size"}) {
				t.Errorf("page_size 的欄位路徑：%v", fields)
			}
		}
	}
	if len(e.reads.calls) != 0 {
		t.Errorf("不合法的請求不得查詢：%v", e.reads.calls)
	}
}

func TestListCustomersPageTokenRoundTripsAndOnlyFitsTheSameStoreFilterAndSort(t *testing.T) {
	e := newEnv(t)
	e.reads.page = crm.CustomerPage{
		Customers: []crm.Customer{sampleCustomer(70)},
		Next:      &crm.CustomerCursor{Number: 4200, At: at, BuyerID: 70},
	}
	base := &pb.ListCustomersRequest{StoreId: "11", Sort: pb.CustomerSort_CUSTOMER_SORT_TOTAL_SPENT, Descending: true}
	resp, err := e.app.ListCustomers(signedIn(), connect.NewRequest(base))
	if err != nil {
		t.Fatal(err)
	}
	token := resp.Msg.GetNextPageToken()
	if token == "" {
		t.Fatal("有下一頁要帶 token")
	}
	next := &pb.ListCustomersRequest{StoreId: "11", Sort: base.Sort, Descending: true, PageToken: token}
	if _, err := e.app.ListCustomers(signedIn(), connect.NewRequest(next)); err != nil {
		t.Fatal(err)
	}
	if a := e.reads.listIn.After; a == nil || a.Number != 4200 || !a.At.Equal(at) || a.BuyerID != 70 {
		t.Errorf("游標沒有帶回模組：%+v", a)
	}
	vip := pb.CustomerType_CUSTOMER_TYPE_VIP
	tag := tagA.String()
	for name, mutate := range map[string]func(*pb.ListCustomersRequest){
		"換排序欄位": func(r *pb.ListCustomersRequest) { r.Sort = pb.CustomerSort_CUSTOMER_SORT_ORDER_COUNT },
		"換方向":   func(r *pb.ListCustomersRequest) { r.Descending = false },
		"換分類篩選": func(r *pb.ListCustomersRequest) { r.Type = &vip },
		"換標籤篩選": func(r *pb.ListCustomersRequest) { r.TagId = &tag },
	} {
		r := &pb.ListCustomersRequest{StoreId: "11", Sort: base.Sort, Descending: true, PageToken: token}
		mutate(r)
		if _, err := e.app.ListCustomers(signedIn(), connect.NewRequest(r)); err == nil || codeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s 不能沿用 token：%v", name, err)
		}
	}
	if _, err := e.app.ListCustomers(signedIn(), connect.NewRequest(&pb.ListCustomersRequest{StoreId: "11", PageToken: "garbage"})); err == nil || codeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("亂填的 token：%v", err)
	}
}

func TestGetCustomerReadsTheStoresCustomerAndMapsNotFound(t *testing.T) {
	e := newEnv(t)
	seeded(e)
	resp, err := e.app.GetCustomer(signedIn(), connect.NewRequest(&pb.GetCustomerRequest{StoreId: "11", BuyerId: "70"}))
	if err != nil {
		t.Fatal(err)
	}
	if e.reads.gotStore != 11 || e.reads.gotBuyer != 70 || resp.Msg.GetCustomer().GetBuyerId() != "70" || len(resp.Msg.GetCustomer().GetTags()) != 2 {
		t.Errorf("store=%d buyer=%d customer=%v", e.reads.gotStore, e.reads.gotBuyer, resp.Msg.GetCustomer())
	}
	e.reads.err = apperr.NotFound("none")
	if _, err := e.app.GetCustomer(signedIn(), connect.NewRequest(&pb.GetCustomerRequest{StoreId: "11", BuyerId: "70"})); codeOf(err) != connect.CodeNotFound {
		t.Errorf("沒有這位客戶：%v", err)
	}
	if _, err := e.app.GetCustomer(signedIn(), connect.NewRequest(&pb.GetCustomerRequest{StoreId: "11", BuyerId: "x"})); codeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("買家 ID 不合法：%v", err)
	}
}

func TestUpdateCustomerSetsOnlyTheType(t *testing.T) {
	e := newEnv(t)
	seeded(e)
	req := &pb.UpdateCustomerRequest{StoreId: "11", Customer: &pb.Customer{BuyerId: "70", Type: pb.CustomerType_CUSTOMER_TYPE_NORMAL}, UpdateMask: mask("type")}
	resp, err := e.app.UpdateCustomer(signedIn(), connect.NewRequest(req))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(e.writes.calls, []string{"SetCustomerType"}) || e.writes.storeID != 11 || e.writes.buyerID != 70 || e.writes.typ != crm.TypeNormal {
		t.Errorf("寫入：%v %+v", e.writes.calls, e.writes)
	}
	if resp.Msg.GetCustomer().GetBuyerId() != "70" || len(e.reads.calls) != 0 {
		t.Errorf("回應要是寫入回傳的客戶、不在 commit 之後另外讀：%v %v", resp.Msg.GetCustomer(), e.reads.calls)
	}
	for _, m := range []*fieldmaskpb.FieldMask{mask("*"), mask("type", "display")} {
		e.writes.calls, e.writes.typ = nil, ""
		req.UpdateMask = m
		req.Customer.Type = pb.CustomerType_CUSTOMER_TYPE_VIP
		if _, err := e.app.UpdateCustomer(signedIn(), connect.NewRequest(req)); err != nil || e.writes.typ != crm.TypeVIP {
			t.Errorf("mask %v：typ=%q err=%v", m.GetPaths(), e.writes.typ, err)
		}
	}
}

// 契約：客戶的可見性由模組判斷（寫入前、讀取前回 not_found）；handler 把模組的 not_found 原樣對應，不依 buyer.BatchDisplay 另外判斷。
func TestSingleCustomerResponsesPassTheModulesNotFoundThrough(t *testing.T) {
	notFound := func() error { return apperr.NotFound("%s", "這家店沒有這位客戶") }
	wantCode, wantMsg := codeOf(notFound()), rpc.ConnectError(context.Background(), notFound()).Message()
	for procedure := range calls(newEnv(t), "11") {
		switch procedure {
		case crmv1alpha1connect.CrmAppServiceGetCustomerProcedure, crmv1alpha1connect.CrmAppServiceUpdateCustomerProcedure,
			crmv1alpha1connect.CrmAppServiceAddCustomerTagsProcedure, crmv1alpha1connect.CrmAppServiceRemoveCustomerTagProcedure:
		default:
			continue
		}
		t.Run(procedure, func(t *testing.T) {
			e := newEnv(t)
			seeded(e)
			e.reads.err, e.writes.err = notFound(), notFound()
			err := calls(e, "11")[procedure](signedIn())
			if err == nil || codeOf(err) != connect.CodeNotFound || wantCode != connect.CodeNotFound {
				t.Fatalf("code = %v（%v），預期 not_found", codeOf(err), err)
			}
			if got := rpc.ConnectError(context.Background(), err).Message(); got != wantMsg {
				t.Errorf("訊息 %q，預期 %q", got, wantMsg)
			}
		})
	}
}

// 模組回了客戶（可見性已由模組判斷）時，buyer 標示已刪除只影響 display.deleted，不改變回應成功與否。
func TestDeletedDisplayOnlyMapsToDisplayDeleted(t *testing.T) {
	e := newEnv(t)
	seeded(e)
	e.buyers.displays = []buyer.Display{{BuyerID: 70, Deleted: true}}
	res, err := e.app.GetCustomer(signedIn(), connect.NewRequest(&pb.GetCustomerRequest{StoreId: "11", BuyerId: "70"}))
	if err != nil || !res.Msg.GetCustomer().GetDisplay().GetDeleted() {
		t.Fatalf("res=%v err=%v，預期成功且 display.deleted", res, err)
	}
}

func TestUpdateCustomerRejectsAnyMaskThatIsNotType(t *testing.T) {
	cust := func() *pb.Customer {
		return &pb.Customer{BuyerId: "70", Type: pb.CustomerType_CUSTOMER_TYPE_VIP, TotalSpent: 1, OrderCount: 1}
	}
	for name, m := range map[string]*fieldmaskpb.FieldMask{
		"累計金額":       mask("total_spent"),
		"訂單數":        mask("order_count"),
		"標籤":         mask("tags"),
		"顯示資料":       mask("display"),
		"買家 ID":      mask("buyer_id"),
		"不存在的欄位":     mask("phone"),
		"空的 mask 路徑": mask(),
	} {
		e := newEnv(t)
		seeded(e)
		req := &pb.UpdateCustomerRequest{StoreId: "11", Customer: cust(), UpdateMask: m}
		if name == "空的 mask 路徑" {
			req.Customer = &pb.Customer{BuyerId: "70"}
		}
		_, err := e.app.UpdateCustomer(signedIn(), connect.NewRequest(req))
		if err == nil || codeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s：%v", name, err)
		}
		if len(e.writes.calls) != 0 {
			t.Errorf("%s：不得寫入 %v", name, e.writes.calls)
		}
	}
	e := newEnv(t)
	if _, err := e.app.UpdateCustomer(signedIn(), connect.NewRequest(&pb.UpdateCustomerRequest{StoreId: "11", UpdateMask: mask("type")})); err == nil || codeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("沒有 customer：%v", err)
	}
}

func TestUpdateCustomerPassesAnUnspecifiedTypeToTheModuleWhichRejectsItOnTheTypeField(t *testing.T) {
	e := newEnv(t)
	seeded(e)
	e.writes.err = apperr.InvalidInputOn([]string{"CustomerType"}, "bad type")
	req := &pb.UpdateCustomerRequest{StoreId: "11", Customer: &pb.Customer{BuyerId: "70"}, UpdateMask: mask("type")}
	_, err := e.app.UpdateCustomer(signedIn(), connect.NewRequest(req))
	if codeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("code = %v", codeOf(err))
	}
	if e.writes.typ != "" {
		t.Errorf("UNSPECIFIED 不得被當成合法分類傳下去：%q", e.writes.typ)
	}
	if _, fields := detailsOf(err); !slices.Equal(fields, []string{"customer.type"}) {
		t.Errorf("欄位路徑：%v", fields)
	}
}

func TestCreateTagMapsTheResultTheNameFieldAndBothReasons(t *testing.T) {
	e := newEnv(t)
	seeded(e)
	resp, err := e.app.CreateTag(signedIn(), connect.NewRequest(&pb.CreateTagRequest{StoreId: "11", Name: " 新標籤 "}))
	if err != nil {
		t.Fatal(err)
	}
	if e.writes.storeID != 11 || e.writes.name != " 新標籤 " || resp.Msg.GetTag().GetId() != fresh.String() || resp.Msg.GetTag().GetName() != "新標籤" {
		t.Errorf("name=%q tag=%v", e.writes.name, resp.Msg.GetTag())
	}
	for _, c := range []struct {
		err    error
		code   connect.Code
		reason string
	}{
		{withReason(apperr.AlreadyExists("taken"), crm.ReasonTagNameTaken), connect.CodeAlreadyExists, "CRM_TAG_NAME_TAKEN"},
		{withReason(apperr.FailedPrecondition("limit"), crm.ReasonTagLimit), connect.CodeFailedPrecondition, "CRM_TAG_LIMIT"},
	} {
		e.writes.err = c.err
		_, err := e.app.CreateTag(signedIn(), connect.NewRequest(&pb.CreateTagRequest{StoreId: "11", Name: "x"}))
		if reason, _ := detailsOf(err); codeOf(err) != c.code || reason != c.reason {
			t.Errorf("%v → code=%v reason=%q", c.err, codeOf(err), reason)
		}
	}
	e.writes.err = apperr.InvalidInputOn([]string{"Name"}, "bad")
	_, err = e.app.CreateTag(signedIn(), connect.NewRequest(&pb.CreateTagRequest{StoreId: "11", Name: ""}))
	if _, fields := detailsOf(err); codeOf(err) != connect.CodeInvalidArgument || !slices.Equal(fields, []string{"name"}) {
		t.Errorf("名稱不合法：%v %v", codeOf(err), fields)
	}
}

func TestListTagsReturnsTheStoresTags(t *testing.T) {
	e := newEnv(t)
	e.reads.tags = []crm.Tag{{ID: tagA, Name: "A"}, {ID: tagB, Name: "B"}}
	resp, err := e.app.ListTags(signedIn(), connect.NewRequest(&pb.ListTagsRequest{StoreId: "11"}))
	if err != nil {
		t.Fatal(err)
	}
	got := resp.Msg.GetTags()
	if e.reads.gotStore != 11 || len(got) != 2 || got[0].GetId() != tagA.String() || got[0].GetName() != "A" || got[1].GetName() != "B" {
		t.Errorf("store=%d tags=%v", e.reads.gotStore, got)
	}
	e.reads.tags = nil
	resp, err = e.app.ListTags(signedIn(), connect.NewRequest(&pb.ListTagsRequest{StoreId: "11"}))
	if err != nil || len(resp.Msg.GetTags()) != 0 {
		t.Errorf("沒有標籤是空清單：%v %v", resp, err)
	}
}

func TestUpdateTagRenamesAndReturnsTheWrittenTag(t *testing.T) {
	e := newEnv(t)
	e.writes.renamed = crm.Tag{ID: tagA, Name: "改名"}
	resp, err := e.app.UpdateTag(signedIn(), connect.NewRequest(&pb.UpdateTagRequest{
		StoreId: "11", Tag: &pb.Tag{Id: tagA.String(), Name: "  改名  "}, UpdateMask: mask("name"),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if e.writes.storeID != 11 || e.writes.tagID != tagA || e.writes.name != "  改名  " {
		t.Errorf("寫入：%+v", e.writes)
	}
	if resp.Msg.GetTag().GetId() != tagA.String() || resp.Msg.GetTag().GetName() != "改名" {
		t.Errorf("回應要是寫入回傳的標籤：%v", resp.Msg.GetTag())
	}
	if len(e.reads.calls) != 0 {
		t.Errorf("不在 commit 之後另外讀：%v", e.reads.calls)
	}
}

func TestUpdateTagRejectsMasksOtherThanNameAndMapsErrors(t *testing.T) {
	for name, m := range map[string]*fieldmaskpb.FieldMask{"id": mask("id"), "空": mask(), "不存在的欄位": mask("color")} {
		e := newEnv(t)
		req := &pb.UpdateTagRequest{StoreId: "11", Tag: &pb.Tag{Id: tagA.String()}, UpdateMask: m}
		_, err := e.app.UpdateTag(signedIn(), connect.NewRequest(req))
		if err == nil || codeOf(err) != connect.CodeInvalidArgument || len(e.writes.calls) != 0 {
			t.Errorf("%s：%v %v", name, err, e.writes.calls)
		}
	}
	e := newEnv(t)
	for name, req := range map[string]*pb.UpdateTagRequest{
		"沒有 tag":   {StoreId: "11", UpdateMask: mask("name")},
		"標籤 ID 壞掉": {StoreId: "11", Tag: &pb.Tag{Id: "x"}, UpdateMask: mask("name")},
	} {
		if _, err := e.app.UpdateTag(signedIn(), connect.NewRequest(req)); codeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s：%v", name, err)
		}
	}
	e.writes.err = withReason(apperr.AlreadyExists("taken"), crm.ReasonTagNameTaken)
	_, err := e.app.UpdateTag(signedIn(), connect.NewRequest(&pb.UpdateTagRequest{StoreId: "11", Tag: &pb.Tag{Id: tagA.String(), Name: "x"}, UpdateMask: mask("name")}))
	if reason, _ := detailsOf(err); codeOf(err) != connect.CodeAlreadyExists || reason != "CRM_TAG_NAME_TAKEN" {
		t.Errorf("撞名：%v %q", codeOf(err), reason)
	}
	e.writes.err = apperr.InvalidInputOn([]string{"Name"}, "bad")
	_, err = e.app.UpdateTag(signedIn(), connect.NewRequest(&pb.UpdateTagRequest{StoreId: "11", Tag: &pb.Tag{Id: tagA.String()}, UpdateMask: mask("name")}))
	if _, fields := detailsOf(err); !slices.Equal(fields, []string{"tag.name"}) {
		t.Errorf("名稱欄位路徑：%v", fields)
	}
	e.writes.err = apperr.NotFound("none")
	if _, err = e.app.UpdateTag(signedIn(), connect.NewRequest(&pb.UpdateTagRequest{StoreId: "11", Tag: &pb.Tag{Id: tagA.String()}, UpdateMask: mask("name")})); codeOf(err) != connect.CodeNotFound {
		t.Errorf("標籤不存在：%v", err)
	}
}

func TestDeleteTagPassesTheStoreAndTagAndMapsNotFound(t *testing.T) {
	e := newEnv(t)
	if _, err := e.app.DeleteTag(signedIn(), connect.NewRequest(&pb.DeleteTagRequest{StoreId: "11", TagId: tagA.String()})); err != nil {
		t.Fatal(err)
	}
	if e.writes.storeID != 11 || e.writes.tagID != tagA {
		t.Errorf("寫入：%+v", e.writes)
	}
	if _, err := e.app.DeleteTag(signedIn(), connect.NewRequest(&pb.DeleteTagRequest{StoreId: "11", TagId: "x"})); codeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("標籤 ID 壞掉：%v", err)
	}
	e.writes.err = apperr.NotFound("none")
	if _, err := e.app.DeleteTag(signedIn(), connect.NewRequest(&pb.DeleteTagRequest{StoreId: "11", TagId: tagA.String()})); codeOf(err) != connect.CodeNotFound {
		t.Errorf("標籤不存在：%v", err)
	}
}

func TestAddCustomerTagsPassesTheNamesReturnsTheCustomerAndMapsErrors(t *testing.T) {
	e := newEnv(t)
	seeded(e)
	resp, err := e.app.AddCustomerTags(signedIn(), connect.NewRequest(&pb.AddCustomerTagsRequest{StoreId: "11", BuyerId: "70", TagNames: []string{"常客", "批發"}}))
	if err != nil {
		t.Fatal(err)
	}
	if e.writes.storeID != 11 || e.writes.buyerID != 70 || !slices.Equal(e.writes.names, []string{"常客", "批發"}) {
		t.Errorf("寫入：%+v", e.writes)
	}
	if len(resp.Msg.GetCustomer().GetTags()) != 2 || len(e.reads.calls) != 0 {
		t.Errorf("回應要是寫入回傳的客戶、不在 commit 之後另外讀：%v %v", resp.Msg.GetCustomer(), e.reads.calls)
	}

	e.writes.err = withReason(apperr.FailedPrecondition("limit"), crm.ReasonTagLimit)
	_, err = e.app.AddCustomerTags(signedIn(), connect.NewRequest(&pb.AddCustomerTagsRequest{StoreId: "11", BuyerId: "70", TagNames: []string{"a"}}))
	if reason, _ := detailsOf(err); codeOf(err) != connect.CodeFailedPrecondition || reason != "CRM_TAG_LIMIT" {
		t.Errorf("上限：%v %q", codeOf(err), reason)
	}
	e.writes.err = apperr.InvalidInputOn([]string{apperr.IndexedField("Names", 1)}, "bad")
	_, err = e.app.AddCustomerTags(signedIn(), connect.NewRequest(&pb.AddCustomerTagsRequest{StoreId: "11", BuyerId: "70", TagNames: []string{"a", ""}}))
	if _, fields := detailsOf(err); !slices.Equal(fields, []string{"tag_names[1]"}) {
		t.Errorf("欄位路徑：%v", fields)
	}
	e.writes.err = apperr.NotFound("none")
	if _, err = e.app.AddCustomerTags(signedIn(), connect.NewRequest(&pb.AddCustomerTagsRequest{StoreId: "11", BuyerId: "70", TagNames: []string{"a"}})); codeOf(err) != connect.CodeNotFound {
		t.Errorf("客戶不存在：%v", err)
	}
	if _, err = e.app.AddCustomerTags(signedIn(), connect.NewRequest(&pb.AddCustomerTagsRequest{StoreId: "11", BuyerId: "x", TagNames: []string{"a"}})); codeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("買家 ID 壞掉：%v", err)
	}
}

func TestRemoveCustomerTagPassesIdsReturnsTheCustomerAndMapsErrors(t *testing.T) {
	e := newEnv(t)
	seeded(e)
	resp, err := e.app.RemoveCustomerTag(signedIn(), connect.NewRequest(&pb.RemoveCustomerTagRequest{StoreId: "11", BuyerId: "70", TagId: tagB.String()}))
	if err != nil {
		t.Fatal(err)
	}
	if e.writes.storeID != 11 || e.writes.buyerID != 70 || e.writes.tagID != tagB || resp.Msg.GetCustomer().GetBuyerId() != "70" || len(e.reads.calls) != 0 {
		t.Errorf("寫入：%+v 回應：%v", e.writes, resp.Msg.GetCustomer())
	}
	if _, err = e.app.RemoveCustomerTag(signedIn(), connect.NewRequest(&pb.RemoveCustomerTagRequest{StoreId: "11", BuyerId: "70", TagId: "x"})); codeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("標籤 ID 壞掉：%v", err)
	}
	e.writes.err = apperr.NotFound("none")
	if _, err = e.app.RemoveCustomerTag(signedIn(), connect.NewRequest(&pb.RemoveCustomerTagRequest{StoreId: "11", BuyerId: "70", TagId: tagB.String()})); codeOf(err) != connect.CodeNotFound {
		t.Errorf("客戶不存在：%v", err)
	}
}

func TestCustomerTypeAndSortEnumsMatchTheModule(t *testing.T) {
	if m := rpc.EnumMismatches[pb.CustomerType]([]crm.CustomerType{crm.TypeNormal, crm.TypeVIP}); len(m) != 0 {
		t.Error(strings.Join(m, "\n"))
	}
	var got []crm.SortBy
	vals := pb.CustomerSort(0).Descriptor().Values()
	for i := range vals.Len() {
		s, err := sortOf(pb.CustomerSort(vals.Get(i).Number()))
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(got, s) {
			got = append(got, s)
		}
	}
	slices.Sort(got)
	if !slices.Equal(got, []crm.SortBy{crm.SortFirstOrderAt, crm.SortTotalSpent, crm.SortOrderCount}) {
		t.Errorf("排序列舉要涵蓋模組的每個排序欄位：%v", got)
	}
}

var contactTokens = []string{"phone", "mobile", "tel", "telephone", "email", "mail", "address", "addr", "street", "postal", "zip", "contact"}

func contactLike(name protoreflect.Name) bool {
	for _, tok := range strings.Split(strings.ToLower(string(name)), "_") {
		if slices.Contains(contactTokens, tok) {
			return true
		}
	}
	return false
}

func fieldNames(md protoreflect.MessageDescriptor) []string {
	var out []string
	for i := range md.Fields().Len() {
		out = append(out, string(md.Fields().Get(i).Name()))
	}
	return out
}

var responseFields = map[protoreflect.FullName][]string{
	"hawkr.crm.v1alpha1.ListCustomersResponse":     {"customers", "next_page_token"},
	"hawkr.crm.v1alpha1.GetCustomerResponse":       {"customer"},
	"hawkr.crm.v1alpha1.UpdateCustomerResponse":    {"customer"},
	"hawkr.crm.v1alpha1.CreateTagResponse":         {"tag"},
	"hawkr.crm.v1alpha1.ListTagsResponse":          {"tags"},
	"hawkr.crm.v1alpha1.UpdateTagResponse":         {"tag"},
	"hawkr.crm.v1alpha1.DeleteTagResponse":         {},
	"hawkr.crm.v1alpha1.AddCustomerTagsResponse":   {"customer"},
	"hawkr.crm.v1alpha1.RemoveCustomerTagResponse": {"customer"},
	"hawkr.crm.v1alpha1.Customer":                  {"buyer_id", "display", "type", "total_spent", "order_count", "first_order_time", "tags"},
	"hawkr.crm.v1alpha1.CustomerDisplay":           {"nickname", "avatar", "deleted"},
	"hawkr.crm.v1alpha1.Tag":                       {"id", "name"},
	"hawkr.common.v1alpha1.Image":                  {"uri_template"},
	"google.protobuf.Timestamp":                    {"seconds", "nanos"},
}

func walk(md protoreflect.MessageDescriptor, path string, allowed map[protoreflect.FullName][]string, seen map[protoreflect.FullName]bool) []string {
	if seen[md.FullName()] {
		return nil
	}
	seen[md.FullName()] = true
	var problems []string
	want, ok := allowed[md.FullName()]
	switch {
	case !ok:
		problems = append(problems, path+"："+string(md.FullName())+" 不在欄位白名單（CD8：新的回應訊息要先確認沒有個資）")
	case !slices.Equal(fieldNames(md), want):
		problems = append(problems, path+"："+string(md.FullName())+" 的欄位是 "+strings.Join(fieldNames(md), ",")+"，白名單是 "+strings.Join(want, ",")+"（新增欄位要先確認不是個資，再改白名單）")
	}
	for i := range md.Fields().Len() {
		fd := md.Fields().Get(i)
		if contactLike(fd.Name()) {
			problems = append(problems, path+"."+string(fd.Name())+" 是聯絡資料類的欄位名（CD8：CRM 不回電話、Email、地址）")
		}
		if fd.Message() != nil {
			problems = append(problems, walk(fd.Message(), path+"."+string(fd.Name()), allowed, seen)...)
		}
	}
	return problems
}

func TestCustomerAndEveryCrmResponseCarryNoContactFields(t *testing.T) {
	svc := serviceDescriptor()
	seen := map[protoreflect.FullName]bool{}
	for i := range svc.Methods().Len() {
		out := svc.Methods().Get(i).Output()
		for _, p := range walk(out, string(out.Name()), responseFields, seen) {
			t.Error(p)
		}
	}
	for _, name := range []protoreflect.FullName{"hawkr.crm.v1alpha1.Customer", "hawkr.crm.v1alpha1.CustomerDisplay", "hawkr.crm.v1alpha1.Tag", "hawkr.common.v1alpha1.Image"} {
		if !seen[name] {
			t.Errorf("遞迴沒有走到 %s：%v", name, seen)
		}
	}
	for name := range responseFields {
		if !seen[name] {
			t.Errorf("白名單裡的 %s 沒有被走訪到（白名單過期）", name)
		}
	}
}

// 走訪本身的自我測試，用只在測試裡建立的 descriptor：一個回應訊息多了 buyer_cellphone（不在 contactTokens 的詞表裡），只有白名單抓得到。
func TestTheWalkFlagsAnUnlistedContactFieldThatTheNameTokensMiss(t *testing.T) {
	str := descriptorpb.FieldDescriptorProto_TYPE_STRING
	opt := descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL
	fd, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
		Name: proto.String("walk_selftest.proto"), Package: proto.String("selftest"), Syntax: proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{{
			Name: proto.String("GetThing"),
			Field: []*descriptorpb.FieldDescriptorProto{
				{Name: proto.String("customer"), Number: proto.Int32(1), Type: &str, Label: &opt},
				{Name: proto.String("buyer_cellphone"), Number: proto.Int32(2), Type: &str, Label: &opt},
			},
		}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	md := fd.Messages().ByName("GetThing")
	if contactLike("buyer_cellphone") {
		t.Fatal("前提：cellphone 不在詞表裡，名稱檢查抓不到")
	}
	got := walk(md, "GetThing", map[protoreflect.FullName][]string{"selftest.GetThing": {"customer"}}, map[protoreflect.FullName]bool{})
	if len(got) != 1 || !strings.Contains(got[0], "buyer_cellphone") {
		t.Errorf("白名單要抓到多出來的 buyer_cellphone：%v", got)
	}
	if got := walk(md, "GetThing", map[protoreflect.FullName][]string{"selftest.GetThing": {"customer", "buyer_cellphone"}}, map[protoreflect.FullName]bool{}); len(got) != 0 {
		t.Errorf("白名單列了就不報：%v", got)
	}
	if got := walk(md, "GetThing", map[protoreflect.FullName][]string{}, map[protoreflect.FullName]bool{}); len(got) == 0 {
		t.Error("不在白名單的訊息要報")
	}
}

func TestContactLikeNamesAreRecognizedAndOrdinaryNamesAreNot(t *testing.T) {
	for _, bad := range []string{"phone", "phone_number", "buyer_email", "shipping_address", "mobile", "contact_name"} {
		if !contactLike(protoreflect.Name(bad)) {
			t.Errorf("%q 應被視為聯絡資料欄位", bad)
		}
	}
	for _, ok := range []string{"buyer_id", "nickname", "total_spent", "hotel", "first_order_time", "tags"} {
		if contactLike(protoreflect.Name(ok)) {
			t.Errorf("%q 不是聯絡資料欄位", ok)
		}
	}
}

func TestFieldPathsPointAtRealRequestFields(t *testing.T) {
	file := pb.File_hawkr_crm_v1alpha1_crm_proto.Messages()
	for name, c := range map[string]struct {
		paths rpc.FieldPaths
		req   string
	}{
		"list":   {listPaths, "ListCustomersRequest"},
		"update": {updatePaths, "UpdateCustomerRequest"},
		"create": {createPaths, "CreateTagRequest"},
		"rename": {renamePaths, "UpdateTagRequest"},
		"add":    {addTagsPaths, "AddCustomerTagsRequest"},
	} {
		if missing := rpc.CheckFieldPaths(file.ByName(protoreflect.Name(c.req)), c.paths); len(missing) != 0 {
			t.Errorf("%s：請求 %s 沒有這些欄位 %v", name, c.req, missing)
		}
	}
}
