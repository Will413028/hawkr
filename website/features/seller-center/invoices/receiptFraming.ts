/**
 * 收據／發票的措辭切換（backend#3066 裁決 B）。
 *
 * 🔴 為什麼需要這一支：免用統一發票的賣家（`APPROVED_INVOICE_EXEMPT` 或
 * `PLATFORM_RECEIPT`）**永遠不會有發票**。在他的畫面上寫「發票異常詳情」
 * 不只是措辭不精確 —— 那是在講一件他這輩子不會遇到的事。
 *
 * 裁決 B 選的是「收據與發票並列、文案與操作各自為政」，不是把字串中性化
 * （那是被否決的 A 案）。所以這裡回的是**兩組不同的詞**，不是一組通用詞。
 *
 * ⚠️ 判準只有一份：`documentKind` 由後端 `InvoiceJobInfo.document_kind` 供給。
 * **不得從 `operationType` 推導** —— 那個 enum 沒有 RECEIPT（收據是 documentKind
 * 不是 operationType，proto `InvoiceJobInfo` 第 4 欄的註解逐字寫明）。
 */

/** `InvoiceDocumentKind` 的 RECEIPT 值。與 CodeMeta 同源。 */
const DOCUMENT_KIND_RECEIPT = "RECEIPT";

/**
 * 這份 job／document 是不是收據。
 *
 * `documentKind` 空字串（尚未載入、或後端漏給）時回 `false` ⇒ 退回發票措辭。
 * 這個預設是刻意的：絕大多數賣家開的是發票，而**猜錯的代價不對稱**
 * —— 對發票賣家寫「收據」是明顯錯誤，對收據賣家寫「發票」是本票要修的問題，
 * 但後者只在真的拿到 `RECEIPT` 時才會發生，不會因為載入中而誤判。
 */
export function isReceiptKind(
	documentKind: string | undefined | null,
): boolean {
	return documentKind === DOCUMENT_KIND_RECEIPT;
}

/** 「發票」或「收據」。用於組句：`${docNoun(kind)}異常詳情`。 */
export function docNoun(documentKind: string | undefined | null): string {
	return isReceiptKind(documentKind) ? "收據" : "發票";
}

/**
 * 賣家中心側欄／麵包屑用的區塊名。
 *
 * 收據賣家沒有稅籍審核以外的發票事務，但「稅籍」對兩者都成立
 * （免用統一發票本身就是一種稅籍狀態）⇒ 只換前半。
 */
export function invoiceSectionLabel(
	documentKind: string | undefined | null,
): string {
	return `${docNoun(documentKind)}與稅籍`;
}

/**
 * 這位賣家是不是「只會有收據」的賣家。
 *
 * 🔴 **與 S01 的 `showsReceiptAuth` 曾經同源，2026-09-05 起刻意分家，不要再把它們
 * 併回一支。** 那一支答的是「能不能簽收據代製授權」，DEC-20260905-015836 之後含四個
 * 審核進行中的狀態；這一支答的是「這位賣家會不會有發票」，是一句對事實的宣稱。
 * 審核中的賣家最後可能被核定為「應開立統一發票」⇒ 現在就把他的側欄改成「收據與
 * 稅籍」、把 S06 改成「收據紀錄」，是在告訴他一件可能為假的事。
 * **多開一個授權入口的代價是撞一次 gate，措辭判錯的代價是他找不到自己的發票。**
 * 兩者不對稱，所以這裡維持只認已核定免用（`APPROVED_INVOICE_EXEMPT`）與已經在走
 * 平台代製收據（`PLATFORM_RECEIPT`）。
 *
 * `overview` 為 null（載入中／停權／查詢失敗）時回 `false` ⇒ 退回發票措辭。
 */
export function isReceiptOnlySeller(
	overview: { taxReviewStatus: string; invoiceMode: string } | null,
): boolean {
	if (!overview) return false;
	return (
		overview.taxReviewStatus === "APPROVED_INVOICE_EXEMPT" ||
		overview.invoiceMode === "PLATFORM_RECEIPT"
	);
}

/** 賣家中心側欄的發票項；收據賣家看到的是「收據與稅籍」。其餘項原樣回傳。 */
export function framedNavItem<T extends { href: string; label: string }>(
	item: T,
	receiptOnly: boolean,
): T {
	if (!receiptOnly || item.href !== "/seller-center/invoices") return item;
	return { ...item, label: "收據與稅籍" };
}

/**
 * S06 紀錄頁的名稱。
 *
 * 一般賣家那一頁同時列得出發票與收據 ⇒「發票與收據紀錄」；
 * 而免用統一發票的賣家在那裡**只會有收據** ⇒ 叫它「發票與收據」是在承諾一個
 * 他永遠看不到的東西。
 */
export function documentsPageLabel(receiptOnly: boolean): string {
	return receiptOnly ? "收據紀錄" : "發票與收據紀錄";
}
