package catalog

import (
	"context"
	"slices"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/catalog/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/catalog/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
)

// fullProduct 把商品的每個欄位都設成互不相同的非零值：映射丟掉哪個欄位，下面的 unsetFields 就看得到。
func fullProduct(status catalog.ProductStatus) catalog.Product {
	first := time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC)
	cmp := int64(150)
	low := int32(7)
	asset := uuid.UUID{9}
	return catalog.Product{
		ID: uuid.UUID{1}, StoreID: 9002, CategoryID: uuid.UUID{2}, Title: "山藥", Status: status, FirstPublishedAt: &first,
		MinPrice: 100, MaxPrice: 120, SalesCount: 31, FavoriteCount: 17, RatingAverage: 4.5, ReviewCount: 6,
		Description: []catalog.Block{
			{Kind: catalog.BlockParagraph, Text: "好吃"},
			{Kind: catalog.BlockImage, Image: &catalog.Image{AssetID: asset, ObjectKey: "img/d", Alt: "描述圖"}},
			{Kind: catalog.BlockList, Items: []string{"甲", "乙"}},
			{Kind: catalog.BlockTable, Rows: [][]string{{"a", "b"}, {"c", "d"}}},
		},
		Gallery:           []catalog.Image{{AssetID: uuid.UUID{8}, ObjectKey: "img/g", Alt: "主圖"}},
		Axes:              []catalog.Axis{{Name: "顏色", Values: []string{"紅", "藍"}}},
		Skus:              []catalog.Sku{{ID: uuid.UUID{3}, Option1: "紅", Option2: "S", Price: 100, CompareAtPrice: &cmp, StockQty: 4}, {ID: uuid.UUID{4}, Option1: "藍", Price: 120, StockQty: 0}},
		Hashtags:          []catalog.Hashtag{{Name: "tea", Display: "Tea"}},
		CreatedAt:         first.Add(-time.Hour),
		UpdatedAt:         first.Add(time.Hour),
		LowStockThreshold: &low,
		Version:           13,
	}
}

func fullSummary(status catalog.ProductStatus, stock int32) catalog.ProductSummary {
	first := time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC)
	low := int32(7)
	return catalog.ProductSummary{
		ID: uuid.UUID{1}, StoreID: 9002, CategoryID: uuid.UUID{2}, Title: "山藥", Status: status, FirstPublishedAt: &first,
		MinPrice: 100, MaxPrice: 120, SalesCount: 31, FavoriteCount: 17, RatingAverage: 4.5, ReviewCount: 6, CoverKey: "img/cover",
		CreatedAt: first.Add(-time.Hour), TotalStock: stock, LowStockThreshold: &low, Version: 13,
	}
}

// unsetFields 列出訊息（遞迴）沒有任何一個實例設值的欄位路徑：repeated 的元素合併計算（任何一個元素設了就算有），Timestamp 不再往內看。
func unsetFields(m protoreflect.Message, path string) []string {
	all, set := map[string]bool{}, map[string]bool{}
	walk(m, path, all, set)
	var out []string
	for p := range all {
		if !set[p] {
			out = append(out, p)
		}
	}
	slices.Sort(out)
	return out
}

func walk(m protoreflect.Message, path string, all, set map[string]bool) {
	fields := m.Descriptor().Fields()
	for i := range fields.Len() {
		f := fields.Get(i)
		if f.ContainingOneof() != nil && !f.ContainingOneof().IsSynthetic() {
			continue // oneof 的各成員由個別的描述區塊測試涵蓋
		}
		name := path + string(f.Name())
		all[name] = true
		if !m.Has(f) {
			continue
		}
		set[name] = true
		switch {
		case f.IsList() && f.Message() != nil:
			l := m.Get(f).List()
			for j := range l.Len() {
				walk(l.Get(j).Message(), name+"[].", all, set)
			}
		case f.Message() != nil && !f.IsMap() && f.Message().FullName() != "google.protobuf.Timestamp":
			walk(m.Get(f).Message(), name+".", all, set)
		}
	}
}

