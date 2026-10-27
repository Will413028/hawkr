import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

/**
 * 色值棘輪（ratchet）：樣式檔裡不得出現 baseline 以外的新色值。
 *
 * 為什麼需要它：`DESIGN.md:327-328` 已經明文禁止「從頁面 raw hex 建立新 palette」，
 * 但那條規則**沒有任何機械偵測**——它只能靠人在 review 時肉眼比對，而新增一個
 * 「看起來很像既有色」的 hex 恰好是最不容易被看出來的一種改動。
 *
 * 這支測試就是那個偵測。它不要求把既有的 179 個色值一次清乾淨（那是另一件事），
 * 只保證**存量不再增加**：要加新色值，就必須先在 baseline 裡登記，而登記這個動作
 * 本身會出現在 diff 裡、需要說明理由。
 *
 * ⚠️ 加進 baseline 前先問一次：這個顏色是不是已經有 token 或既有值可以用？
 * 實例（#2444 當下）：新面板寫了 `background: #e4f0e6`，而 `--admin-success-soft`
 * 是 `#e7f3e9`、同一個檔案裡已經有三處在用它——那是一個造出來的近似色，不是新需求。
 */
const STYLE_ROOTS = ["src/styles", "src/app/globals.css"];
const BASELINE = "tests/fixtures/admin-color-baseline.json";
const HEX = /#[0-9a-fA-F]{3,8}\b/gu;

async function collectCssFiles(target: string): Promise<string[]> {
	const entries = await readdir(target, {
		withFileTypes: true,
		recursive: true,
	}).catch(() => null);
	if (entries === null) return target.endsWith(".css") ? [target] : [];
	return entries
		.filter((e) => e.isFile() && e.name.endsWith(".css"))
		.map((e) => path.join(e.parentPath ?? target, e.name));
}

test("樣式檔不得引入 baseline 以外的新色值", async () => {
	const files = (
		await Promise.all(STYLE_ROOTS.map((r) => collectCssFiles(r)))
	).flat();
	assert.ok(
		files.length > 0,
		"一個樣式檔都沒掃到——目錄結構改了就要同步改這支測試，不是刪掉它",
	);

	const baseline = new Set<string>(
		JSON.parse(await readFile(BASELINE, "utf8")) as string[],
	);
	assert.ok(baseline.size > 0, "baseline 是空的，等於這支測試什麼都沒守");

	const offenders = new Map<string, string[]>();
	for (const file of files) {
		for (const raw of (await readFile(file, "utf8")).match(HEX) ?? []) {
			const hex = raw.toLowerCase();
			if (baseline.has(hex)) continue;
			offenders.set(hex, [...(offenders.get(hex) ?? []), file]);
		}
	}

	assert.deepEqual(
		[...offenders.entries()].map(
			([hex, where]) => `${hex} ← ${where.join(", ")}`,
		),
		[],
		"出現了 baseline 以外的色值。先確認沒有現成 token 或既有值可用；" +
			`真的需要新色，把它加進 ${BASELINE} 並在 PR 說明理由。`,
	);
});
