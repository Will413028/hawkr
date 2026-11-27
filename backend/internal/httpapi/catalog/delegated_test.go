package catalog

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	httpdelegation "github.com/Will413028/hawkr/backend/internal/httpapi/delegation"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/catalog/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/catalog/v1alpha1/catalogv1alpha1connect"
	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
	moddelegation "github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/delegatedactor"
)

const (
	goodCredential = "good-session"
	sessionStore   = int64(5)
	sessionID      = int64(9)
	sessionAdmin   = int64(31)
)

var allProductCaps = []moddelegation.Capability{moddelegation.CapabilityProductsRead, moddelegation.CapabilityProductsWrite, moddelegation.CapabilityProductsPublish}

// fakeSessions 以憑證回 session；AuthorizeDelegated 以與模組相同的規則（非 ACTIVE 或缺能力）拒絕，並記下被拒絕的呼叫（FAILED 稽核由模組的 integration 測試守）。
type fakeSessions struct {
	sessions map[string]moddelegation.DelegationSession
	denied   []string
}

func (f *fakeSessions) TouchDelegation(context.Context, int64) error { return nil }

func (f *fakeSessions) ResolveDelegation(_ context.Context, c string, _ bool) (moddelegation.DelegationSession, error) {
	s, ok := f.sessions[c]
	if !ok {
		return moddelegation.DelegationSession{}, apperr.Unauthenticated("代行憑證無效")
	}
	return s, nil
}

func (f *fakeSessions) AuthorizeDelegated(_ context.Context, c moddelegation.DelegatedCaller) error {
	if !slices.Contains(c.Session.Capabilities, c.Method.Capability) {
		f.denied = append(f.denied, string(c.Method.Action))
		return apperr.PermissionDenied("此工作階段未取得執行該操作的能力")
	}
	return nil
}

func sessionWith(caps ...moddelegation.Capability) moddelegation.DelegationSession {
	return moddelegation.DelegationSession{
		ID: sessionID, Status: moddelegation.DelegationActive, Capabilities: caps,
		Actor:  moddelegation.DelegationActor{AdminID: sessionAdmin},
		Target: moddelegation.DelegationTarget{StoreID: sessionStore},
	}
}

// fakeRunner 像元件一樣先執行 read 與 do，但不寫稽核也不做冪等（那是模組的規則，見 delegation 的 integration 測試）。
type fakeRunner struct {
	write    moddelegation.DelegatedWrite
	replayed bool
	replayID string
	items    []moddelegation.AuditItem
	audit    uuid.UUID
}

func (f *fakeRunner) DelegatedRead(ctx context.Context, c moddelegation.DelegatedCaller, read func(context.Context, seller.Store) error) error {
	return read(ctx, seller.Store{StoreID: c.Session.Target.StoreID})
}

func (f *fakeRunner) DelegatedWrite(ctx context.Context, c moddelegation.DelegatedCaller, w moddelegation.DelegatedWrite,
	do func(context.Context, seller.Store, *delegatedactor.Actor) error) (moddelegation.DelegatedWriteResult, error) {
	f.write = w
	st := seller.Store{StoreID: c.Session.Target.StoreID}
	if f.replayed {
		return moddelegation.DelegatedWriteResult{Store: st, AuditLogID: f.audit, ResourceID: f.replayID, Items: f.items, Replayed: true}, nil
	}
	if err := do(ctx, st, &delegatedactor.Actor{SessionID: c.Session.ID, StoreID: st.StoreID}); err != nil {
		return moddelegation.DelegatedWriteResult{}, err
	}
	return moddelegation.DelegatedWriteResult{Store: st, AuditLogID: f.audit}, nil
}

// fakeDelegatedModule 記錄 catalog 模組收到的店家、上傳者、版本、輸入與代行者。
type fakeDelegatedModule struct {
	calls     []string
	stores    []int64
	uploaders []media.Uploader
	versions  []int64
	inputs    []catalog.ProductInput
	actors    []*delegatedactor.Actor
	requestID uuid.UUID
	statuses  []catalog.ProductStatus
	batch     []catalog.VersionedProduct
	products  map[uuid.UUID]catalog.Product
	product   catalog.Product
	page      catalog.ProductPage
	err       error
	listSize  int32
}

func (f *fakeDelegatedModule) rec(name string, store int64, actor *delegatedactor.Actor) {
	f.calls = append(f.calls, name)
	f.stores = append(f.stores, store)
	f.actors = append(f.actors, actor)
}

func (f *fakeDelegatedModule) ListSellerProducts(_ context.Context, storeID int64, _ catalog.SellerFilter, _ *catalog.ListCursor, size int32) (catalog.ProductPage, error) {
	f.rec("List", storeID, nil)
	f.listSize = size
	return f.page, f.err
}

