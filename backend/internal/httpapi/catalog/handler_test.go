package catalog

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"

	"github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/catalog/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/catalog/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/catalog/v1alpha1/catalogv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

const (
	callerAccount = 3001
	callerStore   = 9002
)

var fixedNow = time.Date(2026, 12, 1, 9, 0, 0, 0, time.UTC)

type fakeModule struct {
	stores   []int64
	uploader []int64
	at       []time.Time
	filters  []catalog.PublicFilter
	sorts    []catalog.ProductSort
	sizes    []int32
	versions []int64
	keys     []string
	deltas   []int32
	calls    []string
	queries  []string

	product catalog.Product
	sku     catalog.SkuState
	page    catalog.ProductPage
	err     error
}

func (f *fakeModule) rec(name string) { f.calls = append(f.calls, name) }

func (f *fakeModule) PublicProduct(_ context.Context, _ uuid.UUID, at time.Time) (catalog.Product, error) {
	f.rec("PublicProduct")
	f.at = append(f.at, at)
	return f.product, f.err
}

func (f *fakeModule) ListPublicProducts(_ context.Context, at time.Time, fl catalog.PublicFilter, sort catalog.ProductSort, _ *catalog.ListCursor, size int32) (catalog.ProductPage, error) {
	f.rec("ListPublicProducts")
	f.at, f.filters, f.sorts, f.sizes = append(f.at, at), append(f.filters, fl), append(f.sorts, sort), append(f.sizes, size)
	return f.page, f.err
}

func (f *fakeModule) SuggestProductNames(_ context.Context, at time.Time, query string, _ int32) ([]string, error) {
	f.rec("SuggestProductNames")
	f.queries = append(f.queries, query)
	f.at = append(f.at, at)
	return nil, f.err
}

func (f *fakeModule) SuggestHashtags(_ context.Context, at time.Time, _ string, _ bool, _ int32) ([]catalog.HashtagCount, error) {
	f.rec("SuggestHashtags")
	f.at = append(f.at, at)
	return nil, f.err
}

func (f *fakeModule) TrendingHashtags(_ context.Context, at time.Time, _ int32) ([]catalog.HashtagCount, error) {
	f.rec("TrendingHashtags")
	f.at = append(f.at, at)
	return nil, f.err
}

func (f *fakeModule) Categories(context.Context) ([]catalog.Category, error) { return nil, f.err }

func (f *fakeModule) ListSellerProducts(_ context.Context, storeID int64, _ catalog.SellerFilter, _ *catalog.ListCursor, _ int32) (catalog.ProductPage, error) {
	f.rec("ListSellerProducts")
	f.stores = append(f.stores, storeID)
	return f.page, f.err
}

func (f *fakeModule) CountSellerProducts(_ context.Context, storeID int64, _ catalog.SellerFilter) (int32, error) {
	f.rec("CountSellerProducts")
	f.stores = append(f.stores, storeID)
	return 0, f.err
}

func (f *fakeModule) Product(_ context.Context, storeID int64, _ uuid.UUID) (catalog.Product, error) {
	f.rec("Product")
	f.stores = append(f.stores, storeID)
	return f.product, f.err
}

func (f *fakeModule) OwnSku(_ context.Context, storeID int64, _ uuid.UUID) (catalog.SkuState, error) {
	f.rec("OwnSku")
	f.stores = append(f.stores, storeID)
	return f.sku, f.err
}

func (f *fakeModule) CreateProduct(_ context.Context, storeID int64, up media.Uploader, _ uuid.UUID, _ catalog.ProductInput) (catalog.Product, error) {
	f.rec("CreateProduct")
	f.stores, f.uploader = append(f.stores, storeID), append(f.uploader, up.ID)
	return f.product, f.err
}

func (f *fakeModule) UpdateProduct(_ context.Context, storeID int64, up media.Uploader, _ uuid.UUID, version int64, _ catalog.ProductInput) (catalog.Product, error) {
	f.rec("UpdateProduct")
	f.stores, f.uploader, f.versions = append(f.stores, storeID), append(f.uploader, up.ID), append(f.versions, version)
	return f.product, f.err
}

func (f *fakeModule) SetProductStatus(_ context.Context, storeID int64, _ uuid.UUID, version int64, _ catalog.ProductStatus) error {
	f.rec("SetProductStatus")
	f.stores, f.versions = append(f.stores, storeID), append(f.versions, version)
	return f.err
}

func (f *fakeModule) DeleteProduct(_ context.Context, storeID int64, _ uuid.UUID, version int64) error {
	f.rec("DeleteProduct")
	f.stores, f.versions = append(f.stores, storeID), append(f.versions, version)
	return f.err
}

func (f *fakeModule) AdjustStock(_ context.Context, storeID int64, key string, _ uuid.UUID, delta int32) error {
	f.rec("AdjustStock")
	f.stores, f.keys, f.deltas = append(f.stores, storeID), append(f.keys, key), append(f.deltas, delta)
	return f.err
}

