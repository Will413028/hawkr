package chat

import (
	"slices"

	"google.golang.org/protobuf/reflect/protoreflect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/chat/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/chat/v1alpha1/chatv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

type declaration struct {
	capability seller.Capability
	exemptReason string
}

func requires(c seller.Capability) declaration { return declaration{capability: c} }

func exempt(reason string) declaration { return declaration{exemptReason: reason} }

const buyerOnlyReason = "只有買家身分能做的動作（店家有對應的店家版方法），沒有賣家能力可宣告"

var declarations = map[string]declaration{
	chatv1alpha1connect.ChatAppServiceListStoreConversationsProcedure:      requires(seller.CapChatRead),
	chatv1alpha1connect.ChatAppServiceGetStoreConversationByBuyerProcedure: requires(seller.CapChatRead),
	chatv1alpha1connect.ChatAppServiceMarkAllStoreReadProcedure:            requires(seller.CapChatRead),
	chatv1alpha1connect.ChatAppServiceGetStoreUnreadCountProcedure:         requires(seller.CapChatRead),
	chatv1alpha1connect.ChatAppServiceGetStorePresenceProcedure:            requires(seller.CapChatRead),
	chatv1alpha1connect.ChatAppServiceStreamStoreEventsProcedure:           requires(seller.CapChatRead),
	chatv1alpha1connect.ChatAppServiceSendMessageToBuyerProcedure:          requires(seller.CapChatSend),

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

func serviceDescriptor() protoreflect.ServiceDescriptor {
	return pb.File_hawkr_chat_v1alpha1_chat_proto.Services().ByName("ChatAppService")
}

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
