import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));

function read(relativePath: string): string {
	return readFileSync(path.join(root, relativePath), "utf8");
}

function stripComments(source: string): string {
	return source
		.replace(/\/\*[\s\S]*?\*\//g, "")
		.split("\n")
		.filter((line) => !line.trimStart().startsWith("//"))
		.join("\n");
}

const draft = stripComments(
	read("features/procurement-assistant/RfqDraftFlow.tsx"),
);
const summary = stripComments(
	read("features/procurement-assistant/QuoteBatchSummary.tsx"),
);

test("price slice names confirmation separately from publication", () => {
	assert.match(
		draft,
		/<h2 id="assistant-rfq-draft-title">核對需求與價格<\/h2>/,
	);
	assert.match(draft, />\s*檢視不可變 RFQ 快照\s*<\/Button>/);
	assert.match(draft, />\s*重新確認 RFQ 快照\s*<\/h3>/);
	assert.match(draft, />\s*確認並準備發布\s*<\/Button>/);
	assert.match(draft, />\s*確認並送出邀請\s*<\/Button>/);
	assert.doesNotMatch(draft, /投入許願池/);
	assert.match(draft, /這會發布一筆詢價需求，不代表供應商已報價、供貨或成交。/);
	assert.doesNotMatch(draft, /發布詢價（尚未開放）/);
});

test("confirmation success only claims the server-owned price projection", () => {
	assert.match(draft, /需求版本與價格範圍已固定，等待發布。/);
	assert.match(draft, /確認後會固定這一版需求與希望區間/);
	assert.match(draft, /這次確認不包含候選名單、截止時間與通知設定/);
	assert.doesNotMatch(draft, /候選名單已固定|邀請已建立|通知已排入處理/);
});

test("draft copy never implies that a supplier received an unpublished RFQ", () => {
	const bannedPatterns = [
		/寄送|寄出|發送/,
		/(?:立即|立刻|馬上|即時).{0,12}(?:收到|送達|通知)/,
		/(?:已|成功).{0,6}(?:收到|送達|通知)/,
		/建立邀價/,
	];
	for (const banned of bannedPatterns) {
		assert.doesNotMatch(draft, banned, `draft copy matched ${banned}`);
	}
});

test("published summary keeps its own delivery truth disclaimer", () => {
	// 「幾筆邀請」的唯一伺服器來源換成 publication 的 recipientCount（legacy
	// `batch.invites` 已下架）；界線句照舊，它是「已建立」與「已送達」之間
	// 唯一的分界。
	assert.match(
		summary,
		/已建立 1 組詢價・\$\{publication\.data\.recipientCount\} 筆邀請/,
	);
	assert.match(summary, /通知已排入處理；不代表通知或聯絡已送達。/);
	// ⛔ 三個 count 是發布當下的意圖分布，不是投遞結果：`queuedCount` 標的是
	// 「已排入處理」，不得被寫成任何一種送出／送達的宣稱。「已送達」只准出現在
	// 上面那句否定式界線句裡，所以這裡只擋肯定式的兩個詞。
	assert.match(summary, /這三個數字是發布當下的收件人分布，不是投遞結果。/);
	assert.doesNotMatch(summary, /已送出|已寄送/);
});

test("draft does not invent a quality state before backend returns one", () => {
	assert.doesNotMatch(
		draft,
		/BUYER_QUALITY_UNKNOWN|data-quality-state|品相狀態|品相未知狀態|以買家品相未知保存/,
	);
});
