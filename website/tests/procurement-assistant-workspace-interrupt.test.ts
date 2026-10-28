import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

/**
 * Fix 5 — AssistantWorkspace.tsx 對 session.pendingInterrupt 的 wiring。跟
 * `procurement-assistant-workspace-handoff.test.ts` 同一個慣例：這個檔案在本
 * repo沒有 render-based 測試（沒有 JSX harness），改用 source-text 斷言鎖住
 * 關鍵行為，取代逐一手動點擊驗證。
 */
const root = fileURLToPath(new URL("..", import.meta.url));
const source = readFileSync(
	path.join(root, "features/procurement-assistant/AssistantWorkspace.tsx"),
	"utf8",
);

test("crop 消歧卡的選項點擊：只在 pendingInterrupt 存在時提供，並呼叫 resume({optionId})", () => {
	const clarifyChatStart = source.indexOf("<ProcurementClarifyChat");
	const clarifyChatEnd = source.indexOf("{/* 還有問題", clarifyChatStart);
	assert.ok(clarifyChatStart > -1, "ProcurementClarifyChat 應該還在原位");
	const wiring = source.slice(clarifyChatStart, clarifyChatEnd);
	assert.match(wiring, /cropDisambiguation=\{session\.cropDisambiguation\}/);
	assert.match(wiring, /onChooseCropDisambiguation=\{/);
	// 分支決策留在 AssistantWorkspace（session.pendingInterrupt／session.resume
	// 都已經在這一層拿得到），不下推進 leaf component——維持單一 prop 介面。
	assert.match(wiring, /session\.pendingInterrupt/);
	assert.match(
		wiring,
		/session\.resume\(\{\s*optionId:\s*option\.optionId\s*\}\)/,
	);
	assert.doesNotMatch(wiring, /chooseCropDisambiguation/);
});

test("pendingInterrupt 存在時鎖住自由輸入 composer——沿用既有『為什麼現在不能打字』的 pending 慣例，不另開新機制", () => {
	const composerStart = source.indexOf("<ProcurementClarifyComposer");
	const composerBlockEnd = source.indexOf("variant={showEntry", composerStart);
	assert.ok(composerStart > -1, "composer 應該還在原位");
	const composerWiring = source.slice(composerStart, composerBlockEnd);
	assert.match(
		composerWiring,
		/pending=\{\s*isProcessing \|\|\s*Boolean\(session\.pendingInterrupt\)/,
	);
});

test("Crop 卡本身（leaf component）不需要知道 interrupt 分支——CropDisambiguationCard 沒有被改動成認識 pendingInterrupt", () => {
	const cropCard = readFileSync(
		path.join(
			root,
			"features/procurement-assistant/CropDisambiguationCard.tsx",
		),
		"utf8",
	);
	assert.doesNotMatch(cropCard, /pendingInterrupt/);
	assert.doesNotMatch(cropCard, /resume/);
});
