package order

import (
	"testing"

	pb "github.com/Will413028/hawkr/go-pkg/proto/order"
)

// non-catalog line（RFQ 成交的訂單明細）沒有平台 SKU：order_items.sku_id 為 NULL，
// order-service 的 OrderProtoConverter 經 ProtoConversionUtils.formatLong（null-safe）
// 輸出**空字串**。本檔守的是 invoice 這一端不把它當壞資料擋下來。
//
// 🔴 為什麼需要這組測試：2026-09-21 實查發現 mapItems 對 sku_id 用的是
// parseRequiredInt，空字串直接 error ⇒ RFQ 訂單根本組不出發票行。而 non-catalog
// 的 ADR 原本判定 invoice 是「假相依」，理由是 SellerDefaultTaxResolver.ResolveProduct
// 忽略 skuID —— 那個判定只看到 domain 層，沒看到 parse 擋在更前面，根本走不到它。
const testSkuUUID = "0192a3b4-0000-7000-8000-000000000011"

func nonCatalogItem(skuID string) *pb.OrderInvoiceSnapshotItem {
	return &pb.OrderInvoiceSnapshotItem{
		OrderItemId: "7001",
		SkuId:       skuID,
		ProductName: "文旦柚",
		SkuName:     "文旦柚 200 公斤",
		Quantity:    "1",
		UnitPrice:   "30000",
	}
}

func TestMapItems_NonCatalogLine_EmptySkuIDAccepted(t *testing.T) {
	out, err := mapItems([]*pb.OrderInvoiceSnapshotItem{nonCatalogItem("")})
	if err != nil {
		t.Fatalf("non-catalog line 不該被擋下: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("len(out) = %d, want 1", len(out))
	}
	// 空字串是 domain 層「無此值」的慣例（Line.SkuID 的註解），
	// 最終寫進 invoice_document_lines.sku_id 會是 NULL。
	if out[0].SkuID != "" {
		t.Errorf("SkuID = %q, want 空字串（無 SKU）", out[0].SkuID)
	}
	// ⚠️ 品名不受影響：sku_name 是 NOT NULL 且禁止 sentinel（doc 64 §6.4.9），
	//    那條禁令只約束品名、不約束 sku_id 指向誰。
	if out[0].SkuName != "文旦柚 200 公斤" {
		t.Errorf("SkuName = %q，non-catalog 不該動到品名", out[0].SkuName)
	}
}

func TestMapItems_CatalogLine_SkuIDStillParsed(t *testing.T) {
	out, err := mapItems([]*pb.OrderInvoiceSnapshotItem{nonCatalogItem(testSkuUUID)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[0].SkuID != testSkuUUID {
		t.Errorf("SkuID = %q, want %q", out[0].SkuID, testSkuUUID)
	}
}

// 放寬的只有「缺值」，不是「亂值」：非空但非 UUID 代表上游壞了，仍然要擋。
func TestMapItems_MalformedSkuIDStillRejected(t *testing.T) {
	if _, err := mapItems([]*pb.OrderInvoiceSnapshotItem{nonCatalogItem("5150")}); err == nil {
		t.Fatal("非 UUID 的 sku_id 應該仍然被擋下")
	}
}

// 回歸防護：放寬 sku_id **不得**連帶放寬其他數值欄位。
func TestMapItems_OtherNumericFieldsStillRequired(t *testing.T) {
	for _, tc := range []struct {
		name string
		mut  func(*pb.OrderInvoiceSnapshotItem)
	}{
		{"order_item_id", func(i *pb.OrderInvoiceSnapshotItem) { i.OrderItemId = "" }},
		{"quantity", func(i *pb.OrderInvoiceSnapshotItem) { i.Quantity = "" }},
		{"unit_price", func(i *pb.OrderInvoiceSnapshotItem) { i.UnitPrice = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			it := nonCatalogItem(testSkuUUID)
			tc.mut(it)
			if _, err := mapItems([]*pb.OrderInvoiceSnapshotItem{it}); err == nil {
				t.Fatalf("%s 缺值應該仍然被擋下", tc.name)
			}
		})
	}
}
