import { expect, test } from "@playwright/test";
import { HAWKR_AGENT_HANDOFF_STORAGE_KEY } from "../../features/procurement-assistant/handoff";
import {
	seedStoredActiveRole,
	setupGatewayMocks,
} from "./helpers/mock-gateway";

/**
 * Legacy Hawkr Agent 入口的退場契約（設計書 v6 §V.10.12-4）。
 *
 * 這支 spec 以前驗的是 `/assistant/new` 上的 seller-match workspace：SSE 串流、
 * 公開候選卡與 contact 隱私哨兵。v6 把唯一正式新對話入口凍結為
 * `/assistant/procurement`，`/assistant/new` 與 `/account/find-sellers` 一律
 * redirect，那個表面已經沒有任何路由掛載得到——所以它的串流斷言不再有可執行的
 * 對象，隨路由一起退場（`HawkrAgentChatWorkspace` 的實作依 §V.14.4-2 先留著，
 * 等 access metric 證明零流量後才移除）。
 *
 * 保留下來的是**退場本身**與那條仍然活著的隱私要求：搜尋草稿只走 sessionStorage
 * handoff，不進 URL，落地後不自動送出、並從 storage 清掉。
 */
const LEGACY_LIVE_PATH = "/account/messages/assistant/new";
const LEGACY_FIND_SELLERS_PATH = "/account/find-sellers";
const PROCUREMENT_PATH = "/account/messages/assistant/procurement";
const SEARCH_DRAFT =
	"我要採購水果禮盒 50 箱，希望在三週後送到桃園市，每箱預算 800 元，幫我把條件整理成完整的採購需求。";

