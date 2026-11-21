package productservice

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"

	productpb "github.com/Will413028/hawkr/backend/internal/legacy/gen/product"
)

const (
	searchPageSize = 50
	// listedStatus 是商品狀態碼的「已上架」。
	listedStatus = "4004"
)

// SearchHit 是關鍵字搜尋結果的一筆上架商品。
type SearchHit struct {
	ID, StoreID        int64
	Name, ImageKey     string
	MinPrice, MaxPrice int64
}

// SearchProducts 以關鍵字搜尋上架商品，只取第一頁（最多 50 筆），不分頁。
func (c *Client) SearchProducts(ctx context.Context, keyword string) ([]SearchHit, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil, fmt.Errorf("productservice：SearchProducts 缺少關鍵字")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := c.rpc.GetProductList(ctx, connect.NewRequest(&productpb.GetProductListRequest{
		Keyword:  keyword,
		Page:     "1",
		PageSize: strconv.Itoa(searchPageSize),
		Status:   listedStatus,
	}))
	if err != nil {
		return nil, fmt.Errorf("productservice：GetProductList：%w", err)
	}
	if resp.Msg.GetCode() != 0 {
		return nil, fmt.Errorf("productservice：GetProductList code=%d", resp.Msg.GetCode())
	}
	if len(resp.Msg.GetProducts()) > searchPageSize {
		return nil, fmt.Errorf("productservice：GetProductList 回傳超過 %d 筆", searchPageSize)
	}
	out := make([]SearchHit, 0, len(resp.Msg.GetProducts()))
	for _, item := range resp.Msg.GetProducts() {
		id, err := positiveID(item.GetId())
		if err != nil {
			return nil, err
		}
		storeID, err := positiveID(item.GetStoreId())
		if err != nil {
			return nil, err
		}
		minPrice, err := nonnegative(item.GetPriceMin())
		if err != nil {
			return nil, err
		}
		maxPrice, err := nonnegative(item.GetPriceMax())
		if err != nil {
			return nil, err
		}
		out = append(out, SearchHit{ID: id, StoreID: storeID, Name: item.GetName(), ImageKey: item.GetImageKey(), MinPrice: minPrice, MaxPrice: maxPrice})
	}
	return out, nil
}
