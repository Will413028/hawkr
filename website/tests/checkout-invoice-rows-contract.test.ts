import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));
const read = (relativePath: string) =>
	readFileSync(path.join(root, relativePath), "utf8");

/**
 * 掃「有沒有寫某個 API」時必須先把註解拿掉：本檔要禁的東西，正好也是那些檔案的
 * 註解裡會逐字解釋「為什麼不用它」的東西 ⇒ 不去註解就會抓到自己的說明文字，而那
 * 是假紅，修法會變成「把註解刪掉」。
 */
const stripComments = (source: string) =>
	source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");

const SUCCESS = "features/checkout/screens/CheckoutSuccessClient.tsx";
const SUBMITTED = "features/checkout/screens/CheckoutSubmittedClient.tsx";
const FAIL = "features/checkout/screens/CheckoutFailClient.tsx";
const ROWS = "features/invoices/checkout/CheckoutInvoiceRows.tsx";

/* ── B03 掛在哪兩頁、不掛在哪一頁 ───────────────────────────────────────── */

test("付款成功與 ATM 待轉帳兩頁都掛上發票三列", () => {
	for (const screen of [SUCCESS, SUBMITTED]) {
		const source = read(screen);
		assert.match(
			source,
			/<CheckoutInvoiceRows\b/,
			`${screen} 沒有渲染 CheckoutInvoiceRows`,
		);
	}
});

test("付款成功頁走 paid、ATM 待轉帳頁走 awaiting-payment", () => {
	// 兩頁的狀態句來源不同：成功頁依 invoiceMode 說「付款完成，…」，而 ATM 頁的錢
	// 還沒到，只能逐字用後端的 stateNotice。variant 傳錯不會編譯錯、不會測試紅，
	// 只會讓待轉帳的買家讀到「付款完成」。
	assert.match(read(SUCCESS), /variant="paid"/);
	assert.match(read(SUBMITTED), /variant="awaiting-payment"/);
});

test("付款失敗頁不掛任何發票區塊——付款失敗沒有發票可談", () => {
	const source = stripComments(read(FAIL));
	assert.doesNotMatch(source, /CheckoutInvoiceRows/);
	assert.doesNotMatch(source, /invoices/);
});

/* ── 這一頁不得引入刷新機制，也不得把發票資料落地 ─────────────────────── */

test("發票三列零輪詢、零 SSE 訂閱", () => {
	const source = stripComments(read(ROWS));
	for (const banned of [
		"setInterval",
		"refetchInterval",
		"useNotificationStream",
	]) {
		assert.ok(
			!source.includes(banned),
			`${ROWS} 出現 ${banned}：B03 是一次性落地頁，發票端點與使用者其他請求共用限流桶`,
		);
	}
});

test("發票三列不把 ?order= 對應的資料寫進瀏覽器儲存", () => {
	// `?order=` 是網址上的可控參數，落地就等於把買受人統編與載具識別碼留在裝置上。
	const source = stripComments(read(ROWS));
	for (const banned of ["localStorage", "sessionStorage", "document.cookie"]) {
		assert.ok(!source.includes(banned), `${ROWS} 出現 ${banned}`);
	}
});

test("9301 與查詢失敗都不渲染，這一頁不長出任何錯誤訊息", () => {
	// 端點 8 是 Tier A：越權與不存在回同一個 9301。前端只要在畫面上分得出來（一句
	// 錯誤訊息就夠），就等於把後端刻意拿掉的那個旗標補了回去。而 5xx 在這一頁同樣
	// 靜默——付款完成頁的工作是告知「錢付了、單成立了」。
	const source = stripComments(read(ROWS));
	assert.match(source, /if\s*\(notFound\)\s*return null;/);
	assert.ok(
		!source.includes('role="alert"'),
		`${ROWS} 長出了錯誤訊息；B03 的失敗處置是不渲染`,
	);
});
