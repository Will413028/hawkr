import { expect, type Page, test } from "@playwright/test";
import {
	type MockStore,
	mockMyDashboard,
	mockMyKyc,
	mockMyStore,
	setupGatewayMocks,
} from "./helpers/mock-gateway";

/**
 * 訂單管理「數字 vs 清單」的可信度契約（#377 第三項）。
 *
 * 該票報「共 27 筆／已完成 27，但 chips 與清單皆 0」。頁面上這兩組數字來自
 * **兩支不同的後端端點**：
 *   - 四張統計卡 + 頁首「共 N 筆訂單」 → `GetMyDashboard`（`order_status_counts`）
 *   - 清單本身 + 「待處理」chip        → `/app/orders?view=seller`（前端再分桶）
 * 而「全部」chip 取的是 `displayTabCounts[activeTab]`，也就是 dashboard 的數字。
 *
 * 於是「已完成 27、清單 0」有兩種完全不同的解釋，讀碼分不出來：
 *   (a) 使用者停在預設的「待出貨」tab，該賣家確實 0 筆待出貨 —— 正確渲染；
 *   (b) 兩個來源真的不一致 —— bug。
 *
 * 這支測試把 (a) 釘死：同一份「pending 0 / in_transit 0 / completed 27」資料
 * 下，預設 tab 必須是「全部 0＋空清單」，切到已完成必須看得到 27 筆。任何一邊
 * 壞掉都會轉紅，於是往後再收到同款回報時能直接分辨是哪一種。
 */

const SELLER_ID = "200";
const ACTIVE_ROLE_KEY = "hkui:auth:active-role";
const LOADER_KEY = "hawkr:initial-loader-seen";
/** 與 #377 截圖同一個數字，讓測試與票面直接對得起來。 */
const COMPLETED_COUNT = 27;
/** SellerOrdersPage 的 ORDER_PAGE_SIZE —— 首屏只鋪這麼多列，其餘靠「載入更多」。 */
const ORDER_PAGE_SIZE = 20;

const SELLER_STORE: MockStore = {
	id: SELLER_ID,
	storeName: "數字對帳測試商店",
};

/** 27 筆已完成訂單 —— `status` 用語意名，repo 的 normalizeStatus 原樣放行。 */
const COMPLETED_ORDERS = Array.from({ length: COMPLETED_COUNT }, (_, i) => {
	const orderNo = `HK-DONE-${String(i + 1).padStart(2, "0")}`;
	return {
		orderNo,
		storeId: SELLER_ID,
		sellerName: SELLER_STORE.storeName,
		buyerId: "300",
		buyerName: `對帳買家${i + 1}`,
		status: "completed",
		totalActualAmount: "120",
		totalOriginalAmount: "120",
		createdAt: 1767225600000 + i * 1000,
		items: [
			{
				orderItemId: `70${i}`,
				productId: "500",
				skuId: "sku-1",
				productName: "對帳測試青菜",
				unitPrice: "120",
				quantity: "1",
				reviewed: false,
			},
		],
		shipping: {
			receiverName: `對帳收件人${i + 1}`,
			receiverPhone: "0912345678",
			address: "臺北市中正區測試路1號",
			method: "3001",
			trackingNo: `TRACK-${orderNo}`,
		},
	};
});

const ORDER_STATUS_COUNTS = {
	pendingShipmentCount: 0,
	inProgressCount: 0,
	closedCount: COMPLETED_COUNT,
	aftersalesCount: 0,
};

