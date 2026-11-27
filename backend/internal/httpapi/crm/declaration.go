package crm

import (
	"slices"

	"google.golang.org/protobuf/reflect/protoreflect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/crm/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/crm/v1alpha1/crmv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/crm"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

var declarations = map[string]seller.Capability{
	crmv1alpha1connect.CrmAppServiceListCustomersProcedure:     crm.RequiredCapability,
	crmv1alpha1connect.CrmAppServiceGetCustomerProcedure:       crm.RequiredCapability,
	crmv1alpha1connect.CrmAppServiceUpdateCustomerProcedure:    crm.RequiredCapability,
	crmv1alpha1connect.CrmAppServiceCreateTagProcedure:         crm.RequiredCapability,
	crmv1alpha1connect.CrmAppServiceListTagsProcedure:          crm.RequiredCapability,
	crmv1alpha1connect.CrmAppServiceUpdateTagProcedure:         crm.RequiredCapability,
	crmv1alpha1connect.CrmAppServiceDeleteTagProcedure:         crm.RequiredCapability,
	crmv1alpha1connect.CrmAppServiceAddCustomerTagsProcedure:   crm.RequiredCapability,
	crmv1alpha1connect.CrmAppServiceRemoveCustomerTagProcedure: crm.RequiredCapability,
}

func serviceDescriptor() protoreflect.ServiceDescriptor {
	return pb.File_hawkr_crm_v1alpha1_crm_proto.Services().ByName("CrmAppService")
}

func checkDeclarations(table map[string]seller.Capability, svc protoreflect.ServiceDescriptor) error {
	return rpc.CheckDeclarations("httpapi/crm", table, []protoreflect.ServiceDescriptor{svc}, func(c seller.Capability) string {
		if c == "" || !slices.Contains(seller.Capabilities(), c) {
			return "宣告了不存在的能力 " + string(c)
		}
		return ""
	})
}
