import { expect, test } from "@playwright/test";
import {
	seedStoredActiveRole,
	setupGatewayMocks,
} from "./helpers/mock-gateway";
import { mockProcurementUnary } from "./helpers/mock-procurement-connect";
import { gotoStreamed, reloadStreamed } from "./helpers/streamed-navigation";

const ACTIVE_PATH = "/account/messages/assistant/1002";
const CONVERSATION_FIXTURES = [
	{ id: "1001", title: "高麗菜配送" },
	{ id: "1002", title: "水果配送" },
	{ id: "1003", title: "海鮮配送" },
	{ id: "1004", title: "肉品配送" },
	{ id: "1005", title: "蛋奶配送" },
	{ id: "1006", title: "加工食品配送" },
	{ id: "1007", title: "香料配送" },
	{ id: "1008", title: "穀物配送" },
	{ id: "1009", title: "花卉配送" },
] as const;

test.describe("Hawkr Agent 對話側欄", { tag: ["@ui"] }, () => {
	test.skip(
		({ browserName, isMobile }) =>
			browserName !== "chromium" || Boolean(isMobile),
		"側欄選中態只需在 desktop Chromium 驗一次",
	);

	test("每個對話一列，active row 沒有左側指示條", async ({ page, baseURL }) => {
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

		await mockProcurementUnary(page, "GetConversationMessages", () => ({
			conversationId: "1002",
			requestId: "",
			status: "REQUEST_STATUS_CLARIFYING",
			title: "水果配送",
			messages: [],
		}));
		await mockProcurementUnary(page, "ListConversations", () => ({
			conversations: CONVERSATION_FIXTURES.map((fixture, index) => ({
				id: fixture.id,
				title: fixture.title,
				lastMessagePreview: `${fixture.title}採購`,
				updateTime: `2026-03-${String(20 - index).padStart(2, "0")}T10:00:00Z`,
			})),
			nextPageToken: "",
		}));

		const response = await gotoStreamed(page, ACTIVE_PATH);
		expect(response?.status()).toBeLessThan(400);

		for (const expected of CONVERSATION_FIXTURES) {
			const row = page.locator(
				`a.chat-sidebar__item[href="/account/messages/assistant/${expected.id}"]`,
			);
			await expect(row).toBeVisible();
			await expect(row).toContainText(expected.title);
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
		await reloadStreamed(page);
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
