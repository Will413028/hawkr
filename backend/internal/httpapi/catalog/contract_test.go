package catalog

import (
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"

	catalogv1 "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/catalog/v1alpha1"
)

// reachable 回傳從 root 訊息能走到的每個訊息與 enum 全名（遞迴）。
func reachable(root protoreflect.MessageDescriptor) (messages, enums map[string]bool) {
	messages, enums = map[string]bool{}, map[string]bool{}
	var walk func(m protoreflect.MessageDescriptor)
	walk = func(m protoreflect.MessageDescriptor) {
		if messages[string(m.FullName())] {
			return
		}
		messages[string(m.FullName())] = true
		for i := range m.Fields().Len() {
			f := m.Fields().Get(i)
			switch {
			case f.Message() != nil:
				walk(f.Message())
			case f.Enum() != nil:
				enums[string(f.Enum().FullName())] = true
			}
		}
	}
	walk(root)
	return messages, enums
}

// 公開訊息只帶 availability，不暴露賣家的狀態 enum（D24 Q14）：公開 service 每個 RPC 的請求與回應能走到的欄位裡都沒有 ProductStatus，
// 也沒有只屬於賣家的訊息（SellerProduct、SellerProductSummary）。
func TestPublicMessagesCarryNoSellerStatusEnum(t *testing.T) {
	t.Parallel()
	svc := catalogv1.File_hawkr_catalog_v1alpha1_catalog_proto.Services().ByName("CatalogPublicService")
	if svc == nil {
		t.Fatal("CatalogPublicService not found")
	}
	const status = "hawkr.catalog.v1alpha1.ProductStatus"
	sawAvailability := false
	for i := range svc.Methods().Len() {
		m := svc.Methods().Get(i)
		for _, root := range []protoreflect.MessageDescriptor{m.Input(), m.Output()} {
			msgs, enums := reachable(root)
			if enums[status] {
				t.Errorf("%s 的 %s 能走到賣家的狀態 enum ProductStatus", m.Name(), root.Name())
			}
			for _, seller := range []string{"hawkr.catalog.v1alpha1.SellerProduct", "hawkr.catalog.v1alpha1.SellerProductSummary"} {
				if msgs[seller] {
					t.Errorf("%s 的 %s 能走到賣家的訊息 %s", m.Name(), root.Name(), seller)
				}
			}
			if enums["hawkr.catalog.v1alpha1.ProductAvailability"] {
				sawAvailability = true
			}
		}
	}
	if !sawAvailability {
		t.Error("公開訊息應該帶 availability，檢查範圍不對")
	}
	// 反向確認檢查本身有效：賣家的讀取訊息確實帶 ProductStatus。
	_, enums := reachable((&catalogv1.SellerProduct{}).ProtoReflect().Descriptor())
	if !enums[status] {
		t.Error("SellerProduct 應該帶 ProductStatus，走訪邏輯有誤")
	}
}

// 公開讀取不帶賣家才有的欄位：etag、內部的建立與修改時間、低庫存門檻。
func TestPublicProductHasNoSellerOnlyFields(t *testing.T) {
	t.Parallel()
	for _, m := range []protoreflect.MessageDescriptor{
		(&catalogv1.Product{}).ProtoReflect().Descriptor(),
		(&catalogv1.ProductSummary{}).ProtoReflect().Descriptor(),
	} {
		for _, name := range []protoreflect.Name{"etag", "create_time", "update_time", "low_stock_threshold", "total_stock", "status"} {
			if m.Fields().ByName(name) != nil {
				t.Errorf("%s 不應該有欄位 %s", m.Name(), name)
			}
		}
	}
}
