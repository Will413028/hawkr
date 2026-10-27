import { expect, type Page, test } from "@playwright/test";
import {
	seedStoredActiveRole,
	setupGatewayMocks,
} from "./helpers/mock-gateway";

/**
 * 側欄清單底部的計數（設計稿 01／03／04 的「顯示 1 - 6 / 6」）。
 *
 * 數字規則的單元測試在 `lib/chat/__tests__/list-range.unit.test.ts`，接線的源碼
 * 斷言在 `tests/conversation-sidebar-range.test.ts`。這一支補的是那兩層都證明
 * 不了的事：這一列真的畫進側欄、而且捲動清單時它留在原地。
 */
const PEERS = [
	"梨山盛花園",
	"綠野有機農場",
	"晨光牧場",
	"和風食材行",
	"南島漁產",
	"清晨麵包坊",
];

async function setupInbox(page: Page): Promise<void> {
	await page.route("**/api/v1/**", (route) =>
		route.fulfill({
			status: 200,
			contentType: "application/json",
			body: JSON.stringify({ code: 0, data: {} }),
		}),
	);
	await seedStoredActiveRole(page, "buyer");
	await setupGatewayMocks(page, {
		identity: { accountId: "300", roles: ["buyer"] },
		session: "identity",
	});
	await page.context().addCookies([
		{
			name: "hk_session",
			value: "opaque",
			url: `http://localhost:${process.env.PLAYWRIGHT_PORT || "3000"}`,
		},
	]);
	await page.route("**/chat/conversations*", (route) =>
		route.fulfill({
			status: 200,
			contentType: "application/json",
			body: JSON.stringify({
				code: 0,
				data: {
					records: PEERS.map((nickname, index) => ({
						targetUserId: 1000 + index,
						targetNickname: nickname,
						lastMessage: "感謝您的詢問，最新報價已更新。",
						lastMessageTime: Date.now() - index * 3_600_000,
						unreadCount: 0,
					})),
				},
			}),
		}),
	);
	await page.route("**/chat/unread*", (route) =>
		route.fulfill({
			status: 200,
			contentType: "application/json",
			body: JSON.stringify({ code: 0, data: { unread: 0 } }),
		}),
	);
	await page.goto("/account/messages");
}

test.describe("訊息側欄 — 清單計數", { tag: ["@ui"] }, () => {
	test("載入 6 筆對話時底部顯示「顯示 1 - 6 / 6」，且不跟著清單捲走", async ({
		page,
	}) => {
		test.setTimeout(45_000);
		await setupInbox(page);

		const range = page.locator(".chat-sidebar__range");
		await expect(range).toBeVisible({ timeout: 15_000 });
		await expect(range).toHaveText("顯示 1 - 6 / 6");

		// 它住在可捲動面板外面，所以把清單捲到底之後仍在原位。單元測試看得到
		// DOM 順序，看不到「捲動之後還在不在畫面上」。
		const before = await range.boundingBox();
		await page.locator(".chat-sidebar__panel").evaluate((element) => {
			element.scrollTop = element.scrollHeight;
		});
		await expect(range).toBeVisible();
		const after = await range.boundingBox();
		expect(after?.y).toBe(before?.y);
	});

	test("搜尋縮小清單時分子跟著看得到的列走，分母維持載入了幾筆", async ({
		page,
	}) => {
		test.setTimeout(45_000);
		await setupInbox(page);

		const range = page.locator(".chat-sidebar__range");
		await expect(range).toHaveText("顯示 1 - 6 / 6", { timeout: 15_000 });

		await page.getByRole("searchbox").fill("農場");
		await expect(range).toHaveText("顯示 1 - 1 / 6");

		// 一列都搜不到時整行消失——空狀態已經在講話。
		await page.getByRole("searchbox").fill("不存在的商家");
		await expect(range).toHaveCount(0);
	});
});
