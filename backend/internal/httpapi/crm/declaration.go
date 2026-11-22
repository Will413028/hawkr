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

// declarations 是每個 CrmAppService 方法需要的賣家能力，key 是生成的 procedure 常數。方法與宣告必須一一對應
// （checkDeclarations 在 NewApp 時檢查，缺了或多了都不能啟動）。宣告表只記錄與對照，不執行：能力的檢查在 crm 模組裡（寫入在 transaction 內、
// 讀取在查詢之前，seller.Authorize），handler 只推導店家身分（seller.CurrentStore）、不檢查能力（ADR 2026-09-29 的修訂）。
// 值都是 crm.RequiredCapability，宣告與模組的檢查是同一個常數。CRM 沒有豁免的方法：整個 service 都是店家以店主身分經營。
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
