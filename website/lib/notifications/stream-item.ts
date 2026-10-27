import type { NotificationItem } from "@/lib/notifications/types";
import { asBool, asStr } from "@/lib/sse/utils";

/** Normalize the gateway's notification.new item without trusting its JSON shape. */
export function normalizeNotificationStreamItem(
	raw: unknown,
): NotificationItem {
	const item = (raw ?? {}) as Record<string, unknown>;
	const userType = asStr(item.userType).trim().toLowerCase();
	const data =
		item.data && typeof item.data === "object"
			? Object.fromEntries(
					Object.entries(item.data as Record<string, unknown>).map(
						([key, value]) => [key, asStr(value)],
					),
				)
			: undefined;

	return {
		id: typeof item.id === "number" ? item.id : asStr(item.id),
		// `userId` is the role recipient key (buyer_id/store_id), not the login
		// account id. Only preserve an explicitly named accountId so callers never
		// compare two different identity spaces by accident.
		accountId: item.accountId != null ? asStr(item.accountId) : undefined,
		userType:
			userType === "buyer" || userType === "seller" ? userType : undefined,
		type: asStr(item.type, "system"),
		title: asStr(item.title),
		body: item.body != null ? asStr(item.body) : undefined,
		imageUrl: item.imageUrl != null ? asStr(item.imageUrl) : undefined,
		actionUrl: item.actionUrl != null ? asStr(item.actionUrl) : undefined,
		// REST promotes data.refType/refId to the top level; mirror that for SSE so
		// the same inbox row resolves to the same destination on both transports.
		refType:
			item.refType != null ? asStr(item.refType) : data?.refType || undefined,
		refId: item.refId != null ? asStr(item.refId) : data?.refId || undefined,
		isRead: asBool(item.isRead),
		readAt: item.readAt != null ? asStr(item.readAt) : undefined,
		createdAt: asStr(item.createdAt),
		data,
	};
}
