import { expect, test } from "@playwright/test";
import { ABORT_REQUEST } from "./helpers/mock-procurement-connect";
import {
	pricePreviewData,
	setupRfqPriceReview,
} from "./helpers/mock-procurement-rfq";

test.describe("procurement assistant — RFQ price preview recovery", {
	tag: ["@ui"],
}, () => {
	// 價格試算是純計算、沒有 client_request_id；「同一個操作重試」就是同一份請求訊息原樣再送一次。
	test("a lost preview response retries the exact same request", async ({
		page,
	}) => {
		test.setTimeout(45_000);
		const bodies: Array<Record<string, unknown>> = [];
		await setupRfqPriceReview(page, {
			previewHandler: (request) => {
				bodies.push(request);
				if (bodies.length === 1) return ABORT_REQUEST;
				return pricePreviewData();
			},
		});

		await expect(
			page.getByRole("heading", { name: "目前無法完成價格確認" }),
		).toBeVisible();
		await page.getByRole("button", { name: "以原操作重試" }).click();
		await expect(page.getByText("你設定的希望詢價區間")).toBeVisible();
		expect(bodies).toHaveLength(2);
		expect(bodies[1]).toEqual(bodies[0]);
	});

	test("a malformed 2xx response fails closed and offers no retry", async ({
		page,
	}) => {
		test.setTimeout(45_000);
		const bodies: Array<Record<string, unknown>> = [];
		await setupRfqPriceReview(page, {
			previewHandler: (request) => {
				bodies.push(request);
				// 缺 buyer_requested_range：不猜、不補值。
				return {};
			},
		});

		await expect(
			page.getByRole("heading", { name: "目前無法完成價格確認" }),
		).toBeVisible();
		await expect(
			page.getByRole("button", { name: "檢視不可變 RFQ 快照" }),
		).toBeDisabled();
		// 回應不完整不是網路問題，重送同一份請求沒有意義，所以沒有「以原操作重試」。
		await expect(
			page.getByRole("button", { name: "以原操作重試" }),
		).toHaveCount(0);
		expect(bodies).toHaveLength(1);
	});
});