func (f *fakeModule) SetStock(_ context.Context, storeID int64, key string, _ uuid.UUID, _, _ int32) error {
	f.rec("SetStock")
	f.stores, f.keys = append(f.stores, storeID), append(f.keys, key)
	return f.err
}

func (f *fakeModule) AdminProduct(context.Context, uuid.UUID) (catalog.Product, error) {
	f.rec("AdminProduct")
	return f.product, f.err
}

func (f *fakeModule) ListAdminProducts(context.Context, catalog.AdminFilter, *catalog.ListCursor, int32) (catalog.ProductPage, int32, error) {
	f.rec("ListAdminProducts")
	return f.page, 0, f.err
}

type fakeActing struct {
	asked []seller.Capability
	err   error
}

func (a *fakeActing) ActingStore(_ context.Context, accountID int64, c seller.Capability) (seller.Store, error) {
	a.asked = append(a.asked, c)
	if a.err != nil {
		return seller.Store{}, a.err
	}
	if accountID != callerAccount {
		return seller.Store{}, errors.New("沒有這個帳號的店")
	}
	return seller.Store{StoreID: callerStore, AccountID: accountID}, nil
}

type fakeCards struct{}

func (fakeCards) StoreCards(_ context.Context, ids []int64) ([]seller.StoreCard, error) {
	out := make([]seller.StoreCard, len(ids))
	for i, id := range ids {
		name := "店" + rpc.FormatID(id)
		logo, cover, town := "logo/"+name, "cover/"+name, "63000010"
		out[i] = seller.StoreCard{StoreID: id, StoreCode: id + 5, Name: &name, Status: seller.CardOpen, LogoKey: &logo, CoverKey: &cover, TownCode: &town}
	}
	return out, nil
}

type rig struct {
	m      *fakeModule
	acting *fakeActing
	app    *App
	pub    *Public
	adm    *Admin
}

func newRig(t *testing.T) *rig {
	t.Helper()
	images := characterize.Images(t)
	m, acting := &fakeModule{}, &fakeActing{}
	app, err := NewApp(m, acting, images, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	presenter, err := NewPresenter(fakeCards{}, characterize.Stats(t), images)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := NewPublic(m, presenter, func() time.Time { return fixedNow })
	if err != nil {
		t.Fatal(err)
	}
	adm, err := NewAdmin(m, images)
	if err != nil {
		t.Fatal(err)
	}
	return &rig{m: m, acting: acting, app: app, pub: pub, adm: adm}
}

func signedIn() context.Context {
	return rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAccount, ID: callerAccount})
}

func adminCtx() context.Context {
	return rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAdmin, ID: 11})
}

func codeOf(err error) connect.Code {
	return connect.CodeOf(rpc.ConnectError(context.Background(), err))
}

func reasonAndFields(err error) (reason string, fields []string) {
	for _, d := range rpc.ConnectError(context.Background(), err).Details() {
		v, e := d.Value()
		if e != nil {
			continue
		}
		switch x := v.(type) {
		case *errdetails.ErrorInfo:
			reason = x.GetReason()
		case *errdetails.BadRequest:
			for _, f := range x.GetFieldViolations() {
				fields = append(fields, f.GetField())
			}
		}
	}
	return reason, fields
}

func sampleProduct() catalog.Product {
	return catalog.Product{ID: uuid.UUID{1}, StoreID: callerStore, CategoryID: uuid.UUID{2}, Title: "山藥", Status: catalog.StatusActive,
		Skus: []catalog.Sku{{ID: uuid.UUID{3}, Price: 100, StockQty: 4}}, Version: 3}
}

func validRequestID() string { return uuid.NewV4().String() }

func sampleInput() *pb.ProductInput {
	return &pb.ProductInput{Title: "山藥", CategoryId: uuid.UUID{2}.String(), Skus: []*pb.SkuInput{{Price: 100}}}
}

