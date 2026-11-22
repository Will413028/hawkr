package moderation

import (
	"slices"

	"google.golang.org/protobuf/reflect/protoreflect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/moderation/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/moderation/v1alpha1/moderationv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// declaration 是 ModerationAppService 一個方法的能力宣告（同 httpapi/chat）：要不然是以店家身分動作時需要的賣家能力，要不然明列豁免與理由。
// 宣告表只記錄與對照，不執行：能力的檢查在模組裡（moderation 以 im 的 MessageForReport 核對店家方的聊天讀取能力），handler 不檢查。
type declaration struct {
	capability seller.Capability
	// exemptReason 非空表示豁免。
	exemptReason string
}

func requires(c seller.Capability) declaration { return declaration{capability: c} }

func exempt(reason string) declaration { return declaration{exemptReason: reason} }

const (
	buyerOnlyReason = "只有買家身分能做的動作（店家有對應的店家版方法），沒有賣家能力可宣告"
	ownStoreReason  = "moderation.MyStoreReports 只以 handler 推導的店家 ID 讀這家店送出的檢舉，模組不檢查賣家能力，停權的店家仍可讀自己店的檢舉"
)

// declarations 是每個 ModerationAppService 方法的宣告，key 是生成的 procedure 常數。方法與宣告必須一一對應
// （checkDeclarations 在 NewApp 時檢查，缺了或多了都不能啟動）。
var declarations = map[string]declaration{
	// 對象是聊天訊息時由訊息的歸屬推導身分：店家方動作需要 CapChatRead（買家方與其他對象不適用）。
	moderationv1alpha1connect.ModerationAppServiceSubmitReportProcedure: requires(seller.CapChatRead),

	moderationv1alpha1connect.ModerationAppServiceListMyStoreReportsProcedure: exempt(ownStoreReason),
	moderationv1alpha1connect.ModerationAppServiceListMyReportsProcedure:      exempt(buyerOnlyReason),
}

// serviceDescriptor 是 ModerationAppService 的 descriptor，宣告表以它為準。
func serviceDescriptor() protoreflect.ServiceDescriptor {
	return pb.File_hawkr_moderation_v1alpha1_moderation_proto.Services().ByName("ModerationAppService")
}

// checkDeclarations 雙向對照宣告表與 service 的方法（rpc.CheckDeclarations），並檢查每個宣告恰好是一個存在的能力或一個有理由的豁免。
func checkDeclarations(table map[string]declaration, svc protoreflect.ServiceDescriptor) error {
	return rpc.CheckDeclarations("httpapi/moderation", table, []protoreflect.ServiceDescriptor{svc}, func(d declaration) string {
		switch {
		case (d.capability == "") == (d.exemptReason == ""):
			return "的宣告必須是一個能力或一個有理由的豁免"
		case d.capability != "" && !slices.Contains(seller.Capabilities(), d.capability):
			return "宣告了不存在的能力 " + string(d.capability)
		}
		return ""
	})
}
