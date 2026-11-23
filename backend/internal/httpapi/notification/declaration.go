package notification

import (
	"slices"

	"google.golang.org/protobuf/reflect/protoreflect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/notification/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/notification/v1alpha1/notificationv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// declaration 是 NotificationAppService 一個方法的能力宣告（同 httpapi/chat）：要不然是以店家身分（STORE 受眾）動作時需要的賣家能力，要不然明列豁免與理由。
// 宣告表只記錄與對照，不執行：STORE 受眾的能力由 handler 以 seller.ActingStore 檢查。
type declaration struct {
	capability seller.Capability
	// exemptReason 非空表示豁免。
	exemptReason string
}

func requires(c seller.Capability) declaration { return declaration{capability: c} }

func exempt(reason string) declaration { return declaration{exemptReason: reason} }

const accountScopedReason = "請求不帶受眾，只以登入的帳號為範圍（通知的擁有者是帳號），不以店家身分動作，沒有賣家能力可宣告"

// declarations 是每個 NotificationAppService 方法的宣告，key 是生成的 procedure 常數。方法與宣告必須一一對應
// （checkDeclarations 在 NewApp 時檢查，缺了或多了都不能啟動）。CapNotification 在停權白名單內：停權的賣家仍要讀到停權通知。
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

// serviceDescriptor 是 NotificationAppService 的 descriptor，宣告表以它為準。
func serviceDescriptor() protoreflect.ServiceDescriptor {
	return pb.File_hawkr_notification_v1alpha1_notification_proto.Services().ByName("NotificationAppService")
}

// checkDeclarations 雙向對照宣告表與 service 的方法（rpc.CheckDeclarations），並檢查每個宣告恰好是一個存在的能力或一個有理由的豁免。
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
