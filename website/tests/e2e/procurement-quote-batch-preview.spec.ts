import { expect, test } from "@playwright/test";
import {
	RFQ_PREVIEW_HASH,
	setupRfqPriceReview,
} from "./helpers/mock-procurement-rfq";

test.describe("procurement assistant — RFQ price preview recovery", {
	tag: ["@ui"],
}, () => {
	test("a lost preview response retries the exact same command identity", async ({
		page,
	}) => {
		test.setTimeout(45_000);
		const bodies: Array<Record<string, unknown>> = [];
		await setupRfqPriceReview(page, {
			previewHandler: (route) => {
				bodies.push(route.request().postDataJSON() as Record<string, unknown>);
				if (bodies.length === 1) return route.abort("failed");
				return route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({
						code: 0,
						data: {
							pricePreviewId: "1234567890123456801",
							version: "1",
							buyerRequestedRange: {
								currency: "TWD",
								priceComponent: "GOODS_ONLY",
								lowerBoundAmount: "30",
								upperBoundAmount: "42",
								basisQuantity: "1",
								basisUnit: "KG",
							},
							payloadHash: RFQ_PREVIEW_HASH,
							quoteTotalRange: null,
						},
					}),
				});
			},
		});

		await expect(
			page.getByText(/連線中斷，無法確認這次操作的結果/),
		).toBeVisible();
		await page.getByRole("button", { name: "以原操作重試" }).click();
		await expect(page.getByText("你設定的希望詢價區間")).toBeVisible();
		expect(bodies).toHaveLength(2);
		expect(bodies[1]).toEqual(bodies[0]);
	});

	test("a malformed 2xx response fails closed and keeps the same retry identity", async ({
		page,
	}) => {
		test.setTimeout(45_000);
		const bodies: Array<Record<string, unknown>> = [];
		await setupRfqPriceReview(page, {
			previewHandler: (route) => {
				bodies.push(route.request().postDataJSON() as Record<string, unknown>);
				return route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({ code: 0, data: { version: "1" } }),
				});
			},
		});
		expect(bodies[1]?.clientRequestId).not.toBe(bodies[0]?.clientRequestId);

		await expect(
			page.getByText(/價格服務回傳不完整，無法確認這次操作的結果/),
		).toBeVisible();
		await expect(
			page.getByRole("button", { name: "檢視不可變 RFQ 快照" }),
		).toBeDisabled();
		await page.getByRole("button", { name: "以原操作重試" }).click();
		await expect.poll(() => bodies.length).toBe(2);
		expect(bodies[1]).toEqual(bodies[0]);
	});
});
