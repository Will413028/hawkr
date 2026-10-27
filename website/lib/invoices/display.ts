/**
 * 發票的純格式化與欄位可見性判準（§B-2.3 (5)）。呈現決策（tone／label／CTA）不放
 * 這裡，那些在各自的 feature 內。
 *
 * ⚠️ 本檔原本只服務賣家面，`hasRecipientTaxId` 之後**買賣兩側共用**：B05
 * （`features/invoices/screens/InvoiceDocumentDetailClient.tsx`）與 S11
 * （`features/seller-center/invoices/SellerInvoiceDetailPage.tsx`）印的是同一份
 * 資料的同一個欄位，判準寫兩份就會分岔——#3012 修的正是「兩處各自用純真值判斷」。
 */

import {
	InvoiceDocumentKind,
	SHIPPING_STATUS_LABEL,
} from "@/lib/codemeta/generated";
import { taipeiParts } from "@/lib/date/taipei";

/**
 * 金額格式化。API 一律回**字串**、單位「元」。
 *
 * 🔴 只做千分位，**不得 `Number()` 之後再算數**：發票金額是法定憑證上的數字，
 * 前端做任何加減乘除都是在製造第二份真相。這裡的 `Number` 僅用於分組，輸出
 * 若失敗就原樣回傳。
 *
 * repo 沒有共用金額格式器（`WalletClient.tsx` 與 `ProductPurchaseBox.tsx` 各一份）；
 * 本波只加這一份、**不合併既有兩份**（那是另一個 root cause 的工作）。
 */
export function formatInvoiceAmount(raw: string): string {
	if (raw === "") return "";
	const parsed = Number(raw);
	if (!Number.isFinite(parsed)) return raw;
	return `NT$ ${parsed.toLocaleString("en-US")}`;
}

/** 檔案大小（bytes 字串）→ 人類可讀。稅籍證明的檔案列用。 */
export function formatFileSize(bytes: string): string {
	const parsed = Number(bytes);
	if (!Number.isFinite(parsed) || parsed <= 0) return "";
	if (parsed < 1024) return `${parsed} B`;
	if (parsed < 1024 * 1024) return `${Math.round(parsed / 1024)} KB`;
	return `${(parsed / (1024 * 1024)).toFixed(1)} MB`;
}

/**
 * 自然日倒數：從「現在」到 `deadlineAt` 還剩幾天，**以台北日界計**。
 *
 * 🔴 不得用 `Math.ceil((deadline - now) / 86400000)`（§B-2.0 T12）：那是以「現在
 * 這一刻」為原點的 24 小時倍數，賣家 23:00 看到 41、01:00 看到 42。正確的問題是
 * 「台北的今天到台北的期限那天，中間隔幾個日界」⇒ 兩端都取台北日期部分再相減。
 *
 * 回 `null` 代表 `deadlineAt` 讀不出來（空字串／格式壞）⇒ 呼叫端整個倒數不渲染，
 * **不要顯示 0 天**（那會被讀成「今天就到期」）。
 *
 * 與工作天倒數（`./slaCountdown.ts`）是兩件事，不可合成一個：那一支的值全部由
 * 後端給、前端零計算；這一支是自然日，前端算得出來也算得對。
 */
export function taipeiDaysUntil(
	deadlineAt: string,
	now: number = Date.now(),
): number | null {
	const deadline = taipeiParts(deadlineAt);
	const today = taipeiParts(now);
	if (!deadline || !today) return null;
	// Date.UTC 只用來把「台北的某一天」轉成一個可相減的整數日序，時區在此已消掉。
	const deadlineDay = Date.UTC(deadline.year, deadline.month - 1, deadline.day);
	const todayDay = Date.UTC(today.year, today.month - 1, today.day);
	return Math.round((deadlineDay - todayDay) / 86_400_000);
}

/** `chk_invdoc_b2b` 的鏡像：`recipient_tax_id ~ '^[0-9]{8}$'`。 */
const BUSINESS_TAX_ID = /^[0-9]{8}$/;

/**
 * 這份文件有沒有「買受人統編」可以顯示。
 *
 * 🔴 **`"0000000000"` 不是統編，是 DB CHECK 強制的哨兵值。** `chk_invdoc_b2c`
 * 逐字要求
 * `document_kind <> 'B2C' OR (tax_amount = 0 AND recipient_tax_id = '0000000000')`
 * ——二聯式在定義上就沒有買受人統編，那一欄恆為十個零。而十個零是**非空字串
 * ⇒ truthy**，用 `detail.recipientTaxId ? … : null` 判斷會把哨兵值當成資料印給
 * 消費者看（#3012）。
 *
 * 🔴 **判準錨在語意，不在字面值**：問的是「這張是不是三聯式」，不是「值等不等於
 * `0000000000`」。後者在哨兵值改變的那天會靜默失效——十個零照樣印出去，沒有任何
 * 東西會轉紅。
 *
 * ⚠️ **兩個條件都要，而且理由不同**：
 *   `documentKind === B2B` 是**語意閘門**（只有三聯式才有買受人統編）；
 *   8 碼數字是 `chk_invdoc_b2b` 的鏡像，擋的是**資料漂移**（document_kind 是
 *   B2B 卻帶著哨兵值）。
 * 只留前者，十個零會從三聯式那條路溜出去；只留後者就是字面值比對的變形
 * （哨兵值是 10 碼，今天剛好被 8 碼規則擋掉，明天換一個哨兵值就不一定）。
 *
 * ⚠️ `RECEIPT`（平台代製收據）走 `false`：它的買受人是自然人，`chk_invdoc_receipt`
 * 也沒有對 `recipient_tax_id` 設任何約束。
 */
export function hasRecipientTaxId(
	documentKind: string,
	recipientTaxId: string,
): boolean {
	return (
		documentKind === InvoiceDocumentKind.B2B &&
		BUSINESS_TAX_ID.test(recipientTaxId)
	);
}

/**
 * S06 出貨狀態欄的三態（backend#3019）。
 *
 * 🔴 **`none`（這張訂單還沒有物流單）與 `degraded`（我們沒查到）是兩件事。**
 * shipping-service 不可用時後端整批回空字串並把 `shipmentStatusDegraded` 設 true；
 * 只看 `code` 的實作會把每一列都畫成「還沒出貨」——一句我們無從得知的話。
 * 這一欄在 backend#3019 之前後端恆為空字串，正是為了不說這句話而只畫 `—`；
 * 欄位有值之後，同一個風險會換一個形式重演。
 *
 * ⚠️ 判定順序不可對調：`degraded` 必須在 `code === ""` 之前判。
 *
 * 🔴 值域是 `ShippingStatus` 的 value（數字字串，例 `"210"`）⇒ 走 CodeMeta 查表。
 * 直接把碼值畫上去，賣家看到的是「210」。
 */
export type ShipmentStatusDisplay =
	| { kind: "degraded" }
	| { kind: "none" }
	| { kind: "status"; label: string };

export function shipmentStatusDisplay(
	code: string,
	degraded: boolean,
): ShipmentStatusDisplay {
	if (degraded) return { kind: "degraded" };
	if (code === "") return { kind: "none" };
	return { kind: "status", label: SHIPPING_STATUS_LABEL[code] ?? code };
}