// 每個賣家方法都以 seller.ActingStore 推導的店家呼叫模組：模組收到的店家是 callerStore（不是帳號 ID，也不是請求裡的任何值）；上傳者是帳號。
func TestSellerMethodsUseTheActingStoreForEveryCall(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.m.product = sampleProduct()
	r.m.page = catalog.ProductPage{}
	ctx := signedIn()
	id := uuid.UUID{1}.String()
	sku := uuid.UUID{3}.String()
	calls := []func() error{
		func() error {
			_, e := r.app.ListMyProducts(ctx, connect.NewRequest(&pb.ListMyProductsRequest{}))
			return e
		},
		func() error {
			_, e := r.app.GetMyProduct(ctx, connect.NewRequest(&pb.GetMyProductRequest{ProductId: id}))
			return e
		},
		func() error {
			_, e := r.app.CountMyProducts(ctx, connect.NewRequest(&pb.CountMyProductsRequest{}))
			return e
		},
		func() error {
			_, e := r.app.CreateProduct(ctx, connect.NewRequest(&pb.CreateProductRequest{Product: sampleInput(), RequestId: validRequestID()}))
			return e
		},
		func() error {
			_, e := r.app.UpdateProduct(ctx, connect.NewRequest(&pb.UpdateProductRequest{ProductId: id, Etag: "3", Product: sampleInput()}))
			return e
		},
		func() error {
			_, e := r.app.SetProductStatus(ctx, connect.NewRequest(&pb.SetProductStatusRequest{ProductId: id, Etag: "3", Status: pb.ProductStatus_PRODUCT_STATUS_ARCHIVED}))
			return e
		},
		func() error {
			_, e := r.app.DeleteProduct(ctx, connect.NewRequest(&pb.DeleteProductRequest{ProductId: id}))
			return e
		},
		func() error {
			_, e := r.app.AdjustStock(ctx, connect.NewRequest(&pb.AdjustStockRequest{SkuId: sku, Delta: 2, RequestId: validRequestID()}))
			return e
		},
		func() error {
			_, e := r.app.SetStock(ctx, connect.NewRequest(&pb.SetStockRequest{SkuId: sku, Quantity: 9, ExpectedQuantity: 4, RequestId: validRequestID()}))
			return e
		},
	}
	for i, call := range calls {
		if err := call(); err != nil {
			t.Fatalf("第 %d 個呼叫：%v", i, err)
		}
	}
	if len(r.m.stores) == 0 {
		t.Fatal("模組沒有被呼叫")
	}
	for _, s := range r.m.stores {
		if s != callerStore {
			t.Fatalf("模組收到的店家 = %d，預期推導出的 %d（帳號是 %d）", s, callerStore, callerAccount)
		}
	}
	for _, u := range r.m.uploader {
		if u != callerAccount {
			t.Fatalf("上傳者 = %d，預期帳號 %d", u, callerAccount)
		}
	}
	if len(r.acting.asked) != len(calls) {
		t.Fatalf("向 seller 要店家 %d 次，預期每個方法一次（%d）", len(r.acting.asked), len(calls))
	}
	for _, c := range r.acting.asked {
		if c != seller.CapProductManage {
			t.Fatalf("要的能力 = %s，預期 CAP_PRODUCT_MANAGE", c)
		}
	}
}

// 店家推導失敗（停權、沒有店）就不碰模組；管理員身分不能呼叫賣家方法。
func TestSellerMethodsStopBeforeTheModuleWhenTheStoreCannotBeResolved(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.acting.err = rpc.Error(connect.CodePermissionDenied, "%s", "店家停權中")
	_, err := r.app.AdjustStock(signedIn(), connect.NewRequest(&pb.AdjustStockRequest{SkuId: uuid.UUID{3}.String(), Delta: 1, RequestId: validRequestID()}))
	if codeOf(err) != connect.CodePermissionDenied || len(r.m.calls) != 0 {
		t.Fatalf("停權：code=%v 呼叫=%v", codeOf(err), r.m.calls)
	}
	r.acting.err = nil
	_, err = r.app.GetMyProduct(adminCtx(), connect.NewRequest(&pb.GetMyProductRequest{ProductId: uuid.UUID{1}.String()}))
	if codeOf(err) != connect.CodePermissionDenied || len(r.m.calls) != 0 {
		t.Fatalf("管理員：code=%v 呼叫=%v", codeOf(err), r.m.calls)
	}
}

// 宣告表與 service 的方法一一對應：缺一列、多一列都不能啟動；宣告的能力就是向 seller 要的能力。
func TestDeclarationsMatchTheService(t *testing.T) {
	t.Parallel()
	if err := checkDeclarations(declarations, serviceDescriptor()); err != nil {
		t.Fatal(err)
	}
	for procedure := range declarations {
		table := map[string]declaration{}
		for k, v := range declarations {
			if k != procedure {
				table[k] = v
			}
		}
		if err := checkDeclarations(table, serviceDescriptor()); err == nil {
			t.Fatalf("缺 %s 的宣告要不能啟動", procedure)
		}
	}
	extra := map[string]declaration{"/hawkr.catalog.v1alpha1.SellerCatalogAppService/Nope": requires(seller.CapProductManage)}
	for k, v := range declarations {
		extra[k] = v
	}
	if err := checkDeclarations(extra, serviceDescriptor()); err == nil {
		t.Fatal("多一個不存在方法的宣告要不能啟動")
	}
	bad := map[string]declaration{}
	for k, v := range declarations {
		bad[k] = v
	}
	bad[catalogv1alpha1connect.SellerCatalogAppServiceSetStockProcedure] = declaration{capability: "CAP_NOPE"}
	if err := checkDeclarations(bad, serviceDescriptor()); err == nil {
		t.Fatal("不存在的能力要不能啟動")
	}
	r := newRig(t)
	if _, err := newApp(map[string]declaration{}, r.m, r.acting, characterize.Images(t), slog.New(slog.DiscardHandler)); err == nil {
		t.Fatal("空的宣告表要不能啟動")
	}
}