test.describe("Hawkr Agent legacy 入口退場", { tag: ["@ui"] }, () => {
	test.skip(
		({ browserName }) => browserName !== "chromium",
		"redirect 與 entry state 的契約在 desktop 與 mobile Chromium 執行",
	);

	test("legacy 入口 redirect 到唯一正式入口，搜尋草稿落在中央 composer 且不自動送出", async ({
		page,
		baseURL,
		isMobile,
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

		await page.addInitScript(
			([storageKey, query]) => {
				sessionStorage.setItem("hawkr:initial-loader-seen", "1");
				sessionStorage.setItem(
					storageKey,
					JSON.stringify({
						version: 1,
						id: "handoff-e2e",
						query,
						source: "home-search",
						createdAt: Date.now(),
					}),
				);
			},
			[HAWKR_AGENT_HANDOFF_STORAGE_KEY, SEARCH_DRAFT] as const,
		);

		const response = await page.goto(LEGACY_LIVE_PATH, {
			waitUntil: "domcontentloaded",
		});
		expect(response?.status()).toBeLessThan(400);
		// §V.10.12-4：不得再掛 legacy workspace，必須落在 procurement。
		expect(new URL(page.url()).pathname).toBe(PROCUREMENT_PATH);
		// 搜尋內容永遠不進 URL（redirect 也不得把它帶成 query）。
		expect(new URL(page.url()).search).toBe("");
		expect(page.url()).not.toContain(encodeURIComponent(SEARCH_DRAFT));

		const workspace = page.locator('[data-assistant-mode="live"]');
		await expect(workspace).toBeVisible();
		// 無既有 conversation → entry state（§V.11.0 Screen Contract 00）。
		await expect(workspace).toHaveAttribute("data-assistant-view", "entry");
		await expect(
			page.getByRole("heading", { name: "今天想請 Hawkr 幫你完成什麼？" }),
		).toBeVisible();
		await expect(page.getByRole("heading", { level: 1 })).toHaveCount(1);
		const heroMark = page.locator(".hawkr-agent-entry__sprite");
		await expect(heroMark).toBeVisible();
		await expect(heroMark).toHaveAttribute("aria-hidden", "true");
		await expect(heroMark.locator("img")).toBeVisible();
		for (const capability of [
			{ id: "find-suppliers", label: "找供應商" },
			{ id: "organise-requirements", label: "整理採購需求" },
			{ id: "compare-candidates", label: "比較候選" },
		]) {
			const button = page.locator(
				`button[data-capability-id="${capability.id}"]`,
			);
			await expect(button).toBeVisible();
			await expect(button).toContainText(capability.label);
			const icon = button.locator(`[data-capability-icon="${capability.id}"]`);
			await expect(icon).toHaveAttribute("aria-hidden", "true");
			await expect(icon.locator("svg")).toBeVisible();
			const box = await button.boundingBox();
			expect(box?.width ?? 0).toBeGreaterThanOrEqual(44);
			expect(box?.height ?? 0).toBeGreaterThanOrEqual(44);
		}
		// 草稿帶進唯一那個 composer，且沒有被自動送出。
		const composer = page.locator("#procurement-clarify-input");
		await expect(composer).toHaveValue(SEARCH_DRAFT);
		await expect(page.locator("#procurement-clarify-input")).toHaveCount(1);
		expect(
			await page.evaluate(
				(key) => sessionStorage.getItem(key),
				HAWKR_AGENT_HANDOFF_STORAGE_KEY,
			),
		).toBeNull();

		// 送出鍵的觸控目標維持 44×44（§V.11.0 Composer）。
		const sendButton = page.getByRole("button", { name: "送出" });
		const sendBox = await sendButton.boundingBox();
		expect(sendBox?.width ?? 0).toBeGreaterThanOrEqual(44);
		expect(sendBox?.height ?? 0).toBeGreaterThanOrEqual(44);
		await expect(sendButton.locator('[data-icon-name="send"]')).toBeVisible();
		await expect(
			page.locator(".hawkr-agent-entry__composer .thread-page__compose"),
		).toHaveCSS("border-top-width", "0px");
		await expect(composer).toHaveAttribute("rows", "1");
		await expect(composer).toHaveAttribute("wrap", "soft");
		await expect(composer).toHaveCSS("white-space", "pre-wrap");
		if (!isMobile) {
			// 中央 composer 仍吃滿內容欄，可借 12px × 2 gutter，並由 960px 封頂。
			const desktopLayout = await composer.evaluate((element) => {
				const shell = element.closest<HTMLElement>(
					".hawkr-agent-entry__composer",
				);
				const column = element.closest<HTMLElement>(".hawkr-agent-entry");
				if (!shell || !column)
					throw new Error("Entry composer layout is missing");
				const columnStyle = getComputedStyle(column);
				return {
					shellWidth: shell.getBoundingClientRect().width,
					shellMaxWidth: Number.parseFloat(getComputedStyle(shell).maxWidth),
					columnContentWidth:
						column.getBoundingClientRect().width -
						Number.parseFloat(columnStyle.paddingLeft) -
						Number.parseFloat(columnStyle.paddingRight),
				};
			});
			expect(desktopLayout.shellMaxWidth).toBe(960);
			expect(desktopLayout.shellWidth).toBeGreaterThanOrEqual(
				desktopLayout.columnContentWidth,
			);
			expect(desktopLayout.shellWidth).toBeLessThanOrEqual(
				Math.min(960, desktopLayout.columnContentWidth + 24) + 0.5,
			);
		}

		// 現行 composer 支援多行：依內容由 44px 增高至 120px，超過後在欄內
		// 垂直捲動。驗版面邊界與可捲動性，不依賴 macOS/Linux 的字體折行數。
		const longDraft = ["採購需求".repeat(60), "配送與品項".repeat(60)].join(
			"\n",
		);
		const widths = isMobile
			? [320, 390, 430]
			: [page.viewportSize()?.width ?? 1280];
		for (const width of widths) {
			await test.step(`${width}px：多行草稿增高後在框內垂直捲動`, async () => {
				if (isMobile) await page.setViewportSize({ width, height: 851 });
				await composer.fill("採購需求");
				const shortHeight = await composer.evaluate(
					(input) => input.getBoundingClientRect().height,
				);
				expect(shortHeight).toBeGreaterThanOrEqual(44);
				expect(shortHeight).toBeLessThanOrEqual(120);
				await composer.fill(longDraft);
				await expect(composer).toHaveValue(longDraft);
				await expect
					.poll(() =>
						composer.evaluate((input) => input.getBoundingClientRect().height),
					)
					.toBeGreaterThan(shortHeight);
				const geometry = await composer.evaluate((element) => {
					const input = element as HTMLTextAreaElement;
					const send = input
						.closest(".thread-page__field")
						?.querySelector<HTMLButtonElement>(".thread-page__send");
					const inputRect = input.getBoundingClientRect();
					const sendRect = send?.getBoundingClientRect();
					const style = getComputedStyle(input);
					// 主動捲到底，證明長文能讀到尾端，不只把超出高度的內容裁掉。
					input.scrollTop = input.scrollHeight;
					return {
						whiteSpace: style.whiteSpace,
						overflowY: style.overflowY,
						fontSize: Number.parseFloat(style.fontSize),
						documentWidth: Math.max(
							document.documentElement.scrollWidth,
							document.body.scrollWidth,
						),
						viewportWidth: document.documentElement.clientWidth,
						input: {
							right: inputRect.right,
							height: inputRect.height,
							scrollTop: input.scrollTop,
							scrollWidth: input.scrollWidth,
							clientWidth: input.clientWidth,
							scrollHeight: input.scrollHeight,
							clientHeight: input.clientHeight,
						},
						send: sendRect
							? {
									left: sendRect.left,
									width: sendRect.width,
									height: sendRect.height,
								}
							: null,
					};
				});
				expect(geometry.whiteSpace).toBe("pre-wrap");
				expect(geometry.overflowY).toBe("auto");
				expect(geometry.fontSize).toBeGreaterThanOrEqual(16);
				expect(geometry.input.height).toBeGreaterThanOrEqual(44);
				expect(geometry.input.height).toBeLessThanOrEqual(120);
				expect(geometry.input.scrollHeight).toBeGreaterThan(
					geometry.input.clientHeight,
				);
				expect(geometry.input.scrollTop).toBeGreaterThan(0);
				expect(geometry.input.scrollWidth).toBeLessThanOrEqual(
					geometry.input.clientWidth + 1,
				);
				expect(geometry.documentWidth).toBeLessThanOrEqual(
					geometry.viewportWidth + 1,
				);
				expect(geometry.send).not.toBeNull();
				expect(geometry.input.right).toBeLessThanOrEqual(
					(geometry.send?.left ?? 0) + 0.5,
				);
				expect(geometry.send?.width ?? 0).toBeGreaterThanOrEqual(44);
				expect(geometry.send?.height ?? 0).toBeGreaterThanOrEqual(44);
			});
		}
		await composer.fill(SEARCH_DRAFT);
		await expect(composer).toHaveValue(SEARCH_DRAFT);
		await expect(workspace).toHaveAttribute("data-assistant-view", "entry");
		expect(new URL(page.url()).search).toBe("");
	});

	test("find-sellers 也直接 redirect 到 procurement，不繞經 legacy 路徑", async ({
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

		const response = await page.goto(LEGACY_FIND_SELLERS_PATH, {
			waitUntil: "domcontentloaded",
		});
		expect(response?.status()).toBeLessThan(400);
		expect(new URL(page.url()).pathname).toBe(PROCUREMENT_PATH);
		expect(new URL(page.url()).search).toBe("");
	});
});