async function prepareSellerOrdersPage(page: Page, baseURL: string) {
	// 最低優先度的安全網：頁面周邊的雜項查詢（通知、標籤…）不該讓測試紅。
	await page.route("**/api/v1/**", (route) =>
		route.fulfill({
			status: 200,
			contentType: "application/json",
			body: JSON.stringify({ code: 0, data: {} }),
		}),
	);
	await setupGatewayMocks(page, {
		identity: {
			accountId: SELLER_ID,
			roles: ["seller"],
		},
		session: "identity",
	});
	// 之後註冊的 handler 優先，所以這三支具體路由會壓過上面的 catch-all。
	await mockMyStore(page, SELLER_STORE);
	await mockMyKyc(page, { status: "APPROVED" });
	await mockMyDashboard(page, { orderStatusCounts: ORDER_STATUS_COUNTS });
	await page.route("**/api/v1/app/orders?**", (route) => {
		const url = new URL(route.request().url());
		// useAllSellerOrders 只有帶 view=seller 才是履約佇列；買家視角回空集合，
		// 這樣「清單空掉」若源自 view 掉了，測試會抓到而不是被 mock 補起來。
		const isSellerView = url.searchParams.get("view") === "seller";
		const records = isSellerView ? COMPLETED_ORDERS : [];
		route.fulfill({
			status: 200,
			contentType: "application/json",
			body: JSON.stringify({
				code: 0,
				data: {
					records,
					page: Number(url.searchParams.get("page") ?? "1"),
					page_size: Number(url.searchParams.get("page_size") ?? "100"),
					total: records.length,
				},
			}),
		});
	});

	await page
		.context()
		.addCookies([{ name: "hk_session", value: "opaque", url: baseURL }]);
	await page.addInitScript(
		([roleKey, loaderKey]) => {
			localStorage.setItem(roleKey, "seller");
			sessionStorage.setItem(loaderKey, "1");
		},
		[ACTIVE_ROLE_KEY, LOADER_KEY] as const,
	);
}

/** 統計卡（SummaryStatCard 是 button[role=tab]）：以卡片標題為錨。 */
function statCard(page: Page, label: string) {
	return page
		.locator(".seller-inventory__stats .seller-inventory__stat")
		.filter({ hasText: label })
		.first();
}

/** quick-filter chip：「全部」/「待處理」。 */
function quickFilterChip(page: Page, label: string) {
	return page
		.locator(".seller-orders__quick-filters button")
		.filter({ hasText: label })
		.first();
}

/** 桌面 table 與手機 card list 會同時存在 DOM，但 responsive CSS 一次只呈現一份。
 *  所有清單斷言都必須鎖在目前可見的 presentation，否則文字 locator 會同時命中
 *  隱藏的另一份 DOM，且 mobile project 也只會驗到隱藏的桌面 table。 */
function visibleOrdersSurface(page: Page) {
	return page.locator(
		".seller-orders__desktop-view:visible, .seller-orders__mobile-view:visible",
	);
}

function orderRows(page: Page) {
	return visibleOrdersSurface(page).locator(
		".seller-orders__table--orders tbody tr, .seller-orders__mobile-list > li",
	);
}

function emptyOrdersState(page: Page) {
	return visibleOrdersSurface(page).getByText("目前沒有訂單", {
		exact: true,
	});
}

/** 只在 EVIDENCE_DIR 有設時截圖（本機取證用；CI 不設，等於 no-op）。 */
async function captureEvidence(page: Page, name: string) {
	const dir = process.env.EVIDENCE_DIR;
	if (!dir) return;
	// 首屏 loader 是覆蓋層：toBeVisible 不看遮擋，所以不等它卸載就截圖會拍到滿版
	// logo（實測踩過）。等 .initial-loader 脫離 DOM 才是真的看得到頁面。
	await expect(page.locator(".initial-loader")).toHaveCount(0);
	await page.locator(".seller-inventory__stats").scrollIntoViewIfNeeded();
	await page.screenshot({ path: `${dir}/${name}.png` });
}

