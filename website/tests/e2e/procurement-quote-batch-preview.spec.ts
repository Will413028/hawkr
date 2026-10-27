import { expect, test } from "@playwright/test";
import {
	RFQ_PREVIEW_HASH,
	RFQ_REVISION_ID,
	setupRfqPriceReview,
} from "./helpers/mock-procurement-rfq";

test.describe("procurement assistant — RFQ price preview recovery", {
	tag: ["@ui"],
}, () => {
	test("MAPPING_MISSING requires an explicit switch and no-reference stays unconfirmable without basis", async ({
		page,
	}) => {
		test.setTimeout(45_000);
		const bodies: Array<Record<string, unknown>> = [];
		await setupRfqPriceReview(page, {
			previewHandler: (route) => {
				const body = route.request().postDataJSON() as Record<string, unknown>;
				bodies.push(body);
				if (body.pricingMode === "PRICEBOOK_GUIDED") {
					return route.fulfill({
						status: 422,
						contentType: "application/json",
						body: JSON.stringify({
							code: 422,
							reason: "MAPPING_MISSING",
							message: "mapping missing",
						}),
					});
				}
				return route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({
						code: 0,
						data: {
							pricePreviewId: "1234567890123456801",
							version: "1",
							pricingMode: "BUYER_ENTERED_NO_REFERENCE",
							referenceAvailability: "UNAVAILABLE",
							referencePriceRange: null,
							buyerRequestedRange: {
								currency: "TWD",
								priceComponent: "GOODS_ONLY",
								lowerBoundAmount: "30",
								upperBoundAmount: "42",
								basisQuantity: "",
								basisUnit: "",
							},
							dealTemperature: {
								status: "UNKNOWN",
								reasonCode: "MAPPING_MISSING",
							},
							payloadHash: RFQ_PREVIEW_HASH,
						},
					}),
				});
			},
		});

		await expect(
			page.getByText("Pricebook 找不到可用的品項與區域參考。"),
		).toBeVisible();
		expect(bodies).toHaveLength(1);
		await page.getByRole("button", { name: "改用自行設定價格" }).click();
		await expect(
			page.getByRole("radio", { name: /自行設定價格/ }),
		).toBeChecked();
		await page.getByLabel("希望下限").fill("30");
		await page.getByLabel("希望上限").fill("42");
		await page.getByRole("button", { name: "取得無參考價試算" }).click();

		await expect(page.getByText("價格基準尚不可確認")).toBeVisible();
		await expect(
			page.getByRole("checkbox", {
				name: "我已核對這份需求版本與價格範圍",
			}),
		).toBeDisabled();
		expect(bodies).toHaveLength(2);
		expect(bodies[1]).toEqual({
			expectedRevisionId: RFQ_REVISION_ID,
			clientRequestId: expect.any(String),
			pricingMode: "BUYER_ENTERED_NO_REFERENCE",
			deliveryCity: "桃園市",
			buyerRequestedRange: {
				currency: "TWD",
				priceComponent: "GOODS_ONLY",
				lowerBoundAmount: "30",
				upperBoundAmount: "42",
			},
		});
	});

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
							pricingMode: "PRICEBOOK_GUIDED",
							referenceAvailability: "AVAILABLE",
							referencePriceRange: {
								lowerBoundAmount: "30",
								midpointAmount: "36",
								upperBoundAmount: "42",
							},
							buyerRequestedRange: {
								currency: "TWD",
								priceComponent: "GOODS_ONLY",
								lowerBoundAmount: "30",
								upperBoundAmount: "42",
								basisQuantity: "1",
								basisUnit: "KG",
							},
							dealTemperature: {
								status: "UNKNOWN",
								reasonCode: "BUYER_RANGE_NOT_SET",
							},
							payloadHash: RFQ_PREVIEW_HASH,
							marketPriceBasis: {
								regionName: "桃園市",
								supportingMarketCount: "3",
								sourceDataAsOf: "2026-08-14T00:00:00Z",
								freshnessStatus: "fresh",
							},
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
		await expect(page.getByText("已取得行情參考範圍")).toBeVisible();
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
