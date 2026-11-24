import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { DeliveryFailureReason } from "@hawkr/api/gen/hawkr/admin/notification/v1alpha1/notification_pb";
import { NotificationChannel } from "@hawkr/api/gen/hawkr/notification/v1alpha1/notification_pb";
import {
	channelResultText,
	failureReasonText,
	UNREACHED_DAYS,
} from "../src/lib/unreached-notification";

test("每個失敗原因都有顯示文字，UNSPECIFIED 與不認得的值顯示未知", () => {
	for (const value of Object.values(DeliveryFailureReason)) {
		if (typeof value !== "number" || value === 0) continue;
		assert.notEqual(failureReasonText(value), "未知", String(value));
	}
	assert.equal(failureReasonText(DeliveryFailureReason.UNSPECIFIED), "未知");
	assert.equal(failureReasonText(999 as DeliveryFailureReason), "未知");
});

test("渠道結果：渠道名稱來自共用文案目錄，後接失敗原因", () => {
	const text = channelResultText(
		NotificationChannel.PUSH,
		DeliveryFailureReason.NO_DEVICE_TOKEN,
	);
	assert.ok(text.endsWith("：沒有可推播的裝置"), text);
	assert.ok(!text.startsWith("："), text);
});

test("往回看的天數在伺服器上限內", () => {
	assert.ok(UNREACHED_DAYS >= 1 && UNREACHED_DAYS <= 90);
});

const page = readFileSync(
	"src/app/admin/unreached-notifications/page.tsx",
	"utf8",
);
const sidebar = readFileSync("src/components/Layout/Sidebar.tsx", "utf8");

test("頁面走 ListUnreachedNotifications，帶固定天數與分頁，側欄有入口", () => {
	assert.match(
		page,
		/useQuery\(\s*NotificationAdminService\.method\.listUnreachedNotifications/,
	);
	assert.match(page, /days: UNREACHED_DAYS/);
	assert.match(page, /pageToken: currentToken\(pageTokens\)/);
	assert.match(sidebar, /href: "\/admin\/unreached-notifications"/);
});