// 公開讀取以請求的 clock 呼叫模組（不是 client 的值，也不是 handler 自己讀的系統時間）。
func TestPublicReadsPassTheRequestClock(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.m.product = sampleProduct()
	ctx := context.Background()
	if _, err := r.pub.ListProducts(ctx, connect.NewRequest(&pb.ListProductsRequest{})); err != nil {
		t.Fatal(err)
	}
	if _, err := r.pub.SearchProducts(ctx, connect.NewRequest(&pb.SearchProductsRequest{Query: "山藥"})); err != nil {
		t.Fatal(err)
	}
	if _, err := r.pub.GetProduct(ctx, connect.NewRequest(&pb.GetProductRequest{ProductId: uuid.UUID{1}.String()})); err != nil {
		t.Fatal(err)
	}
	if _, err := r.pub.SuggestProductNames(ctx, connect.NewRequest(&pb.SuggestProductNamesRequest{Query: "山"})); err != nil {
		t.Fatal(err)
	}
	if _, err := r.pub.SuggestHashtags(ctx, connect.NewRequest(&pb.SuggestHashtagsRequest{Query: "茶"})); err != nil {
		t.Fatal(err)
	}
	if _, err := r.pub.SuggestTrendingHashtags(ctx, connect.NewRequest(&pb.SuggestTrendingHashtagsRequest{})); err != nil {
		t.Fatal(err)
	}
	if len(r.m.at) != 6 {
		t.Fatalf("模組被呼叫 %d 次，預期 6", len(r.m.at))
	}
	for _, at := range r.m.at {
		if !at.Equal(fixedNow) {
			t.Fatalf("模組收到的時間 = %v，預期請求的 clock %v", at, fixedNow)
		}
	}
}

// 公開的列表與搜尋：不接受的輸入在入口就擋下，不碰模組；搜尋預設相關度，列表不接受相關度。
func TestPublicListAndSearchInputRules(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	ctx := context.Background()
	cases := []struct {
		name  string
		err   func() error
		field string
	}{
		{"負的 page_size", func() error {
			_, e := r.pub.ListProducts(ctx, connect.NewRequest(&pb.ListProductsRequest{PageSize: -1}))
			return e
		}, "page_size"},
		{"列表不接受相關度", func() error {
			_, e := r.pub.ListProducts(ctx, connect.NewRequest(&pb.ListProductsRequest{Sort: pb.ProductSort_PRODUCT_SORT_RELEVANCE}))
			return e
		}, "sort"},
		{"空白關鍵字", func() error {
			_, e := r.pub.SearchProducts(ctx, connect.NewRequest(&pb.SearchProductsRequest{Query: " 　 "}))
			return e
		}, "query"},
		{"關鍵字太長", func() error {
			q := make([]rune, int(maxSearchRunes)+1)
			for i := range q {
				q[i] = '山'
			}
			_, e := r.pub.SearchProducts(ctx, connect.NewRequest(&pb.SearchProductsRequest{Query: string(q)}))
			return e
		}, "query"},
		{"店家太多", func() error {
			ids := make([]string, int(maxFilterStores)+1)
			for i := range ids {
				ids[i] = rpc.FormatID(int64(i + 1))
			}
			_, e := r.pub.ListProducts(ctx, connect.NewRequest(&pb.ListProductsRequest{Filter: &pb.ProductFilter{StoreIds: ids}}))
			return e
		}, "filter.store_ids"},
		{"壞的店家 ID", func() error {
			_, e := r.pub.ListProducts(ctx, connect.NewRequest(&pb.ListProductsRequest{Filter: &pb.ProductFilter{StoreIds: []string{"abc"}}}))
			return e
		}, "filter.store_ids"},
		{"壞的分類 ID", func() error {
			_, e := r.pub.ListProducts(ctx, connect.NewRequest(&pb.ListProductsRequest{Filter: &pb.ProductFilter{CategoryIds: []string{"x"}}}))
			return e
		}, "filter.category_ids"},
		{"負的 max_results", func() error {
			_, e := r.pub.SuggestHashtags(ctx, connect.NewRequest(&pb.SuggestHashtagsRequest{Query: "a", MaxResults: -1}))
			return e
		}, "max_results"},
	}
	for _, c := range cases {
		err := c.err()
		_, fields := reasonAndFields(err)
		if codeOf(err) != connect.CodeInvalidArgument || !slices.Equal(fields, []string{c.field}) {
			t.Errorf("%s：code=%v 欄位=%v，預期 invalid_argument 指向 %s", c.name, codeOf(err), fields, c.field)
		}
	}
	if len(r.m.calls) != 0 {
		t.Fatalf("擋在入口的輸入不該碰模組：%v", r.m.calls)
	}
	if _, err := r.pub.SearchProducts(ctx, connect.NewRequest(&pb.SearchProductsRequest{Query: "  山藥 "})); err != nil {
		t.Fatal(err)
	}
	if got := r.m.sorts[len(r.m.sorts)-1]; got != catalog.SortRelevance {
		t.Fatalf("搜尋沒有指定排序的預設 = %q，預期 RELEVANCE", got)
	}
	if got := r.m.filters[len(r.m.filters)-1].Query; got != "山藥" {
		t.Fatalf("搜尋關鍵字要去頭尾空白：%q", got)
	}
}

