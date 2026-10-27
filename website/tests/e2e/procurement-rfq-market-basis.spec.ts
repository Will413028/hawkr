import { expect, test } from "@playwright/test";
import {
	guidedPreviewData,
	RFQ_PREVIEW_HASH,
	setupRfqPriceReview,
} from "./helpers/mock-procurement-rfq";

/**
 * v6 稿 03 的市場價格參考與詢價總額區間（plan A4）。
 *
 * 三件事在這裡釘住，因為它們都是「後端明說、前端很容易做錯」的那一類：
 *  1. 計價基準以重量單位呈現（每 1 KG），不是稿上畫的「／箱」——AllowedUnits 只有
 *     KG／CATTY／GRAM，Will 已拍板不新增結構化包裝規格 slot。
 *  2. quoteTotalRange 為 null 時整張總額卡消失，畫面上不得出現 NT$0。
 *  3. sourceDataAsOf 缺席時說「行情來源未提供」，不得補成今天假裝資料是新的。
 */
function previewWith(overrides: Record<string, unknown>) {
	return { ...guidedPreviewData(), ...overrides };
}

function respondWith(data: Record<string, unknown>) {
	return (route: import("@playwright/test").Route) =>
		route.fulfill({
			status: 200,
			contentType: "application/json",
			body: JSON.stringify({ code: 0, data }),
		});
}

