import { createClient } from "@connectrpc/connect";
import { NotificationAppService } from "@hawkr/api/gen/hawkr/notification/v1alpha1/notification_pb";
import { streamTransport } from "./transport";

/**
 * NotificationAppService 的串流呼叫（`WatchNotifications`）：用沒有逾時、沒有重試的 `streamTransport`，
 * 串流的壽命由伺服器決定，重連由 `lib/notifications/watch.ts` 負責。
 */
export function notificationStreamClient() {
	return createClient(NotificationAppService, streamTransport());
}
