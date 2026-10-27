import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import {
	deriveMotionState,
	spriteMotion,
} from "../features/procurement-assistant/agent-motion";

/** 搜尋期間的公開進度留在主對話區，與正式回覆分層。
 * 右側只放成果；桌機與手機都必須直接看得到目前狀態，不用另外打開面板。
 * 公開列、動畫與 screen reader 播報仍由相同的當前 run 資料決定。 */
const root = fileURLToPath(new URL("..", import.meta.url));

function read(rel: string): string {
	return readFileSync(path.join(root, rel), "utf8");
}

const workspace = read("features/procurement-assistant/AssistantWorkspace.tsx");
const processing = read("features/procurement-assistant/AgentTaskPanel.tsx");
const rail = read("features/procurement-assistant/AgentActivityRail.tsx");
const clarify = read(
	"features/procurement-assistant/ProcurementClarifyChat.tsx",
);

test("searching 期間完整處理清單在主對話區，中欄不留空白", () => {
	assert.match(
		workspace,
		/const isProcessing = session\.pendingTurn \|\| session\.stage === "searching";/,
	);
	assert.match(
		workspace,
		/<ProcurementClarifyChat[\s\S]*?pending=\{isProcessing\}[\s\S]*?processing=\{\s*<AgentTaskPanel/,
	);
	assert.match(
		clarify,
		/<li className="hawkr-agent-processing-row">\{processing\}<\/li>/,
	);
	assert.doesNotMatch(workspace, /<HawkrAgentTypingRow/);
});

test("公開搜尋列與 screen reader 播報讀同一份當前 run 資料", () => {
	assert.match(workspace, /searchActivity=\{searchActivity\}/);
	assert.match(processing, /activity=\{searchActivity\}/);
	assert.match(workspace, /lastRunningActivity\(searchActivity\)/);
	assert.match(workspace, /activityLabel\(searchingRunningRow\)/);
	assert.match(rail, /activityLabel\(event\)/);
	assert.match(workspace, /\(searchingLabel \?\? ""\)/);
});

test("沒有 RUNNING 列時播報總述，搜尋事件缺席時顯示既有輪詢進度", () => {
	assert.match(workspace, /: "正在搜尋候選供應商"/);
	assert.match(processing, /\{searchActivity\.length > 0 \? \(/);
	assert.match(
		processing,
		/<CandidateSearchProgress[\s\S]*?candidateSet=\{candidateSet\}/,
	);
});

test("搜尋表情沿用 deriveMotionState，不在主區寫死動畫狀態", () => {
	assert.match(workspace, /const motion = deriveMotionState\(/);
	assert.match(workspace, /<AgentTaskPanel[\s\S]*?motion=\{motion\}/);
	assert.match(processing, /<AgentActivityRail[\s\S]*?motion=\{motion\}/);
	assert.match(rail, /<HawkrAgentSprite[\s\S]*?state=\{motion\}/);
});

test("搜尋錯誤壓過循環動畫；連線異常仍提供重試入口", () => {
	assert.match(workspace, /hasError: Boolean\(session\.error\)/);
	const state = deriveMotionState({
		hasError: true,
		pending: false,
		latestStepCode: "SEARCH_CANDIDATES",
		awaitingInput: false,
		stage: "searching",
	});
	assert.equal(state, "RECOVERABLE_ERROR");
	assert.equal(spriteMotion(state).loop, false);
	assert.match(
		processing,
		/<SearchPollErrorAlert onRetryPolling=\{onRetryPolling\}/,
	);
	assert.match(
		processing,
		/<CandidateSearchProgress[\s\S]*?pollError=\{pollError\}[\s\S]*?onRetryPolling=\{onRetryPolling\}/,
	);
});

test("釐清與搜尋共用主區處理 slot，不重複繪製單行等待提示", () => {
	assert.match(
		clarify,
		/\{pending && processing \? \(\s*<li className="hawkr-agent-processing-row">\{processing\}<\/li>\s*\) : pending \? \(/,
	);
	assert.doesNotMatch(workspace, /className="hawkr-agent-typing-row"/);
	assert.doesNotMatch(processing, /HawkrAgentTypingRow/);
	assert.match(
		processing,
		/if \(stage !== "searching" && !pending\) return null;/,
	);
});

test("逐列進度不轟炸朗讀，仍由父層唯一 atomic polite status 播報", () => {
	assert.equal((workspace.match(/aria-live="polite"/g) ?? []).length, 1);
	assert.match(
		workspace,
		/className="sr-only"\s*role="status"\s*aria-live="polite"\s*aria-atomic="true"/,
	);
	assert.match(rail, /aria-hidden="true"/);
	assert.doesNotMatch(processing, /aria-live=|role="status"/);
});
