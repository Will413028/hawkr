import { expect, test } from "@playwright/test";
import {
	pricePreviewData,
	publicationResultJson,
	RFQ_APPROVAL_ID,
	RFQ_REQUEST_ID,
	RFQ_REVISION_ID,
	RFQ_SELLER_PAYLOAD_HASH,
	runtimeSnapshotJson,
	setupRfqPriceReview,
} from "./helpers/mock-procurement-rfq";

test.describe("procurement assistant — RFQ price review", {
	tag: ["@ui"],
}, () => {
	test("price preview carries the buyer range without publishing anything", async ({
		page,
	}) => {
		test.setTimeout(45_000);
		const previewBodies: Array<Record<string, unknown>> = [];
		const publicationRequests: Array<Record<string, unknown>> = [];
		await setupRfqPriceReview(page, {
			previewHandler: (request) => {
				previewBodies.push(request);
				return pricePreviewData();
			},
			publicationPreviewHandler: (request) => {
				publicationRequests.push(request);
				throw new Error("價格試算階段不應該取得發布核准");
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
			requestId: RFQ_REQUEST_ID,
			expectedRevisionId: RFQ_REVISION_ID,
			buyerRequestedRange: {
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

		await expect(
			page.getByRole("heading", { name: "重新確認 RFQ 快照" }),
		).toBeVisible();
		await expect(
			page.getByRole("button", { name: "確認並準備發布" }),
		).toBeDisabled();

		await page.getByRole("button", { name: "返回調整" }).click();
		await expect(
			page.getByRole("button", { name: "取得無參考價試算" }),
		).toHaveCount(0);
		expect(publicationRequests).toHaveLength(0);
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

		await expect(
			panel.getByRole("heading", { name: "確認價格範圍" }),
		).toBeVisible();

		await panel.getByRole("button", { name: "回到對話", exact: true }).click();
		await expect(panel).toBeHidden();
		await expect(panel).toHaveCount(1);
		await expect(page.locator(".thread-page")).toBeVisible();
		await page.getByTestId("agent-panel-open").click();
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

		await page.getByTestId("agent-panel-open").click();
		await expect(panel).toBeVisible();
	});

	test("取得發布核准後，最終 CTA 真的把邀請送到 execute 端點", async ({
		page,
	}) => {
		test.setTimeout(45_000);
		const executeBodies: Array<Record<string, unknown>> = [];
		await setupRfqPriceReview(page, {
			publicationExecuteHandler: (request) => {
				executeBodies.push(request);
				return publicationResultJson();
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
			requestId: RFQ_REQUEST_ID,
			clientRequestId: expect.any(String),
			approvalId: RFQ_APPROVAL_ID,
			approvalVersion: "1",
			expectedRevisionId: RFQ_REVISION_ID,
			payloadHash: RFQ_SELLER_PAYLOAD_HASH,
		});

		await expect(
			page.getByRole("heading", { name: "詢價已發布" }),
		).toBeVisible();
		const summary = page.getByTestId("quote-batch-summary");
		await expect(
			summary.getByText("已建立 1 組詢價・1 筆邀請（50 KG）"),
		).toBeVisible();
		await expect(summary.getByText("珍珠芭樂")).toBeVisible();
		await expect(summary.getByText("桃園市・2026/09/01")).toBeVisible();
		await expect(summary.getByText("這次發布的收件人")).toBeVisible();
		await expect(
			summary.getByText("這三個數字是發布當下的收件人分布，不是投遞結果。"),
		).toBeVisible();
		await expect(summary.getByText("0 筆")).toBeVisible();
		await expect(summary.getByText("尚未成立")).toBeVisible();
		await expect(summary.getByText("尚未建立")).toBeVisible();
		await expect(summary.getByRole("button", { name: /撤回/ })).toHaveCount(0);

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
			runtimeSnapshotHandler: () =>
				runtimeSnapshotJson({ publishedRfq: undefined }),
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
		await expect(summary.getByText(/已建立 1 組詢價/)).toHaveCount(0);
		await expect(summary.getByText("這次發布的收件人")).toHaveCount(0);
	});
});
