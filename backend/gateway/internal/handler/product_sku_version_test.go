package handler

import (
	"encoding/json"
	"testing"

	pb "github.com/Will413028/hawkr/go-pkg/proto/product"
)

// #2545：SKU 樂觀鎖 token 的出口。
//
// 樂觀鎖是往返協定——賣家要先讀得到 skus[].version，下次寫回才有 expectedVersion 可送。
// 這兩條守住那個出口的**兩端**：賣家／admin 端必須看得到，匿名 PDP 必須看不到。
//
// 為什麼匿名端要擋：version 是**寫入次數計數器**，扣庫存與訂單取消各 +1，公開等於把
// 每個規格的成交／取消事件次數（比商品層的 salesCount 更細）攤在匿名端；而匿名買家
// 對它零用途。公開 DTO 是 deny-by-default 白名單（API 設計規範 §2）。
func detailWithSkuVersion() *pb.ProductDetail {
	return &pb.ProductDetail{
		Id:   "866542646616428481",
		Name: "測試商品",
		Skus: []*pb.ProductSku{{
			Id:       "310",
			SkuName:  "一箱",
			Unit:     "3301",
			Price:    "500",
			StockQty: "30",
			Version:  "7",
		}},
	}
}

func TestSellerProductDetailExposesSkuVersion(t *testing.T) {
	detail := convertToProductDetail("https://media.example", detailWithSkuVersion())

	if len(detail.Skus) != 1 {
		t.Fatalf("測試前提：只有一個 SKU，實得 %d", len(detail.Skus))
	}
	if detail.Skus[0].Version != "7" {
		t.Fatalf("賣家端必須拿得到 skus[].version（拿不到就沒有 expectedVersion 可以回送，"+
			"SKU 樂觀鎖形同不存在），實得 %q", detail.Skus[0].Version)
	}
}

func TestPublicProductDetailHidesSkuVersion(t *testing.T) {
	public := convertToPublicProductDetail("https://media.example", detailWithSkuVersion())

	if len(public.Skus) != 1 {
		t.Fatalf("測試前提：只有一個 SKU，實得 %d", len(public.Skus))
	}
	if public.Skus[0].Version != "" {
		t.Fatalf("匿名 PDP 不得帶 SKU 寫入次數計數器，實得 %q", public.Skus[0].Version)
	}

	// 不只看欄位值：omitempty 之下「鍵不該出現在 JSON 裡」才是真正的對外契約。
	raw, err := json.Marshal(public)
	if err != nil {
		t.Fatalf("marshal 失敗: %v", err)
	}
	var decoded struct {
		Skus []map[string]json.RawMessage `json:"skus"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal 失敗: %v", err)
	}
	if len(decoded.Skus) != 1 {
		t.Fatalf("測試前提：序列化後仍只有一個 SKU，實得 %d", len(decoded.Skus))
	}
	if _, present := decoded.Skus[0]["version"]; present {
		t.Fatalf("公開回應的 skus[0] 不得出現 version 鍵，實得 %v", decoded.Skus[0])
	}
	// 鑑別力自查：這個 map 讀得出東西嗎？讀不出來的話上面那個「鍵不存在」是假綠。
	if _, present := decoded.Skus[0]["stockQty"]; !present {
		t.Fatalf("對照組失敗：連 stockQty 都讀不到，表示這支測試的解析方式壞了")
	}
}