func (f *fakeDelegatedModule) Product(_ context.Context, storeID int64, id uuid.UUID) (catalog.Product, error) {
	f.rec("Product", storeID, nil)
	if p, ok := f.products[id]; ok {
		return p, f.err
	}
	return f.product, f.err
}

func (f *fakeDelegatedModule) CreateProduct(_ context.Context, storeID int64, up media.Uploader, rid uuid.UUID, in catalog.ProductInput, a *delegatedactor.Actor) (catalog.Product, error) {
	f.rec("Create", storeID, a)
	f.uploaders, f.requestID, f.inputs = append(f.uploaders, up), rid, append(f.inputs, in)
	return f.product, f.err
}

func (f *fakeDelegatedModule) UpdateProduct(_ context.Context, storeID int64, up media.Uploader, _ uuid.UUID, version int64, in catalog.ProductInput, a *delegatedactor.Actor) (catalog.Product, error) {
	f.rec("Update", storeID, a)
	f.uploaders, f.versions, f.inputs = append(f.uploaders, up), append(f.versions, version), append(f.inputs, in)
	return f.product, f.err
}

func (f *fakeDelegatedModule) SetProductStatus(_ context.Context, storeID int64, _ uuid.UUID, version int64, to catalog.ProductStatus, a *delegatedactor.Actor) (catalog.Product, error) {
	f.rec("SetStatus", storeID, a)
	f.versions, f.statuses = append(f.versions, version), append(f.statuses, to)
	return f.product, f.err
}

func (f *fakeDelegatedModule) UnpublishProducts(_ context.Context, storeID int64, items []catalog.VersionedProduct, a *delegatedactor.Actor) ([]catalog.UnpublishedItem, error) {
	f.rec("UnpublishMany", storeID, a)
	f.batch = items
	out := make([]catalog.UnpublishedItem, len(items))
	for i, it := range items {
		out[i] = catalog.UnpublishedItem{ProductID: it.ProductID, VersionBefore: it.Version, VersionAfter: it.Version + 1, Changed: true}
	}
	return out, f.err
}

type delegatedRig struct {
	sessions *fakeSessions
	runner   *fakeRunner
	m        *fakeDelegatedModule
	client   catalogv1alpha1connect.CatalogDelegatedServiceClient
}

func newDelegatedRig(t *testing.T, caps ...moddelegation.Capability) *delegatedRig {
	t.Helper()
	images, err := rpc.NewImages("https://media.example.test")
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.DiscardHandler)
	sessions := &fakeSessions{sessions: map[string]moddelegation.DelegationSession{goodCredential: sessionWith(caps...)}}
	runner := &fakeRunner{audit: uuid.New()}
	m := &fakeDelegatedModule{product: sampleProduct()}
	h, err := NewDelegated(runner, m, images, logger)
	if err != nil {
		t.Fatal(err)
	}
	in, err := httpdelegation.NewInterceptor(sessions, logger, DelegatedDeclaration)
	if err != nil {
		t.Fatal(err)
	}
	path, handler := catalogv1alpha1connect.NewCatalogDelegatedServiceHandler(h, connect.WithInterceptors(rpc.NewInterceptor(logger), in))
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &delegatedRig{sessions: sessions, runner: runner, m: m, client: catalogv1alpha1connect.NewCatalogDelegatedServiceClient(srv.Client(), srv.URL)}
}

func asSession[T any](msg *T) *connect.Request[T] {
	req := connect.NewRequest(msg)
	req.Header().Set(httpdelegation.OperationSessionHeader, goodCredential)
	return req
}

func operation() *commonpb.DelegatedOperation {
	return &commonpb.DelegatedOperation{RequestId: validRequestID(), OperationReason: "  店家請我們代為處理  "}
}

