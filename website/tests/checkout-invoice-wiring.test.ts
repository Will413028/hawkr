import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

/**
 * B01／B02 在結帳頁的**接線**契約。
 *
 * 🔴 這支測試守的是一種特別安靜的失效：**元件在 bundle 裡，但沒有任何東西能打開它。**
 *
 * 2026-09-01 dev 實測抓到：`InvoiceMethodSheet` 已渲染在頁尾、`invoiceSheetOpen`
 * state 也在、`confirmInvoiceOf` 也接進了 payload——唯獨
 * `<CheckoutInvoiceSection>` 少了 `onChangeMethod` ⇒ 沒有任何東西呼叫
 * `setInvoiceSheetOpen(true)` ⇒ 買家在結帳頁**看不到「變更發票方式」那顆鍵**，
 * B02 整個功能不可達。
 *
 * TypeScript 不叫（`onChangeMethod` 是選填 prop）、biome 不叫（import 有被用到，
 * 因為 Sheet 確實渲染了）、既有測試不叫。只有真的去看一眼結帳頁才會發現。
 */
const SOURCE = readFileSync(
	new URL("../features/checkout/screens/CheckoutClient.tsx", import.meta.url),
	"utf8",
);

test("B02 的 Sheet 有人打得開（onChangeMethod 真的接到 CheckoutInvoiceSection）", () => {
	// ① Sheet 有渲染
	assert.match(
		SOURCE,
		/<InvoiceMethodSheet/,
		"結帳頁沒有渲染 InvoiceMethodSheet",
	);
	// ② 有人把它打開——**這一條才是重點**
	assert.match(
		SOURCE,
		/onChangeMethod=\{\(\) => setInvoiceSheetOpen\(true\)\}/,
		"CheckoutInvoiceSection 沒有接 onChangeMethod：Sheet 在 bundle 裡但打不開",
	);
	// ③ 選擇會回流到卡片上（不然買家選完看不出來有沒有生效）
	assert.match(
		SOURCE,
		/selection=\{invoiceSelection\}/,
		"CheckoutInvoiceSection 沒有接 selection：買家選完之後卡片還是顯示預設值",
	);
});

test("買家的選擇會進 confirm payload，且沒選時整包省略", () => {
	// 🔴 「整包省略」是已上架買家 App 的送法，那條路必須恆成立。
	assert.match(
		SOURCE,
		/\.\.\.\(confirmInvoiceOf\(invoiceSelection\)/,
		"confirm payload 沒有帶上買家的發票選擇",
	);
	// confirmInvoiceOf 對 undefined 與 kind:"none" 都回 undefined ⇒ 展開後不產生 invoice 欄
	assert.match(
		SOURCE,
		/case "none":\s*\n\s*return undefined;/,
		"「不使用載具」沒有回 undefined：送一個空的 invoice 區塊會撞 chk_ois_pref_pair",
	);
});
