//go:build integration

package catalog_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"uuid"

	"connectrpc.com/connect"

	"github.com/Will413028/hawkr/backend/internal/httpapi"
	httpcatalog "github.com/Will413028/hawkr/backend/internal/httpapi/catalog"
	"github.com/Will413028/hawkr/backend/internal/httpapi/catalog/catalogtest"
	"github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/catalog/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/catalog/v1alpha1/catalogv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion"
	"github.com/Will413028/hawkr/backend/internal/modules/assistant"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon"
	"github.com/Will413028/hawkr/backend/internal/modules/crm"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/discovery"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/imageai"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation"
	"github.com/Will413028/hawkr/backend/internal/modules/notification"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
	"github.com/Will413028/hawkr/backend/internal/modules/review"
	"github.com/Will413028/hawkr/backend/internal/modules/reviewqueue"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/bizcalendar"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

// acting 以資料庫裡的店家（seller.stores 的 account_id）推導呼叫帳號的店，與 seller.ActingStore 的對應相同；
// 能力與停權的判斷在 seller 的測試。
type acting struct{ w *catalogtest.World }

func (a acting) ActingStore(ctx context.Context, accountID int64, _ seller.Capability) (seller.Store, error) {
	var id int64
	if err := a.w.DB.Pool().QueryRow(ctx, `SELECT store_id FROM seller.stores WHERE account_id = $1`, accountID).Scan(&id); err != nil {
		return seller.Store{}, apperr.NotFound("沒有店家")
	}
	return seller.Store{StoreID: id, AccountID: accountID}, nil
}

type cards struct{}

func (cards) StoreCards(_ context.Context, ids []int64) ([]seller.StoreCard, error) {
	out := make([]seller.StoreCard, len(ids))
	for i, id := range ids {
		name := "店" + strconv.FormatInt(id, 10)
		out[i] = seller.StoreCard{StoreID: id, StoreCode: id, Name: &name, Status: seller.CardOpen}
	}
	return out, nil
}

const accountHeader = "X-Test-Account"

type env struct {
	w      *catalogtest.World
	url    string
	public catalogv1alpha1connect.CatalogPublicServiceClient
}

// setup 起 catalog 的 Connect handler（與正式組裝相同的 interceptor）；測試的呼叫者以 X-Test-Account 標示帳號，
// 店家由 acting 從資料庫推導。
func setup(t *testing.T) *env {
	t.Helper()
	w := catalogtest.New(t, pg)
	images := characterize.Images(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	presenter, err := httpcatalog.NewPresenter(cards{}, characterize.Stats(t), images)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := httpcatalog.NewPublic(w.Module, presenter, w.Now)
	if err != nil {
		t.Fatal(err)
	}
	app, err := httpcatalog.NewApp(w.Module, acting{w}, images, log)
	if err != nil {
		t.Fatal(err)
	}
	opts := connect.WithInterceptors(rpc.NewInterceptor(log))
	mux := http.NewServeMux()
	path, h := catalogv1alpha1connect.NewCatalogPublicServiceHandler(pub, opts)
	mux.Handle(path, h)
	path, h = catalogv1alpha1connect.NewSellerCatalogAppServiceHandler(app, opts)
	mux.Handle(path, h)
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if v := r.Header.Get(accountHeader); v != "" {
			id, _ := strconv.ParseInt(v, 10, 64)
			r = r.WithContext(rpc.WithPrincipal(r.Context(), identity.Principal{Kind: identity.PrincipalAccount, ID: id}))
		}
		mux.ServeHTTP(rw, r)
	}))
	t.Cleanup(srv.Close)
	return &env{w: w, url: srv.URL, public: catalogv1alpha1connect.NewCatalogPublicServiceClient(srv.Client(), srv.URL)}
}

func (e *env) seller(account int64) catalogv1alpha1connect.SellerCatalogAppServiceClient {
	as := connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			req.Header().Set(accountHeader, strconv.FormatInt(account, 10))
			return next(ctx, req)
		}
	})
	return catalogv1alpha1connect.NewSellerCatalogAppServiceClient(http.DefaultClient, e.url, connect.WithInterceptors(as))
}

func wantCode(t *testing.T, err error, code connect.Code) {
	t.Helper()
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != code {
		t.Fatalf("err=%v，預期 %v", err, code)
	}
}

func listedIDs(products []*pb.ProductSummary) []string {
	out := make([]string, len(products))
	for i, p := range products {
		out[i] = p.GetId()
	}
	return out
}

