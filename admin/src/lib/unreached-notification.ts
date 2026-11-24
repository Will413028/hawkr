/**
 * 未觸達通知頁的畫面邏輯：失敗原因與渠道結果的顯示文字。
 * 觸達只看推播與 email 的投遞紀錄，收件匣不是渠道；分類與渠道的文字來自共用文案目錄，失敗原因只有後台這一端顯示。
 */

import { DeliveryFailureReason } from "@hawkr/api/gen/hawkr/admin/notification/v1alpha1/notification_pb";
import type { NotificationChannel } from "@hawkr/api/gen/hawkr/notification/v1alpha1/notification_pb";
import { notificationNotificationChannel } from "@hawkr/ui-text/keys";
import { staffTextOrUnknown, unknownText } from "./ui-text";

/** 往回看的天數（伺服器上限 90 天）。 */
export const UNREACHED_DAYS = 7;

const FAILURE_REASON_TEXT: Partial<Record<DeliveryFailureReason, string>> = {
	[DeliveryFailureReason.RECIPIENT_GONE]: "收件人已刪除或退場",
	[DeliveryFailureReason.INTENT_MISSING]: "通知意圖已被清除",
	[DeliveryFailureReason.TYPE_UNREGISTERED]: "通知型別已不存在",
	[DeliveryFailureReason.RENDER_FAILED]: "文字組裝失敗",
	[DeliveryFailureReason.NO_DEVICE_TOKEN]: "沒有可推播的裝置",
	[DeliveryFailureReason.TOKEN_UNREGISTERED]: "裝置 token 已失效",
	[DeliveryFailureReason.PUSH_REJECTED]: "推播服務拒絕",
	[DeliveryFailureReason.NO_EMAIL_ADDRESS]: "沒有已驗證的信箱",
	[DeliveryFailureReason.EMAIL_SUPPRESSED]: "信箱在停寄名單",
	[DeliveryFailureReason.EMAIL_REJECTED]: "寄信服務拒絕",
	[DeliveryFailureReason.RETRIES_EXHAUSTED]: "暫時性失敗用盡重試次數",
	[DeliveryFailureReason.NO_LONGER_VALID]: "來源事實已不成立，刻意不送",
	[DeliveryFailureReason.CHANNEL_UNSUPPORTED]: "渠道不支援",
};

/** 不在對照表的值與 UNSPECIFIED 顯示「未知」。 */
export function failureReasonText(reason: DeliveryFailureReason): string {
	return FAILURE_REASON_TEXT[reason] ?? unknownText();
}

/** 一個渠道的結果，例如「推播：沒有可推播的裝置」。 */
export function channelResultText(
	channel: NotificationChannel,
	reason: DeliveryFailureReason,
): string {
	return `${staffTextOrUnknown(notificationNotificationChannel(channel))}：${failureReasonText(reason)}`;
}
