import assert from "node:assert/strict";
import { test } from "node:test";
import { resolveNotification } from "../lib/notifications/resolve";
import type { NotificationItem } from "../lib/notifications/types";

function item(overrides: Partial<NotificationItem>): NotificationItem {
	return {
		createdAt: "2026-06-20T08:00:00.000Z",
		id: 1,
		isRead: false,
		title: "新訂單",
		type: "order",
		...overrides,
	};
}

test("seller order notification resolves buyer copy and order link without actionUrl", () => {
	const resolved = resolveNotification(
		item({
			body: "你有一筆新訂單",
			data: {
				buyer_name: "測試買家",
				order_no: "HK20260620001",
			},
		}),
		"seller",
	);

	assert.equal(resolved.href, "/seller-center/orders?focus=HK20260620001");
	assert.equal(resolved.buyerName, "測試買家");
	assert.match(resolved.body ?? "", /買家：測試買家/);
});

test("buyer order notification resolves to buyer order detail", () => {
	const resolved = resolveNotification(
		item({
			data: {
				orderNo: "HK20260620002",
			},
		}),
		"buyer",
	);

	assert.equal(resolved.href, "/account/orders/HK20260620002");
});

test("notification actionUrl has priority over inferred order link", () => {
	const resolved = resolveNotification(
		item({
			actionUrl: "/seller-center/orders/custom",
			data: {
				order_no: "HK20260620003",
			},
		}),
		"seller",
	);

	assert.equal(resolved.href, "/seller-center/orders/custom");
});

test("notification actionUrl rejects protocol-relative and app deep links", () => {
	for (const actionUrl of [
		"//evil.example/phishing",
		"/\\evil.example/phishing",
		"http://evil.example/phishing",
		"hawkrbuyer://orders/1",
		"javascript:alert(1)",
	]) {
		const resolved = resolveNotification(
			item({ actionUrl, type: "announcement" }),
			"buyer",
		);
		assert.equal(resolved.href, undefined, actionUrl);
	}
});
