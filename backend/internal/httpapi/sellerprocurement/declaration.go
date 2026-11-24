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

// declarations 是每個 SellerProcurementAppService 方法需要的賣家能力，key 是生成的 procedure 常數。方法與宣告必須一一對應
// （checkDeclarations 在 NewApp 時檢查，缺了或多了都不能啟動）。宣告表只記錄與對照，不執行：能力的檢查在 procurement 模組裡（讀取在查詢之前、
// 寫入在 transaction 內，seller.Authorize），handler 只推導店家身分（seller.CurrentStore）、不檢查能力。
// 值都是 procurement.RequiredSellerCapability，宣告與模組的檢查是同一個常數。這個 service 沒有豁免的方法：整個 service 都是店家回價。
var declarations = map[string]seller.Capability{
	procurementv1alpha1connect.SellerProcurementAppServiceListSellerPublicationsProcedure:    procurement.RequiredSellerCapability,
	procurementv1alpha1connect.SellerProcurementAppServiceGetSellerPublicationProcedure:      procurement.RequiredSellerCapability,
	procurementv1alpha1connect.SellerProcurementAppServiceUpsertSellerResponseDraftProcedure: procurement.RequiredSellerCapability,
	procurementv1alpha1connect.SellerProcurementAppServiceSubmitSellerResponseProcedure:      procurement.RequiredSellerCapability,
	procurementv1alpha1connect.SellerProcurementAppServiceWithdrawSellerResponseProcedure:    procurement.RequiredSellerCapability,
	procurementv1alpha1connect.SellerProcurementAppServiceDeclineRfqPublicationProcedure:     procurement.RequiredSellerCapability,
}

// serviceDescriptor 是 SellerProcurementAppService 的 descriptor，宣告表以它為準。
func serviceDescriptor() protoreflect.ServiceDescriptor {
	return pb.File_hawkr_procurement_v1alpha1_procurement_proto.Services().ByName("SellerProcurementAppService")
}

// checkDeclarations 雙向對照宣告表與 service 的方法（rpc.CheckDeclarations），並檢查每個宣告都是存在的能力。
func checkDeclarations(table map[string]seller.Capability, svc protoreflect.ServiceDescriptor) error {
	return rpc.CheckDeclarations("httpapi/sellerprocurement", table, []protoreflect.ServiceDescriptor{svc}, func(c seller.Capability) string {
		if c == "" || !slices.Contains(seller.Capabilities(), c) {
			return "宣告了不存在的能力 " + string(c)
		}
		return ""
	})
}