// calls 是每個方法的一個合法請求。
func (r *delegatedRig) calls(t *testing.T) map[string]func() error {
	t.Helper()
	ctx := t.Context()
	id := uuid.UUID{1}.String()
	return map[string]func() error{
		catalogv1alpha1connect.CatalogDelegatedServiceListStoreProductsProcedure: func() error {
			_, err := r.client.ListStoreProducts(ctx, asSession(&pb.ListStoreProductsRequest{}))
			return err
		},
		catalogv1alpha1connect.CatalogDelegatedServiceGetStoreProductProcedure: func() error {
			_, err := r.client.GetStoreProduct(ctx, asSession(&pb.GetStoreProductRequest{ProductId: id}))
			return err
		},
		catalogv1alpha1connect.CatalogDelegatedServiceCreateStoreProductProcedure: func() error {
			_, err := r.client.CreateStoreProduct(ctx, asSession(&pb.CreateStoreProductRequest{Operation: operation(), Product: sampleInput()}))
			return err
		},
		catalogv1alpha1connect.CatalogDelegatedServiceUpdateStoreProductProcedure: func() error {
			_, err := r.client.UpdateStoreProduct(ctx, asSession(&pb.UpdateStoreProductRequest{Operation: operation(), ProductId: id, Etag: "3", Product: &pb.ProductInput{Title: "新名"}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"title"}}}))
			return err
		},
		catalogv1alpha1connect.CatalogDelegatedServicePublishStoreProductProcedure: func() error {
			_, err := r.client.PublishStoreProduct(ctx, asSession(&pb.PublishStoreProductRequest{Operation: operation(), ProductId: id, Etag: "3"}))
			return err
		},
		catalogv1alpha1connect.CatalogDelegatedServiceUnpublishStoreProductProcedure: func() error {
			_, err := r.client.UnpublishStoreProduct(ctx, asSession(&pb.UnpublishStoreProductRequest{Operation: operation(), ProductId: id, Etag: "3"}))
			return err
		},
		catalogv1alpha1connect.CatalogDelegatedServiceBatchUnpublishStoreProductsProcedure: func() error {
			_, err := r.client.BatchUnpublishStoreProducts(ctx, asSession(&pb.BatchUnpublishStoreProductsRequest{Operation: operation(), Items: []*pb.BatchUnpublishItem{{ProductId: id, Etag: "3"}}}))
			return err
		},
	}
}

// 每個 RPC 需要的能力是一張寫死的對照表：宣告被改弱（例如 publish 只要 products.write）時這裡變紅，不是拿宣告對宣告。
func TestEveryDelegatedMethodDeclaresItsCapabilityAndAuditAction(t *testing.T) {
	t.Parallel()
	want := map[string]moddelegation.DelegatedMethod{
		catalogv1alpha1connect.CatalogDelegatedServiceListStoreProductsProcedure:           {Capability: "products.read", Action: "PRODUCT_READ", Resource: "PRODUCT"},
		catalogv1alpha1connect.CatalogDelegatedServiceGetStoreProductProcedure:             {Capability: "products.read", Action: "PRODUCT_READ", Resource: "PRODUCT"},
		catalogv1alpha1connect.CatalogDelegatedServiceCreateStoreProductProcedure:          {Capability: "products.write", Action: "PRODUCT_CREATE", Resource: "PRODUCT"},
		catalogv1alpha1connect.CatalogDelegatedServiceUpdateStoreProductProcedure:          {Capability: "products.write", Action: "PRODUCT_UPDATE", Resource: "PRODUCT"},
		catalogv1alpha1connect.CatalogDelegatedServicePublishStoreProductProcedure:         {Capability: "products.publish", Action: "PRODUCT_PUBLISH", Resource: "PRODUCT"},
		catalogv1alpha1connect.CatalogDelegatedServiceUnpublishStoreProductProcedure:       {Capability: "products.publish", Action: "PRODUCT_UNPUBLISH", Resource: "PRODUCT"},
		catalogv1alpha1connect.CatalogDelegatedServiceBatchUnpublishStoreProductsProcedure: {Capability: "products.publish", Action: "PRODUCT_BATCH_UNPUBLISH", Resource: "PRODUCT"},
	}
	if len(want) != len(DelegatedDeclaration.Methods) {
		t.Fatalf("宣告 %d 個方法，預期 %d 個", len(DelegatedDeclaration.Methods), len(want))
	}
	for proc, w := range want {
		if got := DelegatedDeclaration.Methods[proc]; got != w {
			t.Errorf("%s 的宣告 = %+v，預期 %+v", proc, got, w)
		}
	}
}

// 缺少該方法需要的能力：每個 RPC 都是 permission_denied，不到元件與模組；FAILED 稽核由模組寫（這裡記下被拒絕的動作）。
func TestMissingCapabilityIsPermissionDeniedForEveryMethod(t *testing.T) {
	t.Parallel()
	for proc, w := range DelegatedDeclaration.Methods {
		r := newDelegatedRig(t, slices.DeleteFunc(slices.Clone(allProductCaps), func(c moddelegation.Capability) bool { return c == w.Capability })...)
		err := r.calls(t)[proc]()
		if connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("%s 沒有 %s：%v，預期 permission_denied", proc, w.Capability, err)
		}
		if len(r.m.calls) != 0 {
			t.Errorf("%s 被拒絕後仍呼叫了模組：%v", proc, r.m.calls)
		}
		if len(r.sessions.denied) != 1 || r.sessions.denied[0] != string(w.Action) {
			t.Errorf("%s 拒絕的稽核動作 = %v，預期 [%s]", proc, r.sessions.denied, w.Action)
		}
	}
}

