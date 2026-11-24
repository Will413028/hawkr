import {
	NotificationCategory,
	NotificationChannel,
	type PreferenceEntry,
} from "@hawkr/api/gen/hawkr/notification/v1alpha1/notification_pb";
import { describe, expect, it } from "vitest";
import { uiTextFor } from "@/lib/i18n/ui-text";
import { buildNotificationFilters } from "./filters";

const entry = (
	category: NotificationCategory,
	channel: NotificationChannel,
): PreferenceEntry =>
	({ category, channel, enabled: true, canDisable: true }) as PreferenceEntry;

describe("buildNotificationFilters", () => {
	const ut = uiTextFor("zh-TW");

	it("全部之後依伺服器回傳的順序列出分類，同一分類的多個渠道只出現一次，名稱來自文案目錄", () => {
		const out = buildNotificationFilters(ut, [
			entry(NotificationCategory.ORDER, NotificationChannel.INBOX),
			entry(NotificationCategory.ORDER, NotificationChannel.PUSH),
			entry(NotificationCategory.SHIPMENT_ANOMALY, NotificationChannel.PUSH),
		]);
		expect(out).toEqual([
			{ value: 0, label: "全部" },
			{ value: NotificationCategory.ORDER, label: "訂單" },
			{ value: NotificationCategory.SHIPMENT_ANOMALY, label: "配送異常" },
		]);
	});

	it("沒有偏好項目時只有全部", () => {
		expect(buildNotificationFilters(ut, [])).toEqual([
			{ value: 0, label: "全部" },
		]);
	});
});
