package catalog

import (
	"slices"

	"google.golang.org/protobuf/reflect/protoreflect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/catalog/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/catalog/v1alpha1/catalogv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// declaration 是 SellerCatalogAppService 一個方法需要的賣家能力（同 httpapi/coupon）：宣告表是執行的依據，App 的每個方法依它呼叫
// seller.ActingStore（停權與前置條件在那裡判斷），不另外寫第二份。
type declaration struct {
	capability seller.Capability
}

func requires(c seller.Capability) declaration { return declaration{capability: c} }

// declarations 是每個 SellerCatalogAppService 方法的宣告，key 是生成的 procedure 常數。方法與宣告必須一一對應（NewApp 時檢查，缺了或多了都不能啟動）。
// 契約規定每個方法都需要賣家能力 CAP_PRODUCT_MANAGE（含讀取與計數）。
var declarations = map[string]declaration{
	catalogv1alpha1connect.SellerCatalogAppServiceListMyProductsProcedure:   requires(seller.CapProductManage),
	catalogv1alpha1connect.SellerCatalogAppServiceGetMyProductProcedure:     requires(seller.CapProductManage),
	catalogv1alpha1connect.SellerCatalogAppServiceCountMyProductsProcedure:  requires(seller.CapProductManage),
	catalogv1alpha1connect.SellerCatalogAppServiceCreateProductProcedure:    requires(seller.CapProductManage),
	catalogv1alpha1connect.SellerCatalogAppServiceUpdateProductProcedure:    requires(seller.CapProductManage),
	catalogv1alpha1connect.SellerCatalogAppServiceSetProductStatusProcedure: requires(seller.CapProductManage),
	catalogv1alpha1connect.SellerCatalogAppServiceDeleteProductProcedure:    requires(seller.CapProductManage),
	catalogv1alpha1connect.SellerCatalogAppServiceAdjustStockProcedure:      requires(seller.CapProductManage),
	catalogv1alpha1connect.SellerCatalogAppServiceSetStockProcedure:         requires(seller.CapProductManage),
}

// serviceDescriptor 是 SellerCatalogAppService 的 descriptor，宣告表以它為準。
func serviceDescriptor() protoreflect.ServiceDescriptor {
	return pb.File_hawkr_catalog_v1alpha1_catalog_proto.Services().ByName("SellerCatalogAppService")
}

// checkDeclarations 雙向對照宣告表與 service 的方法（rpc.CheckDeclarations），並檢查每個宣告是一個存在的能力。
func checkDeclarations(table map[string]declaration, svc protoreflect.ServiceDescriptor) error {
	return rpc.CheckDeclarations("httpapi/catalog", table, []protoreflect.ServiceDescriptor{svc}, func(d declaration) string {
		if d.capability == "" || !slices.Contains(seller.Capabilities(), d.capability) {
			return "的宣告必須是一個存在的賣家能力"
		}
		return ""
	})
}
