package notification

import (
	"slices"

	"google.golang.org/protobuf/reflect/protoreflect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/notification/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/notification/v1alpha1/notificationv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

type declaration struct {
	capability   seller.Capability
	exemptReason string
}

func requires(c seller.Capability) declaration { return declaration{capability: c} }

func exempt(reason string) declaration { return declaration{exemptReason: reason} }

const accountScopedReason = "請求不帶受眾，只以登入的帳號為範圍（通知的擁有者是帳號），不以店家身分動作，沒有賣家能力可宣告"

var declarations = map[string]declaration{
	notificationv1alpha1connect.NotificationAppServiceListInboxItemsProcedure:               requires(seller.CapNotification),
	notificationv1alpha1connect.NotificationAppServiceGetUnreadCountProcedure:               requires(seller.CapNotification),
	notificationv1alpha1connect.NotificationAppServiceMarkAllInboxItemsReadProcedure:        requires(seller.CapNotification),
	notificationv1alpha1connect.NotificationAppServiceGetNotificationPreferencesProcedure:   requires(seller.CapNotification),
	notificationv1alpha1connect.NotificationAppServiceUpdateNotificationPreferenceProcedure: requires(seller.CapNotification),
	notificationv1alpha1connect.NotificationAppServiceWatchNotificationsProcedure:           requires(seller.CapNotification),

	notificationv1alpha1connect.NotificationAppServiceMarkInboxItemReadProcedure:     exempt(accountScopedReason),
	notificationv1alpha1connect.NotificationAppServiceBatchDeleteInboxItemsProcedure: exempt(accountScopedReason),
	notificationv1alpha1connect.NotificationAppServiceRegisterDeviceTokenProcedure:   exempt(accountScopedReason),
	notificationv1alpha1connect.NotificationAppServiceDeleteDeviceTokenProcedure:     exempt(accountScopedReason),
}

func serviceDescriptor() protoreflect.ServiceDescriptor {
	return pb.File_hawkr_notification_v1alpha1_notification_proto.Services().ByName("NotificationAppService")
}

func checkDeclarations(table map[string]declaration, svc protoreflect.ServiceDescriptor) error {
	return rpc.CheckDeclarations("httpapi/notification", table, []protoreflect.ServiceDescriptor{svc}, func(d declaration) string {
		switch {
		case (d.capability == "") == (d.exemptReason == ""):
			return "的宣告必須是一個能力或一個有理由的豁免"
		case d.capability != "" && !slices.Contains(seller.Capabilities(), d.capability):
			return "宣告了不存在的能力 " + string(d.capability)
		}
		return ""
	})
}