// page token 綁定篩選：用某組篩選取得的 token 換了篩選就不能用。
func TestPageTokenIsBoundToTheFilter(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.m.page = catalog.ProductPage{Items: []catalog.ProductSummary{{ID: uuid.UUID{1}, StoreID: 7}}, Next: &catalog.ListCursor{Key: 5, ID: uuid.UUID{1}}}
	ctx := context.Background()
	first, err := r.pub.ListProducts(ctx, connect.NewRequest(&pb.ListProductsRequest{Filter: &pb.ProductFilter{StoreIds: []string{"7"}}}))
	if err != nil {
		t.Fatal(err)
	}
	token := first.Msg.GetNextPageToken()
	if token == "" {
		t.Fatal("有下一頁要回 token")
	}
	if _, err := r.pub.ListProducts(ctx, connect.NewRequest(&pb.ListProductsRequest{Filter: &pb.ProductFilter{StoreIds: []string{"7"}}, PageToken: token})); err != nil {
		t.Fatalf("同一組篩選要接得上：%v", err)
	}
	for name, req := range map[string]*pb.ListProductsRequest{
		"換店家":     {Filter: &pb.ProductFilter{StoreIds: []string{"8"}}, PageToken: token},
		"換排序":     {Filter: &pb.ProductFilter{StoreIds: []string{"7"}}, Sort: pb.ProductSort_PRODUCT_SORT_SALES, PageToken: token},
		"壞 token": {Filter: &pb.ProductFilter{StoreIds: []string{"7"}}, PageToken: "garbage"},
	} {
		if _, err := r.pub.ListProducts(ctx, connect.NewRequest(req)); codeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s：code=%v，預期 invalid_argument", name, codeOf(err))
		}
	}
	sellerFirst, err := r.app.ListMyProducts(signedIn(), connect.NewRequest(&pb.ListMyProductsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.app.ListMyProducts(signedIn(), connect.NewRequest(&pb.ListMyProductsRequest{PageToken: sellerFirst.Msg.GetNextPageToken(), Filter: &pb.MyProductFilter{Query: "別的"}})); codeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("賣家清單的 token 換了篩選要被拒：%v", err)
	}
}

// 分類樹依請求語系取名稱，沒有英文名稱時用 zh-TW；頂層在前、底下掛葉節點。
func TestCategoryTreeNamesFollowTheLocale(t *testing.T) {
	t.Parallel()
	top, leaf, bare := uuid.UUID{1}, uuid.UUID{2}, uuid.UUID{3}
	cats := []catalog.Category{
		{ID: top, NameZhTW: "蔬果", NameEn: "Produce"},
		{ID: leaf, ParentID: &top, NameZhTW: "葉菜", NameEn: "Leafy"},
		{ID: bare, ParentID: &top, NameZhTW: "根莖"},
	}
	en := categoryTree(cats, uitext.EN)
	if len(en) != 1 || en[0].GetName() != "Produce" || len(en[0].GetChildren()) != 2 || en[0].GetChildren()[0].GetName() != "Leafy" || en[0].GetChildren()[1].GetName() != "根莖" {
		t.Fatalf("en = %v", en)
	}
	zh := categoryTree(cats, uitext.ZhTW)
	if zh[0].GetName() != "蔬果" || zh[0].GetChildren()[0].GetName() != "葉菜" || len(zh[0].GetChildren()[0].GetChildren()) != 0 {
		t.Fatalf("zh = %v", zh)
	}
}

