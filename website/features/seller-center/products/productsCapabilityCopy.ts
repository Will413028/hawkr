/**
 * 商品管理頁在代行工作階段下**該說什麼、給什麼動作**（§5.3 導航顯示規則：
 * 有 read、沒有 write ⇒ 頁面呈唯讀，不出現 write CTA 或 handler）。
 *
 * 抽成純函式而不是寫在 JSX 裡的三元式：這兩句話都是**對操作者宣稱他有什麼權限**，
 * 而唯讀工作階段的頁面此前恆稱「本工作階段可編輯與發布商品」、每一列恆給「編輯」。
 * 宣稱要能被逐組合驗證，寫在 JSX 裡就只驗得到「頁面有沒有炸」。
 */

/**
 * 商品管理頁的代行副標。
 *
 * 四種組合都要有自己的話 —— 兩個 capability 是分開核發的（`products.write` 管內容、
 * `products.publish` 管上下架與批次），只講其中一個會在另一半上說謊。
 */
export function delegatedProductsSubtitle(
	canWrite: boolean,
	canPublish: boolean,
): string {
	if (canWrite && canPublish) return "本工作階段可編輯與發布商品";
	if (canWrite) return "本工作階段可編輯商品，不可上架或下架";
	if (canPublish) return "本工作階段可上架或下架商品，不可編輯內容";
	return "本工作階段僅可檢視商品";
}

/**
 * 列表「操作」欄那顆入口的字。
 *
 * 「檢視」原本只看**商品狀態**（上架中的商品內容不可編輯，product-service 回 3019），
 * 於是缺 `products.write` 的工作階段在草稿與未啟用的每一列都寫著「編輯」——
 * 點進去是一份全部鎖住的表單。權限與商品狀態是兩個獨立的鎖，任一個鎖著就不是編輯。
 */
export function productRowActionLabel(
	canWrite: boolean,
	shelfActive: boolean,
): "檢視" | "編輯" {
	return canWrite && !shelfActive ? "編輯" : "檢視";
}

/**
 * 那顆入口的 `title`：說明它為什麼是「檢視」。
 *
 * 兩個原因同時成立時報權限 —— 「請先下架」是一句**可執行的指示**，而這個工作階段
 * 連下架都做不到，照著做只會撞到第二道牆。
 */
export function productRowActionHint(
	canWrite: boolean,
	shelfActive: boolean,
): string | undefined {
	if (!canWrite) return "本工作階段沒有核發商品編輯權限，只能檢視";
	if (shelfActive) return "上架中無法編輯內容，請先下架";
	return undefined;
}
