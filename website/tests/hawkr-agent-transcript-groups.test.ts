import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import {
	groupLabel,
	shouldStartGroup,
	TRANSCRIPT_GROUP_GAP_MS,
} from "../features/procurement-assistant/transcript-groups";
import type { ProcurementChatTurn } from "../features/procurement-assistant/types";

const root = fileURLToPath(new URL("..", import.meta.url));

/** 註解要先剝掉再比對「不准出現 X」——這支檔案的註解刻意寫出 getFullYear 家族
 * 來解釋為什麼不用它們。剝法與 scripts/check-no-emoji.ts 同一支。 */
function stripComments(source: string): string {
	return source
		.replace(/\/\*[\s\S]*?\*\//g, "")
		.split("\n")
		.filter((line) => !line.trimStart().startsWith("//"))
		.join("\n");
}

function turn(createdAt: number): ProcurementChatTurn {
	return { id: `${createdAt}`, role: "buyer", text: "x", createdAt };
}

/** 2026-08-17 07:30 台北 = 2026-08-16T23:30Z。 */
const T0730 = Date.UTC(2026, 7, 16, 23, 30);

test("同一批連續對話只在最前面插一條分隔線", () => {
	const first = turn(T0730);
	const soon = turn(T0730 + 90_000);
	assert.equal(shouldStartGroup(first, undefined), true);
	assert.equal(shouldStartGroup(soon, first), false);
});

test("間隔達到門檻就另起一組", () => {
	const first = turn(T0730);
	const later = turn(T0730 + TRANSCRIPT_GROUP_GAP_MS);
	assert.equal(shouldStartGroup(later, first), true);
	// 差一毫秒還不算。
	assert.equal(
		shouldStartGroup(turn(T0730 + TRANSCRIPT_GROUP_GAP_MS - 1), first),
		false,
	);
});

test("跨日一律另起一組，即使只差幾分鐘", () => {
	// 台北 2026-08-17 23:58 → 2026-08-18 00:03，只差 5 分鐘但換日了。
	const before = turn(Date.UTC(2026, 7, 17, 15, 58));
	const after = turn(Date.UTC(2026, 7, 17, 16, 3));
	assert.ok(after.createdAt - before.createdAt < TRANSCRIPT_GROUP_GAP_MS);
	assert.equal(shouldStartGroup(after, before), true);
});

test("createdAt 為 0（還原的舊對話沒有時間）不插分隔線，也不當分組基準", () => {
	const unknown = turn(0);
	assert.equal(shouldStartGroup(unknown, undefined), false);
	assert.equal(shouldStartGroup(unknown, turn(T0730)), false);
	// 前一則沒有時間、這一則有 → 這一則自己開一組。
	assert.equal(shouldStartGroup(turn(T0730), unknown), true);
});

test("今天只印時間，其他日子連日期一起印", () => {
	assert.equal(groupLabel(T0730, T0730), "07:30");
	const nextDay = Date.UTC(2026, 7, 17, 23, 30);
	assert.match(groupLabel(T0730, nextDay), /8\/17/);
});

test("氣泡裡不再各自掛時間戳——時間只存在於分組分隔線", () => {
	const chat = readFileSync(
		path.join(
			root,
			"features/procurement-assistant/ProcurementClarifyChat.tsx",
		),
		"utf8",
	);
	assert.doesNotMatch(chat, /<small>\{clock\}<\/small>/);
	assert.doesNotMatch(chat, /formatTaipeiTime/);
	assert.match(chat, /shouldStartGroup\(turn, turns\[index - 1\]\)/);
	assert.match(chat, /className="thread-page__separator"/);

	// workspace 不再印寫死的「今天」——那是一個不看資料的假標籤。
	const workspace = readFileSync(
		path.join(root, "features/procurement-assistant/AssistantWorkspace.tsx"),
		"utf8",
	);
	assert.doesNotMatch(workspace, /<span>今天<\/span>/);
});

test("separator 的 render 不直接呼叫 Date.now，SSR/client 首屏使用同一個 fallback", () => {
	const chat = readFileSync(
		path.join(
			root,
			"features/procurement-assistant/ProcurementClarifyChat.tsx",
		),
		"utf8",
	);
	assert.doesNotMatch(chat, /groupLabel\(turn\.createdAt, Date\.now\(\)\)/);
	assert.match(chat, /groupNow \?\? turn\.createdAt/);
	assert.match(chat, /setGroupNow\(Date\.now\(\)\)/);
});

test("時區一律走 taipei helper，不用讀執行環境時區的 Date 方法", () => {
	const source = stripComments(
		readFileSync(
			path.join(root, "features/procurement-assistant/transcript-groups.ts"),
			"utf8",
		),
	);
	assert.doesNotMatch(source, /getFullYear|getMonth|getDate|getHours/);
	assert.match(source, /taipeiParts/);
});