// 每個方法都以 session 的店家呼叫模組（不是管理員 ID、不是請求裡的任何值）；每個寫入都把元件交來的代行者原樣傳給模組（傳 nil 會被當成店主寫入而不稽核）；
// 新圖的上傳者是 session 的管理員並綁被代行的店家。
func TestEveryDelegatedMethodUsesTheSessionStoreAndPassesTheActor(t *testing.T) {
	t.Parallel()
	r := newDelegatedRig(t, allProductCaps...)
	for proc, do := range r.calls(t) {
		if err := do(); err != nil {
			t.Fatalf("%s：%v", proc, err)
		}
	}
	for i, s := range r.m.stores {
		if s != sessionStore {
			t.Errorf("模組的第 %d 次呼叫 %s 收到店家 %d，預期 session 的店家 %d", i, r.m.calls[i], s, sessionStore)
		}
	}
	for i, name := range r.m.calls {
		write := name != "List" && name != "Product"
		if write && (r.m.actors[i] == nil || r.m.actors[i].SessionID != sessionID) {
			t.Errorf("%s 傳給模組的代行者 = %+v，預期元件交來的那一個", name, r.m.actors[i])
		}
	}
	want := media.Uploader{ID: sessionAdmin, Admin: true, Store: sessionStore}
	for _, up := range r.m.uploaders {
		if up != want {
			t.Errorf("上傳者 = %+v，預期 %+v", up, want)
		}
	}
}

