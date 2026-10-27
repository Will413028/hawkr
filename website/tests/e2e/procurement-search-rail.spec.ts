import { expect, type Page, test } from "@playwright/test";
import {
	RFQ_REQUEST_ID,
	RFQ_RUN_ID,
	setupRfqPriceReview,
} from "./helpers/mock-procurement-rfq";

/** 三列 activity 加上一筆重複 heartbeat，形狀取自 gateway golden fixture
 * custom-activity-candidate-search-rail.sse；`id:` 行是 GET replay 專屬的
 * ledger-seq cursor。中間那個 `: ping` 是 gateway 每 20 秒無條件寫的 keepalive
 * （`ai_run_events_stream.go` 的 sseKeepaliveInterval）：任何超過 20 秒的搜尋
 * 都會收到，它一旦被當成事件分派就會讓整條訂閱在 20 秒死掉，所以第三列收不
 * 收得到就是這條線活著的證據。 */
const RAIL_SSE = [
	'id: 1\ndata: {"type":"CUSTOM","name":"activity","value":{"attempt":1,"code":"SEARCH_CANDIDATES","label":"搜尋候選供應商","maxAttempts":1,"retryGroupKey":"9001","rowId":"9001","status":"RUNNING","text":""}}\n\n',
	'id: 2\ndata: {"type":"CUSTOM","name":"activity","value":{"attempt":1,"code":"SEARCH_PLATFORM","count":"12","label":"翻平台上的賣家","maxAttempts":1,"parentRowId":"9001","retryGroupKey":"9002","rowId":"9002","status":"SUCCEEDED","text":""}}\n\n',
	": ping\n\n",
	'id: 3\ndata: {"type":"CUSTOM","name":"activity","value":{"attempt":1,"code":"SEARCH_PLATFORM","count":"12","label":"翻平台上的賣家","maxAttempts":1,"parentRowId":"9001","retryGroupKey":"9002","rowId":"9002","status":"SUCCEEDED","text":""}}\n\n',
	'id: 4\ndata: {"type":"CUSTOM","name":"activity","value":{"attempt":1,"code":"SUMMARIZE","label":"整理成給你的建議","maxAttempts":1,"retryGroupKey":"9004","rowId":"9004","status":"PENDING","text":""}}\n\n',
].join("");

/** 一直回 RUNNING，讓 searching 階段停得夠久看得到軌。 */
function runningPoll() {
	return {
		status: 200,
		contentType: "application/json",
		body: JSON.stringify({
			code: 0,
			data: {
				requestId: "1234567890123456788",
				runId: "1234567890123456800",
				candidateSetId: "candidate-set-e2e",
				status: "RUNNING",
				platformLaneStatus: "PENDING",
				externalLaneStatus: "PENDING",
				failureReason: "",
				candidates: { platform: [], external: [] },
			},
		}),
	};
}

async function settleFiniteAnimations(page: Page) {
	await page.evaluate(async () => {
		await Promise.all(
			document
				.getAnimations()
				.filter(
					(animation) => animation.effect?.getTiming().iterations !== Infinity,
				)
				.map((animation) => animation.finished.catch(() => undefined)),
		);
	});
}

