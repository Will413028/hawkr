import { expect, test } from "@playwright/test";
import { fulfillConnect, setupGatewayMocks } from "./helpers/mock-gateway";

/**
 * Regression test for the "checkout pops an unrelated old notification"
 * bug: a buyer with a pre-existing unread notification that was never
 * toasted (e.g. an order-cancelled notice missed while offline) must not
 * see it resurface as a toast just because a genuinely new notification
 * arrives afterward: the bridge toasts only the ids the stream signal names.
 */

const OLD_ID = "inbox-old";
const NEW_ID = "inbox-new";

const OLD_TITLE = "訂單已取消";
const NEW_TITLE = "訂單成立通知";

function inboxItem(
	id: string,
	title: string,
	body: string,
	createTime: string,
) {
	return {
		id,
		category: "NOTIFICATION_CATEGORY_ORDER",
		presentation: "INBOX_PRESENTATION_ORDER",
		title,
		body,
		deepLink: "/account/orders",
		read: false,
		createTime,
	};
}

const OLD_ITEM = inboxItem(
	OLD_ID,
	OLD_TITLE,
	"您的訂單 #O392027160 已取消。",
	"2026-07-21T16:01:51Z",
);
const NEW_ITEM = inboxItem(
	NEW_ID,
	NEW_TITLE,
	"訂單成立！感謝您的訂購。",
	"2026-07-22T13:32:58Z",
);

const method = (name: string) =>
	new RegExp(
		`/api/connect/hawkr\\.notification\\.v1alpha1\\.NotificationAppService/${name}(?:\\?|$)`,
	);

function envelope(flags: number, json: unknown): Buffer {
	const payload = Buffer.from(JSON.stringify(json));
	const frame = Buffer.alloc(5 + payload.length);
	frame.writeUInt8(flags, 0);
	frame.writeUInt32BE(payload.length, 1);
	payload.copy(frame, 5);
	return frame;
}

test("stale unread backlog never resurfaces as a toast alongside a genuinely new notification", {
	tag: ["@ui"],
}, async ({ page, baseURL }) => {
	await page
		.context()
		.addCookies([{ name: "hk_session", value: "opaque", url: baseURL }]);

	await page.route("**/api/v1/**", (route) => {
		route.fulfill({
			status: 200,
			contentType: "application/json",
			body: JSON.stringify({ code: 0, data: {} }),
		});
	});

	await setupGatewayMocks(page, {
		session: "identity",
		identity: { accountId: "1", roles: ["buyer"] },
	});

	await page.route(method("ListInboxItems"), (route) => {
		void fulfillConnect(route, {
			items: [NEW_ITEM, OLD_ITEM],
			nextPageToken: "",
			totalSize: 2,
		});
	});

	await page.route(method("GetUnreadCount"), (route) => {
		void fulfillConnect(route, { unreadCount: 2 });
	});

	let streamConnections = 0;
	await page.route(method("WatchNotifications"), async (route) => {
		if (streamConnections++ > 0) return;
		void route.fulfill({
			status: 200,
			contentType: "application/connect+json",
			body: Buffer.concat([
				envelope(0x00, {
					inbox: {
						unreadCount: 2,
						latestInboxItemId: NEW_ID,
						newInboxItemIds: [NEW_ID],
					},
				}),
				envelope(0x02, {}),
			]),
		});
	});

	await page.goto("/account/notifications");

	const toasts = page.locator(".hk-notification-toast");
	await expect(toasts.filter({ hasText: NEW_TITLE })).toBeVisible();
	await expect(toasts.filter({ hasText: OLD_TITLE })).toHaveCount(0, {
		timeout: 2000,
	});
});
