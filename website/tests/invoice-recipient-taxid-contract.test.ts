import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

/**
 * #3012｜「買受人統編 0000000000」不得出現在任何畫面。
 *
 * ── 這個 bug 為什麼能活著 ────────────────────────────────────────────────
 * `"0000000000"` 不是統編，是 `chk_invdoc_b2c` 強制的**哨兵值**（二聯式在定義上
 * 沒有買受人統編）。而它是**非空字串 ⇒ truthy**，所以
 *
 *     {detail.recipientTaxId ? <dt>買受人統編</dt> … : null}
 *
 * 會把十個零當成資料印給消費者看。型別是 `string`、值非空、沒有錯誤、沒有測試
 * 會叫——它在 B2B 上看起來還完全正常，因為 B2B 那一欄真的是統編。
 *
 * ── 本檔守兩件事，缺一都會讓同一個形狀復活 ──────────────────────────────
 *   ① 兩個渲染點（買家 B05 ／ 賣家 S11）都不得用純真值判斷
 *   ② 判準只有一份。兩邊印的是同一份資料的同一個欄位，判準寫兩份就會分岔——
 *      而分岔的症狀是「其中一個畫面修好了」，看起來像已經處理完。
 *
 * 🔴 **不斷言「畫面上沒有 0000000000」**：那個字串本來就不在源碼裡，那樣的斷言
 * 恆真、零鑑別力。要錨的是**條件判斷的形狀**。
 */

const root = fileURLToPath(new URL("..", import.meta.url));
const read = (relativePath: string) =>
	readFileSync(path.join(root, relativePath), "utf8");
const stripComments = (source: string) =>
	source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");

/** 兩個渲染點：買家 B05 與賣家 S11。 */
const SITES = [
	"features/invoices/screens/InvoiceDocumentDetailClient.tsx",
	"features/seller-center/invoices/SellerInvoiceDetailPage.tsx",
] as const;

test("兩個渲染點都不得用純真值判斷 recipientTaxId", () => {
	for (const site of SITES) {
		const source = stripComments(read(site));
		// `{detail.recipientTaxId ? …` / `{recipientTaxId ? …` 這個形狀。
		assert.ok(
			!/\brecipientTaxId\s*\?/.test(source),
			`${site} 用純真值判斷 recipientTaxId ⇒ B2C 的哨兵值 0000000000 會被印出來（#3012）`,
		);
		assert.ok(
			/hasRecipientTaxId\(/.test(source),
			`${site} 沒有走共用判準 hasRecipientTaxId`,
		);
	}
});

test("判準只有一份，且兩邊都餵 documentKind", () => {
	for (const site of SITES) {
		const source = stripComments(read(site));
		// 🔴 錨在「有沒有把 documentKind 傳進去」：只傳值不傳 kind 的話，判準就退化
		// 成字面值比對的變形，而那在哨兵值改變的那天會靜默失效。
		assert.ok(
			/hasRecipientTaxId\(\s*[\s\S]{0,60}?documentKind/.test(source),
			`${site} 呼叫 hasRecipientTaxId 但沒有傳 documentKind`,
		);
		// 判準不得在畫面層被就地重寫（例如自己再比一次 "0000000000"）。
		assert.ok(
			!source.includes("0000000000"),
			`${site} 出現硬編碼的哨兵值 ⇒ 判準被複製了一份，哨兵值改變時會分岔`,
		);
	}

	// 判準的唯一實作處。
	const helper = stripComments(read("lib/invoices/display.ts"));
	assert.ok(
		/export function hasRecipientTaxId/.test(helper),
		"hasRecipientTaxId 不在 lib/invoices/display.ts",
	);
	// 語意閘門必須錨在 documentKind === B2B，不是「值不等於哨兵值」。
	assert.ok(
		/InvoiceDocumentKind\.B2B/.test(helper),
		"判準沒有錨在 documentKind === B2B（改成字面值比對會在哨兵值改變時靜默失效）",
	);
});
