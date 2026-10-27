import { expect, test } from "@playwright/test";
import {
	seedStoredActiveRole,
	setupGatewayMocks,
} from "./helpers/mock-gateway";

const ACTIVE_PATH = "/account/messages/assistant/1002";
const CONVERSATIONS_API = "**/api/v1/app/buyers/me/procurement/conversations**";
const CATEGORY_FIXTURES = [
	{ id: "1001", category: "vegetables", title: "高麗菜配送" },
	{ id: "1002", category: "fruits", title: "水果配送" },
	{ id: "1003", category: "seafood", title: "海鮮配送" },
	{ id: "1004", category: "meat", title: "肉品配送" },
	{ id: "1005", category: "egg-dairy", title: "蛋奶配送" },
	{ id: "1006", category: "processed", title: "加工食品配送" },
	{ id: "1007", category: "spices", title: "香料配送" },
	{ id: "1008", category: "grains", title: "穀物配送" },
	{ id: "1009", category: "flowers", title: "花卉配送" },
] as const;

test.describe("Hawkr Agent 對話分類彈珠", { tag: ["@ui"] }, () => {
	test.skip(
		({ browserName, isMobile }) =>
			browserName !== "chromium" || Boolean(isMobile),
		"分類圖與側欄選中態只需在 desktop Chromium 驗一次",
	);

	test("九大分類欄位會決定各列彈珠，active row 沒有左側指示條", async ({
		page,
		baseURL,
	}) => {
		const appBaseURL = baseURL ?? "http://localhost:3000";
		await seedStoredActiveRole(page, "buyer");
		await setupGatewayMocks(page, {
			identity: {
				accountId: "300",
				roles: ["buyer"],
			},
			session: "identity",
		});
		await page
			.context()
			.addCookies([{ name: "hk_session", value: "opaque", url: appBaseURL }]);
		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});

		await page.route(CONVERSATIONS_API, (route) => {
			const pathname = new URL(route.request().url()).pathname;
			if (pathname.endsWith("/1002/messages")) {
				route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({
						code: 0,
						data: {
							conversationId: "1002",
							requestId: "",
							status: "CLARIFYING",
							title: "水果配送",
							records: [],
						},
					}),
				});
				return;
			}
			route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({
					code: 0,
					data: {
						records: CATEGORY_FIXTURES.map((fixture, index) => ({
							conversationId: fixture.id,
							title: fixture.title,
							lastMessagePreview: `${fixture.title}採購`,
							updatedAt: `2026-03-${String(20 - index).padStart(2, "0")}T10:00:00Z`,
							primaryCategoryId: fixture.category,
						})),
						hasMore: false,
						nextCursorId: null,
					},
				}),
			});
		});

		const response = await page.goto(ACTIVE_PATH, {
			waitUntil: "domcontentloaded",
		});
		expect(response?.status()).toBeLessThan(400);

		for (const expected of CATEGORY_FIXTURES) {
			const row = page.locator(
				`a.chat-sidebar__item[href="/account/messages/assistant/${expected.id}"]`,
			);
			await expect(row).toBeVisible();
			const marble = row.locator(".chat-sidebar__category-marble");
			await expect(marble).toHaveAttribute("data-category", expected.category);
			await expect(marble).toHaveAttribute("role", "img");
			await expect(marble).toHaveAttribute("aria-label", /^大分類：/);
			await expect(marble.locator("svg")).toBeVisible();
		}

		const activeRow = page.locator(
			'a.chat-sidebar__item[href="/account/messages/assistant/1002"]',
		);
		await expect(activeRow).toHaveAttribute("aria-current", "page");
		const activeStyle = await activeRow.evaluate((node) => {
			const style = getComputedStyle(node);
			return {
				outlineStyle: style.outlineStyle,
				outlineWidth: style.outlineWidth,
				indicatorContent: getComputedStyle(node, "::before").content,
			};
		});
		expect(activeStyle.outlineStyle).toBe("solid");
		expect(activeStyle.outlineWidth).toBe("1px");
		expect(activeStyle.indicatorContent).toBe("none");

		await page.setViewportSize({ width: 1024, height: 800 });
		await page.evaluate(() => {
			localStorage.setItem("hkui:chat:sidebar-collapsed", "1");
		});
		await page.reload({ waitUntil: "domcontentloaded" });
		await expect(page.locator(".chat-shell")).toHaveAttribute(
			"data-sidebar-collapsed",
			"true",
		);
		const collapsedActiveRow = page.locator(
			'a.chat-sidebar__item[href="/account/messages/assistant/1002"]',
		);
		await collapsedActiveRow.focus();
		const collapsedFocusStyle = await collapsedActiveRow.evaluate((node) => ({
			outlineOffset: getComputedStyle(node).outlineOffset,
			indicatorContent: getComputedStyle(node, "::before").content,
		}));
		expect(collapsedFocusStyle.outlineOffset).toBe("-2px");
		expect(collapsedFocusStyle.indicatorContent).toBe("none");
	});
});
