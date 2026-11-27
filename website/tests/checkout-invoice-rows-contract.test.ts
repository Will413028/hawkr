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
const ROWS = "features/invoices/checkout/CheckoutInvoiceRows.tsx";

test("付款成功頁掛上發票三列", () => {
	assert.match(
		read(SUCCESS),
		/<CheckoutInvoiceRows\b/,
		`${SUCCESS} 沒有渲染 CheckoutInvoiceRows`,
	);
});

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
	const source = stripComments(read(ROWS));
	for (const banned of ["localStorage", "sessionStorage", "document.cookie"]) {
		assert.ok(!source.includes(banned), `${ROWS} 出現 ${banned}`);
	}
});

test("9301 與查詢失敗都不渲染，這一頁不長出任何錯誤訊息", () => {
	const source = stripComments(read(ROWS));
	assert.match(source, /if\s*\(notFound\)\s*return null;/);
	assert.ok(
		!source.includes('role="alert"'),
		`${ROWS} 長出了錯誤訊息；B03 的失敗處置是不渲染`,
	);
});
