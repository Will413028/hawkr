import type { NotificationPreferenceItem } from "./types";

export interface NotificationFilterOption {
	value: string;
	label: string;
}

/**
 * 將偏好設定清單轉換成篩選 chips 選項。
 * 第一個元素固定為「全部」（value 為空字串）。
 * 若某項目缺少 title，以 category key 作為顯示文字。
 */
export function buildNotificationFilters(
	prefs: NotificationPreferenceItem[],
): NotificationFilterOption[] {
	return [
		{ value: "", label: "全部" },
		...prefs.map((p) => ({ value: p.category, label: p.title || p.category })),
	];
}
