// Package productservice 呼叫遷移期的 product-service。
package productservice

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"

	productpb "github.com/Will413028/hawkr/backend/internal/legacy/gen/product"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/product/productconnect"
	"github.com/Will413028/hawkr/backend/internal/legacy/grpcclient"
)

// Client 讀取商品顯示資料。
type Client struct {
	rpc productconnect.ProductServiceClient
}

// Card 是 product-service 線上資料的已解析顯示投影；缺席的 ID 表示已刪除。
type Card struct {
	ID, StoreID        int64
	Name, ImageKey     string
	MinPrice, MaxPrice int64
	Rating             *float32
	Available          bool
}

// New 建立以 h2c 呼叫並簽章的 client。
func New(addr, secret string) (*Client, error) {
	if addr == "" {
		return nil, fmt.Errorf("productservice：缺少位址")
	}
	return &Client{rpc: productconnect.NewProductServiceClient(grpcclient.HTTPClient(), "http://"+addr, connect.WithGRPC(), connect.WithInterceptors(grpcclient.Signer(secret, nil)))}, nil
}

// Cards 批次讀取商品；未回傳的 ID 表示商品已刪除。
func (c *Client) Cards(ctx context.Context, ids []int64, includeInactive bool) (map[int64]Card, error) {
	if len(ids) == 0 {
		return map[int64]Card{}, nil
	}
	values := make([]string, len(ids))
	for i, id := range ids {
		values[i] = strconv.FormatInt(id, 10)
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := c.rpc.GetProductsByIDs(ctx, connect.NewRequest(&productpb.GetProductsByIDsRequest{ProductIds: values, IncludeInactive: includeInactive}))
	if err != nil {
		return nil, fmt.Errorf("productservice：GetProductsByIDs：%w", err)
	}
	if resp.Msg.GetCode() != 0 {
		return nil, fmt.Errorf("productservice：GetProductsByIDs code=%d", resp.Msg.GetCode())
	}
	out := make(map[int64]Card, len(resp.Msg.GetProducts()))
	for _, product := range resp.Msg.GetProducts() {
		card, err := parseCard(product)
		if err != nil {
			return nil, err
		}
		out[card.ID] = card
	}
	return out, nil
}

// StoresWithPublicProducts 回傳至少有一件公開上架商品的店家 ID 全集（沒有公開商品的店不在其中）。
// 失敗就回錯誤：「查不到」與「回空集合」語意不同，呼叫端不得把錯誤當成空集合。
func (c *Client) StoresWithPublicProducts(ctx context.Context) ([]int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := c.rpc.ListStoreIDsWithPublicProductsInternal(ctx, connect.NewRequest(&productpb.ListStoreIDsWithPublicProductsInternalRequest{}))
	if err != nil {
		return nil, fmt.Errorf("productservice：ListStoreIDsWithPublicProductsInternal：%w", err)
	}
	if resp.Msg.GetCode() != 0 {
		return nil, fmt.Errorf("productservice：ListStoreIDsWithPublicProductsInternal code=%d", resp.Msg.GetCode())
	}
	out := make([]int64, 0, len(resp.Msg.GetStoreIds()))
	seen := make(map[int64]bool, len(resp.Msg.GetStoreIds()))
	for _, raw := range resp.Msg.GetStoreIds() {
		id, err := positiveID(raw)
		if err != nil {
			return nil, err
		}
		if seen[id] {
			return nil, fmt.Errorf("productservice：店家 ID 重複: %d", id)
		}
		seen[id] = true
		out = append(out, id)
	}
	return out, nil
}

func positiveID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("productservice：ID 不合法: %q", raw)
	}
	return id, nil
}

func nonnegative(raw string) (int64, error) {
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v < 0 {
		return 0, fmt.Errorf("productservice：數量或金額不合法: %q", raw)
	}
	return v, nil
}

func productRating(raw string) (*float32, error) {
	if raw == "" {
		return nil, nil
	}
	v, err := strconv.ParseFloat(raw, 32)
	if err != nil {
		return nil, fmt.Errorf("productservice：評分不合法: %w", err)
	}
	if v == 0 {
		return nil, nil
	}
	rating := float32(v)
	return &rating, nil
}

func parseCard(item *productpb.ProductListItem) (Card, error) {
	id, err := positiveID(item.GetId())
	if err != nil {
		return Card{}, err
	}
	storeID, err := positiveID(item.GetStoreId())
	if err != nil {
		return Card{}, err
	}
	minPrice, err := nonnegative(item.GetPriceMin())
	if err != nil {
		return Card{}, err
	}
	maxPrice, err := nonnegative(item.GetPriceMax())
	if err != nil {
		return Card{}, err
	}
	stock, err := nonnegative(item.GetStockQty())
	if err != nil {
		return Card{}, err
	}
	rating, err := productRating(item.GetRating())
	if err != nil {
		return Card{}, err
	}
	return Card{ID: id, StoreID: storeID, Name: item.GetName(), ImageKey: item.GetImageKey(), MinPrice: minPrice, MaxPrice: maxPrice, Rating: rating, Available: strings.EqualFold(item.GetStatus(), "ACTIVE") && stock > 0}, nil
}