// 模組的錯誤對到契約的分類與原因；欄位對到請求路徑；不是這家店的商品與不存在一樣是 not_found。
func TestModuleErrorsMapToTheContract(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		err    error
		paths  rpc.FieldPaths
		code   connect.Code
		reason string
		fields []string
	}{
		{"商品不存在", catalog.ErrProductNotFound, noPaths, connect.CodeNotFound, "", nil},
		{"規格不存在", catalog.ErrSkuNotFound, noPaths, connect.CodeNotFound, "", nil},
		{"版本衝突", catalog.ErrVersionConflict, noPaths, connect.CodeAborted, "VERSION_CONFLICT", nil},
		{"上架沒有主圖", catalog.ErrActiveRequiresGalleryImage, noPaths, connect.CodeFailedPrecondition, "", nil},
		{"request_id 內容不同", catalog.ErrIdempotencyKeyReused, createPaths, connect.CodeInvalidArgument, "", []string{"request_id"}},
		{"庫存不足", &catalog.InsufficientStockError{}, adjustPaths, connect.CodeFailedPrecondition, "", nil},
		{"庫存預期不符", &catalog.StockMismatchError{Current: 12}, adjustPaths, connect.CodeFailedPrecondition, "", nil},
		{"標題必填", &catalog.InvalidInputError{Field: "Title", Reason: catalog.ReasonRequired}, createPaths, connect.CodeInvalidArgument, "", []string{"product.title"}},
		{"第二個 SKU 價格", &catalog.InvalidInputError{Field: "Skus[1].Price", Reason: catalog.ReasonOutOfRange}, updatePaths, connect.CodeInvalidArgument, "", []string{"product.skus[1].price"}},
		{"初始庫存", &catalog.InvalidInputError{Field: "Skus[0].StockQty", Reason: catalog.ReasonOutOfRange}, createPaths, connect.CodeInvalidArgument, "", []string{"product.skus[0].initial_stock"}},
		{"描述的段落", &catalog.InvalidInputError{Field: "Description[2].Text", Reason: catalog.ReasonRequired}, createPaths, connect.CodeInvalidArgument, "", []string{"product.description[2].paragraph"}},
		{"表格儲存格", &catalog.InvalidInputError{Field: "Description[1].Rows[3][4]", Reason: catalog.ReasonTooLong}, createPaths, connect.CodeInvalidArgument, "", []string{"product.description[1].table.rows[3].cells[4]"}},
		{"圖片重複的索引是圖片序號不是區塊序號", &catalog.InvalidInputError{Field: "Description.Image[5]", Reason: catalog.ReasonDuplicate}, createPaths, connect.CodeInvalidArgument, "", []string{"product.description"}},
		{"標籤", &catalog.InvalidInputError{Field: "Hashtags", Reason: catalog.ReasonTooMany}, createPaths, connect.CodeInvalidArgument, "", []string{"product.hashtag_inputs"}},
		{"賣家列表的狀態篩選", &catalog.InvalidInputError{Field: "StockStatus", Reason: catalog.ReasonUnknown}, sellerListPaths, connect.CodeInvalidArgument, "", []string{"filter.stock_status"}},
		{"沒有分類的錯誤原樣", errors.New("db down"), noPaths, connect.CodeInternal, "", nil},
	}
	for _, c := range cases {
		err := failure(c.err, c.paths)
		reason, fields := reasonAndFields(err)
		if codeOf(err) != c.code || reason != c.reason || !slices.Equal(fields, c.fields) {
			t.Errorf("%s：code=%v reason=%q 欄位=%v，預期 %v %q %v", c.name, codeOf(err), reason, fields, c.code, c.reason, c.fields)
		}
	}
	if failure(nil, noPaths) != nil {
		t.Fatal("nil 要回 nil")
	}
}

// 欄位對應表裡的每個路徑都存在於請求訊息：proto 改名時對應表要跟著改。
func TestFieldPathsExistInTheRequests(t *testing.T) {
	t.Parallel()
	checks := []struct {
		name  string
		paths rpc.FieldPaths
		desc  func() []string
	}{
		{"create", createPaths, func() []string {
			return rpc.CheckFieldPaths((&pb.CreateProductRequest{}).ProtoReflect().Descriptor(), createPaths)
		}},
		{"update", updatePaths, func() []string {
			return rpc.CheckFieldPaths((&pb.UpdateProductRequest{}).ProtoReflect().Descriptor(), updatePaths)
		}},
		{"status", statusPaths, func() []string {
			return rpc.CheckFieldPaths((&pb.SetProductStatusRequest{}).ProtoReflect().Descriptor(), statusPaths)
		}},
		{"delete", deletePaths, func() []string {
			return rpc.CheckFieldPaths((&pb.DeleteProductRequest{}).ProtoReflect().Descriptor(), deletePaths)
		}},
		{"adjust", adjustPaths, func() []string {
			return rpc.CheckFieldPaths((&pb.AdjustStockRequest{}).ProtoReflect().Descriptor(), adjustPaths)
		}},
		{"set", setPaths, func() []string {
			return rpc.CheckFieldPaths((&pb.SetStockRequest{}).ProtoReflect().Descriptor(), setPaths)
		}},
		{"seller list", sellerListPaths, func() []string {
			return rpc.CheckFieldPaths((&pb.ListMyProductsRequest{}).ProtoReflect().Descriptor(), sellerListPaths)
		}},
		{"public list", publicListPaths, func() []string {
			return rpc.CheckFieldPaths((&pb.ListProductsRequest{}).ProtoReflect().Descriptor(), publicListPaths)
		}},
	}
	for _, c := range checks {
		if missing := c.desc(); len(missing) != 0 {
			t.Errorf("%s 的對應表有請求裡不存在的路徑：%v", c.name, missing)
		}
	}
}

