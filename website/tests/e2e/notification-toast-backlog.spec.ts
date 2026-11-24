import { expect, test } from "@playwright/test";
import { fulfillConnect, setupGatewayMocks } from "./helpers/mock-gateway";

/**
 * Regression test for the "checkout pops an unrelated old notification"
 * bug: a buyer with a pre-existing unread notification that was never
 * toasted (e.g. an order-cancelled notice missed while offline) must not
 * see it resurface as a toast just because a genuinely new notification
 * arrives afterward. See `NotificationToastBridge.tsx`'s seen-set comment
 * for the mechanism this asserts.
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
	// Middleware gates protected routes on the cookie's mere presence before
	// any client-side mock applies — same reason `protected-static-routes.spec.ts`
	// sets this alongside the gateway session mock below.
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

	// Registered after setupGatewayMocks so these win over its empty defaults.
	//
	// ListInboxItems — newest first, matching the real inbox order. The first
	// request is NotificationToastBridge's baseline read: it happens BEFORE the
	// new notification exists, so the honest answer is OLD_ITEM alone. Every
	// later request (the one the inbox signal triggers) also holds NEW_ITEM.
	let inboxReads = 0;
	await page.route(method("ListInboxItems"), (route) => {
		const items = inboxReads++ === 0 ? [OLD_ITEM] : [NEW_ITEM, OLD_ITEM];
		void fulfillConnect(route, {
			items,
			nextPageToken: "",
			totalSize: items.length,
		});
	});

	// The baseline the bridge primes from: only OLD_ITEM is unread at this point.
	await page.route(method("GetUnreadCount"), (route) => {
		void fulfillConnect(route, { unreadCount: 1 });
	});

	// "The client has finished priming" — as observed FROM THE CLIENT. The
	// bridge reads the baseline page only after it has flipped its primed flag,
	// so seeing that request on the wire is a real happens-after edge. The first
	// stream frame is held until then so the backlog cutoff is deterministic.
	const clientPrimed = page.waitForRequest(
		(req) =>
			req.method() === "POST" && method("ListInboxItems").test(req.url()),
		{ timeout: 20_000 },
	);

	// First connection carries the signal for the genuinely new notification and
	// then ends normally; reconnects get no answer. Replaying the signal on every
	// reconnect would hand the assertions an unbounded number of chances to come
	// true, which is exactly how an ordering bug could stay hidden.
	let streamConnections = 0;
	await page.route(method("WatchNotifications"), async (route) => {
		if (streamConnections++ > 0) return;
		await clientPrimed;
		void route.fulfill({
			status: 200,
			contentType: "application/connect+json",
			body: Buffer.concat([
				envelope(0x00, {
					inbox: { unreadCount: 2, latestInboxItemId: NEW_ID },
				}),
				envelope(0x02, {}),
			]),
		});
	});

	await page.goto("/account/notifications");

	// Scope to the toast cards — the notifications LIST on this same page
	// legitimately renders both rows; only the toast popup is what this bug is
	// about.
	const toasts = page.locator(".hk-notification-toast");
	await expect(toasts.filter({ hasText: NEW_TITLE })).toBeVisible();
	// A short, explicit timeout — well under the toast's own 5s auto-dismiss
	// (see NotificationToast.tsx's DEFAULT_DURATION). Without capping this,
	// `toHaveCount(0)`'s eventually-true polling would pass regardless of the
	// bug: every toast disappears on its own within 5s, so waiting out to
	// Playwright's default 15s timeout can't tell "never appeared" from
	// "appeared, then auto-dismissed."
	await expect(toasts.filter({ hasText: OLD_TITLE })).toHaveCount(0, {
		timeout: 2000,
	});
});
