import assert from "node:assert/strict";
import { existsSync, readdirSync, readFileSync } from "node:fs";
import { dirname, join, relative } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import {
	codeLabel,
	SHIPPING_METHOD_LABEL,
	ShippingMethod,
} from "../src/lib/codemeta";

/**
 * 物流 provider 的「二元 fallback」禁令。
 *
 * ## 這支在防什麼
 *
 * 前端的分流一旦寫成「不是 A 就當 B」，後端新增的每一個配送方式都會被靜默畫成 B：
 *
 * ```ts
 * const lgsType = isTaqbin ? LGS_TYPE.HOME : LGS_TYPE.C2C;  // 3003/3004/3005 全變 C2C
 * {isTaqbin ? "黑貓宅配" : "7-11 賣貨便"}                     // 標題顯示別人家的物流
 * ```
 *
 * 它對未知值回的是一個**合法值**：型別對、不拋例外、測試全綠，只有使用者看得到。
 * website 與兩個 Flutter repo 各有一批這種寫法（共 8 列）。
 *
 * ## ADM 現在是零，這支是要讓它保持零
 *
 * ADM 全部走 `codeLabel(生成碼表, value)`：空 → `"—"`、未知 → 原碼，沒有任何
 * provider 判定。這是本波四個 repo 裡唯一一開始就對的。**「現在是對的」不會自己
 * 維持下去** —— 第一個要加物流畫面的人（AD-01～AD-08 都是從零建）沒有任何東西
 * 攔得住他從 website 抄一段 `isTaqbin ? … : …` 過來。
 *
 * ## 這支擋不住什麼（誠實標註，§1.9.3 的邊界）
 *
 * - **觀測窗是 `src/` 這個目錄**。有人把 provider 判斷抽成 helper 放到 `src/` 外
 *   再 import 回來，它看不到。
 * - 它認的是**形狀**（provider 述詞、以 provider 名稱字串為分支的三元），不是語意。
 *   用變數間接繞開（`const label = isA ? a : b` 而 a/b 從別處來）它抓不到。
 * - 它不驗「新增的 provider 有沒有被正確處理」，只驗「有沒有用二元 fallback 處理」。
 *
 * 抓不到不代表可以寫。這支只是其中一種形態的機械保證。
 */

const projectRoot = dirname(dirname(fileURLToPath(import.meta.url)));
const SRC = join(projectRoot, "src");

/** 生成碼表本身當然含 provider 名稱與 300x 碼值，那是它的職責。 */
const EXCLUDED_DIRS = [join(SRC, "lib", "codemeta")];

/**
 * provider 身分述詞。這幾個識別字在 ADM 完全不該出現 —— 它們是 PayUni／黑貓
 * 專屬的軸，ADM 只吃 CodeMeta 的 `ShippingMethod`。
 */
const PROVIDER_PREDICATE = /\bisTaqbin\w*|\bLGS_TYPE\s*\.|\blgsType\s*===?/g;

/**
 * 「以 provider 名稱字串當分支」的三元 —— `cond ? "黑貓宅配" : "7-11 賣貨便"`。
 *
 * 錨在**結構**（`?` 字串 `:`）而不是單純的字面值，是因為 ADM 的註解與測試 fixture
 * 裡本來就有「3001＝黑貓宅急便」這種說明文字；只掃字面值會誤報，而會誤報的 gate
 * 最後一定會被加一長串 allowlist，然後沒人看得懂它在守什麼。
 */