// 管理員才能呼叫後台方法；顧客帳號身分被擋。
func TestAdminMethodsRequireAnAdmin(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	if _, err := r.adm.ListProducts(signedIn(), connect.NewRequest(&adminpb.ListProductsRequest{})); codeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("顧客：%v", err)
	}
	if _, err := r.adm.GetProduct(signedIn(), connect.NewRequest(&adminpb.GetProductRequest{ProductId: uuid.UUID{1}.String()})); codeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("顧客：%v", err)
	}
	if len(r.m.calls) != 0 {
		t.Fatalf("被擋的呼叫不該碰模組：%v", r.m.calls)
	}
	if _, err := r.adm.GetProduct(adminCtx(), connect.NewRequest(&adminpb.GetProductRequest{ProductId: "not-a-uuid"})); codeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("壞 ID：%v", err)
	}
}

// 寫入請求的必填欄位與格式在入口檢查，不碰模組：request_id 要 UUIDv4、etag 必填、狀態不得是 UNSPECIFIED。
func TestSellerWriteRequestsAreCheckedAtTheDoor(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	ctx := signedIn()
	id, sku := uuid.UUID{1}.String(), uuid.UUID{3}.String()
	cases := []struct {
		name  string
		err   func() error
		field string
	}{
		{"建立缺 request_id", func() error {
			_, e := r.app.CreateProduct(ctx, connect.NewRequest(&pb.CreateProductRequest{Product: sampleInput()}))
			return e
		}, "request_id"},
		{"建立的 request_id 不是 v4", func() error {
			_, e := r.app.CreateProduct(ctx, connect.NewRequest(&pb.CreateProductRequest{Product: sampleInput(), RequestId: uuid.NewV7().String()}))
			return e
		}, "request_id"},
		{"建立缺 product", func() error {
			_, e := r.app.CreateProduct(ctx, connect.NewRequest(&pb.CreateProductRequest{RequestId: validRequestID()}))
			return e
		}, "product"},
		{"修改缺 etag", func() error {
			_, e := r.app.UpdateProduct(ctx, connect.NewRequest(&pb.UpdateProductRequest{ProductId: id, Product: sampleInput()}))
			return e
		}, "etag"},
		{"切換狀態缺 etag", func() error {
			_, e := r.app.SetProductStatus(ctx, connect.NewRequest(&pb.SetProductStatusRequest{ProductId: id, Status: pb.ProductStatus_PRODUCT_STATUS_ACTIVE}))
			return e
		}, "etag"},
		{"切換狀態是 UNSPECIFIED", func() error {
			_, e := r.app.SetProductStatus(ctx, connect.NewRequest(&pb.SetProductStatusRequest{ProductId: id, Etag: "3"}))
			return e
		}, "status"},
		{"調整量為 0", func() error {
			_, e := r.app.AdjustStock(ctx, connect.NewRequest(&pb.AdjustStockRequest{SkuId: sku, RequestId: validRequestID()}))
			return e
		}, "delta"},
		{"調整缺 request_id", func() error {
			_, e := r.app.AdjustStock(ctx, connect.NewRequest(&pb.AdjustStockRequest{SkuId: sku, Delta: 1}))
			return e
		}, "request_id"},
		{"設定的庫存為負", func() error {
			_, e := r.app.SetStock(ctx, connect.NewRequest(&pb.SetStockRequest{SkuId: sku, Quantity: -1, RequestId: validRequestID()}))
			return e
		}, "quantity"},
	}
	for _, c := range cases {
		err := c.err()
		_, fields := reasonAndFields(err)
		if codeOf(err) != connect.CodeInvalidArgument || !slices.Equal(fields, []string{c.field}) {
			t.Errorf("%s：code=%v 欄位=%v，預期 invalid_argument 指向 %s", c.name, codeOf(err), fields, c.field)
		}
	}
	if _, err := r.app.UpdateProduct(ctx, connect.NewRequest(&pb.UpdateProductRequest{ProductId: id, Etag: "abc", Product: sampleInput()})); codeOf(err) != connect.CodeAborted {
		t.Fatalf("亂寫的 etag：%v", err)
	}
	if len(r.m.calls) != 0 {
		t.Fatalf("擋在入口的請求不該碰模組：%v", r.m.calls)
	}
}