test.describe("賣家訂單管理：統計數字與清單的對帳", { tag: ["@ui"] }, () => {
	test("已完成 27／待出貨 0 時，預設 tab 是空的但已結束 tab 有 27 筆", async ({
		page,
		baseURL,
	}) => {
		test.skip(!baseURL, "baseURL is required");
		await prepareSellerOrdersPage(page, baseURL as string);

		await page.goto("/seller-center/orders");

		// 頁首彙總 = dashboard 三桶相加，與截圖上的「共 27 筆訂單」同一個數字。
		await expect(page.getByText(`共 ${COMPLETED_COUNT} 筆訂單`)).toBeVisible();
		// 統計卡：已結束 27、待出貨 0（標籤取自 SELLER_ORDER_TAB_LABEL.completed，
		// 不是「已完成」——見 orderTabs.ts 的說明，該桶含取消／退款，不是成交數）。
		await expect(statCard(page, "已結束")).toContainText(
			String(COMPLETED_COUNT),
		);
		await expect(statCard(page, "待出貨")).toContainText("0");

		// #377 截圖的狀態：停在預設「待出貨」tab。全部 chip = 0、清單空 —— 這是
		// 正確渲染（該賣家真的 0 筆待出貨），不是數字與清單打架。
		await expect(quickFilterChip(page, "全部")).toContainText("0");
		await expect(emptyOrdersState(page)).toBeVisible();
		await captureEvidence(page, "377-item3-default-tab");

		// 切到已結束：同一份資料必須把 27 筆放出來。若這裡是 0，才是真的 bug。
		await statCard(page, "已結束").click();
		await expect(quickFilterChip(page, "全部")).toContainText(
			String(COMPLETED_COUNT),
		);
		await expect(orderRows(page)).toHaveCount(ORDER_PAGE_SIZE);
		await expect(
			visibleOrdersSurface(page).getByText("HK-DONE-01", { exact: true }),
		).toBeVisible();
		await captureEvidence(page, "377-item3-completed-tab");

		// 「載入更多」把剩下的 7 筆補齊，總數回到 27。
		await visibleOrdersSurface(page)
			.getByRole("button", { name: "載入更多" })
			.click();
		await expect(orderRows(page)).toHaveCount(COMPLETED_COUNT);
		await expect(
			visibleOrdersSurface(page).getByText(`HK-DONE-${COMPLETED_COUNT}`, {
				exact: true,
			}),
		).toBeVisible();
	});

	test("quick-filter 不會跨 tab 殘留，已結束 tab 不會被上一個 tab 的待處理鎖成空的", async ({
		page,
		baseURL,
	}) => {
		test.skip(!baseURL, "baseURL is required");
		await prepareSellerOrdersPage(page, baseURL as string);

		// 直接用 URL 帶著 filter=pending_action 進站，模擬 #377 懷疑的第二種成因：
		// 帶著上一個 tab 的 quick-filter 切過去，讓新 tab 看起來是空的。
		await page.goto("/seller-center/orders?tab=pending&filter=pending_action");
		await expect(emptyOrdersState(page)).toBeVisible();
		await page.evaluate(() => {
			document.documentElement.dataset.sellerOrdersSawTarget = "false";
			document.documentElement.dataset.sellerOrdersSawTransientEmpty = "false";
			document.documentElement.dataset.sellerOrdersSawTabRebound = "false";

			const observer = new MutationObserver(() => {
				const selectedTab = document.querySelector<HTMLElement>(
					'.seller-orders__status-tabs [role="tab"][aria-selected="true"]',
				);
				const targetSelected = selectedTab?.textContent?.includes("已結束");
				const targetWasSeen =
					document.documentElement.dataset.sellerOrdersSawTarget === "true";

				if (targetSelected) {
					document.documentElement.dataset.sellerOrdersSawTarget = "true";
					if (document.querySelector(".seller-orders__mobile-empty")) {
						document.documentElement.dataset.sellerOrdersSawTransientEmpty =
							"true";
					}
				} else if (targetWasSeen) {
					document.documentElement.dataset.sellerOrdersSawTabRebound = "true";
				}
			});
			observer.observe(document.body, {
				attributes: true,
				childList: true,
				characterData: true,
				subtree: true,
			});
		});

		await statCard(page, "已結束").click();
		// tab + filter 由同一次 URL/view-state commit 切換；不只驗最終態，
		// MutationObserver 也會抓中間空態與舊 tab 回彈。
		await expect(quickFilterChip(page, "全部")).toContainText(
			String(COMPLETED_COUNT),
		);
		await expect(orderRows(page)).toHaveCount(ORDER_PAGE_SIZE);
		await expect
			.poll(() =>
				page.evaluate(
					() =>
						document.documentElement.dataset.sellerOrdersSawTarget ?? "false",
				),
			)
			.toBe("true");
		expect(
			await page.evaluate(
				() => document.documentElement.dataset.sellerOrdersSawTransientEmpty,
			),
		).toBe("false");
		expect(
			await page.evaluate(
				() => document.documentElement.dataset.sellerOrdersSawTabRebound,
			),
		).toBe("false");
	});
});
