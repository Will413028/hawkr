import type { SellerInvoiceDocumentItem } from "@/lib/invoices/seller-types";

/**
 * 從一張訂單的發票清單裡挑出「賣家面主卡要顯示的那一張」。
 *
 * 作廢重開之後常態是「舊的 SUPERSEDED ＋ 新的 ISSUED」並存，而賣家面一張訂單
 * 只給一張卡 ⇒ 必須挑一張。
 *
 * 🔴 **不依賴後端的排序方向。** 原本呼叫端取 `items[items.length - 1]`，註解寫
 * 「後端依 issuedAt 遞增，取尾端」——那是個錯的假前提：端點 30 的
 * `ListSellerInvoiceDocuments` 是
 * `ORDER BY COALESCE(provider_issued_at, created_at) DESC, id DESC`（**遞減**）
 * ⇒ 取到的是**最舊**那一張。dev 第 3 輪 E2E 實測：作廢重開後賣家看到被取代的
 * 「已被重開取代」舊票，而買家同一張訂單看到的是新票 —— 兩端不一致。
 *
 * 排序方向是跨 repo 的隱性契約，後端改一次這裡就再壞一次，所以這裡自己挑。
 *
 * 挑選規則（依序）：
 *   ① 生效中的憑證優先於已被取代的（SUPERSEDED 最後）
 *      —— 作廢重開的當下兩者時間戳可能落在同一個事務內、非常接近，只比時間會挑錯
 *   ② 同一層級內比時間（`summary.issuedAt`，退回 `updatedAt`）
 *   ③ 都比不出來時取列表第一筆（後端 DESC ⇒ 那已經是最新的）
 */
export function pickPrimaryInvoiceDocument(
	items: readonly SellerInvoiceDocumentItem[],
): SellerInvoiceDocumentItem | null {
	if (items.length === 0) return null;
	const rank = (x: SellerInvoiceDocumentItem) =>
		x.summary.status === "SUPERSEDED" ? 0 : 1;
	const at = (x: SellerInvoiceDocumentItem) =>
		x.summary.issuedAt || x.updatedAt || "";
	return items.reduce((best, cur) => {
		if (rank(cur) !== rank(best)) return rank(cur) > rank(best) ? cur : best;
		return at(cur) > at(best) ? cur : best;
	}, items[0]);
}