func requireAllSet(t *testing.T, name string, msg proto.Message, allowed ...string) {
	t.Helper()
	skip := map[string]bool{}
	for _, a := range allowed {
		skip[a] = true
	}
	for _, f := range unsetFields(msg.ProtoReflect(), "") {
		if !skip[f] {
			t.Errorf("%s 的欄位 %s 沒有設值：映射丟掉了它（或測試的來源資料沒有填）", name, f)
		}
	}
}

func newPresenterForTest(t *testing.T) *Presenter {
	t.Helper()
	p, err := NewPresenter(fakeCards{}, characterize.Stats(t), characterize.Images(t))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// 公開商品的欄位逐一對得上來源：公開訊息沒有 etag、狀態與資產 ID 是設計（D24 Q14），其餘每個欄位都要有值。
// 店家評分（store.rating_average）由卡片組裝的評價來源提供，這裡的來源沒有資料，不檢查。
func TestPublicProductMapsEveryField(t *testing.T) {
	t.Parallel()
	p := newPresenterForTest(t)
	got, err := p.Product(context.Background(), fullProduct(catalog.StatusActive))
	if err != nil {
		t.Fatal(err)
	}
	requireAllSet(t, "Product", got, "store.review_count", "store.rating_average", "gallery[].asset_id", "description[].image.asset_id")
	if got.GetStore().GetId() != "9002" {
		t.Fatalf("店家卡片 = %v", got.GetStore())
	}
	if got.GetGallery()[0].GetAssetId() != "" {
		t.Fatal("公開讀取不帶資產 ID")
	}
	if got.GetSkus()[0].GetCompareAtPrice() != 150 || got.GetSkus()[0].GetOption2() != "S" {
		t.Fatalf("SKU 的選項與原價 = %v", got.GetSkus()[0])
	}
	if len(got.GetDescription()) != 4 || got.GetDescription()[1].GetImage().GetImage() == nil ||
		len(got.GetDescription()[3].GetTable().GetRows()) != 2 || got.GetDescription()[3].GetTable().GetRows()[1].GetCells()[1] != "d" ||
		got.GetDescription()[2].GetBulletList().GetItems()[1] != "乙" || got.GetDescription()[0].GetParagraph() != "好吃" {
		t.Fatalf("描述 = %v", got.GetDescription())
	}
}

func TestPublicSummaryMapsEveryField(t *testing.T) {
	t.Parallel()
	p := newPresenterForTest(t)
	got, err := p.Summaries(context.Background(), []catalog.ProductSummary{fullSummary(catalog.StatusActive, 5)})
	if err != nil {
		t.Fatal(err)
	}
	requireAllSet(t, "ProductSummary", got[0], "store.review_count", "store.rating_average")
}

// 可購買性：只有 ACTIVE 而且有庫存才可購買；ARCHIVED、草稿與沒有庫存都不可購買。商品與 SKU 各自判斷。
func TestAvailabilityFollowsStatusAndStock(t *testing.T) {
	t.Parallel()
	p := newPresenterForTest(t)
	ctx := context.Background()
	avail, unavail := pb.ProductAvailability_PRODUCT_AVAILABILITY_AVAILABLE, pb.ProductAvailability_PRODUCT_AVAILABILITY_UNAVAILABLE
	for _, c := range []struct {
		status catalog.ProductStatus
		want   pb.ProductAvailability
		sku0   pb.ProductAvailability // 庫存 4 的 SKU
		sku1   pb.ProductAvailability // 庫存 0 的 SKU
	}{
		{catalog.StatusActive, avail, avail, unavail},
		{catalog.StatusArchived, unavail, unavail, unavail},
		{catalog.StatusDraft, unavail, unavail, unavail},
	} {
		got, err := p.Product(ctx, fullProduct(c.status))
		if err != nil {
			t.Fatal(err)
		}
		if got.GetAvailability() != c.want || got.GetSkus()[0].GetAvailability() != c.sku0 || got.GetSkus()[1].GetAvailability() != c.sku1 {
			t.Errorf("%s：商品 %v SKU %v/%v，預期 %v %v/%v", c.status, got.GetAvailability(), got.GetSkus()[0].GetAvailability(), got.GetSkus()[1].GetAvailability(), c.want, c.sku0, c.sku1)
		}
		if got.GetSkus()[0].GetStockQty() != 4 {
			t.Errorf("%s：已下架的商品頁仍顯示庫存 %d", c.status, got.GetSkus()[0].GetStockQty())
		}
	}
	// ACTIVE 但所有 SKU 都沒庫存：商品不可購買。
	soldOut := fullProduct(catalog.StatusActive)
	soldOut.Skus[0].StockQty = 0
	got, err := p.Product(ctx, soldOut)
	if err != nil || got.GetAvailability() != unavail {
		t.Fatalf("售完的商品 = %v, %v", got.GetAvailability(), err)
	}
	rows, err := p.Summaries(ctx, []catalog.ProductSummary{
		fullSummary(catalog.StatusActive, 5), fullSummary(catalog.StatusActive, 0), fullSummary(catalog.StatusArchived, 5),
	})
	if err != nil || rows[0].GetAvailability() != avail || rows[1].GetAvailability() != unavail || rows[2].GetAvailability() != unavail {
		t.Fatalf("列表的可購買性 = %v, %v", rows, err)
	}
}

// 沒有評價不設平均評分（0 不是評分）。
func TestRatingIsUnsetWithoutReviews(t *testing.T) {
	t.Parallel()
	if rating(0, 0) != nil || rating(4.5, 0) != nil {
		t.Fatal("沒有評價不該有平均評分")
	}
	if got := rating(4.5, 1); got == nil || *got != 4.5 {
		t.Fatalf("有評價 = %v", got)
	}
}

// 賣家讀自己的商品：含 etag、狀態、低庫存門檻、建立與修改時間與資產 ID，每個欄位都有對應。
func TestSellerProductMapsEveryField(t *testing.T) {
	t.Parallel()
	images := characterize.Images(t)
	got, err := sellerProduct(images, fullProduct(catalog.StatusArchived))
	if err != nil {
		t.Fatal(err)
	}
	requireAllSet(t, "SellerProduct", got)
	if got.GetEtag() != catalog.Etag(13) || got.GetStatus() != pb.ProductStatus_PRODUCT_STATUS_ARCHIVED || got.GetLowStockThreshold() != 7 {
		t.Fatalf("etag/狀態/門檻 = %q %v %d", got.GetEtag(), got.GetStatus(), got.GetLowStockThreshold())
	}
	if got.GetGallery()[0].GetAssetId() != (uuid.UUID{8}).String() || got.GetDescription()[1].GetImage().GetAssetId() != (uuid.UUID{9}).String() {
		t.Fatal("賣家讀取要帶資產 ID（沿用現有的圖時寫入要用）")
	}
	sums, err := sellerSummaries(images, []catalog.ProductSummary{fullSummary(catalog.StatusDraft, 12)})
	if err != nil {
		t.Fatal(err)
	}
	requireAllSet(t, "SellerProductSummary", sums[0])
	if sums[0].GetTotalStock() != 12 || sums[0].GetEtag() != catalog.Etag(13) {
		t.Fatalf("庫存總和與 etag = %d %q", sums[0].GetTotalStock(), sums[0].GetEtag())
	}
}

// 庫存調整後回的 SKU：availability 依商品狀態與調整後的庫存；找不到 SKU 是內部錯誤（調整已成功）。
func TestSellerSkuAfterStockChange(t *testing.T) {
	t.Parallel()
	pr := fullProduct(catalog.StatusActive)
	got, err := sellerSku(pr, uuid.UUID{3})
	if err != nil || got.GetStockQty() != 4 || got.GetAvailability() != pb.ProductAvailability_PRODUCT_AVAILABILITY_AVAILABLE {
		t.Fatalf("sku = %v, %v", got, err)
	}
	pr.Status = catalog.StatusDraft
	if got, _ := sellerSku(pr, uuid.UUID{3}); got.GetAvailability() != pb.ProductAvailability_PRODUCT_AVAILABILITY_UNAVAILABLE {
		t.Fatalf("草稿的 SKU = %v", got)
	}
	if _, err := sellerSku(pr, uuid.UUID{77}); err == nil {
		t.Fatal("找不到 SKU 要回錯誤")
	}
}

// 後台讀取：每個欄位都有對應；列表帶總數與 store_id。
func TestAdminProductMapsEveryField(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.m.product = fullProduct(catalog.StatusDraft)
	got, err := r.adm.GetProduct(adminCtx(), connect.NewRequest(&adminpb.GetProductRequest{ProductId: uuid.UUID{1}.String()}))
	if err != nil {
		t.Fatal(err)
	}
	requireAllSet(t, "admin Product", got.Msg.GetProduct(), "gallery[].asset_id", "description[].image.asset_id")
	if got.Msg.GetProduct().GetStoreId() != "9002" || got.Msg.GetProduct().GetStatus() != pb.ProductStatus_PRODUCT_STATUS_DRAFT {
		t.Fatalf("store/狀態 = %v", got.Msg.GetProduct())
	}
	r.m.page = catalog.ProductPage{Items: []catalog.ProductSummary{fullSummary(catalog.StatusActive, 3)}}
	list, err := r.adm.ListProducts(adminCtx(), connect.NewRequest(&adminpb.ListProductsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	requireAllSet(t, "admin ProductSummary", list.Msg.GetProducts()[0])
}

// 輸入轉換：每個欄位都帶進模組的輸入；沒有設的 oneof 轉成模組認得的空值（由模組回欄位錯誤），不在這裡猜。
func TestToInputCarriesEveryField(t *testing.T) {
	t.Parallel()
	low, cmp := int32(7), int64(150)
	asset := uuid.UUID{9}
	in, err := toInput(&pb.ProductInput{
		Title: "山藥", CategoryId: uuid.UUID{2}.String(), HashtagInputs: []string{"#tea"}, LowStockThreshold: &low,
		Gallery: []*pb.ImageInput{{Source: &pb.ImageInput_StagedToken{StagedToken: "tok"}, Alt: "a"}, {Source: &pb.ImageInput_AssetId{AssetId: asset.String()}}, {}},
		Description: []*pb.DescriptionBlock{
			{Block: &pb.DescriptionBlock_Paragraph{Paragraph: "段"}},
			{Block: &pb.DescriptionBlock_Image{Image: &pb.ImageInput{Source: &pb.ImageInput_StagedToken{StagedToken: "t2"}}}},
			{Block: &pb.DescriptionBlock_BulletList{BulletList: &pb.BulletList{Items: []string{"x"}}}},
			{Block: &pb.DescriptionBlock_Table{Table: &pb.Table{Rows: []*pb.TableRow{{Cells: []string{"1", "2"}}}}}},
			{},
		},
		Axes: []*pb.OptionAxisInput{{Name: "色", Values: []string{"紅"}}},
		Skus: []*pb.SkuInput{{Option1: "紅", Option2: "S", Price: 100, CompareAtPrice: &cmp, InitialStock: 8}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if in.Title != "山藥" || in.CategoryID != (uuid.UUID{2}) || in.Hashtags[0] != "#tea" || *in.LowStockThreshold != 7 {
		t.Fatalf("純量欄位 = %+v", in)
	}
	if in.Gallery[0].Token != "tok" || in.Gallery[0].Alt != "a" || in.Gallery[1].AssetID != asset || (in.Gallery[2] != catalog.Image{}) {
		t.Fatalf("圖庫 = %+v", in.Gallery)
	}
	d := in.Description
	if d[0].Kind != catalog.BlockParagraph || d[0].Text != "段" || d[1].Kind != catalog.BlockImage || d[1].Image.Token != "t2" ||
		d[2].Kind != catalog.BlockList || d[2].Items[0] != "x" || d[3].Kind != catalog.BlockTable || d[3].Rows[0][1] != "2" || d[4].Kind != "" {
		t.Fatalf("描述 = %+v", d)
	}
	s := in.Skus[0]
	if in.Axes[0].Name != "色" || s.Option1 != "紅" || s.Option2 != "S" || s.Price != 100 || *s.CompareAtPrice != 150 || s.StockQty != 8 {
		t.Fatalf("選項與 SKU = %+v %+v", in.Axes, s)
	}
	if _, err := toInput(&pb.ProductInput{CategoryId: "nope"}); err == nil {
		t.Fatal("壞的分類 ID 要回錯誤")
	}
}