// etag、request_id 與預期庫存原樣傳給模組（版本是 etag 還原的數字，冪等鍵是 request_id 的字串）。
func TestEtagAndRequestIDReachTheModule(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.m.product = sampleProduct()
	ctx := signedIn()
	id, sku := uuid.UUID{1}.String(), uuid.UUID{3}.String()
	rid := validRequestID()
	if _, err := r.app.UpdateProduct(ctx, connect.NewRequest(&pb.UpdateProductRequest{ProductId: id, Etag: catalog.Etag(41), Product: sampleInput()})); err != nil {
		t.Fatal(err)
	}
	if _, err := r.app.SetProductStatus(ctx, connect.NewRequest(&pb.SetProductStatusRequest{ProductId: id, Etag: catalog.Etag(42), Status: pb.ProductStatus_PRODUCT_STATUS_ARCHIVED})); err != nil {
		t.Fatal(err)
	}
	if _, err := r.app.DeleteProduct(ctx, connect.NewRequest(&pb.DeleteProductRequest{ProductId: id, Etag: catalog.Etag(43)})); err != nil {
		t.Fatal(err)
	}
	if _, err := r.app.DeleteProduct(ctx, connect.NewRequest(&pb.DeleteProductRequest{ProductId: id})); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(r.m.versions, []int64{41, 42, 43, 0}) {
		t.Fatalf("版本 = %v，預期 [41 42 43 0]（刪除不帶 etag 是 0 不比對）", r.m.versions)
	}
	if _, err := r.app.AdjustStock(ctx, connect.NewRequest(&pb.AdjustStockRequest{SkuId: sku, Delta: -2, RequestId: rid})); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(r.m.keys, []string{rid}) || !slices.Equal(r.m.deltas, []int32{-2}) {
		t.Fatalf("冪等鍵 %v、變動量 %v", r.m.keys, r.m.deltas)
	}
}

// 庫存調整已生效之後的讀回只依模組回報的 SKU 狀態：SKU 或商品已被並行刪除（不可購買）也回成功與已生效的庫存，不回 not_found。
func TestStockWritesRespondWithTheAppliedStateEvenWhenTheSkuIsGone(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.m.sku = catalog.SkuState{Sku: catalog.Sku{ID: uuid.UUID{3}, Price: 100, StockQty: 7}, Sellable: false}
	ctx := signedIn()
	sku := uuid.UUID{3}.String()
	adj, err := r.app.AdjustStock(ctx, connect.NewRequest(&pb.AdjustStockRequest{SkuId: sku, Delta: 2, RequestId: validRequestID()}))
	if err != nil || adj.Msg.GetSku().GetStockQty() != 7 || adj.Msg.GetSku().GetAvailability() != pb.ProductAvailability_PRODUCT_AVAILABILITY_UNAVAILABLE {
		t.Fatalf("AdjustStock = %v, %v", adj, err)
	}
	set, err := r.app.SetStock(ctx, connect.NewRequest(&pb.SetStockRequest{SkuId: sku, Quantity: 7, ExpectedQuantity: 5, RequestId: validRequestID()}))
	if err != nil || set.Msg.GetSku().GetStockQty() != 7 {
		t.Fatalf("SetStock = %v, %v", set, err)
	}
}

// 名稱建議與搜尋一樣先去頭尾空白再比長度、再交給模組：頭尾空白不佔長度上限，模組收到的是去空白後的關鍵字。
func TestSuggestProductNamesNormalizesTheQueryBeforeCheckingAndPassingIt(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	ctx := context.Background()
	exact := strings.Repeat("山", int(maxSuggestRunes))
	for _, q := range []string{"  " + exact + "  ", "\u3000" + exact + "\u3000"} {
		if _, err := r.pub.SuggestProductNames(ctx, connect.NewRequest(&pb.SuggestProductNamesRequest{Query: q})); err != nil {
			t.Fatalf("去掉頭尾空白剛好是上限的關鍵字要接受：%v", err)
		}
	}
	if len(r.m.queries) != 2 || r.m.queries[0] != exact || r.m.queries[1] != exact {
		t.Fatalf("模組收到 %q，預期去空白後的關鍵字", r.m.queries)
	}
	_, err := r.pub.SuggestProductNames(ctx, connect.NewRequest(&pb.SuggestProductNamesRequest{Query: exact + "山"}))
	if codeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("超過上限要回 invalid_argument：%v", err)
	}
}

// 後台列表的關鍵字上限取自後台請求自己的欄位宣告，同樣先去頭尾空白。
func TestAdminListProductsChecksTheQueryAgainstItsOwnLimit(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	ctx := adminCtx()
	exact := strings.Repeat("山", int(maxAdminQueryRunes))
	if _, err := r.adm.ListProducts(ctx, connect.NewRequest(&adminpb.ListProductsRequest{Query: " " + exact + " "})); err != nil {
		t.Fatalf("去掉頭尾空白剛好是上限的關鍵字要接受：%v", err)
	}
	_, err := r.adm.ListProducts(ctx, connect.NewRequest(&adminpb.ListProductsRequest{Query: exact + "山"}))
	if codeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("超過上限要回 invalid_argument：%v", err)
	}
}
