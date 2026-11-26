package catalog

import (
	"context"
	"fmt"
	"time"
	"uuid"

	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/catalog/v1alpha1"
	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// Presenter 把 catalog 的商品組成對外的訊息：圖片網址、店家卡片（含評分）與可購買性。公開 service 與 buyer 的「追蹤中」動態共用。
type Presenter struct {
	stores StoreSource
	stats  *httpseller.StatsSource
	images rpc.Images
}

// NewPresenter 建立 Presenter；stats 是店家卡片評分的組裝（httpseller.NewStatsSource，必填）。
func NewPresenter(stores StoreSource, stats *httpseller.StatsSource, images rpc.Images) (*Presenter, error) {
	switch {
	case stores == nil:
		return nil, fmt.Errorf("httpapi/catalog：需要店家卡片的來源")
	case stats == nil:
		return nil, httpseller.ErrStatsSourceRequired
	}
	return &Presenter{stores: stores, stats: stats, images: images}, nil
}

// storeCards 讀一批店家的卡片，key 是 store_id；沒有卡片（從沒公開過或不存在）的店家不在 map 裡，client 看到 store 不設。
// 讀取失敗整個請求失敗（不降級成沒有店家）；評分讀不到時只有評分不設。
func (p *Presenter) storeCards(ctx context.Context, storeIDs []int64) (map[int64]*commonpb.StoreCard, error) {
	if len(storeIDs) == 0 {
		return map[int64]*commonpb.StoreCard{}, nil
	}
	cards, err := p.stores.StoreCards(ctx, storeIDs)
	if err != nil {
		return nil, apperr.WithCause(apperr.Unavailable("暫時無法讀取店家"), err)
	}
	out := make(map[int64]*commonpb.StoreCard, len(cards))
	ids := make([]int64, len(cards))
	rated := make([]*commonpb.StoreCard, len(cards))
	for i, c := range cards {
		if rated[i], err = httpseller.ToStoreCard(p.images, c); err != nil {
			return nil, err
		}
		out[c.StoreID], ids[i] = rated[i], c.StoreID
	}
	p.stats.RateCards(ctx, ids, rated)
	return out, nil
}

func distinctStores[T any](items []T, storeOf func(T) int64) []int64 {
	seen := map[int64]bool{}
	var out []int64
	for _, it := range items {
		if id := storeOf(it); !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// Summaries 是公開列表的商品卡片，順序同 items。
func (p *Presenter) Summaries(ctx context.Context, items []catalog.ProductSummary) ([]*pb.ProductSummary, error) {
	cards, err := p.storeCards(ctx, distinctStores(items, func(s catalog.ProductSummary) int64 { return s.StoreID }))
	if err != nil {
		return nil, err
	}
	out := make([]*pb.ProductSummary, len(items))
	for i, s := range items {
		if err := rpc.ObjectKey(s.CoverKey); err != nil {
			return nil, err
		}
		out[i] = &pb.ProductSummary{
			Id: s.ID.String(), Store: cards[s.StoreID], CategoryId: s.CategoryID.String(), Title: s.Title,
			CoverImage:       p.images.Image(s.CoverKey),
			Availability:     availability(s.Status == catalog.StatusActive && s.TotalStock > 0),
			FirstPublishTime: optTime(s.FirstPublishedAt),
			MinPrice:         s.MinPrice, MaxPrice: s.MaxPrice, SalesCount: s.SalesCount, FavoriteCount: s.FavoriteCount,
			RatingAverage: rating(s.RatingAverage, s.ReviewCount), ReviewCount: s.ReviewCount,
		}
	}
	return out, nil
}

// Product 是公開商品頁：ARCHIVED 的商品照樣回傳，availability 是不可購買。
func (p *Presenter) Product(ctx context.Context, pr catalog.Product) (*pb.Product, error) {
	cards, err := p.storeCards(ctx, []int64{pr.StoreID})
	if err != nil {
		return nil, err
	}
	v, err := newView(p.images, pr, false)
	if err != nil {
		return nil, err
	}
	return &pb.Product{
		Id: pr.ID.String(), Store: cards[pr.StoreID], CategoryId: pr.CategoryID.String(), Title: pr.Title,
		Availability:     availability(v.available),
		FirstPublishTime: optTime(pr.FirstPublishedAt),
		MinPrice:         pr.MinPrice, MaxPrice: pr.MaxPrice, SalesCount: pr.SalesCount, FavoriteCount: pr.FavoriteCount,
		RatingAverage: rating(pr.RatingAverage, pr.ReviewCount), ReviewCount: pr.ReviewCount,
		Gallery: v.gallery, Description: v.description, Axes: v.axes, Skus: v.skus, Hashtags: v.hashtags,
	}, nil
}

// view 是商品的共用輸出片段（圖庫、描述、選項軸、SKU、標籤）與可購買性；三種讀取（公開、賣家、後台）的差別只在外層欄位。
type view struct {
	gallery     []*pb.ProductImage
	description []*pb.ProductDescriptionBlock
	axes        []*pb.OptionAxis
	skus        []*pb.Sku
	hashtags    []*pb.ProductHashtag
	// available 是商品現在能不能買：上架中而且至少一個 SKU 有庫存。
	available bool
}

// newView 組出商品的共用片段。withAsset 是賣家與後台讀取：圖片帶資產 ID（寫入沿用現有的圖時要）；公開讀取不帶。
func newView(images rpc.Images, p catalog.Product, withAsset bool) (view, error) {
	var v view
	var err error
	if v.gallery, err = toImages(images, p.Gallery, withAsset); err != nil {
		return v, err
	}
	if v.description, err = toBlocks(images, p.Description, withAsset); err != nil {
		return v, err
	}
	for _, a := range p.Axes {
		v.axes = append(v.axes, &pb.OptionAxis{Name: a.Name, Values: a.Values})
	}
	active := p.Status == catalog.StatusActive
	for _, s := range p.Skus {
		sellable := active && s.StockQty > 0
		v.available = v.available || sellable
		v.skus = append(v.skus, &pb.Sku{
			Id: s.ID.String(), Option1: s.Option1, Option2: s.Option2, Price: s.Price, CompareAtPrice: s.CompareAtPrice,
			StockQty: s.StockQty, Availability: availability(sellable),
		})
	}
	for _, h := range p.Hashtags {
		v.hashtags = append(v.hashtags, &pb.ProductHashtag{Name: h.Name, Display: h.Display})
	}
	return v, nil
}

func toImage(images rpc.Images, img catalog.Image, withAsset bool) (*pb.ProductImage, error) {
	if err := rpc.ObjectKey(img.ObjectKey); err != nil {
		return nil, err
	}
	out := &pb.ProductImage{Image: images.Image(img.ObjectKey), Alt: img.Alt}
	if withAsset {
		out.AssetId = img.AssetID.String()
	}
	return out, nil
}

func toImages(images rpc.Images, in []catalog.Image, withAsset bool) ([]*pb.ProductImage, error) {
	out := make([]*pb.ProductImage, len(in))
	for i, img := range in {
		var err error
		if out[i], err = toImage(images, img, withAsset); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func toBlocks(images rpc.Images, in []catalog.Block, withAsset bool) ([]*pb.ProductDescriptionBlock, error) {
	out := make([]*pb.ProductDescriptionBlock, len(in))
	for i, b := range in {
		blk := &pb.ProductDescriptionBlock{}
		switch b.Kind {
		case catalog.BlockParagraph:
			blk.Block = &pb.ProductDescriptionBlock_Paragraph{Paragraph: b.Text}
		case catalog.BlockImage:
			if b.Image == nil {
				return nil, fmt.Errorf("httpapi/catalog：描述的圖片區塊沒有圖片")
			}
			img, err := toImage(images, *b.Image, withAsset)
			if err != nil {
				return nil, err
			}
			blk.Block = &pb.ProductDescriptionBlock_Image{Image: img}
		case catalog.BlockList:
			blk.Block = &pb.ProductDescriptionBlock_BulletList{BulletList: &pb.BulletList{Items: b.Items}}
		case catalog.BlockTable:
			rows := make([]*pb.TableRow, len(b.Rows))
			for r, cells := range b.Rows {
				rows[r] = &pb.TableRow{Cells: cells}
			}
			blk.Block = &pb.ProductDescriptionBlock_Table{Table: &pb.Table{Rows: rows}}
		default:
			return nil, fmt.Errorf("httpapi/catalog：不認得的描述區塊 %q", b.Kind)
		}
		out[i] = blk
	}
	return out, nil
}

// sellerProduct 是賣家讀自己的商品（含 etag、狀態、低庫存門檻與資產 ID）。
func sellerProduct(images rpc.Images, p catalog.Product) (*pb.SellerProduct, error) {
	v, err := newView(images, p, true)
	if err != nil {
		return nil, err
	}
	return &pb.SellerProduct{
		Id: p.ID.String(), CategoryId: p.CategoryID.String(), Title: p.Title, Status: rpc.EnumToProto[pb.ProductStatus](p.Status),
		FirstPublishTime: optTime(p.FirstPublishedAt),
		MinPrice:         p.MinPrice, MaxPrice: p.MaxPrice, SalesCount: p.SalesCount, FavoriteCount: p.FavoriteCount,
		RatingAverage: rating(p.RatingAverage, p.ReviewCount), ReviewCount: p.ReviewCount,
		Gallery: v.gallery, Description: v.description, Axes: v.axes, Skus: v.skus, Hashtags: v.hashtags,
		LowStockThreshold: p.LowStockThreshold, Etag: catalog.Etag(p.Version),
		CreateTime: timestamppb.New(p.CreatedAt), UpdateTime: timestamppb.New(p.UpdatedAt),
	}, nil
}

func sellerSummaries(images rpc.Images, items []catalog.ProductSummary) ([]*pb.SellerProductSummary, error) {
	out := make([]*pb.SellerProductSummary, len(items))
	for i, s := range items {
		if err := rpc.ObjectKey(s.CoverKey); err != nil {
			return nil, err
		}
		out[i] = &pb.SellerProductSummary{
			Id: s.ID.String(), CategoryId: s.CategoryID.String(), Title: s.Title, CoverImage: images.Image(s.CoverKey),
			Status: rpc.EnumToProto[pb.ProductStatus](s.Status), FirstPublishTime: optTime(s.FirstPublishedAt),
			MinPrice: s.MinPrice, MaxPrice: s.MaxPrice, SalesCount: s.SalesCount, FavoriteCount: s.FavoriteCount,
			RatingAverage: rating(s.RatingAverage, s.ReviewCount), ReviewCount: s.ReviewCount,
			TotalStock: s.TotalStock, LowStockThreshold: s.LowStockThreshold, Etag: catalog.Etag(s.Version),
			CreateTime: timestamppb.New(s.CreatedAt),
		}
	}
	return out, nil
}

// sellerSku 是庫存調整後回給賣家的 SKU：沒有商品狀態可依，availability 由商品狀態與庫存決定，所以要帶商品現在是不是 ACTIVE。
func sellerSku(p catalog.Product, skuID uuid.UUID) (*pb.Sku, error) {
	for _, s := range p.Skus {
		if s.ID == skuID {
			sellable := p.Status == catalog.StatusActive && s.StockQty > 0
			return &pb.Sku{
				Id: s.ID.String(), Option1: s.Option1, Option2: s.Option2, Price: s.Price, CompareAtPrice: s.CompareAtPrice,
				StockQty: s.StockQty, Availability: availability(sellable),
			}, nil
		}
	}
	return nil, fmt.Errorf("httpapi/catalog：商品 %s 讀不到剛調整的規格 %s", p.ID, skuID)
}

func availability(ok bool) pb.ProductAvailability {
	if ok {
		return pb.ProductAvailability_PRODUCT_AVAILABILITY_AVAILABLE
	}
	return pb.ProductAvailability_PRODUCT_AVAILABILITY_UNAVAILABLE
}

// rating 是平均評分；沒有評價不設。
func rating(avg float64, reviews int32) *float32 {
	if reviews <= 0 {
		return nil
	}
	v := float32(avg)
	return &v
}

func optTime(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}
