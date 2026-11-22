package chat

import (
	"slices"

	"google.golang.org/protobuf/reflect/protoreflect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/chat/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/chat/v1alpha1/chatv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// declaration 是 ChatAppService 一個方法的能力宣告（同 httpapi/review）：要不然是以店家身分動作時需要的賣家能力，要不然明列豁免與理由。
// 宣告表只記錄與對照，不執行：能力的檢查在 im 模組裡（Party.Authorize、送出的守門），handler 不檢查。
type declaration struct {
	capability seller.Capability
	// exemptReason 非空表示豁免。
	exemptReason string
}

func requires(c seller.Capability) declaration { return declaration{capability: c} }

func exempt(reason string) declaration { return declaration{exemptReason: reason} }

const buyerOnlyReason = "只有買家身分能做的動作（店家有對應的店家版方法），沒有賣家能力可宣告"

// declarations 是每個 ChatAppService 方法的宣告，key 是生成的 procedure 常數。方法與宣告必須一一對應
// （checkDeclarations 在 NewApp 時檢查，缺了或多了都不能啟動）。
// 由對話歸屬推導身分的方法（ListMessages、SendMessage、MarkConversationRead）兩種身分都可能：宣告店家身分時需要的能力，買家身分不適用。
var declarations = map[string]declaration{
	// 店家身分：讀取、已讀、未讀、在線狀態與即時事件流要 CapChatRead（im.Party.Authorize），送出要 CapChatSend（送出的守門 G2；店家寄件人同時要 CapChatRead）。
	chatv1alpha1connect.ChatAppServiceListStoreConversationsProcedure: requires(seller.CapChatRead),
	chatv1alpha1connect.ChatAppServiceMarkAllStoreReadProcedure:       requires(seller.CapChatRead),
	chatv1alpha1connect.ChatAppServiceGetStoreUnreadCountProcedure:    requires(seller.CapChatRead),
	chatv1alpha1connect.ChatAppServiceGetStorePresenceProcedure:       requires(seller.CapChatRead),
	chatv1alpha1connect.ChatAppServiceStreamStoreEventsProcedure:      requires(seller.CapChatRead),
	chatv1alpha1connect.ChatAppServiceSendMessageToBuyerProcedure:     requires(seller.CapChatSend),

	// 由對話歸屬推導：店家方動作時的能力（買家方不適用）。
	chatv1alpha1connect.ChatAppServiceListMessagesProcedure:         requires(seller.CapChatRead),
	chatv1alpha1connect.ChatAppServiceMarkConversationReadProcedure: requires(seller.CapChatRead),
	chatv1alpha1connect.ChatAppServiceSendMessageProcedure:          requires(seller.CapChatSend),

	chatv1alpha1connect.ChatAppServiceListConversationsProcedure:  exempt(buyerOnlyReason),
	chatv1alpha1connect.ChatAppServiceSendMessageToStoreProcedure: exempt(buyerOnlyReason),
	chatv1alpha1connect.ChatAppServiceMarkAllReadProcedure:        exempt(buyerOnlyReason),
	chatv1alpha1connect.ChatAppServiceGetUnreadCountProcedure:     exempt(buyerOnlyReason),
	chatv1alpha1connect.ChatAppServiceGetPresenceProcedure:        exempt(buyerOnlyReason),
	chatv1alpha1connect.ChatAppServiceStreamEventsProcedure:       exempt(buyerOnlyReason),
}

// serviceDescriptor 是 ChatAppService 的 descriptor，宣告表以它為準。
func serviceDescriptor() protoreflect.ServiceDescriptor {
	return pb.File_hawkr_chat_v1alpha1_chat_proto.Services().ByName("ChatAppService")
}

// checkDeclarations 雙向對照宣告表與 service 的方法（rpc.CheckDeclarations），並檢查每個宣告恰好是一個存在的能力或一個有理由的豁免。
func checkDeclarations(table map[string]declaration, svc protoreflect.ServiceDescriptor) error {
	return rpc.CheckDeclarations("httpapi/chat", table, []protoreflect.ServiceDescriptor{svc}, func(d declaration) string {
		switch {
		case (d.capability == "") == (d.exemptReason == ""):
			return "的宣告必須是一個能力或一個有理由的豁免"
		case d.capability != "" && !slices.Contains(seller.Capabilities(), d.capability):
			return "宣告了不存在的能力 " + string(d.capability)
		}
		return ""
	})
}
