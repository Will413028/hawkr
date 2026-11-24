import {
	type InboxItem,
	NotificationAudience,
	type NotificationCategory,
	type NotificationChannel,
	type PreferenceEntry,
} from "@hawkr/api/gen/hawkr/notification/v1alpha1/notification_pb";
import type { RoleSlot } from "@/lib/auth/active-role";
import { notificationAppClient } from "@/lib/connect/notification";

export type { InboxItem, PreferenceEntry };

/** 買家工作區讀 BUYER 收件匣，賣家工作區讀 STORE 收件匣。 */
export function audienceForRole(role: RoleSlot): NotificationAudience {
	return role === "seller"
		? NotificationAudience.STORE
		: NotificationAudience.BUYER;
}

export interface InboxPage {
	items: InboxItem[];
	nextPageToken: string;
	totalSize: number;
}

export async function listInboxItems(params: {
	audience: NotificationAudience;
	category?: NotificationCategory;
	pageSize?: number;
	pageToken?: string;
}): Promise<InboxPage> {
	const res = await notificationAppClient().listInboxItems({
		audience: params.audience,
		category: params.category,
		pageSize: params.pageSize ?? 20,
		pageToken: params.pageToken ?? "",
	});
	return {
		items: res.items,
		nextPageToken: res.nextPageToken,
		totalSize: res.totalSize,
	};
}

export async function fetchUnreadCount(
	audience: NotificationAudience,
): Promise<number> {
	const res = await notificationAppClient().getUnreadCount({ audience });
	return res.unreadCount;
}

export async function markInboxItemRead(id: string): Promise<void> {
	await notificationAppClient().markInboxItemRead({ id });
}

export async function markAllInboxItemsRead(
	audience: NotificationAudience,
): Promise<void> {
	await notificationAppClient().markAllInboxItemsRead({ audience });
}

export async function deleteInboxItems(ids: string[]): Promise<void> {
	if (ids.length === 0) return;
	await notificationAppClient().batchDeleteInboxItems({ ids });
}

export async function fetchNotificationPreferences(
	audience: NotificationAudience,
): Promise<PreferenceEntry[]> {
	const res = await notificationAppClient().getNotificationPreferences({
		audience,
	});
	return res.entries;
}

/** 開關一個（分類、渠道）；回傳伺服器確認後的那一列。 */
export async function updateNotificationPreference(params: {
	audience: NotificationAudience;
	category: NotificationCategory;
	channel: NotificationChannel;
	enabled: boolean;
}): Promise<PreferenceEntry | undefined> {
	const res =
		await notificationAppClient().updateNotificationPreference(params);
	return res.entry;
}
