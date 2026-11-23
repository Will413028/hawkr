import { expect, test } from "@playwright/test";
import {
	fulfillConnect,
	mockChatConversations,
	mockMyKyc,
	mockMyStore,
	seedStoredActiveRole,
	setupGatewayMocks,
} from "./e2e/helpers/mock-gateway";

const BUYER_ID = "300";
const SELLER_ID = "200";
const BUYER_NAME = "抽屜測試買家";

const crmMethod = (method: string) =>
	new RegExp(
		`/api/connect/hawkr\\.crm\\.v1alpha1\\.CrmAppService/${method}(?:\\?|$)`,
	);

const json = (data: unknown) => ({
	status: 200,
	contentType: "application/json",
	body: JSON.stringify({ code: 0, data }),
});

for (const failFirst of [false, true]) {
	test(`global chat drawer ${failFirst ? "recovers from chunk failure with keyboard focus" : "shows a cold shell, restores focus, and reopens in place"}`, {
		tag: ["@ui"],
	}, async ({ page, isMobile }) => {
		// Lowest-priority catch-all makes the surface independent from the shared
		// gateway. Specific routes below are registered later and win in Playwright.
		await page.route("**/api/v1/**", (route) => route.fulfill(json({})));
		await seedStoredActiveRole(page, "seller");
		await setupGatewayMocks(page, {
			identity: { accountId: SELLER_ID, roles: ["seller"] },
			session: "identity",
		});
		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});

		await mockMyStore(page, { id: SELLER_ID, storeName: "抽屜測試商店" });
		await mockMyKyc(page, { status: "APPROVED" });
		await page.route(crmMethod("ListCustomers"), (route) =>
			fulfillConnect(route, { customers: [], nextPageToken: "" }),
		);
		await page.route(crmMethod("ListTags"), (route) =>
			fulfillConnect(route, { tags: [] }),
		);
		await page.route("**/api/v1/app/orders?**", (route) =>
			route.fulfill(json({ records: [], total: 0 })),
		);
		await mockChatConversations(page, "store", [
			{
				id: "conv-drawer-1",
				peerId: BUYER_ID,
				peerName: BUYER_NAME,
				lastText: "請問今天可以出貨嗎？",
			},
		]);

		await page.goto("/seller-center/messages");
		const desktopTrigger = page
			.getByRole("button", { name: `開啟與 ${BUYER_NAME} 的聊天室` })
			.first();
		let trigger = desktopTrigger;
		if (isMobile) {
			await page
				.getByRole("button", { name: `與 ${BUYER_NAME} 的對話` })
				.click();
			trigger = page.getByRole("button", { name: "開啟聊天室" }).last();
		}
		await expect(trigger).toBeVisible();

		let releaseThreadChunk = () => {};
		let threadChunkHeld = false;
		let chunkAvailable = !failFirst;
		const threadChunkGate = new Promise<void>((resolve) => {
			releaseThreadChunk = resolve;
		});
		// Install after the workspace is ready. Only the post-click ThreadClient
		// chunk is held; matching source markers work for both dev and minified CI
		// builds without coupling the test to a content-hashed filename.
		await page.route(
			/\/_next\/static\/chunks\/.*\.js(?:\?.*)?$/,
			async (route) => {
				const response = await route.fetch();
				const body = await response.body();
				const source = body.toString("utf8");
				if (
					source.includes("message_thread") &&
					source.includes("subscribeToSocket")
				) {
					threadChunkHeld = true;
					if (!chunkAvailable) {
						await route.abort("failed");
						return;
					}
					await threadChunkGate;
				}
				await route.fulfill({ response, body });
			},
		);

		const locationBefore = await page.evaluate(() => ({
			historyLength: window.history.length,
			path: `${window.location.pathname}${window.location.search}`,
		}));

		try {
			if (failFirst) {
				// Exercise local recovery after the existing one-shot deploy repair.
				await page.evaluate(() =>
					sessionStorage.setItem("__chunk_reload_at", String(Date.now())),
				);
			}
			await trigger.click();
			const drawer = page.getByRole("dialog");
			await expect(drawer).toBeVisible();
			await expect(drawer.getByRole("status")).toContainText("載入聊天室中");
			await expect.poll(() => threadChunkHeld).toBe(true);
			if (failFirst) {
				const retry = drawer.getByRole("button", {
					name: "重新載入",
					exact: true,
				});
				await expect(retry).toBeVisible({ timeout: 8_000 });
				chunkAvailable = true;
				await retry.focus();
				await retry.press("Enter");
				await expect(drawer).toBeFocused();
				await expect(drawer.getByRole("status")).toContainText("載入聊天室中");
			}

			releaseThreadChunk();
			await expect(
				drawer.getByRole("region", { name: "對話內容" }),
			).toBeVisible();
			await expect(
				drawer.getByText(BUYER_NAME, { exact: true }).first(),
			).toBeVisible();
			await expect(drawer).toHaveCount(1);
			if (failFirst) await expect(drawer).toBeFocused();

			await drawer.getByRole("button", { name: "關閉聊天室" }).click();
			await expect(drawer).toHaveCount(0);
			await expect(trigger).toBeFocused();
			expect(
				await page.evaluate(() => ({
					historyLength: window.history.length,
					path: `${window.location.pathname}${window.location.search}`,
				})),
			).toEqual(locationBefore);

			await trigger.click();
			await expect(page.getByRole("dialog")).toHaveCount(1);
			await expect(
				page.getByRole("dialog").getByRole("region", { name: "對話內容" }),
			).toBeVisible();
			await expect(
				page.getByRole("dialog").getByText("載入聊天室中…", { exact: true }),
			).toHaveCount(0);
			expect(
				await page.evaluate(() => ({
					historyLength: window.history.length,
					path: `${window.location.pathname}${window.location.search}`,
				})),
			).toEqual(locationBefore);
		} finally {
			releaseThreadChunk();
		}
	});
}
