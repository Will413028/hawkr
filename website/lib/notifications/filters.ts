import type {
	NotificationCategory,
	PreferenceEntry,
} from "@hawkr/api/gen/hawkr/notification/v1alpha1/notification_pb";
import { notificationNotificationCategory } from "@hawkr/ui-text/keys";
import { optionalText, type UiText } from "@/lib/i18n/ui-text";

export interface NotificationFilterOption {
	/** 0 表示「全部」。 */
	value: NotificationCategory | 0;
	label: string;
}

/**
 * 把偏好項目的分類（伺服器對這個受眾回的分類，依回傳順序、不重複）轉成篩選 chips 選項。
 * 第一個元素固定為「全部」；分類名稱來自共用文案目錄。
 */
export function buildNotificationFilters(
	ut: UiText,
	entries: PreferenceEntry[],
): NotificationFilterOption[] {
	const seen = new Set<NotificationCategory>();
	const options: NotificationFilterOption[] = [{ value: 0, label: "全部" }];
	for (const entry of entries) {
		if (seen.has(entry.category)) continue;
		seen.add(entry.category);
		const label = optionalText(
			ut,
			notificationNotificationCategory(entry.category),
		);
		if (label === undefined) continue;
		options.push({ value: entry.category, label });
	}
	return options;
}
