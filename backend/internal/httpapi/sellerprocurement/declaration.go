package sellerprocurement

import (
	"slices"

	"google.golang.org/protobuf/reflect/protoreflect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/procurement/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/procurement/v1alpha1/procurementv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

var declarations = map[string]seller.Capability{
	procurementv1alpha1connect.SellerProcurementAppServiceListSellerPublicationsProcedure:    procurement.RequiredSellerCapability,
	procurementv1alpha1connect.SellerProcurementAppServiceGetSellerPublicationProcedure:      procurement.RequiredSellerCapability,
	procurementv1alpha1connect.SellerProcurementAppServiceUpsertSellerResponseDraftProcedure: procurement.RequiredSellerCapability,
	procurementv1alpha1connect.SellerProcurementAppServiceSubmitSellerResponseProcedure:      procurement.RequiredSellerCapability,
	procurementv1alpha1connect.SellerProcurementAppServiceWithdrawSellerResponseProcedure:    procurement.RequiredSellerCapability,
	procurementv1alpha1connect.SellerProcurementAppServiceDeclineRfqPublicationProcedure:     procurement.RequiredSellerCapability,
}

func serviceDescriptor() protoreflect.ServiceDescriptor {
	return pb.File_hawkr_procurement_v1alpha1_procurement_proto.Services().ByName("SellerProcurementAppService")
}

func checkDeclarations(table map[string]seller.Capability, svc protoreflect.ServiceDescriptor) error {
	return rpc.CheckDeclarations("httpapi/sellerprocurement", table, []protoreflect.ServiceDescriptor{svc}, func(c seller.Capability) string {
		if c == "" || !slices.Contains(seller.Capabilities(), c) {
			return "宣告了不存在的能力 " + string(c)
		}
		return ""
	})
}
