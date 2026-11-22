package crm

import (
	"slices"

	"google.golang.org/protobuf/reflect/protoreflect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/crm/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/crm/v1alpha1/crmv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// declarations 是每個 CrmAppService 方法需要的賣家能力，key 是生成的 procedure 常數。方法與宣告必須一一對應
// （checkDeclarations 在 NewApp 時檢查，缺了或多了都不能啟動）。能力由 handler 在每個方法開頭以 seller.ActingStore 檢查（authorize），
// CRM 沒有豁免的方法：整個 service 都是店家以店主身分經營。
var declarations = map[string]seller.Capability{
	crmv1alpha1connect.CrmAppServiceListCustomersProcedure:     seller.CapMarketing,
	crmv1alpha1connect.CrmAppServiceGetCustomerProcedure:       seller.CapMarketing,
	crmv1alpha1connect.CrmAppServiceUpdateCustomerProcedure:    seller.CapMarketing,
	crmv1alpha1connect.CrmAppServiceCreateTagProcedure:         seller.CapMarketing,
	crmv1alpha1connect.CrmAppServiceListTagsProcedure:          seller.CapMarketing,
	crmv1alpha1connect.CrmAppServiceUpdateTagProcedure:         seller.CapMarketing,
	crmv1alpha1connect.CrmAppServiceDeleteTagProcedure:         seller.CapMarketing,
	crmv1alpha1connect.CrmAppServiceAddCustomerTagsProcedure:   seller.CapMarketing,
	crmv1alpha1connect.CrmAppServiceRemoveCustomerTagProcedure: seller.CapMarketing,
}

// serviceDescriptor 是 CrmAppService 的 descriptor，宣告表以它為準。
func serviceDescriptor() protoreflect.ServiceDescriptor {
	return pb.File_hawkr_crm_v1alpha1_crm_proto.Services().ByName("CrmAppService")
}

// checkDeclarations 雙向對照宣告表與 service 的方法（rpc.CheckDeclarations），並檢查每個宣告都是存在的能力。
func checkDeclarations(table map[string]seller.Capability, svc protoreflect.ServiceDescriptor) error {
	return rpc.CheckDeclarations("httpapi/crm", table, []protoreflect.ServiceDescriptor{svc}, func(c seller.Capability) string {
		if c == "" || !slices.Contains(seller.Capabilities(), c) {
			return "宣告了不存在的能力 " + string(c)
		}
		return ""
	})
}
