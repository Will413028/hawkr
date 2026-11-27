import { expect, type Page, test } from "@playwright/test";
import {
	ConnectFailure,
	mockProcurementUnary,
	mockWatchRun,
	turnResultJson,
	watchFrame,
} from "./helpers/mock-procurement-connect";
import {
	candidateJson,
	candidateSearchJson,
	RFQ_CONVERSATION_ID,
	RFQ_IDS,
	RFQ_REQUEST_ID,
	RFQ_RUN_ID,
	setupRfqPriceReview,
	snapshotReadyJson,
} from "./helpers/mock-procurement-rfq";

/** 三列 activity 加上一筆重複 heartbeat（同一列同一狀態再來一次，cursor 不同），
 * 最後以 `finished` 正常結束串流。第三列收不收得到，就是重複的那一筆沒有把列重複畫出來、
 * 也沒有讓訂閱中斷的證據。 */
function railFrames() {
	const searchParent = {
		rowId: "9001",
		code: "SEARCH_CANDIDATES",
		label: "搜尋候選供應商",
		status: "RUNNING",
	} as const;
	const platformRow = {
		rowId: "9002",
		code: "SEARCH_PLATFORM",
		label: "翻平台上的賣家",
		status: "SUCCEEDED",
		parentRowId: "9001",
		count: 12,
	} as const;
	return [
		watchFrame.accepted("s1", RFQ_RUN_ID, RFQ_CONVERSATION_ID),
		watchFrame.activity("s2", searchParent),
		watchFrame.activity("s3", platformRow),
		watchFrame.activity("s4", platformRow),
		watchFrame.activity("s5", {
			rowId: "9004",
			code: "SUMMARIZE",
			label: "整理成給你的建議",
			status: "PENDING",
		}),
		watchFrame.finished("s6"),
	];
}

/** 一直回 RUNNING，讓 searching 階段停得夠久看得到軌。 */
function runningPoll() {
	return candidateSearchJson("RUNNING");
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
			candidateSearchPollHandler: runningPoll,
			searchWatchFrames: railFrames,
		});

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
		await expect(page.getByText("正在搜尋候選")).toHaveCount(0);
		await expect(
			rail.locator('.hawkr-agent-rail__children [data-row-id="9002"]'),
		).toHaveCount(1);

		const announcement = page.locator(
			'.sr-only[role="status"][aria-atomic="true"]',
		);
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

	test("搜尋 run 沒有 activity 事件時退回輪詢進度卡片", async ({ page }) => {
		test.setTimeout(45_000);
		await setupRfqPriceReview(page, {
			stopAtSearching: true,
			candidateSearchPollHandler: runningPoll,
		});

		const processing = page.getByTestId("agent-processing");
		await expect(processing).toBeVisible();
		await expect(page.locator("#hawkr-agent-side-panel")).toHaveCount(0);

		await expect(
			processing.getByText("正在搜尋候選", { exact: true }),
		).toBeVisible({
			timeout: 15_000,
		});
		await expect(
			processing.getByText("搜尋候選供應商", { exact: true }),
		).toBeVisible();
		await expect(page.getByText("外部業者")).toHaveCount(0);
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
			candidateSearchPollHandler: () =>
				searchFinished
					? candidateSearchJson("SUCCEEDED", [
							candidateJson("seller-green", "範例鮮果合作社"),
						])
					: runningPoll(),
			searchWatchFrames: railFrames,
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
		const revisedSlots = {
			item: "珍珠芭樂",
			quantity: 80,
			unit: "KG",
			deliveryCity: "桃園市",
		};
		let snapshotRecovered = false;
		let failedSnapshots = 0;
		await mockProcurementUnary(page, "GetSnapshot", () => {
			if (!snapshotRecovered) {
				failedSnapshots += 1;
				throw new ConnectFailure("internal", "snapshot unavailable");
			}
			return snapshotReadyJson({
				revisionId: revisedRevisionId,
				slots: revisedSlots,
			});
		});
		const turnBodies: Record<string, unknown>[] = [];
		await mockProcurementUnary(page, "SubmitTurn", (request) => {
			turnBodies.push(request);
			return {
				requestId: RFQ_REQUEST_ID,
				conversationId: RFQ_CONVERSATION_ID,
				runId: "run-revised",
			};
		});
		await mockWatchRun(page, ({ runId }) =>
			runId === "run-revised"
				? [
						watchFrame.accepted("r1", "run-revised", RFQ_CONVERSATION_ID),
						watchFrame.turnResult(
							"r2",
							turnResultJson(
								{ ...RFQ_IDS, revisionId: revisedRevisionId },
								{
									slots: revisedSlots,
									reply: "已把數量更新為 80 KG。",
									question: {
										slotKey: "SLOT_KEY_PACKAGING",
										prompt: "這批芭樂需要分裝嗎？",
										options: [{ optionId: "pack-box", label: "分箱包裝" }],
										allowFreeText: true,
										canSkip: true,
									},
								},
							),
						),
						watchFrame.finished("r3"),
					]
				: [
						watchFrame.accepted("x1", runId, RFQ_CONVERSATION_ID),
						watchFrame.finished("x2"),
					],
		);
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