const PROVIDER_NAME_TERNARY =
	/\?\s*(["'])[^"'\n]*(黑貓|7-?11|賣貨便|超商|宅配)[^"'\n]*\1\s*:/g;

function collectSourceFiles(dir: string): string[] {
	const out: string[] = [];
	for (const entry of readdirSync(dir, { withFileTypes: true })) {
		const full = join(dir, entry.name);
		if (EXCLUDED_DIRS.some((d) => full.startsWith(d))) continue;
		if (entry.isDirectory()) {
			out.push(...collectSourceFiles(full));
		} else if (/\.tsx?$/.test(entry.name)) {
			out.push(full);
		}
	}
	return out;
}

test("掃描範圍真的存在且真的被走過（對照組）", () => {
	// 目錄不存在要 FAIL 不是 SKIP：src/ 哪天被搬走，下面兩支會靜默恆綠。
	assert.ok(existsSync(SRC), `掃描根目錄不存在：${SRC}`);
	const files = collectSourceFiles(SRC);
	assert.ok(files.length > 100, `只掃到 ${files.length} 個檔，範圍不對`);
	// 「查得到一個確知存在的東西」才輪到談否定結論。
	const corpus = files.map((f) => readFileSync(f, "utf8")).join("\n");
	assert.ok(
		corpus.includes("SHIPPING_METHOD_LABEL"),
		"掃到的內容裡找不到確知存在的字串，掃描沒有真的讀到檔案",
	);
});

test("src/ 沒有任何物流 provider 的二元 fallback", () => {
	const offenders: string[] = [];
	for (const file of collectSourceFiles(SRC)) {
		const source = readFileSync(file, "utf8");
		for (const re of [PROVIDER_PREDICATE, PROVIDER_NAME_TERNARY]) {
			for (const m of source.matchAll(re)) {
				const line = source.slice(0, m.index).split("\n").length;
				offenders.push(
					`${relative(projectRoot, file)}:${line}  ${m[0].trim()}`,
				);
			}
		}
	}
	assert.deepEqual(
		offenders,
		[],
		[
			"發現 provider 二元 fallback：",
			...offenders,
			"",
			"分流一律 switch + default，default 走「未知配送方式」顯示：",
			"  switch (method) { case ShippingMethod.TAQBIN: …; default: /* 未知 */ }",
			"顯示文字一律 codeLabel(SHIPPING_METHOD_LABEL, method) —— 空回「—」、未知回原碼。",
		].join("\n"),
	);
});

test("偵測式本身有鑑別力（抓得到刻意造出來的違規，且不誤報說明文字）", () => {
	// 沒有這一條，上面兩條正則哪天被改壞成永不命中，前一支會靜默恆綠。
	const planted = [
		"const lgsType = isTaqbin ? LGS_TYPE.HOME : LGS_TYPE.C2C;",
		'<span>{isTaqbinMethod(m) ? "黑貓宅配" : "7-11 賣貨便"}</span>',
	].join("\n");
	assert.ok(
		[...planted.matchAll(PROVIDER_PREDICATE)].length >= 2,
		"述詞偵測式抓不到已知的違規樣本",
	);
	assert.equal(
		[...planted.matchAll(PROVIDER_NAME_TERNARY)].length,
		1,
		"三元偵測式抓不到已知的違規樣本",
	);

	// 反向：ADM 現有的說明文字與正確寫法不該被誤判。
	const legit = [
		"// shippingMethod 是 CodeMeta 的數字 Value（3001＝黑貓宅急便、3201＝常溫）",
		"<span>{codeLabel(SHIPPING_METHOD_LABEL, o.shippingMethod)}</span>",
	].join("\n");
	assert.deepEqual([...legit.matchAll(PROVIDER_PREDICATE)], []);
	assert.deepEqual([...legit.matchAll(PROVIDER_NAME_TERNARY)], []);
});

test("codeLabel 對未知／空的配送方式不得回傳任何已知物流商的名稱", () => {
	const knownLabels = Object.values(SHIPPING_METHOD_LABEL);

	// 空值 → "—"（不是「宅配」，也不是任何 provider 名稱）。
	for (const empty of ["", null, undefined]) {
		const label = codeLabel(SHIPPING_METHOD_LABEL, empty);
		assert.equal(label, "—");
		assert.ok(!knownLabels.includes(label), `空值被畫成了 ${label}`);
	}

	// 後端新增、ADM 碼表還沒同步到的值 → 原碼照顯，不猜。
	for (const unknown of ["3999", "SOME_NEW_CARRIER"]) {
		const label = codeLabel(SHIPPING_METHOD_LABEL, unknown);
		assert.equal(label, unknown);
		assert.ok(!knownLabels.includes(label), `未知碼被畫成了 ${label}`);
	}
});

test("本波兩個新通路各有自己的標籤，不會被畫成 7-11 店到店", () => {
	// 碼表沒同步的話，3004/3005 會落到「原碼照顯」——那是安全的，但也代表 ADM 的
	// 物流欄位會顯示一串數字。這支釘住「拷貝已同步」這件事。
	for (const value of [
		ShippingMethod.SEVEN_ELEVEN_BULK_WAREHOUSE,
		ShippingMethod.MERCHANT_DELIVERY,
	]) {
		const label = codeLabel(SHIPPING_METHOD_LABEL, value);
		assert.notEqual(label, value, `${value} 沒有標籤（碼表拷貝未同步？）`);
		assert.notEqual(
			label,
			SHIPPING_METHOD_LABEL[ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP],
			`${value} 被畫成了 7-11 店到店`,
		);
		assert.notEqual(
			label,
			SHIPPING_METHOD_LABEL[ShippingMethod.TAQBIN],
			`${value} 被畫成了黑貓`,
		);
	}
});
