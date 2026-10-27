import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";

/**
 * 對話框的版面（admin/src/styles/AdminVisualRefresh.css）：外層限高並排成直欄，頁首與頁尾不動，只有 .modal-body 捲動。
 * 共用的 Dialog 元件（src/components/UI/Dialog.tsx）吃這組結構；不能回到「外層 .modal-content 捲動」——
 * 外層也捲就會有兩層捲軸、頁首與送出鍵跟著內容捲走。
 */

const css = readFileSync(
	join(import.meta.dirname, "../src/styles/AdminVisualRefresh.css"),
	"utf8",
);

/** 取出所有選擇器恰好是 selector 的規則的宣告（合併）。 */
function declarations(selector: string): string {
	const blocks: string[] = [];
	const re = /([^{}]+)\{([^{}]*)\}/gu;
	for (const m of css.matchAll(re)) {
		const selectors = m[1]
			.replace(/\/\*[\s\S]*?\*\//gu, "")
			.split(",")
			.map((s) => s.trim());
		if (selectors.includes(selector)) blocks.push(m[2]);
	}
	return blocks.join("\n");
}

test(".modal-content：限高、直欄，自己不捲動", () => {
	const d = declarations(".modal-content");
	assert.match(d, /display:\s*flex/u);
	assert.match(d, /flex-direction:\s*column/u);
	assert.match(d, /max-height:\s*calc\(100dvh - 40px\)/u);
	assert.doesNotMatch(d, /overflow/u, "外層不捲動：捲動的是 .modal-body");
});

test(".modal-body 在對話框裡捲動並可縮到比內容小；頁首與頁尾固定", () => {
	const body = declarations(".modal-content .modal-body");
	assert.match(body, /overflow-y:\s*auto/u);
	assert.match(body, /min-height:\s*0/u);
	assert.match(body, /flex:\s*1 1 auto/u);
	const bars = declarations(".modal-content .modal-footer");
	assert.match(bars, /flex:\s*none/u);
	assert.equal(
		declarations(".modal-content .modal-header"),
		bars,
		"頁首與頁尾同一條規則",
	);
});

test("包著 body 與 footer 的 form 也是直欄（CreateProspectDialog）", () => {
	const form = declarations(".modal-content > form");
	assert.match(form, /display:\s*flex/u);
	assert.match(form, /flex-direction:\s*column/u);
	assert.match(form, /min-height:\s*0/u);
});

test("捲動由共用 Dialog 的結構負責：對話框不在 body 或外層另設高度與捲動", () => {
	for (const file of [
		"../src/app/admin/seller-prospects/MergeDialog.tsx",
		"../src/app/admin/seller-prospects/CreateProspectDialog.tsx",
	]) {
		const source = readFileSync(join(import.meta.dirname, file), "utf8");
		assert.doesNotMatch(source, /maxHeight|overflowY|overflow:/u, file);
	}
});
