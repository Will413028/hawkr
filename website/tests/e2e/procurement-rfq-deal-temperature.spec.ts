import { expect, test } from "@playwright/test";
import {
	RFQ_TEMPERATURE_BAND_LABEL,
	RFQ_TEMPERATURE_UNKNOWN_COPY,
	RFQ_TEMPERATURE_UNKNOWN_DETAIL,
} from "@/features/procurement-assistant/rfq-guidance";
import {
	guidedPreviewData,
	setupRfqPriceReview,
} from "./helpers/mock-procurement-rfq";

function respondWith(data: Record<string, unknown>) {
	return (route: import("@playwright/test").Route) =>
		route.fulfill({
			status: 200,
			contentType: "application/json",
			body: JSON.stringify({ code: 0, data }),
		});
}

test.describe("procurement assistant — 成交溫度計", { tag: ["@ui"] }, () => {
	test("dealTemperature UNKNOWN／BUYER_RANGE_NOT_SET 顯示未知與引導文案", async ({
		page,
	}) => {
		test.setTimeout(45_000);
		await setupRfqPriceReview(page, {
			previewHandler: respondWith(guidedPreviewData()),
		});

		await expect(page.getByText(RFQ_TEMPERATURE_UNKNOWN_COPY)).toBeVisible();
		await expect(
			page.getByText(RFQ_TEMPERATURE_UNKNOWN_DETAIL.BUYER_RANGE_NOT_SET),
		).toBeVisible();
		// UNKNOWN 分支只留下凍結字串「成交溫度計：未知」，不再有第二行標題
		// （AVAILABLE 才有「成交溫度計」小標）——兩個並排會讀成同一句話講兩遍。
		await expect(page.getByText("成交溫度計", { exact: true })).toHaveCount(0);

		// UNKNOWN 不得出現量表或色帶——ADR invariant：不顯示 0%、百分比或 band。
		await expect(
			page.locator(".assistant-rfq-guidance__temperature-gauge"),
		).toHaveCount(0);
		await expect(
			page.locator(".assistant-rfq-guidance__temperature[data-band]"),
		).toHaveCount(0);
	});

	test("dealTemperature AVAILABLE 顯示單一數字（出價位置百分位）與色帶", async ({
		page,
	}) => {
		test.setTimeout(45_000);
		await setupRfqPriceReview(page, {
			previewHandler: respondWith(
				guidedPreviewData({
					dealTemperature: {
						status: "AVAILABLE",
						reasonCode: "",
						probabilityLowerBps: "4100",
						probabilityUpperBps: "7800",
						band: "WARM",
						outcomeDefinition: "BUYER_RANGE_VS_PRICEBOOK_FORECAST_COVERAGE",
						outcomeDefinitionVersion: "1",
						policyVersion: "DEAL_TEMPERATURE_COVERAGE_V1",
						bandVersion: "BANDS_V2",
					},
				}),
			),
		});

		// 畫面上只有一個數字：寬鬆界（買家上限所在的行情百分位）。
		await expect(page.getByText("78%", { exact: true })).toBeVisible();
		await expect(
			page.getByText(`出價位置：${RFQ_TEMPERATURE_BAND_LABEL.WARM}`),
		).toBeVisible();
		await expect(
			page.getByText(/行情有 78% 落在你的出價上限之下/),
		).toBeVisible();
		await expect(page.getByText("成交溫度計", { exact: true })).toBeVisible();
		// 嚴格界（41%）照收但不得出現在畫面上——它對買家是反指標。
		await expect(page.getByText(/41%/)).toHaveCount(0);

		// 稿 P-01 的量表：軸固定 0%–100%，單一刻度依 server 的寬鬆界定位，
		// 顏色只由 server 的 band 決定（畫面上就是容器的 data-band）。
		const gauge = page.locator(".assistant-rfq-guidance__temperature-gauge");
		await expect(gauge).toBeVisible();
		const axis = gauge.locator(".assistant-rfq-guidance__temperature-axis");
		await expect(axis).toHaveCount(2);
		await expect(axis.first()).toHaveText("0%");
		await expect(axis.last()).toHaveText("100%");
		await expect(
			page.locator('.assistant-rfq-guidance__temperature[data-band="WARM"]'),
		).toHaveCount(1);
		const point = page.locator(".assistant-rfq-guidance__temperature-point");
		await expect(point).toHaveCount(1);
		await expect(point).toHaveAttribute("style", /left:\s*78%/);
	});

	// 這一支是 v1 defect 本體的回歸測試：guided 模式一開始是 UNKNOWN（上面第一
	// 支已經覆蓋），買家調整參考預填的希望詢價區間、按「重新試算」之後，
	// command 帶上編輯後的區間、伺服器據此算出的溫度必須真的顯示出來——不是
	// 永遠停在 UNKNOWN。
	test("guided 模式調整希望詢價區間並重新試算後顯示 AVAILABLE 溫度", async ({
		page,
	}) => {
		test.setTimeout(45_000);
		let previewCalls = 0;
		await setupRfqPriceReview(page, {
			previewHandler: (route) => {
				previewCalls += 1;
				if (previewCalls === 1) {
					return route.fulfill({
						status: 200,
						contentType: "application/json",
						body: JSON.stringify({ code: 0, data: guidedPreviewData() }),
					});
				}
				const body = route.request().postDataJSON() as {
					buyerRequestedRange?: {
						lowerBoundAmount: string;
						upperBoundAmount: string;
					};
				};
				return route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({
						code: 0,
						data: guidedPreviewData({
							buyerRequestedRange: body.buyerRequestedRange,
							dealTemperature: {
								status: "AVAILABLE",
								reasonCode: "",
								probabilityLowerBps: "5200",
								probabilityUpperBps: "8300",
								band: "HOT",
								outcomeDefinition: "BUYER_RANGE_VS_PRICEBOOK_FORECAST_COVERAGE",
								outcomeDefinitionVersion: "1",
								policyVersion: "DEAL_TEMPERATURE_COVERAGE_V1",
								bandVersion: "BANDS_V2",
							},
						}),
					}),
				});
			},
		});

		// 第一次 preview（guided 預設模式，尚未編輯）：跟第一支測試一樣是 UNKNOWN。
		await expect(page.getByText(RFQ_TEMPERATURE_UNKNOWN_COPY)).toBeVisible();

		await page.getByLabel("希望下限").fill("35");
		await page.getByLabel("希望上限").fill("48");
		await page.getByRole("button", { name: "重新試算" }).click();

		await expect(page.getByText("83%", { exact: true })).toBeVisible();
		await expect(
			page.getByText(`出價位置：${RFQ_TEMPERATURE_BAND_LABEL.HOT}`),
		).toBeVisible();
		expect(previewCalls).toBe(2);
	});
});
