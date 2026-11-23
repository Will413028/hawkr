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
	// SearchPageSize 是搜尋每頁的筆數；回傳少於它就是最後一頁。
	SearchPageSize = 50
	// listedStatus 是商品狀態碼的「已上架」。
	listedStatus = "4004"
	// salesSort 是 ProductSort 的銷量排序。
	salesSort = "SALES"
)

// SearchHit 是搜尋結果的一筆上架商品。
type SearchHit struct {
	ID, StoreID        int64
	Name, ImageKey     string
	MinPrice, MaxPrice int64
}

// SearchQuery 是一次上架商品搜尋：分類 ID 與關鍵字二擇一，依銷量排序，Page 從 1 起、每頁 SearchPageSize 筆。
type SearchQuery struct {
	CategoryID int64
	Keyword    string
	Page       int
}

// SearchProducts 搜尋上架商品的一頁（最多 50 筆，銷量高的在前）。
func (c *Client) SearchProducts(ctx context.Context, q SearchQuery) ([]SearchHit, error) {
	keyword := strings.TrimSpace(q.Keyword)
	if (q.CategoryID > 0) == (keyword != "") || q.CategoryID < 0 {
		return nil, fmt.Errorf("productservice：SearchProducts 要分類 ID 或關鍵字其中一個")
	}
	if q.Page < 1 {
		return nil, fmt.Errorf("productservice：SearchProducts 頁碼 %d 小於 1", q.Page)
	}
	req := &productpb.GetProductListRequest{
		Keyword:  keyword,
		Page:     strconv.Itoa(q.Page),
		PageSize: strconv.Itoa(SearchPageSize),
		Sort:     salesSort,
		Status:   listedStatus,
	}
	if q.CategoryID > 0 {
		req.CategoryId = []string{strconv.FormatInt(q.CategoryID, 10)}
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := c.rpc.GetProductList(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, fmt.Errorf("productservice：GetProductList：%w", err)
	}
	if resp.Msg.GetCode() != 0 {
		return nil, fmt.Errorf("productservice：GetProductList code=%d", resp.Msg.GetCode())
	}
	if len(resp.Msg.GetProducts()) > SearchPageSize {
		return nil, fmt.Errorf("productservice：GetProductList 回傳超過 %d 筆", SearchPageSize)
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
