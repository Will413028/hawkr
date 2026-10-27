import type { DelegatedCapability } from "./state/session";

/**
 * 代行工作區的側欄政策（56 號 §6.1.14／定案第 3 條）。
 *
 * 規則只有一條：**只維護一份「禁區排除清單」，其餘一律渲染。**
 * 有 capability 對應的可操作；沒有的渲染成「不在本次代行授權範圍」空狀態。
 * 這樣賣家中心新增模組時代行側欄自動跟上，而「渲染但不可操作」是**宣告出來的
 * 表**、不是散在 render 分支（L1079）。
 */

/**
 * 禁區：連選單項都不渲染（§8.1.1 層 1）。這是唯一需要手動維護的清單。
 *
 * ⚠️ 三條的理由不同，不要合併理解：
 * - `wallet`：財務＋認證，禁區機器名 `finance`／`certification`
 * - `me/shipping`：履約／出貨，禁區機器名 `fulfillment`／`shipment`
 * - `preview`：**不是禁區**，但它與商店設定是同一個模組（§6.1.14 #8），完整版
 *   把預覽收進設定頁 ⇒ 不該有獨立入口
 * - `invoices`：58 號電子發票。這一條**不是取捨**——`gateway/internal/router/
 *   seller_route_assert.go` 的 `delegatedPathForbiddenWords` 含 `"invoice"`，
 *   任何掛在代行 namespace 底下的發票路徑會讓 Gateway **開機 panic** ⇒ 代行永遠
 *   拿不到發票資料，側欄留著只會是一個點進去必定空白的入口（58A §B-2.0 T02）。
 * - `notifications`：通知收件匣綁定賣家本人與即時串流，代行沒有通知讀取
 *   capability；顯示入口會讓操作者誤以為能讀取賣家私人通知。
 * - `measures`：措施與申覆是賣家本人帳號範圍的頁（`/seller/measures`，不在工作區內），
 *   申覆只能由賣家本人提出；代行的區段對照也對這個網址不成立。
 *   ⚠️ 這份清單是**唯一需要手動維護**的：不決定就是默認對代行開放，而且不編譯錯、
 *   不測試紅。
 */
export const DELEGATED_HIDDEN_HREFS: ReadonlySet<string> = new Set([
	"/seller-center/wallet",
	"/seller-center/me/shipping",
	"/seller-center/preview",
	"/seller-center/invoices",
	"/seller-center/notifications",
	"/seller/measures",
]);

/**
 * 可操作：區塊 → capability。表以外的一律渲染成空狀態。
 *
 * `null` ＝ 渲染且可進入，但不由單一 capability 決定（總覽是各 read capability
 * 的彙總畫面，KPI 由三支 list 的 `total` 自繪）。
 */
export const DELEGATED_AREA_CAPABILITY: Record<
	string,
	DelegatedCapability | null
> = {
	"/seller-center/dashboard": null,
	"/seller-center/orders": "orders.read",
	"/seller-center/products": "products.read",
	"/seller-center/posts": "posts.read",
	"/seller-center/me": "storefront.read",
};

/**
 * 賣家中心路由 → 代行工作區路由的區段對照。
 *
 * 表以外的項目一律取 `/seller-center/` 之後的字串當區段（例：`/seller-center/
 * coupons` → `/adm/{storeId}/coupons`），由 `[...area]` 的空狀態頁承接 ——
 * 賣家中心之後新增的模組不必回來改這張表。
 */
const DELEGATED_AREA_SEGMENT: Record<string, string> = {
	"/seller-center/me": "storefront",
};

export function delegatedHref(sellerHref: string, storeId: string): string {
	const path = sellerHref.split("?")[0];
	const segment =
		DELEGATED_AREA_SEGMENT[path] ?? path.replace(/^\/seller-center\/?/, "");
	const base = `/adm/${encodeURIComponent(storeId)}`;
	return segment ? `${base}/${segment}` : base;
}

/** 該選單項在代行下能不能操作（沒有對應 capability ⇒ 空狀態頁）。 */
export function delegatedAreaOperable(
	sellerHref: string,
	capabilities: readonly DelegatedCapability[],
): boolean {
	const path = sellerHref.split("?")[0];
	if (!(path in DELEGATED_AREA_CAPABILITY)) return false;
	const required = DELEGATED_AREA_CAPABILITY[path];
	return required === null || capabilities.includes(required);
}

export function isDelegatedHiddenHref(sellerHref: string): boolean {
	return DELEGATED_HIDDEN_HREFS.has(sellerHref.split("?")[0]);
}