test.describe("procurement RFQ — 市場價格參考與詢價總額區間", {
	tag: ["@ui"],
}, () => {
	test("renders the market basis on a weight basis and the server-computed total", async ({
		page,
	}) => {
		test.setTimeout(45_000);
		await setupRfqPriceReview(page, {
			previewHandler: respondWith(guidedPreviewData()),
		});

		await expect(page.getByText("已取得行情參考範圍")).toBeVisible();

		// 資料出處：資料截至日（台北日期，非 UTC 直出）、映射（品項／區域）、
		// 貢獻市場家數與重量計價基準
		await expect(page.getByText("2026/08/14", { exact: true })).toBeVisible();
		await expect(page.getByText("珍珠芭樂／桃園市")).toBeVisible();
		await expect(page.getByText("3 個批發市場・每 1 KG")).toBeVisible();

		// 稿 P-01 的參考區間數線：三個刻度用稿上的標籤
		await expect(page.getByText("區間下限")).toBeVisible();
		await expect(page.getByText("中位數")).toBeVisible();
		await expect(page.getByText("區間上限")).toBeVisible();

		// wire 上沒有 confidence 欄位，稿上的「信心：中」不得出現
		await expect(page.getByText(/信心/)).toHaveCount(0);
		await expect(
			page.getByText("僅供設定詢價範圍，不是供應商報價。"),
		).toBeVisible();

		// 總額區間由伺服器算，前端只顯示；單位跟著 basisUnit 走
		await expect(page.getByText("詢價總額區間")).toBeVisible();
		await expect(page.getByText("NT$15,000 — NT$21,000")).toBeVisible();
		await expect(page.getByText("500 KG・依本次單價範圍計算")).toBeVisible();

		// 拍板結果：不得出現稿上的「箱」
		await expect(page.getByText(/／箱|\/箱/)).toHaveCount(0);
	});

	test("hides the whole total card when the quantity slot is unfilled", async ({
		page,
	}) => {
		test.setTimeout(45_000);
		await setupRfqPriceReview(page, {
			previewHandler: respondWith(previewWith({ quoteTotalRange: null })),
		});

		await expect(page.getByText("已取得行情參考範圍")).toBeVisible();
		await expect(page.getByText("3 個批發市場・每 1 KG")).toBeVisible();

		// 整張卡消失，而不是渲染 0
		await expect(page.getByText("詢價總額區間")).toHaveCount(0);
		await expect(page.getByText(/NT\$0\b/)).toHaveCount(0);
	});

	test("discloses stale market data and an absent source date without inventing one", async ({
		page,
	}) => {
		test.setTimeout(45_000);
		await setupRfqPriceReview(page, {
			previewHandler: respondWith(
				previewWith({
					marketPriceBasis: {
						regionName: "臺中市",
						supportingMarketCount: "1",
						sourceDataAsOf: null,
						freshnessStatus: "stale",
					},
				}),
			),
		});

		await expect(page.getByText("行情來源未提供")).toBeVisible();
		await expect(page.getByText("價格為模型推估，僅供參考。")).toBeVisible();
		await expect(page.getByText("珍珠芭樂／臺中市")).toBeVisible();
		await expect(page.getByText("1 個批發市場・每 1 KG")).toBeVisible();
	});

	// 2026-09-14：backend 移除 pricebook 的新鮮度硬覆寫之後，freshnessStatus=
	// "unavailable" 第一次能與一個實際顯示出來的參考價並存（在此之前不可達：有參考價
	// 就代表該區域 ready，而 ready 的來源市場必然是 fresh 或 stale）。dev 實測的真實
	// 形狀就是這一組——桃園市只有桃農一個批發市場，資料停在 2026-09-11 仍出得了價。
	//
	// 這支測試守的是「畫面不得自相矛盾」：價格還在的同時，不能再說這個區域沒有行情。
	test("shows a price alongside unavailable freshness without contradicting itself", async ({
		page,
	}) => {
		test.setTimeout(45_000);
		await setupRfqPriceReview(page, {
			previewHandler: respondWith(
				previewWith({
					marketPriceBasis: {
						regionName: "桃園市",
						supportingMarketCount: "1",
						sourceDataAsOf: "2026-09-11T15:59:59Z",
						freshnessStatus: "unavailable",
					},
				}),
			),
		});

		// 參考價照樣拿得到——這正是覆寫移除後的新行為
		await expect(page.getByText("已取得行情參考範圍")).toBeVisible();
		await expect(page.getByText("珍珠芭樂／桃園市")).toBeVisible();
		await expect(page.getByText("1 個批發市場・每 1 KG")).toBeVisible();

		// 資料年齡照實揭露（台北日期，非 UTC 直出）
		await expect(page.getByText("2026/09/11", { exact: true })).toBeVisible();
		await expect(page.getByText("價格為模型推估，僅供參考。")).toBeVisible();

		// 🔴 回歸守衛：舊文案會與畫面上的價格直接打架，一個字都不能出現
		await expect(
			page.getByText("這個區域目前沒有可用的行情資料。"),
		).toHaveCount(0);
	});

	test("drops the market basis together with the reference range", async ({
		page,
	}) => {
		test.setTimeout(45_000);
		// marketPriceBasis 與 referencePriceRange 同生同滅，所以要看到它消失就得真的
		// 走到 no-reference 模式——先讓 guided 查詢吃 MAPPING_MISSING，再由使用者
		// 明確切換（客戶端會擋下 pricingMode 與請求不符的回應）。
		await setupRfqPriceReview(page, {
			previewHandler: (route) => {
				const body = route.request().postDataJSON() as Record<string, unknown>;
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
				return respondWith({
					...guidedPreviewData(),
					pricingMode: "BUYER_ENTERED_NO_REFERENCE",
					referenceAvailability: "UNAVAILABLE",
					referencePriceRange: null,
					marketPriceBasis: null,
					buyerRequestedRange: {
						...guidedPreviewData().buyerRequestedRange,
						basisQuantity: "",
						basisUnit: "",
					},
					dealTemperature: { status: "UNKNOWN", reasonCode: "MAPPING_MISSING" },
					payloadHash: RFQ_PREVIEW_HASH,
					quoteTotalRange: null,
				})(route);
			},
		});

		await expect(
			page.getByText("Pricebook 找不到可用的品項與區域參考。"),
		).toBeVisible();
		await page.getByRole("button", { name: "改用自行設定價格" }).click();
		await page.getByLabel("希望下限").fill("30");
		await page.getByLabel("希望上限").fill("42");
		await page.getByRole("button", { name: "取得無參考價試算" }).click();

		await expect(page.getByText("本次未使用行情參考")).toBeVisible();
		await expect(page.getByText(/個批發市場/)).toHaveCount(0);
		await expect(page.getByText("詢價總額區間")).toHaveCount(0);
	});
});
