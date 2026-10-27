import assert from "node:assert/strict";
import { test } from "node:test";
import { reviewerDisplay } from "../src/lib/product-certifications";

/**
 * 為什麼要有這支測試：`reviewedByAccountId` 為 null 有兩種相反的意思，
 * 而 `a || "（系統自動）"` 這種寫法把它們壓成同一句話——**待審案件會顯示成
 * 「（系統自動）」，讀起來像系統已經裁決過**。
 *
 * 這不是假想：本專案同時存在「自動核准」（政府 API 查得到 ⇒ reviewer 為 null
 * 但 reviewedAt 有值）與「待審」（兩者皆 null）兩種真實資料，dev 上各有實例。
 * 所以不能簡單地把 fallback 拿掉——那會弄壞自動核准案件的顯示。
 * 判斷依據必須是 reviewedAt。
 */
test("待審（兩者皆空）顯示 -，不得說成系統自動", () => {
	assert.equal(reviewerDisplay(null, null), "-");
	assert.equal(reviewerDisplay(undefined, undefined), "-");
	assert.equal(reviewerDisplay("", ""), "-");
	// 防退化：這一句是本測試存在的理由
	assert.notEqual(reviewerDisplay(null, null), "（系統自動）");
});

test("自動核准（有裁決時間、無裁決者）才是系統自動", () => {
	assert.equal(reviewerDisplay("2026-08-26T22:20:42Z", null), "（系統自動）");
	assert.equal(reviewerDisplay("2026-08-26T22:20:42Z", ""), "（系統自動）");
});

test("人工裁決顯示 account_id", () => {
	assert.equal(
		reviewerDisplay("2026-08-26T22:33:55Z", "2613330734"),
		"2613330734",
	);
});

/**
 * 撤銷／駁回同樣寫入 reviewedAt，所以判斷不能綁 status 白名單
 * （新增狀態時白名單會漏，而漏掉的那個又會退回「系統自動」的錯誤顯示）。
 */
test("撤銷／駁回也算已裁決", () => {
	assert.equal(
		reviewerDisplay("2026-08-26T22:40:00Z", "2613330734"),
		"2613330734",
	);
	assert.equal(reviewerDisplay("2026-08-26T22:40:00Z", null), "（系統自動）");
});
