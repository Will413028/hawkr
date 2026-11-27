package order

import (
	"testing"

	pb "github.com/Will413028/hawkr/go-pkg/proto/order"
)

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
	if out[0].SkuID != "" {
		t.Errorf("SkuID = %q, want 空字串（無 SKU）", out[0].SkuID)
	}
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
