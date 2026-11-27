package catalog

import (
	"slices"

	"google.golang.org/protobuf/reflect/protoreflect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/catalog/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/catalog/v1alpha1/catalogv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

type declaration struct {
	capability seller.Capability
}

func requires(c seller.Capability) declaration { return declaration{capability: c} }

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

func serviceDescriptor() protoreflect.ServiceDescriptor {
	return pb.File_hawkr_catalog_v1alpha1_catalog_proto.Services().ByName("SellerCatalogAppService")
}

func checkDeclarations(table map[string]declaration, svc protoreflect.ServiceDescriptor) error {
	return rpc.CheckDeclarations("httpapi/catalog", table, []protoreflect.ServiceDescriptor{svc}, func(d declaration) string {
		if d.capability == "" || !slices.Contains(seller.Capabilities(), d.capability) {
			return "的宣告必須是一個存在的賣家能力"
		}
		return ""
	})
}