test.describe("procurement assistant — 候選搜尋步驟軌", {
	tag: ["@ui"],
}, () => {
	test("searching 不開成果面板就能看到主對話的公開步驟；heartbeat 不重複，已完成列保留", async ({
		page,
		isMobile,
	}, testInfo) => {
		test.setTimeout(45_000);
		await page.setViewportSize(
			isMobile ? { width: 390, height: 844 } : { width: 1440, height: 1000 },
		);
		await setupRfqPriceReview(page, {
			stopAtSearching: true,
			candidateSearchPollHandler: (route) => route.fulfill(runningPoll()),
			runEventsHandler: (route) =>
				route.fulfill({
					status: 200,
					contentType: "text/event-stream",
					body: RAIL_SSE,
				}),
		});

		// 完整處理區已在主對話，不另留同一狀態的單行泡泡。
		await expect(page.locator("li.hawkr-agent-typing-row")).toHaveCount(0);

		const processing = page.getByTestId("agent-processing");
		await expect(processing).toBeVisible();
		await expect(page.locator("#hawkr-agent-side-panel")).toHaveCount(0);
		await expect(page.getByTestId("agent-panel-open")).toHaveCount(0);
		const rail = processing.getByTestId("agent-activity-rail");
		await expect(rail).toBeVisible({ timeout: 15_000 });
		expect(
			await processing.evaluate(
				(node) =>
					node.closest(".thread-page") !== null &&
					node.closest(".thread-page__bubble") === null,
			),
		).toBe(true);
		await expect(
			page
				.getByRole("log", { name: "Hawkr Agent 採購經理紀錄" })
				.getByText("我要採購 50 KG 珍珠芭樂", { exact: true }),
		).toBeVisible();
		await expect(page.locator("#procurement-clarify-input")).toBeVisible();
		await expect(page.locator("#procurement-clarify-input")).toBeDisabled();
		await expect(
			processing.getByRole("heading", { name: "搜尋進度" }),
		).toBeVisible();
		await expect(rail.getByText("搜尋候選供應商")).toBeVisible();
		await expect(rail.getByText("翻平台上的賣家")).toBeVisible();
		await expect(rail.getByText("12 家")).toBeVisible();
		await expect(rail.getByText("整理成給你的建議")).toBeVisible();
		await expect(rail.getByText("等待中")).toBeVisible();
		await expect(rail.locator(".hawkr-agent-rail__row")).toHaveCount(3);
		await expect(rail.locator('[data-row-id="9002"]')).toHaveAttribute(
			"data-status",
			"SUCCEEDED",
		);
		// 舊的兩列 lane 卡片被子列取代，不並存。
		await expect(page.getByText("正在搜尋候選")).toHaveCount(0);
		// lane 子列真的掛在父列底下（一層巢狀），不是攤平的四列。
		await expect(
			rail.locator('.hawkr-agent-rail__children [data-row-id="9002"]'),
		).toHaveCount(1);

		// a11y：軌本身 aria-hidden，唯一的朗讀來源是常駐的 sr-only atomic polite
		// status。它要播報「現在真的在跑」的那一列（父列「搜尋候選供應商」），
		// 不是還沒跑到的收尾列，也不是空字串——單元測試只驗過抽出這句話的
		// helper（lastRunningActivity／activityLabel），沒驗過這條線真的接到
		// AssistantWorkspace 的 DOM 上。
		//
		// 桌機與手機都直接在對話裡看見進度，也都保留唯一的朗讀來源。
		const announcement = page.locator(
			'.sr-only[role="status"][aria-atomic="true"]',
		);
		// toBeVisible 才是這件事的斷言：`.sr-only` 是 1px＋clip，Playwright 仍
		// 視為可見，但被 `display:none` 的祖先蓋掉就不是——而 toHaveText 讀得到
		// 隱藏節點的文字，單靠它抓不出「窄視窗整段被藏起來」。
		await expect(announcement).toBeVisible();
		await expect(announcement).toHaveText("搜尋候選供應商");
		await expect(page.locator(".initial-loader")).toBeHidden();
		await settleFiniteAnimations(page);
		const screenshot = testInfo.outputPath("processing.png");
		await page.screenshot({ path: screenshot, fullPage: !isMobile });
		await testInfo.attach("processing", {
			path: screenshot,
			contentType: "image/png",
		});
	});

	test("端點回 503 時退回既有的兩列 lane 卡片", async ({ page }) => {
		test.setTimeout(45_000);
		await setupRfqPriceReview(page, {
			stopAtSearching: true,
			candidateSearchPollHandler: (route) => route.fulfill(runningPoll()),
			// runEventsHandler 不傳 → helper 預設 503，等同今天的 prod。
		});

		const processing = page.getByTestId("agent-processing");
		await expect(processing).toBeVisible();
		await expect(page.locator("#hawkr-agent-side-panel")).toHaveCount(0);

		// exact: true——兩個 label 都撞到 `CandidateSearchProgress` 自己的說明
		// 文字：sr-only 的常駐播報退回「正在搜尋候選供應商」（「正在搜尋候選」的
		// 超字串），「平台賣家」則是「平台賣家與外部名單分開查詢…」那句的子字串，
		// 鬆散比對都會撞上兩個節點。
		await expect(
			processing.getByText("正在搜尋候選", { exact: true }),
		).toBeVisible({
			timeout: 15_000,
		});
		await expect(page.getByText("平台賣家", { exact: true })).toBeVisible();
		await expect(page.getByText("外部業者")).toBeVisible();
		await expect(
			processing.getByRole("heading", { name: "搜尋進度" }),
		).toHaveCount(0);
	});

	test("搜尋完成保留原需求與 composer；成果關閉重開保留勾選和草稿，新文字走原對話", async ({
		page,
		isMobile,
	}, testInfo) => {
		test.setTimeout(45_000);
		await page.setViewportSize(
			isMobile ? { width: 390, height: 844 } : { width: 1440, height: 1000 },
		);
		let searchFinished = false;
		await setupRfqPriceReview(page, {
			stopAtSearching: true,
			candidateSearchPollHandler: (route) => {
				if (!searchFinished) return route.fulfill(runningPoll());
				return route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({
						code: 0,
						data: {
							requestId: RFQ_REQUEST_ID,
							runId: RFQ_RUN_ID,
							candidateSetId: "candidate-set-e2e",
							status: "SUCCEEDED",
							platformLaneStatus: "SUCCEEDED",
							externalLaneStatus: "SUCCEEDED",
							failureReason: "",
							candidates: {
								platform: [
									{
										candidateId: "seller-green",
										sourceId: "product-green",
										lane: "PLATFORM",
										displayName: "範例鮮果合作社",
										score: 94,
										matchedFacts: [{ key: "item", label: "珍珠芭樂" }],
										unverifiableFacts: [],
										factsAsOf: "2026-08-14T12:00:00Z",
										selected: false,
									},
								],
								external: [],
							},
						},
					}),
				});
			},
			runEventsHandler: (route) =>
				route.fulfill({
					status: 200,
					contentType: "text/event-stream",
					body: RAIL_SSE,
				}),
		});

		const input = page.locator("#procurement-clarify-input");
		const originalRequest = page
			.getByRole("log", { name: "Hawkr Agent 採購經理紀錄" })
			.getByText("我要採購 50 KG 珍珠芭樂", {
				exact: true,
			});
		await expect(originalRequest).toBeVisible();
		await expect(input).toBeVisible();
		await expect(input).toBeDisabled();
		await expect(page.getByTestId("agent-processing")).toBeVisible();
		searchFinished = true;

		await expect(page.getByTestId("agent-processing")).toHaveCount(0);
		await expect(page.getByTestId("agent-activity-rail")).toHaveCount(0);
		await expect(input).toBeEnabled();
		const results = page.locator('aside[aria-label="採購成果"]');
		await expect(results).toHaveCount(1);
		if (await results.isVisible()) {
			await results
				.getByRole("button", { name: "回到對話", exact: true })
				.click();
		}
		await expect(results).toBeHidden();
		await expect(originalRequest).toBeVisible();
		await expect(input).toBeVisible();
		await expect(input).toHaveCount(1);
		await settleFiniteAnimations(page);
		const formalReply = page.locator("li.hawkr-agent-narrative").filter({
			hasText: "需求已整理好，可以開始搜尋符合條件的商家。",
		});
		await expect(formalReply).toHaveCSS("opacity", "1");
		await expect(
			formalReply.locator(".hawkr-agent-narrative__block"),
		).toHaveCSS("opacity", "1");
		await testInfo.attach("formal-reply-computed-style", {
			body: JSON.stringify(
				await formalReply.evaluate((row) =>
					[row, ...row.querySelectorAll(".hawkr-agent-narrative__block")].map(
						(node) => {
							const style = getComputedStyle(node);
							return {
								className: node.className,
								opacity: style.opacity,
								animationName: style.animationName,
								animationPlayState: style.animationPlayState,
							};
						},
					),
				),
			),
			contentType: "application/json",
		});
		await input.fill("數量改成 80 KG");

		const openResults = page.getByTestId("agent-panel-open");
		await expect(openResults).toHaveAccessibleName("查看成果");
		await openResults.click();
		await expect(results).toBeVisible();
		await expect(
			results.getByRole("tab", { name: "任務", exact: true }),
		).toHaveCount(0);
		const selection = results.getByRole("checkbox", {
			name: "選取 範例鮮果合作社",
		});
		await results
			.locator('label[for="procurement-candidate-seller-green"]')
			.click();
		await expect(selection).toBeChecked();
		await expect(selection).toBeEnabled();
		await settleFiniteAnimations(page);
		if (isMobile) {
			await results.evaluate((panel) => {
				for (const node of panel.querySelectorAll<HTMLElement>("*")) {
					if (node.scrollTop > 0) node.scrollTop = 0;
				}
			});
		}
		const screenshot = testInfo.outputPath("candidates.png");
		await page.screenshot({ path: screenshot, fullPage: !isMobile });
		await testInfo.attach("candidates", {
			path: screenshot,
			contentType: "image/png",
		});
		await results
			.getByRole("button", { name: "回到對話", exact: true })
			.click();
		await expect(results).toBeHidden();
		await expect(input).toHaveValue("數量改成 80 KG");
		await expect(input).toBeFocused();
		await openResults.click();
		await expect(selection).toBeChecked();
		await results
			.getByRole("button", { name: "回到對話", exact: true })
			.click();

		const revisedRevisionId = "1234567890123456792";
		let snapshotRecovered = false;
		let failedSnapshots = 0;
		await page.route(
			`**/api/v1/app/buyers/me/procurement/requests/${RFQ_REQUEST_ID}/snapshot`,
			(route) => {
				if (!snapshotRecovered) {
					failedSnapshots += 1;
					return route.fulfill({
						status: 500,
						contentType: "application/json",
						body: JSON.stringify({
							code: 500,
							message: "snapshot unavailable",
						}),
					});
				}
				return route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({
						code: 0,
						data: {
							requestId: RFQ_REQUEST_ID,
							revisionId: revisedRevisionId,
							status: "REQUIREMENT_READY",
							slots: {
								item: "珍珠芭樂",
								quantity: 80,
								unit: "KG",
								delivery_city: "桃園市",
							},
							missingRequired: [],
						},
					}),
				});
			},
		);
		const turnBodies: Record<string, unknown>[] = [];
		await page.route("**/api/v1/app/buyers/me/procurement/turns", (route) => {
			turnBodies.push(route.request().postDataJSON());
			return route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({
					code: 0,
					data: {
						requestId: RFQ_REQUEST_ID,
						revisionId: revisedRevisionId,
						conversationId: "9876543210987654321",
						slots: {
							item: "珍珠芭樂",
							quantity: 80,
							unit: "KG",
							delivery_city: "桃園市",
						},
						missingRequired: [],
						rejected: [],
						degraded: false,
						reply: "已把數量更新為 80 KG。",
						recordedThisTurn: [],
						question: {
							slotKey: "packaging",
							prompt: "這批芭樂需要分裝嗎？",
							options: [
								{ optionId: "pack-box", label: "分箱包裝", value: "分箱包裝" },
							],
							allowFreeText: true,
							canSkip: true,
							source: "FIXED",
						},
					},
				}),
			});
		});
		await page.getByRole("button", { name: "送出", exact: true }).click();
		await expect(
			page.getByText("已把數量更新為 80 KG。", { exact: true }),
		).toBeVisible();
		expect(turnBodies).toHaveLength(1);
		expect(turnBodies[0]).toMatchObject({
			requestId: RFQ_REQUEST_ID,
			message: "數量改成 80 KG",
		});
		expect(turnBodies[0].clientRequestId).toEqual(expect.any(String));
		// 新版本先使舊候選失效。snapshot 暫時失敗時仍顯示後端的新問題，
		// 不能退回舊成果，也不能自行猜成 ready 或把買家留在沒有出口的畫面。
		const question = page.getByTestId("clarify-question-card");
		await expect(question).toContainText("這批芭樂需要分裝嗎？");
		await expect(results).toHaveCount(0);
		await expect(page.getByTestId("agent-panel-open")).toHaveCount(0);
		const recover = page.getByRole("button", {
			name: "重新取得需求狀態",
			exact: true,
		});
		await expect(recover).toBeVisible();
		expect(failedSnapshots).toBeGreaterThan(0);
		snapshotRecovered = true;
		await recover.click();
		await expect(recover).toHaveCount(0);
		await expect(question).toContainText("這批芭樂需要分裝嗎？");
		await expect(
			question.getByRole("button", { name: "分箱包裝" }),
		).toBeEnabled();
		await expect(
			page.getByRole("button", { name: "交給 Hawkr Agent 搜尋" }),
		).toHaveCount(0);
	});
});
