import { expect, test } from "@playwright/test";
import {
	pricePreviewData,
	RFQ_APPROVAL_ID,
	RFQ_CONVERSATION_ID,
	RFQ_PREVIEW_HASH,
	RFQ_PREVIEW_ID,
	RFQ_REQUEST_ID,
	RFQ_REVISION_ID,
	RFQ_SELLER_PAYLOAD_HASH,
	setupRfqPriceReview,
} from "./helpers/mock-procurement-rfq";

test.describe("procurement assistant — RFQ confirmation", {
	tag: ["@ui"],
}, () => {
	test("confirms a guided server projection without publishing a legacy QuoteBatch", async ({
		page,
	}) => {
		test.setTimeout(45_000);
		const previewBodies: Array<Record<string, unknown>> = [];
		const confirmationBodies: Array<Record<string, unknown>> = [];
		const { legacy } = await setupRfqPriceReview(page, {
			previewHandler: (route) => {
				previewBodies.push(
					route.request().postDataJSON() as Record<string, unknown>,
				);
				return route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({
						code: 0,
						data: pricePreviewData(),
					}),
				});
			},
			confirmationHandler: (route) => {
				confirmationBodies.push(
					route.request().postDataJSON() as Record<string, unknown>,
				);
				return route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({
						code: 0,
						data: {
							confirmedRfqSnapshotId: "1234567890123456802",
							payloadHash: "b".repeat(64),
						},
					}),
				});
			},
		});

		await expect(page.getByText("你設定的希望詢價區間")).toBeVisible();
		await expect(page.getByText(/品相狀態/)).toHaveCount(0);
		await page.getByRole("button", { name: /需求版本$/ }).click();
		await expect(page.getByLabel("需求數量")).toHaveAttribute("readonly", "");
		await expect(page.getByLabel("配送城市")).toHaveAttribute("readonly", "");
		await page.getByRole("button", { name: /價格確認$/ }).click();
		await expect
			.poll(() =>
				page
					.locator(".hawkr-agent-canvas__body")
					.evaluate((element) => element.scrollWidth <= element.clientWidth),
			)
			.toBe(true);

		expect(previewBodies[0]).toEqual({
			expectedRevisionId: RFQ_REVISION_ID,
			clientRequestId: expect.any(String),
			deliveryCity: "桃園市",
			buyerRequestedRange: {
				currency: "TWD",
				priceComponent: "GOODS_ONLY",
				lowerBoundAmount: "30",
				upperBoundAmount: "42",
			},
		});

		await page
			.getByRole("checkbox", {
				name: "我已核對這份需求版本與價格範圍",
			})
			.check();
		await page.getByRole("button", { name: "檢視不可變 RFQ 快照" }).click();

		// 稿 P-01 的主 CTA 固定價格之後直接帶買家到稿 R-01——⑤快照追蹤要顯示的
		// snapshotHash 與有效至只有確認後才存在。
		await expect(
			page.getByRole("heading", { name: "重新確認 RFQ 快照" }),
		).toBeVisible();
		await expect(
			page.getByRole("button", { name: "確認並準備發布" }),
		).toBeDisabled();

		// 鎖定不變式仍然成立，只是要回到價格確認那一步才看得到。
		await page.getByRole("button", { name: "返回調整" }).click();
		await expect(
			page.getByText("需求版本與價格範圍已固定，等待發布。"),
		).toBeVisible();
		// 「重新算一次」只有希望詢價卡上的「取得無參考價試算」一顆，確認鎖定後要停用。
		await expect(
			page.getByRole("button", { name: "取得無參考價試算" }),
		).toBeDisabled();
		// toEqual 而不是 toMatchObject 是刻意的：這條擋的是「confirm 的 body 多送了
		// 買家私有欄位」。⛔ 不要改成寬鬆比對——多一個鍵就該讓它紅。
		// buyerNote 沒填時送空字串（backend #3410 那側 trim 後視為沒填，
		// canonical payload 完全不含這個鍵）。
		expect(confirmationBodies[0]).toEqual({
			clientRequestId: expect.any(String),
			expectedRevisionId: RFQ_REVISION_ID,
			pricePreviewId: RFQ_PREVIEW_ID,
			pricePreviewVersion: "1",
			payloadHash: RFQ_PREVIEW_HASH,
			buyerNote: "",
		});
		expect(legacy.calls).toBe(0);
	});

	test("成果關閉再開啟保留價格流程與已取得的價格，不產生任務分頁", async ({
		page,
	}) => {
		test.setTimeout(90_000);
		await setupRfqPriceReview(page);

		const panel = page.locator("#hawkr-agent-side-panel");
		await expect(panel).toBeVisible();
		await expect(panel).toHaveAttribute("aria-label", "採購成果");
		await expect(panel.getByRole("tab", { name: "任務" })).toHaveCount(0);

		// setup 把成果分頁開到 RFQ 草稿視圖。
		await expect(
			panel.getByRole("heading", { name: "確認價格範圍" }),
		).toBeVisible();

		await panel.getByRole("button", { name: "回到對話", exact: true }).click();
		await expect(panel).toBeHidden();
		await expect(panel).toHaveCount(1);
		await expect(page.locator(".thread-page")).toBeVisible();
		await page.getByTestId("agent-panel-open").click();
		// 收合仍保留 CandidateCanvas，重開後不能退回候選清單或遺失取得的價格。
		await expect(
			panel.getByRole("heading", { name: "確認價格範圍" }),
		).toBeVisible();
		await expect(panel.getByText("你設定的希望詢價區間")).toBeVisible();
	});

	test("窄視窗：面板全幅覆蓋，標頭有一條常駐的路進得去，也回得來", async ({
		page,
	}) => {
		test.setTimeout(90_000);
		await page.setViewportSize({ width: 960, height: 900 });
		await setupRfqPriceReview(page);

		const panel = page.locator("#hawkr-agent-side-panel");
		await expect(panel).toBeVisible();

		await page.getByRole("button", { name: "回到對話", exact: true }).click();
		await expect(panel).toBeHidden();
		await expect(page.locator(".thread-page")).toBeVisible();

		// 成果存在時，標頭入口可再次打開同一份成果。
		await page.getByTestId("agent-panel-open").click();
		await expect(panel).toBeVisible();
	});

	test("取得發布核准後，最終 CTA 真的把邀請送到 execute 端點", async ({
		page,
	}) => {
		test.setTimeout(45_000);
		const executeBodies: Array<Record<string, unknown>> = [];
		await setupRfqPriceReview(page, {
			publicationExecuteHandler: (route) => {
				executeBodies.push(
					route.request().postDataJSON() as Record<string, unknown>,
				);
				return route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({
						code: 0,
						data: {
							publicationId: "1234567890123456804",
							publicationVersion: "1",
							state: "PUBLISHED",
							audienceMode: "DIRECT_RECIPIENTS",
							audienceHash: "c".repeat(64),
							sellerFacingPayloadHash: RFQ_SELLER_PAYLOAD_HASH,
							publishedAt: "2026-08-22T12:40:00Z",
							replayed: false,
						},
					}),
				});
			},
		});

		await page
			.getByRole("checkbox", { name: "我已核對這份需求版本與價格範圍" })
			.check();
		await page.getByRole("button", { name: "檢視不可變 RFQ 快照" }).click();
		await expect(
			page.getByRole("heading", { name: "重新確認 RFQ 快照" }),
		).toBeVisible();

		await expect(
			page.getByRole("button", { name: "確認並送出邀請" }),
		).toBeHidden();

		await page
			.getByRole("checkbox", {
				name: "我已重新檢查需求、希望價格與發布受眾；確認後此快照不可變",
			})
			.check();
		await page.getByRole("button", { name: "確認並準備發布" }).click();

		const executeCta = page.getByRole("button", {
			name: "確認並送出邀請",
		});
		await expect(executeCta).toBeVisible();
		await executeCta.click();

		expect(executeBodies).toHaveLength(1);
		expect(executeBodies[0]).toEqual({
			clientRequestId: expect.any(String),
			approvalId: RFQ_APPROVAL_ID,
			approvalVersion: "1",
			expectedRevisionId: RFQ_REVISION_ID,
			payloadHash: RFQ_SELLER_PAYLOAD_HASH,
		});

		// 發布成功之後畫布落到稿 R-02 的三區版面。這是這一格第一次在瀏覽器上
		// 真的走得到：入口原本判 legacy `quoteBatch`，而那個 state 恆為 null。
		await expect(
			page.getByRole("heading", { name: "詢價已發布" }),
		).toBeVisible();
		const summary = page.getByTestId("quote-batch-summary");
		// ①需求來自 slots（品項／數量／配送），不是 legacy 批次。
		await expect(
			summary.getByText("已建立 1 組詢價・1 筆邀請（50 KG）"),
		).toBeVisible();
		await expect(summary.getByText("珍珠芭樂")).toBeVisible();
		await expect(summary.getByText("桃園市・2026/09/01")).toBeVisible();
		// 邀請清單換成 publication 的三個 count，且明說不是投遞結果。
		await expect(summary.getByText("這次發布的收件人")).toBeVisible();
		await expect(
			summary.getByText("這三個數字是發布當下的收件人分布，不是投遞結果。"),
		).toBeVisible();
		// ③空狀態照樣誠實：seller-responses 讀到空清單＝0 筆，不是「尚未取得」。
		await expect(summary.getByText("0 筆")).toBeVisible();
		await expect(summary.getByText("尚未成立")).toBeVisible();
		await expect(summary.getByText("尚未建立")).toBeVisible();
		// ⛔ 撤回入口已下架，不得留一顆停用的按鈕。
		await expect(summary.getByRole("button", { name: /撤回/ })).toHaveCount(0);

		// 回得去也回得來：候選檢視多一列入口，點回去仍是同一份三區版面。
		await page.getByRole("button", { name: "返回候選資料" }).click();
		await expect(page.getByText("這份需求已建立詢價")).toBeVisible();
		await page.getByRole("button", { name: "查看詢價狀態" }).click();
		await expect(
			page.getByRole("heading", { name: "詢價已發布" }),
		).toBeVisible();
	});

	test("發布結果還沒寫進快照時，三區版面照樣進得去而且不宣稱已核准", async ({
		page,
	}) => {
		test.setTimeout(45_000);
		await setupRfqPriceReview(page, {
			// execute 成功但 artifact 還沒落地——這正是「拿 runtime.publication 當
			// 入口守衛」會把買家彈回候選的那個時間窗。
			runtimeSnapshotHandler: (route) =>
				route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({
						code: 0,
						data: {
							conversationId: RFQ_CONVERSATION_ID,
							requestId: RFQ_REQUEST_ID,
							messages: [],
							runs: [],
							artifacts: [],
						},
					}),
				}),
		});

		await page
			.getByRole("checkbox", { name: "我已核對這份需求版本與價格範圍" })
			.check();
		await page.getByRole("button", { name: "檢視不可變 RFQ 快照" }).click();
		await page
			.getByRole("checkbox", {
				name: "我已重新檢查需求、希望價格與發布受眾；確認後此快照不可變",
			})
			.check();
		await page.getByRole("button", { name: "確認並準備發布" }).click();
		await page.getByRole("button", { name: "確認並送出邀請" }).click();

		const summary = page.getByTestId("quote-batch-summary");
		await expect(
			page.getByRole("heading", { name: "詢價建立摘要" }),
		).toBeVisible();
		await expect(
			summary.getByText("後端未提供不可變核准證據；此畫面不代表核准已完成。"),
		).toBeVisible();
		// 沒有發布事實就沒有「幾筆邀請」與收件人分布，不補 0。
		await expect(summary.getByText(/已建立 1 組詢價/)).toHaveCount(0);
		await expect(summary.getByText("這次發布的收件人")).toHaveCount(0);
	});
});
