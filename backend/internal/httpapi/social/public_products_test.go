package social

import (
	"context"
	"log/slog"
	"slices"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

var (
	prod90  = uuid.UUID{15: 90}
	prod91  = uuid.UUID{15: 91}
	prod404 = uuid.UUID{15: 200}
)

func withProduct(p social.Post, id uuid.UUID) social.Post {
	p.ProductID = &id
	return p
}

func TestPostsCarryTheProductSummaryFromOneBatchCall(t *testing.T) {
	r := &fakeReads{feed: social.PostPage{Posts: []social.Post{
		withProduct(post(5), prod90), withProduct(post(5), prod91), withProduct(post(5), prod90), withProduct(post(5), prod404), post(5),
	}}}
	products := &fakeProducts{cards: map[uuid.UUID]ProductInfo{
		prod90: {ID: prod90, StoreID: 5, Name: "Tea", ImageKey: "p/90.jpg", MinPrice: 100, MaxPrice: 150, Available: true},
		prod91: {ID: prod91, StoreID: 5, Name: "Sold out", MinPrice: 200, MaxPrice: 200},
	}}
	h := newPublicWithProducts(t, r, &fakeViews{}, fakeBuyers{id: 7}, &fakeStores{}, products)
	resp, err := h.ListPosts(asBuyer(t.Context()), connect.NewRequest(&pb.ListPostsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(products.asked) != 1 || !slices.Equal(products.asked[0], []uuid.UUID{prod90, prod91, prod404}) {
		t.Fatalf("商品要一個批次查詢、ID 去重：%v", products.asked)
	}
	if products.incl[0] {
		t.Fatal("公開讀取不補下架的商品")
	}
	posts := resp.Msg.GetPosts()
	if got := posts[0].GetProduct(); got.GetId() != prod90.String() || got.GetName() != "Tea" || got.GetMinPrice() != 100 || got.GetMaxPrice() != 150 || !got.GetAvailable() || got.GetImage() == nil {
		t.Fatalf("商品摘要：%v", got)
	}
	if got := posts[1].GetProduct(); got.GetName() != "Sold out" || got.GetAvailable() || got.GetImage() != nil {
		t.Fatalf("售完的商品：%v", got)
	}
	if posts[3].GetProductId() != prod404.String() || posts[3].Product != nil {
		t.Fatalf("讀不到的商品：product 不設、product_id 照實回：%v", posts[3])
	}
	if posts[4].Product != nil || posts[4].ProductId != nil {
		t.Fatalf("沒有商品的貼文：%v", posts[4])
	}
}

func TestGetPostCarriesTheProductSummary(t *testing.T) {
	r := &fakeReads{post: withProduct(post(5), prod90)}
	products := &fakeProducts{cards: map[uuid.UUID]ProductInfo{prod90: {ID: prod90, Name: "Tea", MinPrice: 100, MaxPrice: 100, Available: true}}}
	h := newPublicWithProducts(t, r, &fakeViews{}, fakeBuyers{id: 7}, &fakeStores{}, products)
	resp, err := h.GetPost(asBuyer(t.Context()), connect.NewRequest(&pb.GetPostRequest{PostId: r.post.ID.String()}))
	if err != nil {
		t.Fatal(err)
	}
	if len(products.asked) != 1 || resp.Msg.GetPost().GetProduct().GetName() != "Tea" {
		t.Fatalf("單篇也補商品摘要：%v %v", products.asked, resp.Msg.GetPost().GetProduct())
	}
}

func TestPostsWithoutProductsMakeNoProductCall(t *testing.T) {
	r := &fakeReads{feed: social.PostPage{Posts: []social.Post{post(5)}}}
	products := &fakeProducts{}
	h := newPublicWithProducts(t, r, &fakeViews{}, fakeBuyers{id: 7}, &fakeStores{}, products)
	if _, err := h.ListPosts(asBuyer(t.Context()), connect.NewRequest(&pb.ListPostsRequest{})); err != nil {
		t.Fatal(err)
	}
	if len(products.asked) != 0 {
		t.Fatalf("沒有商品不查：%v", products.asked)
	}
}

func TestProductLookupFailureDegradesTheReadToNoProduct(t *testing.T) {
	r := &fakeReads{feed: social.PostPage{Posts: []social.Post{withProduct(post(5), prod90)}}}
	products := &fakeProducts{err: apperr.Unavailable("product-service down")}
	h := newPublicWithProducts(t, r, &fakeViews{}, fakeBuyers{id: 7}, &fakeStores{}, products)
	resp, err := h.ListPosts(asBuyer(t.Context()), connect.NewRequest(&pb.ListPostsRequest{}))
	if err != nil {
		t.Fatalf("商品查詢失敗不能讓讀取失敗：%v", err)
	}
	posts := resp.Msg.GetPosts()
	if len(posts) != 1 || posts[0].Product != nil || posts[0].GetProductId() != prod90.String() {
		t.Fatalf("product 不設、product_id 照實回：%v", posts)
	}
}

func TestOwnerReadsAskForInactiveProductsToo(t *testing.T) {
	products := &fakeProducts{cards: map[uuid.UUID]ProductInfo{prod90: {ID: prod90, Name: "Hidden", Available: false}}}
	h := &presenter{buyers: fakeBuyers{}, stores: &fakeStores{}, products: products, ownerView: true, images: mustImages(t), webOrigin: mustWeb(t), liked: noViewerLikes, log: slog.New(slog.DiscardHandler)}
	out, err := h.toPosts(t.Context(), social.Caller{}, []social.Post{withProduct(post(5), prod90)})
	if err != nil {
		t.Fatal(err)
	}
	if !products.incl[0] || out[0].GetProduct().GetName() != "Hidden" || out[0].GetProduct().GetAvailable() {
		t.Fatalf("店主讀自己的貼文要補下架的商品（available=false）：%v %v", products.incl, out[0].GetProduct())
	}
}

func TestProductWithABadImageKeyKeepsTheSummaryWithoutTheImage(t *testing.T) {
	r := &fakeReads{feed: social.PostPage{Posts: []social.Post{withProduct(post(5), prod90)}}}
	products := &fakeProducts{cards: map[uuid.UUID]ProductInfo{
		prod90: {ID: prod90, Name: "Tea", ImageKey: "https://legacy.example/p.jpg", MinPrice: 100, MaxPrice: 100, Available: true},
	}}
	h := newPublicWithProducts(t, r, &fakeViews{}, fakeBuyers{id: 7}, &fakeStores{}, products)
	resp, err := h.ListPosts(asBuyer(t.Context()), connect.NewRequest(&pb.ListPostsRequest{}))
	if err != nil {
		t.Fatalf("商品圖片 key 不合法不能讓讀取失敗：%v", err)
	}
	got := resp.Msg.GetPosts()[0].GetProduct()
	if got.GetName() != "Tea" || got.GetImage() != nil {
		t.Fatalf("摘要照回、圖不設：%v", got)
	}
}

type blockingProducts struct{}

func (blockingProducts) Cards(ctx context.Context, _ []uuid.UUID, _ bool) (map[uuid.UUID]ProductInfo, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestProductLookupTimeoutDegradesTheReadToNoProduct(t *testing.T) {
	r := &fakeReads{feed: social.PostPage{Posts: []social.Post{withProduct(post(5), prod90)}}}
	h := newPublicWithProducts(t, r, &fakeViews{}, fakeBuyers{id: 7}, &fakeStores{}, blockingProducts{})
	h.productTimeout = time.Millisecond
	resp, err := h.ListPosts(asBuyer(t.Context()), connect.NewRequest(&pb.ListPostsRequest{}))
	if err != nil {
		t.Fatalf("商品查詢逾時不能讓讀取失敗：%v", err)
	}
	if posts := resp.Msg.GetPosts(); len(posts) != 1 || posts[0].Product != nil || posts[0].GetProductId() != prod90.String() {
		t.Fatalf("逾時降級：%v", posts)
	}
}

type rendezvousStores struct {
	fakeStores
	productsStarted chan struct{}
	storesStarted   chan struct{}
	met             bool
}

func (f *rendezvousStores) StoreCards(ctx context.Context, ids []int64) ([]seller.StoreCard, error) {
	close(f.storesStarted)
	select {
	case <-f.productsStarted:
		f.met = true
	case <-time.After(5 * time.Second):
	}
	return f.fakeStores.StoreCards(ctx, ids)
}

type signalingProducts struct {
	fakeProducts
	started       chan struct{}
	storesStarted chan struct{}
	met           bool
}

func (f *signalingProducts) Cards(ctx context.Context, ids []uuid.UUID, incl bool) (map[uuid.UUID]ProductInfo, error) {
	close(f.started)
	select {
	case <-f.storesStarted:
		f.met = true
	case <-time.After(5 * time.Second):
	}
	return f.fakeProducts.Cards(ctx, ids, incl)
}

func TestProductLookupRunsConcurrentlyWithTheOtherEnrichmentLookups(t *testing.T) {
	started := make(chan struct{})
	storesStarted := make(chan struct{})
	stores := &rendezvousStores{productsStarted: started, storesStarted: storesStarted}
	products := &signalingProducts{started: started, storesStarted: storesStarted}
	r := &fakeReads{feed: social.PostPage{Posts: []social.Post{withProduct(post(5), prod90)}}}
	h := newPublicWithProducts(t, r, &fakeViews{}, fakeBuyers{id: 7}, &stores.fakeStores, products)
	h.stores = stores
	if _, err := h.ListPosts(asBuyer(t.Context()), connect.NewRequest(&pb.ListPostsRequest{})); err != nil {
		t.Fatal(err)
	}
	if !stores.met || !products.met {
		t.Fatal("商品查詢與店家卡片查詢必須同時進行：兩邊都要在對方進行中就已開始")
	}
}