func contains(ids []string, id uuid.UUID) bool {
	for _, x := range ids {
		if x == id.String() {
			return true
		}
	}
	return false
}

// 跨店的讀與寫都被拒（與不存在一樣是 not_found，不洩漏商品存在）：另一家店對這家店的商品與 SKU 做每一種賣家操作，
// 回應都是 not_found，而且商品的內容、狀態、庫存與版本完全沒變。店家 ID（7000 起）與帳號 ID（1001 起）刻意不同值。
func TestSellerCannotReadOrWriteAnotherStoresProduct(t *testing.T) {
	t.Parallel()
	e := setup(t)
	ctx := context.Background()
	storeA, acctA := e.w.Store()
	storeB, acctB := e.w.Store()
	if storeA < catalogtest.FirstStoreID || storeB < catalogtest.FirstStoreID || storeA == acctA || storeB == acctB {
		t.Fatalf("測試的店家 ID（%d、%d）與帳號 ID（%d、%d）要互不相同", storeA, storeB, acctA, acctB)
	}
	a := e.w.Active(storeA, acctA, e.w.Input("A 店的山藥", 5))
	sku := a.Skus[0].ID.String()
	b := e.seller(acctB)

	_, err := b.GetMyProduct(ctx, connect.NewRequest(&pb.GetMyProductRequest{ProductId: a.ID.String()}))
	wantCode(t, err, connect.CodeNotFound)
	_, err = b.UpdateProduct(ctx, connect.NewRequest(&pb.UpdateProductRequest{
		ProductId: a.ID.String(), Etag: catalog.Etag(a.Version),
		Product: &pb.ProductInput{Title: "被 B 改掉", CategoryId: a.CategoryID.String(), Skus: []*pb.SkuInput{{Price: 1}}},
	}))
	wantCode(t, err, connect.CodeNotFound)
	_, err = b.SetProductStatus(ctx, connect.NewRequest(&pb.SetProductStatusRequest{ProductId: a.ID.String(), Etag: catalog.Etag(a.Version), Status: pb.ProductStatus_PRODUCT_STATUS_ARCHIVED}))
	wantCode(t, err, connect.CodeNotFound)
	_, err = b.DeleteProduct(ctx, connect.NewRequest(&pb.DeleteProductRequest{ProductId: a.ID.String()}))
	wantCode(t, err, connect.CodeNotFound)
	_, err = b.AdjustStock(ctx, connect.NewRequest(&pb.AdjustStockRequest{SkuId: sku, Delta: 100, RequestId: uuid.NewV4().String()}))
	wantCode(t, err, connect.CodeNotFound)
	_, err = b.SetStock(ctx, connect.NewRequest(&pb.SetStockRequest{SkuId: sku, Quantity: 999, ExpectedQuantity: 5, RequestId: uuid.NewV4().String()}))
	wantCode(t, err, connect.CodeNotFound)

	list, err := b.ListMyProducts(ctx, connect.NewRequest(&pb.ListMyProductsRequest{}))
	if err != nil || len(list.Msg.GetProducts()) != 0 {
		t.Fatalf("B 的清單 = %v, %v，不該看到 A 的商品", list.Msg.GetProducts(), err)
	}
	count, err := b.CountMyProducts(ctx, connect.NewRequest(&pb.CountMyProductsRequest{}))
	if err != nil || count.Msg.GetCount() != 0 {
		t.Fatalf("B 的計數 = %v, %v", count.Msg.GetCount(), err)
	}

	got, err := e.w.Module.Product(ctx, storeA, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "A 店的山藥" || got.Status != catalog.StatusActive || got.Version != a.Version || got.Skus[0].StockQty != 5 {
		t.Fatalf("A 的商品被改了：%+v", got)
	}
	// A 自己照常讀得到、改得動。
	own, err := e.seller(acctA).GetMyProduct(ctx, connect.NewRequest(&pb.GetMyProductRequest{ProductId: a.ID.String()}))
	if err != nil || own.Msg.GetProduct().GetTitle() != "A 店的山藥" {
		t.Fatalf("A 讀自己的商品 = %v, %v", own, err)
	}
	setReq := &pb.SetStockRequest{SkuId: sku, Quantity: 8, ExpectedQuantity: 5, RequestId: uuid.NewV4().String()}
	set, err := e.seller(acctA).SetStock(ctx, connect.NewRequest(setReq))
	if err != nil || set.Msg.GetSku().GetStockQty() != 8 || set.Msg.GetSku().GetAvailability() != pb.ProductAvailability_PRODUCT_AVAILABILITY_AVAILABLE {
		t.Fatalf("A 設定庫存 = %v, %v", set, err)
	}
	// 同一個 request_id 重送：現值已是 8，預期值 5 不符，只有冪等重放才回成功。
	again, err := e.seller(acctA).SetStock(ctx, connect.NewRequest(setReq))
	if err != nil || again.Msg.GetSku().GetStockQty() != 8 {
		t.Fatalf("同一個 request_id 重送設定庫存 = %v, %v", again, err)
	}
}

// 賣家寫入走完整的 etag 與冪等語意：過時的 etag 回 VERSION_CONFLICT；request_id 重送只建立一個商品；庫存調整不改 etag。
func TestSellerWritesCarryEtagAndRequestID(t *testing.T) {
	t.Parallel()
	e := setup(t)
	ctx := context.Background()
	_, acct := e.w.Store()
	s := e.seller(acct)
	in := e.w.Input("山藥", 3)
	create := &pb.CreateProductRequest{RequestId: uuid.NewV4().String(), Product: &pb.ProductInput{
		Title: in.Title, CategoryId: in.CategoryID.String(),
		Gallery: []*pb.ImageInput{{Source: &pb.ImageInput_StagedToken{StagedToken: in.Gallery[0].Token}}},
		Skus:    []*pb.SkuInput{{Price: 100, InitialStock: 3}},
	}}
	first, err := s.CreateProduct(ctx, connect.NewRequest(create))
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.CreateProduct(ctx, connect.NewRequest(create))
	if err != nil || again.Msg.GetProduct().GetId() != first.Msg.GetProduct().GetId() {
		t.Fatalf("同一個 request_id 重送要回同一個商品：%v, %v", again, err)
	}
	p := first.Msg.GetProduct()
	if p.GetStatus() != pb.ProductStatus_PRODUCT_STATUS_DRAFT || p.GetEtag() == "" || p.GetSkus()[0].GetStockQty() != 3 {
		t.Fatalf("新商品 = %v", p)
	}
	stale := strconv.FormatInt(mustVersion(t, p.GetEtag())+1, 10)
	_, err = s.SetProductStatus(ctx, connect.NewRequest(&pb.SetProductStatusRequest{ProductId: p.GetId(), Etag: stale, Status: pb.ProductStatus_PRODUCT_STATUS_ACTIVE}))
	wantCode(t, err, connect.CodeAborted)
	adjust := &pb.AdjustStockRequest{SkuId: p.GetSkus()[0].GetId(), Delta: 2, RequestId: uuid.NewV4().String()}
	for range 2 {
		// 同一個 request_id 重送只生效一次：庫存 3 + 2 = 5，不是 7。
		adjusted, err := s.AdjustStock(ctx, connect.NewRequest(adjust))
		if err != nil || adjusted.Msg.GetSku().GetStockQty() != 5 {
			t.Fatalf("調整庫存 = %v, %v", adjusted, err)
		}
	}
	active, err := s.SetProductStatus(ctx, connect.NewRequest(&pb.SetProductStatusRequest{ProductId: p.GetId(), Etag: p.GetEtag(), Status: pb.ProductStatus_PRODUCT_STATUS_ACTIVE}))
	if err != nil {
		t.Fatalf("調整庫存不改 etag，用舊 etag 上架要成功：%v", err)
	}
	if active.Msg.GetProduct().GetStatus() != pb.ProductStatus_PRODUCT_STATUS_ACTIVE || active.Msg.GetProduct().GetSkus()[0].GetStockQty() != 5 {
		t.Fatalf("上架後 = %v", active.Msg.GetProduct())
	}
	// 預期庫存不符：failed_precondition，庫存不變。
	_, err = s.SetStock(ctx, connect.NewRequest(&pb.SetStockRequest{SkuId: p.GetSkus()[0].GetId(), Quantity: 1, ExpectedQuantity: 3, RequestId: uuid.NewV4().String()}))
	wantCode(t, err, connect.CodeFailedPrecondition)
	// 低庫存計數：門檻預設 5，庫存 5 的 ACTIVE 商品算偏低。
	count, err := s.CountMyProducts(ctx, connect.NewRequest(&pb.CountMyProductsRequest{Filter: &pb.MyProductFilter{Status: pb.ProductStatus_PRODUCT_STATUS_ACTIVE, StockStatus: pb.StockStatus_STOCK_STATUS_LOW}}))
	if err != nil || count.Msg.GetCount() != 1 || count.Msg.GetDefaultLowStockThreshold() != catalog.DefaultLowStockThreshold {
		t.Fatalf("低庫存計數 = %v, %v", count, err)
	}
}

func mustVersion(t *testing.T, etag string) int64 {
	t.Helper()
	v, err := catalog.ParseEtag(etag)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// 店家不可曝光（KYC 被退回）時，這家店的商品從公開列表、搜尋與商品頁消失，恢復後回來；另一家可曝光的店不受影響。
func TestNonExposableStoreProductsAreInvisibleToThePublic(t *testing.T) {
	t.Parallel()
	e := setup(t)
	ctx := context.Background()
	hiddenStore, hiddenAcct := e.w.Store()
	shownStore, shownAcct := e.w.Store()
	hidden := e.w.Active(hiddenStore, hiddenAcct, e.w.Input("暫停店的蘋果", 3))
	shown := e.w.Active(shownStore, shownAcct, e.w.Input("正常店的蘋果", 3))

	visible := func() (list, search []string, getErr error) {
		l, err := e.public.ListProducts(ctx, connect.NewRequest(&pb.ListProductsRequest{}))
		if err != nil {
			t.Fatal(err)
		}
		s, err := e.public.SearchProducts(ctx, connect.NewRequest(&pb.SearchProductsRequest{Query: "蘋果"}))
		if err != nil {
			t.Fatal(err)
		}
		_, getErr = e.public.GetProduct(ctx, connect.NewRequest(&pb.GetProductRequest{ProductId: hidden.ID.String()}))
		return listedIDs(l.Msg.GetProducts()), listedIDs(s.Msg.GetProducts()), getErr
	}
	list, search, getErr := visible()
	if !contains(list, hidden.ID) || !contains(search, hidden.ID) || getErr != nil {
		t.Fatalf("可曝光時要看得到：list=%v search=%v get=%v", list, search, getErr)
	}
	e.w.SetExposable(hiddenStore, false)
	list, search, getErr = visible()
	if contains(list, hidden.ID) || contains(search, hidden.ID) {
		t.Fatalf("不可曝光的店的商品出現在公開列表 %v 或搜尋 %v", list, search)
	}
	wantCode(t, getErr, connect.CodeNotFound)
	if !contains(list, shown.ID) || !contains(search, shown.ID) {
		t.Fatalf("另一家可曝光的店的商品不該消失：list=%v search=%v", list, search)
	}
	byStore, err := e.public.ListProducts(ctx, connect.NewRequest(&pb.ListProductsRequest{Filter: &pb.ProductFilter{StoreIds: []string{strconv.FormatInt(hiddenStore, 10)}}}))
	if err != nil || len(byStore.Msg.GetProducts()) != 0 {
		t.Fatalf("指定不可曝光的店 = %v, %v", byStore, err)
	}
	e.w.SetExposable(hiddenStore, true)
	list, _, getErr = visible()
	if !contains(list, hidden.ID) || getErr != nil {
		t.Fatalf("恢復曝光後要回來：%v %v", list, getErr)
	}
}

// 已下架（ARCHIVED）的商品頁讀得到、availability 是不可購買、不在列表與搜尋；草稿與已刪除是 not_found；ACTIVE 有庫存是可購買。
func TestArchivedProductPageIsReadableButNotPurchasable(t *testing.T) {
	t.Parallel()
	e := setup(t)
	ctx := context.Background()
	store, acct := e.w.Store()
	p := e.w.Active(store, acct, e.w.Input("香蕉", 4))
	get := func(id uuid.UUID) (*pb.Product, error) {
		r, err := e.public.GetProduct(ctx, connect.NewRequest(&pb.GetProductRequest{ProductId: id.String()}))
		if err != nil {
			return nil, err
		}
		return r.Msg.GetProduct(), nil
	}
	got, err := get(p.ID)
	if err != nil || got.GetAvailability() != pb.ProductAvailability_PRODUCT_AVAILABILITY_AVAILABLE || got.GetSkus()[0].GetAvailability() != pb.ProductAvailability_PRODUCT_AVAILABILITY_AVAILABLE {
		t.Fatalf("ACTIVE = %v, %v", got, err)
	}
	e.w.SetStatus(store, p.ID, catalog.StatusArchived)
	got, err = get(p.ID)
	if err != nil {
		t.Fatalf("下架的商品頁要讀得到：%v", err)
	}
	if got.GetTitle() != "香蕉" || got.GetAvailability() != pb.ProductAvailability_PRODUCT_AVAILABILITY_UNAVAILABLE || got.GetSkus()[0].GetAvailability() != pb.ProductAvailability_PRODUCT_AVAILABILITY_UNAVAILABLE || got.GetSkus()[0].GetStockQty() != 4 {
		t.Fatalf("ARCHIVED 的商品頁 = %v", got)
	}
	l, err := e.public.ListProducts(ctx, connect.NewRequest(&pb.ListProductsRequest{}))
	if err != nil || contains(listedIDs(l.Msg.GetProducts()), p.ID) {
		t.Fatalf("下架的商品不該在列表：%v %v", l, err)
	}
	e.w.SetStatus(store, p.ID, catalog.StatusDraft)
	_, err = get(p.ID)
	wantCode(t, err, connect.CodeNotFound)
	draft := e.w.Create(store, acct, e.w.Input("草稿", 1))
	_, err = get(draft.ID)
	wantCode(t, err, connect.CodeNotFound)
	if err := e.w.Module.DeleteProduct(ctx, store, p.ID, 0); err != nil {
		t.Fatal(err)
	}
	_, err = get(p.ID)
	wantCode(t, err, connect.CodeNotFound)
}

// 列表依 keyset 翻頁不重不漏，token 在公開與賣家各自可用；公開卡片帶店家與主圖。
func TestPublicListPagesWithoutRepeatOrGap(t *testing.T) {
	t.Parallel()
	e := setup(t)
	ctx := context.Background()
	store, acct := e.w.Store()
	var want []uuid.UUID
	for i := range 5 {
		want = append(want, e.w.Active(store, acct, e.w.Input("商品"+strconv.Itoa(i), 2)).ID)
	}
	seen := map[string]bool{}
	token := ""
	for range 10 {
		r, err := e.public.ListProducts(ctx, connect.NewRequest(&pb.ListProductsRequest{PageSize: 2, PageToken: token}))
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range r.Msg.GetProducts() {
			if seen[p.GetId()] {
				t.Fatalf("重複：%s", p.GetId())
			}
			seen[p.GetId()] = true
			if p.GetStore().GetId() != strconv.FormatInt(store, 10) || p.GetCoverImage() == nil {
				t.Fatalf("卡片的店家或主圖沒有組：%v", p)
			}
		}
		if token = r.Msg.GetNextPageToken(); token == "" {
			break
		}
	}
	for _, id := range want {
		if !seen[id.String()] {
			t.Fatalf("漏掉 %s", id)
		}
	}
}

// 公開 GET 帶 Vary（含 Accept-Language）與快取標頭，錯誤回應也一樣；分類名稱依 Accept-Language。走完整的 httpapi 入口（cachePolicy 與 interceptor）。
func TestPublicGetsVaryOnAcceptLanguage(t *testing.T) {
	t.Parallel()
	w := catalogtest.New(t, pg)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h, err := httpapi.NewHandler(httpapi.Deps{
		Procurement: &procurement.Module{}, Catalog: w.Module, ImageAI: &imageai.Module{}, Assistant: &assistant.Module{}, CRM: &crm.Module{}, Coupon: &coupon.Module{},
		Buyer: &buyer.Module{}, Social: &social.Module{}, Moderation: &moderation.Module{}, Notification: &notification.Module{},
		Ready: func(context.Context) error { return nil }, Delegation: delegation.New(delegation.Deps{Seller: &seller.Module{}, Social: &social.Module{}}),
		Deletion: &accountdeletion.Module{}, Seller: &seller.Module{}, Stats: characterize.Stats(t), WebOrigin: characterize.WebOrigin(t), Images: characterize.Images(t),
		DashboardOrders: struct{ httpseller.DashboardOrders }{}, Discovery: &discovery.Module{}, ReviewQueue: &reviewqueue.Module{}, Review: &review.Module{},
		IM: &im.Module{}, Identity: &identity.Module{}, BizCalendar: &bizcalendar.Store{}, Payment: &payment.Module{}, Logger: log,
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	type result struct {
		StatusCode int
		Header     http.Header
	}
	get := func(procedure, message, lang string) (result, string) {
		t.Helper()
		u := srv.URL + procedure + "?" + url.Values{"connect": {"v1"}, "encoding": {"json"}, "message": {message}}.Encode()
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, u, nil)
		if err != nil {
			t.Fatal(err)
		}
		if lang != "" {
			req.Header.Set("Accept-Language", lang)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return result{StatusCode: resp.StatusCode, Header: resp.Header}, string(body)
	}
	varyTokens := func(resp result) map[string]int {
		out := map[string]int{}
		for _, line := range resp.Header.Values("Vary") {
			for _, tok := range strings.Split(line, ",") {
				out[strings.TrimSpace(tok)]++
			}
		}
		return out
	}
	var zh, en string
	if err := w.DB.Pool().QueryRow(t.Context(), `SELECT name->>'zh-TW', COALESCE(name->>'en', '') FROM catalog.categories WHERE parent_id IS NULL ORDER BY position, id LIMIT 1`).Scan(&zh, &en); err != nil {
		t.Fatal(err)
	}
	if en == "" || en == zh {
		t.Fatalf("測試要有與 zh-TW 不同的英文分類名：%q %q", zh, en)
	}
	const list = catalogv1alpha1connect.CatalogPublicServiceListCategoriesProcedure
	for lang, want := range map[string]string{"en": en, "": zh, "zh-TW": zh} {
		resp, body := get(list, "{}", lang)
		if resp.StatusCode != http.StatusOK || resp.Header.Get("Cache-Control") != "public, no-cache" {
			t.Fatalf("%q：%d Cache-Control %q", lang, resp.StatusCode, resp.Header.Get("Cache-Control"))
		}
		tokens := varyTokens(resp)
		for _, name := range []string{"Accept-Language", "Accept-Encoding", "Authorization", "Cookie"} {
			if tokens[name] != 1 {
				t.Fatalf("%q：Vary = %q，缺 %s（或重複）", lang, resp.Header.Values("Vary"), name)
			}
		}
		var parsed struct {
			Categories []struct{ Name string } `json:"categories"`
		}
		if err := json.Unmarshal([]byte(body), &parsed); err != nil || len(parsed.Categories) == 0 || parsed.Categories[0].Name != want {
			t.Fatalf("%q：第一個頂層分類不是 %q：%v %.200s", lang, want, err, body)
		}
	}
	// 錯誤回應也帶 Vary: Accept-Language（內文的語言依它變化）。
	resp, _ := get(catalogv1alpha1connect.CatalogPublicServiceGetProductProcedure, `{"productId":"`+uuid.NewV7().String()+`"}`, "en")
	if resp.StatusCode != http.StatusNotFound || varyTokens(resp)["Accept-Language"] != 1 {
		t.Fatalf("not_found：%d Vary = %q", resp.StatusCode, resp.Header.Values("Vary"))
	}
}

// 庫存調整 commit 之後 SKU 被刪除（並行刪除，或同一個 request_id 重送時已刪除）：已生效的操作仍回成功與已生效的庫存，不回 not_found。
func TestAppliedStockWriteStillSucceedsAfterTheSkuIsDeleted(t *testing.T) {
	t.Parallel()
	e := setup(t)
	ctx := context.Background()
	storeID, acct := e.w.Store()
	p := e.w.Active(storeID, acct, e.w.Input("將被刪除", 5))
	sku := p.Skus[0].ID.String()
	s := e.seller(acct)

	rid := uuid.NewV4().String()
	first, err := s.AdjustStock(ctx, connect.NewRequest(&pb.AdjustStockRequest{SkuId: sku, Delta: 2, RequestId: rid}))
	if err != nil || first.Msg.GetSku().GetStockQty() != 7 {
		t.Fatalf("第一次調整 = %v, %v", first, err)
	}
	if _, err := s.DeleteProduct(ctx, connect.NewRequest(&pb.DeleteProductRequest{ProductId: p.ID.String()})); err != nil {
		t.Fatal(err)
	}
	again, err := s.AdjustStock(ctx, connect.NewRequest(&pb.AdjustStockRequest{SkuId: sku, Delta: 2, RequestId: rid}))
	if err != nil || again.Msg.GetSku().GetStockQty() != 7 || again.Msg.GetSku().GetAvailability() != pb.ProductAvailability_PRODUCT_AVAILABILITY_UNAVAILABLE {
		t.Fatalf("商品刪除後重送同一個 request_id 要回已生效的結果：%v, %v", again, err)
	}
	// 另一家店讀不到這個 SKU，仍是 not_found。
	_, otherAcct := e.w.Store()
	_, err = e.seller(otherAcct).AdjustStock(ctx, connect.NewRequest(&pb.AdjustStockRequest{SkuId: sku, Delta: 1, RequestId: uuid.NewV4().String()}))
	wantCode(t, err, connect.CodeNotFound)
}
