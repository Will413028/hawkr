import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import {
	legacyReplyToBlocks,
	narrativeIcon,
	type PublicNarrativeKind,
} from "../features/procurement-assistant/narrative";

const root = fileURLToPath(new URL("..", import.meta.url));

function read(rel: string): string {
	return readFileSync(path.join(root, rel), "utf8");
}

/** 註解要先剝掉再比對「不准出現 X」——這些檔案的註解刻意引用被禁的字串來解釋
 * 為什麼不用它。剝法與 scripts/check-no-emoji.ts 同一支。 */
function stripComments(source: string): string {
	return source
		.replace(/\/\*[\s\S]*?\*\//g, "")
		.split("\n")
		.filter((line) => !line.trimStart().startsWith("//"))
		.join("\n");
}

const chat = read("features/procurement-assistant/ProcurementClarifyChat.tsx");
const rail = read("features/procurement-assistant/AgentActivityRail.tsx");
const taskPanel = read("features/procurement-assistant/AgentTaskPanel.tsx");

test("§V.10.6-4：舊 reply string 只轉成單一 SUMMARY block，不自己切段猜語意", () => {
	const blocks = legacyReplyToBlocks(
		"好的，記下水果禮盒了。方便告訴我要送到哪個城市嗎？",
	);
	assert.equal(blocks.length, 1);
	assert.equal(blocks[0].kind, "SUMMARY");
	assert.deepEqual(legacyReplyToBlocks("   "), []);
});

test("narrative 與 buyer 原文的換行以 br 保留，不依賴外部 white-space CSS", () => {
	assert.match(chat, /function TextWithLineBreaks/);
	assert.match(chat, /text\.matchAll\(\/\\r\?\\n\/g\)/);
	assert.match(chat, /<br key=\{`line-break:/);
	assert.match(chat, /<TextWithLineBreaks text=\{block\.text\} \/>/);
	assert.match(chat, /<TextWithLineBreaks text=\{turn\.text\} \/>/);
});

test("§F：icon 由 kind 查表決定，模型不得傳入任意 icon name", () => {
	const kinds: PublicNarrativeKind[] = [
		"UNDERSTOOD",
		"CONTEXT",
		"RATIONALE",
		"WARNING",
		"NEXT_STEP",
		"COMPLETE",
		"SUMMARY",
	];
	for (const kind of kinds) {
		assert.ok(narrativeIcon(kind), `${kind} 必須有 allowlisted icon`);
	}
	assert.equal(narrativeIcon("COMPLETE"), "check");
	assert.equal(narrativeIcon("CONTEXT"), "clipboard");
	assert.equal(narrativeIcon("RATIONALE"), "info");
	assert.equal(narrativeIcon("NEXT_STEP"), "arrow-right");
	assert.equal(narrativeIcon("NOT_A_KIND" as PublicNarrativeKind), null);

	const source = read("features/procurement-assistant/narrative.ts");
	assert.doesNotMatch(source, /interface PublicNarrativeBlock \{[^}]*icon/s);
});

test("§V.10.6-4：assistant 回覆是無泡泡正文，且不重複 Agent avatar", () => {
	assert.match(chat, /className="hawkr-agent-narrative"/);
	assert.doesNotMatch(stripComments(chat), /HawkrAgentMark/);
	assert.match(chat, /legacyReplyToBlocks\(turn\.text\)/);
	assert.match(chat, /"thread-page__bubble is-mine"/);
});

test("§V.11.1：進行中那一列有非色彩線索，尚未開放的停止能力不出現在介面", () => {
	assert.match(rail, /step\.status === "RUNNING"/);
	assert.match(rail, /進行中/);
	assert.match(
		rail,
		/running \? \(\s*<HawkrAgentSprite[\s\S]*?\) : \(\s*<AppIcon/,
	);
	assert.doesNotMatch(rail, /className="hawkr-agent-rail__sprite"/);
	assert.doesNotMatch(taskPanel, /AgentStopTaskAction|停止處理|規劃中/);
	assert.doesNotMatch(
		stripComments(
			read("features/procurement-assistant/AssistantWorkspace.tsx"),
		),
		/AgentStopTaskAction/,
	);
	assert.doesNotMatch(rail, /停止處理/);
	assert.doesNotMatch(rail, /onClick/);
});

test("§V.11.1 Composer：RUNNING 時保持 disabled，不重複顯示等待提示", () => {
	assert.match(chat, /disabled=\{pending\}/);
	assert.doesNotMatch(
		stripComments(chat),
		/Hawkr Agent 正在處理這一輪，完成後就能繼續輸入。/,
	);
	assert.doesNotMatch(chat, /hawkr-agent-phase1__compose-hint/);
});

test("§F：PublicNarrativeKind 對齊後端 6 個 block kind，SEARCHED/ORGANIZED 移除", () => {
	const source = read("features/procurement-assistant/narrative.ts");
	assert.match(source, /"UNDERSTOOD"/);
	assert.match(source, /"CONTEXT"/);
	assert.match(source, /"RATIONALE"/);
	assert.match(source, /"WARNING"/);
	assert.match(source, /"NEXT_STEP"/);
	assert.match(source, /"COMPLETE"/);
	assert.doesNotMatch(source, /"SEARCHED"/);
	assert.doesNotMatch(source, /"ORGANIZED"/);
});

test("narrativeIcon: 6 個 §F kind 各自有 icon，legacy SUMMARY 仍保留", () => {
	const kinds: PublicNarrativeKind[] = [
		"UNDERSTOOD",
		"CONTEXT",
		"RATIONALE",
		"WARNING",
		"NEXT_STEP",
		"COMPLETE",
		"SUMMARY",
	];
	for (const kind of kinds) {
		assert.notEqual(narrativeIcon(kind), undefined);
	}
	assert.equal(narrativeIcon("NOT_A_KIND" as PublicNarrativeKind), null);
});

test("#2549 Phase 2：assistant 回覆優先用 turn.blocks，缺席才落回 legacyReplyToBlocks", () => {
	const chat = read(
		"features/procurement-assistant/ProcurementClarifyChat.tsx",
	);
	assert.match(chat, /turn\.blocks\s*\?\?\s*legacyReplyToBlocks\(turn\.text\)/);
});

test("#2549 Phase 2：block 渲染改用 index 當 React key，不再用 block.kind（避免同 kind 重複時 key 碰撞）", () => {
	const chat = read(
		"features/procurement-assistant/ProcurementClarifyChat.tsx",
	);
	assert.doesNotMatch(chat, /key=\{block\.kind\}/);
});