func TestDelegatedWritesRequireAWellFormedOperation(t *testing.T) {
	t.Parallel()
	r := newDelegatedRig(t, allProductCaps...)
	id := uuid.UUID{1}.String()
	ctx := t.Context()
	bad := map[string]*commonpb.DelegatedOperation{
		"missing":       nil,
		"no request id": {OperationReason: "原因"},
		"not a uuidv4":  {RequestId: uuid.UUID{9}.String(), OperationReason: "原因"},
		"no reason":     {RequestId: validRequestID()},
		"blank reason":  {RequestId: validRequestID(), OperationReason: "   "},
		"long reason":   {RequestId: validRequestID(), OperationReason: strings.Repeat("字", moddelegation.OperationReasonMaxRunes+1)},
	}
	for name, op := range bad {
		_, err := r.client.PublishStoreProduct(ctx, asSession(&pb.PublishStoreProductRequest{Operation: op, ProductId: id, Etag: "3"}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s：%v，預期 invalid_argument", name, err)
		}
	}
	if len(r.m.calls) != 0 {
		t.Fatalf("不合法的 operation 仍呼叫了模組：%v", r.m.calls)
	}
}

func TestCreatePassesTheOperationRequestIDAndNotifyDefault(t *testing.T) {
	t.Parallel()
	r := newDelegatedRig(t, allProductCaps...)
	op := operation()
	if _, err := r.client.CreateStoreProduct(t.Context(), asSession(&pb.CreateStoreProductRequest{Operation: op, Product: sampleInput()})); err != nil {
		t.Fatal(err)
	}
	if r.m.requestID.String() != op.GetRequestId() {
		t.Fatalf("模組的 request_id = %s，預期 operation 的 %s", r.m.requestID, op.GetRequestId())
	}
	w := r.runner.write
	if w.Op.RequestID.String() != op.GetRequestId() || w.Op.Reason != "店家請我們代為處理" || !w.Op.NotifySeller {
		t.Fatalf("交給元件的 operation = %+v", w.Op)
	}
	if w.ResourceName != "山藥" || w.PayloadHash == "" {
		t.Fatalf("建立的稽核資料 = %+v", w)
	}
}

func TestUpdateMergesOnlyTheMaskedFieldsIntoTheCurrentProduct(t *testing.T) {
	t.Parallel()
	r := newDelegatedRig(t, allProductCaps...)
	cur := sampleProduct()
	cur.Title, cur.CategoryID = "舊名", uuid.UUID{7}
	cur.Skus = []catalog.Sku{{ID: uuid.UUID{4}, Price: 80, StockQty: 5}}
	cur.Hashtags = []catalog.Hashtag{{Name: "a", Display: "#A"}}
	r.m.product = cur
	id := uuid.UUID{1}.String()
	_, err := r.client.UpdateStoreProduct(t.Context(), asSession(&pb.UpdateStoreProductRequest{
		Operation: operation(), ProductId: id, Etag: catalog.Etag(41),
		Product:    &pb.ProductInput{Title: "新名", CategoryId: uuid.UUID{8}.String(), HashtagInputs: []string{"#忽略"}},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"title"}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	in := r.m.inputs[0]
	if in.Title != "新名" || in.CategoryID != (uuid.UUID{7}) || !slices.Equal(in.Hashtags, []string{"#A"}) || len(in.Skus) != 1 || in.Skus[0].Price != 80 {
		t.Fatalf("合併後的輸入 = %+v，預期只有 title 換掉", in)
	}
	if r.m.versions[0] != 41 {
		t.Fatalf("版本 = %d，預期 etag 的 41", r.m.versions[0])
	}
}

func TestUpdateMaskIsValidatedAndUnmaskedFieldsDoNotChangeThePayloadHash(t *testing.T) {
	t.Parallel()
	r := newDelegatedRig(t, allProductCaps...)
	id := uuid.UUID{1}.String()
	send := func(in *pb.ProductInput, paths ...string) error {
		_, err := r.client.UpdateStoreProduct(t.Context(), asSession(&pb.UpdateStoreProductRequest{
			Operation: operation(), ProductId: id, Etag: "3", Product: in, UpdateMask: &fieldmaskpb.FieldMask{Paths: paths},
		}))
		return err
	}
	for name, paths := range map[string][]string{
		"empty mask":        nil,
		"unknown field":     {"status"},
		"axes without skus": {"axes"},
		"skus without axes": {"skus"},
		"nested path":       {"gallery.alt"},
	} {
		if err := send(&pb.ProductInput{Title: "x"}, paths...); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s：%v，預期 invalid_argument", name, err)
		}
	}
	if len(r.m.calls) != 0 {
		t.Fatalf("不合法的 mask 仍呼叫了模組：%v", r.m.calls)
	}
	if err := send(&pb.ProductInput{Title: "同一個", CategoryId: uuid.UUID{8}.String()}, "title"); err != nil {
		t.Fatal(err)
	}
	h1 := r.runner.write.PayloadHash
	if err := send(&pb.ProductInput{Title: "同一個", CategoryId: uuid.UUID{9}.String()}, "title"); err != nil {
		t.Fatal(err)
	}
	if h2 := r.runner.write.PayloadHash; h1 != h2 {
		t.Fatal("沒有列在 update_mask 的欄位改變了請求內容雜湊")
	}
	if err := send(&pb.ProductInput{Title: "不同"}, "title"); err != nil {
		t.Fatal(err)
	}
	if r.runner.write.PayloadHash == h1 {
		t.Fatal("列在 update_mask 的欄位改變了，請求內容雜湊卻相同")
	}
}

func TestPublishAndUnpublishSetTheTargetStatusWithTheEtag(t *testing.T) {
	t.Parallel()
	r := newDelegatedRig(t, allProductCaps...)
	id := uuid.UUID{1}.String()
	ctx := t.Context()
	if _, err := r.client.PublishStoreProduct(ctx, asSession(&pb.PublishStoreProductRequest{Operation: operation(), ProductId: id, Etag: catalog.Etag(7)})); err != nil {
		t.Fatal(err)
	}
	if _, err := r.client.UnpublishStoreProduct(ctx, asSession(&pb.UnpublishStoreProductRequest{Operation: operation(), ProductId: id, Etag: catalog.Etag(8)})); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(r.m.statuses, []catalog.ProductStatus{catalog.StatusActive, catalog.StatusArchived}) || !slices.Equal(r.m.versions, []int64{7, 8}) {
		t.Fatalf("狀態 %v、版本 %v", r.m.statuses, r.m.versions)
	}
	if _, err := r.client.PublishStoreProduct(ctx, asSession(&pb.PublishStoreProductRequest{Operation: operation(), ProductId: id})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("沒有 etag：%v，預期 invalid_argument", err)
	}
}

func TestReplayReadsBackTheFirstProductWithoutCallingTheModuleToWrite(t *testing.T) {
	t.Parallel()
	r := newDelegatedRig(t, allProductCaps...)
	first := uuid.UUID{6}
	p := sampleProduct()
	p.ID, p.Version = first, 9
	r.m.products = map[uuid.UUID]catalog.Product{first: p}
	r.runner.replayed, r.runner.replayID = true, first.String()
	res, err := r.client.CreateStoreProduct(t.Context(), asSession(&pb.CreateStoreProductRequest{Operation: operation(), Product: sampleInput()}))
	if err != nil {
		t.Fatal(err)
	}
	if res.Msg.GetProduct().GetId() != first.String() || res.Msg.GetAuditLogId() != r.runner.audit.String() {
		t.Fatalf("重送的回應 = %v，預期第一次的商品與稽核列", res.Msg)
	}
	if slices.Contains(r.m.calls, "Create") {
		t.Fatalf("重送不該再呼叫模組寫入：%v", r.m.calls)
	}
}

func TestListClampsThePageSizeAndBindsTheTokenToTheSessionStoreAndFilter(t *testing.T) {
	t.Parallel()
	r := newDelegatedRig(t, allProductCaps...)
	next := &catalog.ListCursor{Key: 5, ID: uuid.UUID{3}}
	r.m.page = catalog.ProductPage{Next: next}
	res, err := r.client.ListStoreProducts(t.Context(), asSession(&pb.ListStoreProductsRequest{PageSize: 500, Filter: &pb.MyProductFilter{Status: pb.ProductStatus_PRODUCT_STATUS_ACTIVE}}))
	if err != nil {
		t.Fatal(err)
	}
	if r.m.listSize != maxDelegatedPageSize {
		t.Fatalf("page size = %d，預期夾到 %d", r.m.listSize, maxDelegatedPageSize)
	}
	token := res.Msg.GetNextPageToken()
	if token == "" {
		t.Fatal("有下一頁卻沒有 next_page_token")
	}
	// token 與篩選綁定：換了狀態就不能用。
	_, err = r.client.ListStoreProducts(t.Context(), asSession(&pb.ListStoreProductsRequest{PageToken: token, Filter: &pb.MyProductFilter{Status: pb.ProductStatus_PRODUCT_STATUS_DRAFT}}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("換篩選用同一個 token：%v，預期 invalid_argument", err)
	}
	if _, err = r.client.ListStoreProducts(t.Context(), asSession(&pb.ListStoreProductsRequest{PageToken: token, Filter: &pb.MyProductFilter{Status: pb.ProductStatus_PRODUCT_STATUS_ACTIVE}})); err != nil {
		t.Fatalf("同篩選的 token：%v", err)
	}
}

func batchRequest(items ...*pb.BatchUnpublishItem) *pb.BatchUnpublishStoreProductsRequest {
	return &pb.BatchUnpublishStoreProductsRequest{Operation: operation(), Items: items}
}

func item(n byte, etag string) *pb.BatchUnpublishItem {
	return &pb.BatchUnpublishItem{ProductId: uuid.UUID{n}.String(), Etag: etag}
}

func TestBatchUnpublishValidatesItems(t *testing.T) {
	t.Parallel()
	r := newDelegatedRig(t, allProductCaps...)
	tooMany := make([]*pb.BatchUnpublishItem, moddelegation.DelegationMaxBatch+1)
	for i := range tooMany {
		tooMany[i] = item(byte(i+1), "3")
	}
	for name, req := range map[string]*pb.BatchUnpublishStoreProductsRequest{
		"no items":  batchRequest(),
		"too many":  batchRequest(tooMany...),
		"duplicate": batchRequest(item(1, "3"), item(1, "4")),
		"no etag":   batchRequest(item(1, "")),
		"bad id":    batchRequest(&pb.BatchUnpublishItem{ProductId: "x", Etag: "3"}),
	} {
		if _, err := r.client.BatchUnpublishStoreProducts(t.Context(), asSession(req)); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s：%v，預期 invalid_argument", name, err)
		}
	}
	if len(r.m.calls) != 0 {
		t.Fatalf("不合法的批次仍呼叫了模組：%v", r.m.calls)
	}
}

// 批次：一次元件呼叫、一次模組呼叫，請求內容雜湊與項目順序無關；回應依請求順序，帶下架後的 etag 與狀態。
func TestBatchUnpublishIsOneComponentCallOneModuleCallAndOrderIndependentHash(t *testing.T) {
	t.Parallel()
	r := newDelegatedRig(t, allProductCaps...)
	a, b := uuid.UUID{2}, uuid.UUID{1}
	pa, pb2 := sampleProduct(), sampleProduct()
	pa.ID, pa.Version, pa.Status = a, 5, catalog.StatusArchived
	pb2.ID, pb2.Version, pb2.Status = b, 8, catalog.StatusArchived
	r.m.products = map[uuid.UUID]catalog.Product{a: pa, b: pb2}
	res, err := r.client.BatchUnpublishStoreProducts(t.Context(), asSession(batchRequest(item(2, catalog.Etag(4)), item(1, catalog.Etag(7)))))
	if err != nil {
		t.Fatal(err)
	}
	if got := r.m.batch; len(got) != 2 || got[0].ProductID != a || got[0].Version != 4 || got[1].ProductID != b || got[1].Version != 7 {
		t.Fatalf("模組收到的項目 = %+v", got)
	}
	unpublish := 0
	for _, c := range r.m.calls {
		if c == "UnpublishMany" {
			unpublish++
		}
	}
	if unpublish != 1 || r.runner.write.ResourceName != "2 項商品" {
		t.Fatalf("模組批次呼叫 %d 次、稽核名稱 %q", unpublish, r.runner.write.ResourceName)
	}
	h1 := r.runner.write.PayloadHash
	if _, err := r.client.BatchUnpublishStoreProducts(t.Context(), asSession(batchRequest(item(1, catalog.Etag(7)), item(2, catalog.Etag(4))))); err != nil {
		t.Fatal(err)
	}
	if r.runner.write.PayloadHash != h1 {
		t.Fatal("同一批商品換了順序，請求內容雜湊不同")
	}
	ps := res.Msg.GetProducts()
	if len(ps) != 2 || ps[0].GetProductId() != a.String() || ps[0].GetEtag() != catalog.Etag(5) || ps[1].GetStatus() != pb.ProductStatus_PRODUCT_STATUS_ARCHIVED {
		t.Fatalf("回應 = %v", ps)
	}
}

// 整批版本衝突：aborted 與 VERSION_CONFLICT，ErrorInfo.metadata 的 conflictingProductIds 是逗號連接、由小到大的衝突商品。
func TestBatchConflictIsAbortedWithTheConflictingIDsInTheErrorDetails(t *testing.T) {
	t.Parallel()
	r := newDelegatedRig(t, allProductCaps...)
	r.m.err = &catalog.BatchVersionConflictError{ProductIDs: []uuid.UUID{{9}, {3}}}
	_, err := r.client.BatchUnpublishStoreProducts(t.Context(), asSession(batchRequest(item(3, "3"), item(9, "3"), item(5, "3"))))
	if connect.CodeOf(err) != connect.CodeAborted {
		t.Fatalf("code = %v，預期 aborted：%v", connect.CodeOf(err), err)
	}
	var info *errdetails.ErrorInfo
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatal(err)
	}
	for _, d := range ce.Details() {
		if v, e := d.Value(); e == nil {
			if x, ok := v.(*errdetails.ErrorInfo); ok {
				info = x
			}
		}
	}
	if info == nil || info.GetReason() != "VERSION_CONFLICT" {
		t.Fatalf("ErrorInfo = %v，預期 reason VERSION_CONFLICT", info)
	}
	want := uuid.UUID{3}.String() + "," + uuid.UUID{9}.String()
	if got := info.GetMetadata()["conflictingProductIds"]; got != want {
		t.Fatalf("conflictingProductIds = %q，預期 %q", got, want)
	}
}

func TestSingleVersionConflictIsAbortedWithVersionConflict(t *testing.T) {
	t.Parallel()
	r := newDelegatedRig(t, allProductCaps...)
	r.m.err = catalog.ErrVersionConflict
	_, err := r.client.UnpublishStoreProduct(t.Context(), asSession(&pb.UnpublishStoreProductRequest{Operation: operation(), ProductId: uuid.UUID{1}.String(), Etag: "3"}))
	if connect.CodeOf(err) != connect.CodeAborted {
		t.Fatalf("code = %v，預期 aborted", connect.CodeOf(err))
	}
}

// 寫入宣告的能力與動作，等於稽核訂閱者對「這個方法發的事件」寫的能力與動作（A7-D27）：宣告由 delegation.ProductWriteFor 衍生。
func TestWriteDeclarationsAgreeWithTheAuditSubscriberMapping(t *testing.T) {
	t.Parallel()
	events := map[string]any{
		catalogv1alpha1connect.CatalogDelegatedServiceCreateStoreProductProcedure:          catalog.ProductCreated{},
		catalogv1alpha1connect.CatalogDelegatedServiceUpdateStoreProductProcedure:          catalog.ProductUpdated{},
		catalogv1alpha1connect.CatalogDelegatedServicePublishStoreProductProcedure:         catalog.ProductPublished{},
		catalogv1alpha1connect.CatalogDelegatedServiceUnpublishStoreProductProcedure:       catalog.ProductUnpublished{},
		catalogv1alpha1connect.CatalogDelegatedServiceBatchUnpublishStoreProductsProcedure: catalog.ProductsUnpublished{},
	}
	for proc, e := range events {
		w, ok := moddelegation.ProductWriteFor(e)
		if !ok {
			t.Fatalf("%T 不是代行會稽核的事件", e)
		}
		if m := DelegatedDeclaration.Methods[proc]; m.Capability != w.Capability || m.Action != w.Action {
			t.Errorf("%s 宣告 %+v，訂閱者寫 %+v", proc, m, w)
		}
	}
	if _, ok := moddelegation.ProductWriteFor(catalog.ProductWritten{}); ok {
		t.Fatal("共用欄位型別不是事件")
	}
}

// 批次下架的回應來自寫入的結果（重送時是稽核列記下的清單），不再讀商品：重送時商品已被刪除也回第一次的結果。
func TestBatchUnpublishReplayAnswersFromTheRecordedItemsWithoutReadingProducts(t *testing.T) {
	t.Parallel()
	r := newDelegatedRig(t, allProductCaps...)
	a, b := uuid.UUID{1}, uuid.UUID{2}
	r.m.err = catalog.ErrProductNotFound
	r.runner.replayed = true
	r.runner.items = []moddelegation.AuditItem{{ResourceID: a.String(), VersionBefore: 4, VersionAfter: 5}, {ResourceID: b.String(), VersionBefore: 7, VersionAfter: 7}}
	res, err := r.client.BatchUnpublishStoreProducts(t.Context(), asSession(batchRequest(item(2, catalog.Etag(7)), item(1, catalog.Etag(4)))))
	if err != nil {
		t.Fatal(err)
	}
	ps := res.Msg.GetProducts()
	if len(ps) != 2 || ps[0].GetProductId() != b.String() || ps[0].GetEtag() != catalog.Etag(7) || ps[1].GetProductId() != a.String() || ps[1].GetEtag() != catalog.Etag(5) {
		t.Fatalf("回應 = %v，預期依請求順序、版本取自稽核列", ps)
	}
	for _, p := range ps {
		if p.GetStatus() != pb.ProductStatus_PRODUCT_STATUS_ARCHIVED {
			t.Fatalf("商品 %s 狀態 %v，預期 ARCHIVED", p.GetProductId(), p.GetStatus())
		}
	}
	if len(r.m.calls) != 0 {
		t.Fatalf("重送不該呼叫模組：%v", r.m.calls)
	}
}

// 第一次的批次下架也不讀商品：回應用模組回傳的寫入結果。
func TestBatchUnpublishFirstCallAnswersFromTheWriteOutcome(t *testing.T) {
	t.Parallel()
	r := newDelegatedRig(t, allProductCaps...)
	if _, err := r.client.BatchUnpublishStoreProducts(t.Context(), asSession(batchRequest(item(1, catalog.Etag(4))))); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(r.m.calls, []string{"UnpublishMany"}) {
		t.Fatalf("模組呼叫 = %v，預期只有 UnpublishMany", r.m.calls)
	}
}

// 單一上架與下架回應用寫入的 transaction 內回傳的商品，不在提交後再讀一次。
func TestPublishAndUnpublishAnswerFromTheWrittenProductWithoutReReading(t *testing.T) {
	t.Parallel()
	r := newDelegatedRig(t, allProductCaps...)
	p := sampleProduct()
	p.ID, p.Version = uuid.UUID{1}, 8
	r.m.product = p
	res, err := r.client.PublishStoreProduct(t.Context(), asSession(&pb.PublishStoreProductRequest{Operation: operation(), ProductId: p.ID.String(), Etag: catalog.Etag(7)}))
	if err != nil {
		t.Fatal(err)
	}
	if res.Msg.GetProduct().GetId() != p.ID.String() || res.Msg.GetProduct().GetEtag() != catalog.Etag(8) {
		t.Fatalf("回應 = %v，預期寫入後的商品", res.Msg.GetProduct())
	}
	if !slices.Equal(r.m.calls, []string{"SetStatus"}) {
		t.Fatalf("模組呼叫 = %v，預期只有 SetStatus（不再讀商品）", r.m.calls)
	}
}

// 代行不能下架草稿：單一與批次都是 failed_precondition，批次在訊息裡列出全部草稿商品的 ID（沒有 ErrorReason，metadata 帶不出去）。
func TestDelegatedUnpublishOfDraftsIsFailedPrecondition(t *testing.T) {
	t.Parallel()
	r := newDelegatedRig(t, allProductCaps...)
	r.m.err = catalog.ErrNotPublished
	_, err := r.client.UnpublishStoreProduct(t.Context(), asSession(&pb.UnpublishStoreProductRequest{Operation: operation(), ProductId: uuid.UUID{1}.String(), Etag: "3"}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("單一下架草稿 code = %v，預期 failed_precondition：%v", connect.CodeOf(err), err)
	}
	r.m.err = &catalog.BatchNotPublishedError{ProductIDs: []uuid.UUID{{3}, {9}}}
	_, err = r.client.BatchUnpublishStoreProducts(t.Context(), asSession(batchRequest(item(3, "3"), item(9, "3"), item(5, "3"))))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("批次含草稿 code = %v，預期 failed_precondition：%v", connect.CodeOf(err), err)
	}
	want := uuid.UUID{3}.String() + "," + uuid.UUID{9}.String()
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("錯誤訊息 %q，預期列出全部草稿商品 %q", err.Error(), want)
	}
}

// 代行建立的 request_id 這家店已用過（不是這個 session 的重送）：failed_precondition，不是 internal。
func TestDelegatedCreateWithAUsedRequestIDIsFailedPrecondition(t *testing.T) {
	t.Parallel()
	r := newDelegatedRig(t, allProductCaps...)
	r.m.err = catalog.ErrCreateReplayed
	_, err := r.client.CreateStoreProduct(t.Context(), asSession(&pb.CreateStoreProductRequest{Operation: operation(), Product: sampleInput()}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("code = %v，預期 failed_precondition：%v", connect.CodeOf(err), err)
	}
}
