import { expect, test } from "@playwright/test";
import { setupGatewayMocks } from "./helpers/mock-gateway";

/**
 * Regression test for the "checkout pops an unrelated old notification"
 * bug: a buyer with a pre-existing unread notification that was never
 * toasted (e.g. an order-cancelled notice missed while offline) must not
 * see it resurface as a toast just because a genuinely new notification
 * arrives afterward. See `NotificationToastBridge.tsx`'s high-water-mark
 * comment for the mechanism this asserts.
 */

const OLD_ID = "10000000000000000001"; // pre-existing unread backlog item
const NEW_ID = "10000000000000000002"; // genuinely new item, arrives via SSE

const OLD_TITLE = "訂單已取消";
const NEW_TITLE = "訂單成立通知";

const OLD_ITEM = {
	id: OLD_ID,
	type: "order",
	title: OLD_TITLE,
	body: "您的訂單 #O392027160 已取消。",
	isRead: false,
	createdAt: "2026-07-21T16:01:51Z",
};

const NEW_ITEM = {
	id: NEW_ID,
	// Backend notification rows use the role recipient key here (buyer_id),
	// which is deliberately different from the signed-in accountId below.
	userId: 7001,
	userType: "buyer",
	type: "order",
	title: NEW_TITLE,
	body: "訂單成立！感謝您的訂購。",
	isRead: false,
	createdAt: "2026-07-22T13:32:58Z",
};

function sseFrame(eventName: string, data: unknown, id: string) {
	return `event: ${eventName}\ndata: ${JSON.stringify(data)}\nid: ${id}\n\n`;
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

	// GET /app/notifications — newest first, matching the real inbox order.
	//
	// `pageSize=1` is NotificationToastBridge's priming probe: it asks for the
	// single newest row so it can mark everything that already exists as
	// pre-dating this session. That request happens BEFORE the new notification
	// exists, so the honest answer to it is OLD_ITEM alone. Answering it with
	// NEW_ITEM (as this mock used to, for every pageSize alike, while its own
	// comment claimed otherwise) hands the bridge a high-water mark that already
	// covers the row the SSE is about to push — and whether the toast then
	// appears comes down to which of the two responses React sees first.
	await page.route("**/api/v1/app/notifications?**", (route) => {
		const request = route.request();
		if (request.method() !== "GET") {
			void route.fallback();
			return;
		}
		const priming = new URL(request.url()).searchParams.get("pageSize") === "1";
		const records = priming ? [OLD_ITEM] : [NEW_ITEM, OLD_ITEM];
		route.fulfill({
			status: 200,
			contentType: "application/json",
			body: JSON.stringify({
				code: 0,
				data: {
					records,
					total: records.length,
					pageSize: 10,
					hasMore: false,
				},
			}),
		});
	});

	// The baseline the bridge primes from. Deliberately 1 (only OLD_ITEM is
	// unread at this point) so the SSE frame's `unreadCount: 2` is a genuine
	// rise. The direct SSE path advances the fallback baseline, so this rise must
	// toast NEW_ITEM without issuing a redundant pageSize=10 inbox fetch.
	await page.route("**/api/v1/app/notifications/unread-count", (route) => {
		route.fulfill({
			status: 200,
			contentType: "application/json",
			body: JSON.stringify({ code: 0, data: { count: 1 } }),
		});
	});

	// "The client has finished priming" — as observed FROM THE CLIENT.
	//
	// NotificationToastBridge issues exactly one `pageSize=1` fetch, and only
	// after it has flipped `primedRef` and copied the settled unread count into
	// `prevCountRef`. Seeing that request on the wire is therefore a real
	// happens-after edge.
	//
	// Gate the first SSE frame after that request so the existing backlog cutoff
	// is deterministic. The new item must then use the direct event payload,
	// while the count-rise fallback stays silent rather than fetching the same
	// inbox again.
	const clientPrimed = page.waitForRequest(
		(req) => {
			if (req.method() !== "GET") return false;
			const url = new URL(req.url());
			return (
				url.pathname.endsWith("/app/notifications") &&
				url.searchParams.get("pageSize") === "1"
			);
		},
		{ timeout: 20_000 },
	);

	// First connection carries the genuinely new notification; reconnects carry
	// only `ready`. Replaying the event on every reconnect would hand the
	// assertions an unbounded number of chances to come true, which is exactly
	// how the ordering bug above stayed hidden.
	let streamConnections = 0;
	await page.route("**/api/v1/app/notifications/stream", async (route) => {
		const isFirst = streamConnections++ === 0;
		const ready = sseFrame(
			"ready",
			{
				topic: "notification:user:7001",
				serverTime: new Date().toISOString(),
				bufferMiss: false,
				replayed: 0,
			},
			"1",
		);
		if (isFirst) await clientPrimed;
		route.fulfill({
			status: 200,
			contentType: "text/event-stream",
			body: isFirst
				? ready +
					sseFrame("notification.new", { item: NEW_ITEM, unreadCount: 2 }, "2")
				: ready,
		});
	});

	let fallbackRefetches = 0;
	page.on("request", (request) => {
		const url = new URL(request.url());
		if (
			url.pathname.endsWith("/app/notifications") &&
			url.searchParams.get("pageSize") === "10"
		) {
			fallbackRefetches += 1;
		}
	});

	await page.goto("/account/notifications");

	// Scope to the toast viewport (`role="status"`) — the notifications LIST
	// on this same page legitimately renders both rows; only the toast popup
	// is what this bug is about.
	// 2026-09-10：通知卡不再各自是 role="status"（三張堆疊會變三個 live region，
	// 藏在後面的也會唸）；改由 <section aria-label="通知"> 內單一 sr-only 宣告。
	// 卡片本體以 class 定位。
	const toasts = page.locator(".hk-notification-toast");
	await expect(toasts.filter({ hasText: NEW_TITLE })).toBeVisible();
	// Give the count effect a render/animation tick. The direct event already
	// advanced its baseline, so it must remain silent instead of fetching the
	// same inbox again.
	await page.waitForTimeout(300);
	expect(fallbackRefetches).toBe(0);
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
