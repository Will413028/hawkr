package social

import (
	"log/slog"
	"slices"
	"testing"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func withProduct(p social.Post, id int64) social.Post {
	p.ProductID = &id
	return p
}

func TestPostsCarryTheProductSummaryFromOneBatchCall(t *testing.T) {
	r := &fakeReads{feed: social.PostPage{Posts: []social.Post{
		withProduct(post(5), 90), withProduct(post(5), 91), withProduct(post(5), 90), withProduct(post(5), 404), post(5),
	}}}
	products := &fakeProducts{cards: map[int64]ProductInfo{
		90: {ID: 90, StoreID: 5, Name: "Tea", ImageKey: "p/90.jpg", MinPrice: 100, MaxPrice: 150, Available: true},
		91: {ID: 91, StoreID: 5, Name: "Sold out", MinPrice: 200, MaxPrice: 200},
	}}
	h := newPublicWithProducts(t, r, &fakeViews{}, fakeBuyers{id: 7}, &fakeStores{}, products)
	resp, err := h.ListPosts(asBuyer(t.Context()), connect.NewRequest(&pb.ListPostsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(products.asked) != 1 || !slices.Equal(products.asked[0], []int64{90, 91, 404}) {
		t.Fatalf("商品要一個批次查詢、ID 去重：%v", products.asked)
	}
	if products.incl[0] {
		t.Fatal("公開讀取不補下架的商品")
	}
	posts := resp.Msg.GetPosts()
	if got := posts[0].GetProduct(); got.GetId() != "90" || got.GetName() != "Tea" || got.GetMinPrice() != 100 || got.GetMaxPrice() != 150 || !got.GetAvailable() || got.GetImage() == nil {
		t.Fatalf("商品摘要：%v", got)
	}
	if got := posts[1].GetProduct(); got.GetName() != "Sold out" || got.GetAvailable() || got.GetImage() != nil {
		t.Fatalf("售完的商品：%v", got)
	}
	if posts[3].GetProductId() != "404" || posts[3].Product != nil {
		t.Fatalf("讀不到的商品：product 不設、product_id 照實回：%v", posts[3])
	}
	if posts[4].Product != nil || posts[4].ProductId != nil {
		t.Fatalf("沒有商品的貼文：%v", posts[4])
	}
}

func TestGetPostCarriesTheProductSummary(t *testing.T) {
	r := &fakeReads{post: withProduct(post(5), 90)}
	products := &fakeProducts{cards: map[int64]ProductInfo{90: {ID: 90, Name: "Tea", MinPrice: 100, MaxPrice: 100, Available: true}}}
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
	r := &fakeReads{feed: social.PostPage{Posts: []social.Post{withProduct(post(5), 90)}}}
	products := &fakeProducts{err: apperr.Unavailable("product-service down")}
	h := newPublicWithProducts(t, r, &fakeViews{}, fakeBuyers{id: 7}, &fakeStores{}, products)
	resp, err := h.ListPosts(asBuyer(t.Context()), connect.NewRequest(&pb.ListPostsRequest{}))
	if err != nil {
		t.Fatalf("商品查詢失敗不能讓讀取失敗：%v", err)
	}
	posts := resp.Msg.GetPosts()
	if len(posts) != 1 || posts[0].Product != nil || posts[0].GetProductId() != "90" {
		t.Fatalf("product 不設、product_id 照實回：%v", posts)
	}
}

func TestOwnerReadsAskForInactiveProductsToo(t *testing.T) {
	products := &fakeProducts{cards: map[int64]ProductInfo{90: {ID: 90, Name: "Hidden", Available: false}}}
	h := &presenter{buyers: fakeBuyers{}, stores: &fakeStores{}, products: products, ownerView: true, images: mustImages(t), webOrigin: mustWeb(t), liked: noViewerLikes, log: slog.New(slog.DiscardHandler)}
	out, err := h.toPosts(t.Context(), social.Caller{}, []social.Post{withProduct(post(5), 90)})
	if err != nil {
		t.Fatal(err)
	}
	if !products.incl[0] || out[0].GetProduct().GetName() != "Hidden" || out[0].GetProduct().GetAvailable() {
		t.Fatalf("店主讀自己的貼文要補下架的商品（available=false）：%v %v", products.incl, out[0].GetProduct())
	}
}
