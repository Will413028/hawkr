//go:build integration

package characterization

import (
	"context"
	"fmt"
	"net/http"
	"uuid"

	"connectrpc.com/connect"

	ch "github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
	admincatalogpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/catalog/v1alpha1"
	admincatalogconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/catalog/v1alpha1/catalogv1alpha1connect"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/catalog/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/catalog/v1alpha1/catalogv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

const connectPageSize = 100

type connectDriver struct {
	public  map[ch.Caller]catalogv1alpha1connect.CatalogPublicServiceClient
	sellers map[ch.Caller]catalogv1alpha1connect.SellerCatalogAppServiceClient
	admins  map[ch.Caller]admincatalogconnect.CatalogAdminServiceClient
	stores  map[ch.Caller]string
}

var _ Driver = (*connectDriver)(nil)

func newConnectDriver(httpClient *http.Client, base string, tokens map[ch.Caller]string, stores map[ch.Caller]string) *connectDriver {
	d := &connectDriver{
		public:  map[ch.Caller]catalogv1alpha1connect.CatalogPublicServiceClient{},
		sellers: map[ch.Caller]catalogv1alpha1connect.SellerCatalogAppServiceClient{},
		admins:  map[ch.Caller]admincatalogconnect.CatalogAdminServiceClient{},
		stores:  stores,
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
		d.public[c] = catalogv1alpha1connect.NewCatalogPublicServiceClient(httpClient, base, auth)
		d.sellers[c] = catalogv1alpha1connect.NewSellerCatalogAppServiceClient(httpClient, base, auth)
		d.admins[c] = admincatalogconnect.NewCatalogAdminServiceClient(httpClient, base, auth)
	}
	return d
}

func (d *connectDriver) Legacy() bool               { return false }
func (d *connectDriver) StoreOf(c ch.Caller) string { return d.stores[c] }
func (d *connectDriver) Unknown() string            { return "00000000-0000-7000-8000-000000000001" }
func newRequestID() string                          { return uuid.NewV4().String() }
func newImageToken() string                         { return "characterize-" + ch.Nonce() + ch.Nonce() }
func (d *connectDriver) seller(c ch.Caller) catalogv1alpha1connect.SellerCatalogAppServiceClient {
	return d.sellers[c]
}

var statusFromProto = map[pb.ProductStatus]Status{
	pb.ProductStatus_PRODUCT_STATUS_DRAFT:    StatusDraft,
	pb.ProductStatus_PRODUCT_STATUS_ACTIVE:   StatusActive,
	pb.ProductStatus_PRODUCT_STATUS_ARCHIVED: StatusArchived,
}

var statusToProto = map[Status]pb.ProductStatus{
	StatusDraft:    pb.ProductStatus_PRODUCT_STATUS_DRAFT,
	StatusActive:   pb.ProductStatus_PRODUCT_STATUS_ACTIVE,
	StatusArchived: pb.ProductStatus_PRODUCT_STATUS_ARCHIVED,
}

func hashtagNames(tags []*pb.ProductHashtag) []string {
	var out []string
	for _, h := range tags {
		out = append(out, h.GetName())
	}
	return out
}

func totalStock(skus []*pb.Sku) int64 {
	var n int64
	for _, s := range skus {
		n += int64(s.GetStockQty())
	}
	return n
}

func (d *connectDriver) sellerProduct(c ch.Caller, p *pb.SellerProduct) Product {
	return Product{
		ID: p.GetId(), Name: p.GetTitle(), StoreID: d.stores[c], CategoryID: p.GetCategoryId(), Status: statusFromProto[p.GetStatus()],
		PriceMin: p.GetMinPrice(), PriceMax: p.GetMaxPrice(), Stock: totalStock(p.GetSkus()), Hashtags: hashtagNames(p.GetHashtags()),
	}
}

func sellerSummary(c ch.Caller, d *connectDriver, p *pb.SellerProductSummary) Product {
	return Product{
		ID: p.GetId(), Name: p.GetTitle(), StoreID: d.stores[c], CategoryID: p.GetCategoryId(), Status: statusFromProto[p.GetStatus()],
		PriceMin: p.GetMinPrice(), PriceMax: p.GetMaxPrice(), Stock: int64(p.GetTotalStock()),
	}
}

func publicProduct(p *pb.Product) Product {
	return Product{
		ID: p.GetId(), Name: p.GetTitle(), StoreID: p.GetStore().GetId(), CategoryID: p.GetCategoryId(),
		PriceMin: p.GetMinPrice(), PriceMax: p.GetMaxPrice(), Stock: totalStock(p.GetSkus()), Hashtags: hashtagNames(p.GetHashtags()),
		Purchasable: p.GetAvailability() == pb.ProductAvailability_PRODUCT_AVAILABILITY_AVAILABLE,
	}
}

func publicSummary(p *pb.ProductSummary) Product {
	return Product{
		ID: p.GetId(), Name: p.GetTitle(), StoreID: p.GetStore().GetId(), CategoryID: p.GetCategoryId(),
		PriceMin: p.GetMinPrice(), PriceMax: p.GetMaxPrice(),
		Purchasable: p.GetAvailability() == pb.ProductAvailability_PRODUCT_AVAILABILITY_AVAILABLE,
	}
}

func pages[T any](fetch func(token string) ([]T, string, error)) ([]T, error) {
	var out []T
	token := ""
	for {
		items, next, err := fetch(token)
		if err != nil {
			return nil, ch.ConnectKind(err)
		}
		out = append(out, items...)
		if next == "" {
			return out, nil
		}
		token = next
	}
}

func (d *connectDriver) Categories(ctx context.Context, c ch.Caller) ([]Category, error) {
	res, err := d.public[c].ListCategories(ctx, connect.NewRequest(&pb.ListCategoriesRequest{}))
	if err != nil {
		return nil, ch.ConnectKind(err)
	}
	var out []Category
	var walk func(parent string, nodes []*pb.Category)
	walk = func(parent string, nodes []*pb.Category) {
		for _, n := range nodes {
			out = append(out, Category{ID: n.GetId(), Name: n.GetName(), ParentID: parent, Leaf: len(n.GetChildren()) == 0})
			walk(n.GetId(), n.GetChildren())
		}
	}
	walk("", res.Msg.GetCategories())
	return out, nil
}

func (d *connectDriver) productInput(in ProductInput) *pb.ProductInput {
	sku := &pb.SkuInput{Price: in.Price, InitialStock: int32(in.Stock)} //nolint:gosec // 情境的庫存很小
	if in.ListPrice != 0 {
		list := in.ListPrice
		sku.CompareAtPrice = &list
	}
	out := &pb.ProductInput{Title: in.Name, CategoryId: in.CategoryID, HashtagInputs: in.Hashtags, Skus: []*pb.SkuInput{sku}}
	if in.MainImage {
		out.Gallery = []*pb.ImageInput{{Source: &pb.ImageInput_StagedToken{StagedToken: newImageToken()}}}
	}
	return out
}

func (d *connectDriver) SellerCreate(ctx context.Context, c ch.Caller, in ProductInput) (Product, error) {
	res, err := d.seller(c).CreateProduct(ctx, connect.NewRequest(&pb.CreateProductRequest{Product: d.productInput(in), RequestId: newRequestID()}))
	if err != nil {
		return Product{}, ch.ConnectKind(err)
	}
	return d.sellerProduct(c, res.Msg.GetProduct()), nil
}

func (d *connectDriver) mine(ctx context.Context, c ch.Caller, id string) (*pb.SellerProduct, error) {
	res, err := d.seller(c).GetMyProduct(ctx, connect.NewRequest(&pb.GetMyProductRequest{ProductId: id}))
	if err != nil {
		return nil, ch.ConnectKind(err)
	}
	return res.Msg.GetProduct(), nil
}

func (d *connectDriver) SellerGet(ctx context.Context, c ch.Caller, id string) (Product, error) {
	p, err := d.mine(ctx, c, id)
	if err != nil {
		return Product{}, err
	}
	return d.sellerProduct(c, p), nil
}

func (d *connectDriver) SellerList(ctx context.Context, c ch.Caller, keyword string) ([]Product, error) {
	items, err := pages(func(token string) ([]*pb.SellerProductSummary, string, error) {
		res, err := d.seller(c).ListMyProducts(ctx, connect.NewRequest(&pb.ListMyProductsRequest{
			Filter: &pb.MyProductFilter{Query: keyword}, PageSize: connectPageSize, PageToken: token,
		}))
		if err != nil {
			return nil, "", err
		}
		return res.Msg.GetProducts(), res.Msg.GetNextPageToken(), nil
	})
	if err != nil {
		return nil, err
	}
	out := make([]Product, len(items))
	for i, p := range items {
		out[i] = sellerSummary(c, d, p)
	}
	return out, nil
}

func (d *connectDriver) SellerUpdate(ctx context.Context, c ch.Caller, id string, in ProductInput) (Product, error) {
	cur, err := d.mine(ctx, c, id)
	if err != nil {
		return Product{}, err
	}
	res, err := d.seller(c).UpdateProduct(ctx, connect.NewRequest(&pb.UpdateProductRequest{ProductId: id, Etag: cur.GetEtag(), Product: d.productInput(in)}))
	if err != nil {
		return Product{}, ch.ConnectKind(err)
	}
	return d.sellerProduct(c, res.Msg.GetProduct()), nil
}

// SellerSetStock 的預期值不符與調整後為負都是 failed_precondition，對應 ErrStockMismatch。
func (d *connectDriver) SellerSetStock(ctx context.Context, c ch.Caller, id string, quantity, expected int64) error {
	cur, err := d.mine(ctx, c, id)
	if err != nil {
		return err
	}
	if len(cur.GetSkus()) != 1 {
		return fmt.Errorf("商品 %s 有 %d 個 SKU，情境的商品只有一個", id, len(cur.GetSkus()))
	}
	_, err = d.seller(c).SetStock(ctx, connect.NewRequest(&pb.SetStockRequest{
		SkuId: cur.GetSkus()[0].GetId(), Quantity: int32(quantity), ExpectedQuantity: int32(expected), RequestId: newRequestID(), //nolint:gosec // 情境的庫存很小
	}))
	return preconditionAs(err, ErrStockMismatch)
}

// SellerSetListed 上架是 ACTIVE、下架是 ARCHIVED（D16）；ACTIVE 沒有主圖是 failed_precondition（D22），對應 ErrWrongStatus。
func (d *connectDriver) SellerSetListed(ctx context.Context, c ch.Caller, id string, listed bool) error {
	cur, err := d.mine(ctx, c, id)
	if err != nil {
		return err
	}
	to := statusToProto[StatusArchived]
	if listed {
		to = statusToProto[StatusActive]
	}
	_, err = d.seller(c).SetProductStatus(ctx, connect.NewRequest(&pb.SetProductStatusRequest{ProductId: id, Status: to, Etag: cur.GetEtag()}))
	return preconditionAs(err, ErrWrongStatus)
}

func (d *connectDriver) SellerDelete(ctx context.Context, c ch.Caller, id string) error {
	_, err := d.seller(c).DeleteProduct(ctx, connect.NewRequest(&pb.DeleteProductRequest{ProductId: id}))
	return ch.ConnectKind(err)
}

func preconditionAs(err, kind error) error {
	if err == nil {
		return nil
	}
	if connect.CodeOf(err) == connect.CodeFailedPrecondition {
		return apperr.WithCause(kind, err)
	}
	return ch.ConnectKind(err)
}

func (d *connectDriver) PublicGet(ctx context.Context, c ch.Caller, id string) (Product, error) {
	res, err := d.public[c].GetProduct(ctx, connect.NewRequest(&pb.GetProductRequest{ProductId: id}))
	if err != nil {
		return Product{}, ch.ConnectKind(err)
	}
	return publicProduct(res.Msg.GetProduct()), nil
}

func (d *connectDriver) PublicList(ctx context.Context, c ch.Caller, q PublicQuery) ([]Product, error) {
	filter := &pb.ProductFilter{}
	if q.CategoryID != "" {
		filter.CategoryIds = []string{q.CategoryID}
	}
	if q.StoreID != "" {
		filter.StoreIds = []string{q.StoreID}
	}
	if q.PriceMin != 0 {
		v := q.PriceMin
		filter.PriceMin = &v
	}
	if q.PriceMax != 0 {
		v := q.PriceMax
		filter.PriceMax = &v
	}
	if q.Keyword != "" {
		return d.search(ctx, c, q.Keyword, filter)
	}
	items, err := pages(func(token string) ([]*pb.ProductSummary, string, error) {
		res, err := d.public[c].ListProducts(ctx, connect.NewRequest(&pb.ListProductsRequest{Filter: filter, PageSize: connectPageSize, PageToken: token}))
		if err != nil {
			return nil, "", err
		}
		return res.Msg.GetProducts(), res.Msg.GetNextPageToken(), nil
	})
	return summaries(items, err)
}

func (d *connectDriver) Search(ctx context.Context, c ch.Caller, keyword string) ([]Product, error) {
	return d.search(ctx, c, keyword, nil)
}

func (d *connectDriver) search(ctx context.Context, c ch.Caller, keyword string, filter *pb.ProductFilter) ([]Product, error) {
	items, err := pages(func(token string) ([]*pb.ProductSummary, string, error) {
		res, err := d.public[c].SearchProducts(ctx, connect.NewRequest(&pb.SearchProductsRequest{Query: keyword, Filter: filter, PageSize: connectPageSize, PageToken: token}))
		if err != nil {
			return nil, "", err
		}
		return res.Msg.GetProducts(), res.Msg.GetNextPageToken(), nil
	})
	return summaries(items, err)
}

func summaries(items []*pb.ProductSummary, err error) ([]Product, error) {
	if err != nil {
		return nil, err
	}
	out := make([]Product, len(items))
	for i, p := range items {
		out[i] = publicSummary(p)
	}
	return out, nil
}

func (d *connectDriver) Suggest(ctx context.Context, c ch.Caller, prefix string) ([]string, error) {
	res, err := d.public[c].SuggestProductNames(ctx, connect.NewRequest(&pb.SuggestProductNamesRequest{Query: prefix, MaxResults: 20}))
	if err != nil {
		return nil, ch.ConnectKind(err)
	}
	return res.Msg.GetNames(), nil
}

func (d *connectDriver) HashtagAutocomplete(ctx context.Context, c ch.Caller, prefix string) ([]Hashtag, error) {
	res, err := d.public[c].SuggestHashtags(ctx, connect.NewRequest(&pb.SuggestHashtagsRequest{Query: prefix, MaxResults: 20}))
	if err != nil {
		return nil, ch.ConnectKind(err)
	}
	out := make([]Hashtag, len(res.Msg.GetHashtags()))
	for i, h := range res.Msg.GetHashtags() {
		out[i] = Hashtag{Name: h.GetName(), Count: h.GetProductCount()}
	}
	return out, nil
}

func (d *connectDriver) AdminList(ctx context.Context, c ch.Caller, keyword string) ([]Product, error) {
	items, err := pages(func(token string) ([]*admincatalogpb.ProductSummary, string, error) {
		res, err := d.admins[c].ListProducts(ctx, connect.NewRequest(&admincatalogpb.ListProductsRequest{Query: keyword, PageSize: connectPageSize, PageToken: token}))
		if err != nil {
			return nil, "", err
		}
		return res.Msg.GetProducts(), res.Msg.GetNextPageToken(), nil
	})
	if err != nil {
		return nil, err
	}
	out := make([]Product, len(items))
	for i, p := range items {
		out[i] = Product{
			ID: p.GetId(), Name: p.GetTitle(), StoreID: p.GetStoreId(), Status: statusFromProto[p.GetStatus()],
			PriceMin: p.GetMinPrice(), PriceMax: p.GetMaxPrice(),
		}
	}
	return out, nil
}

func (d *connectDriver) AdminGet(ctx context.Context, c ch.Caller, id string) (Product, error) {
	res, err := d.admins[c].GetProduct(ctx, connect.NewRequest(&admincatalogpb.GetProductRequest{ProductId: id}))
	if err != nil {
		return Product{}, ch.ConnectKind(err)
	}
	p := res.Msg.GetProduct()
	return Product{
		ID: p.GetId(), Name: p.GetTitle(), StoreID: p.GetStoreId(), CategoryID: p.GetCategoryId(), Status: statusFromProto[p.GetStatus()],
		PriceMin: p.GetMinPrice(), PriceMax: p.GetMaxPrice(), Stock: totalStock(p.GetSkus()), Hashtags: hashtagNames(p.GetHashtags()),
	}, nil
}
