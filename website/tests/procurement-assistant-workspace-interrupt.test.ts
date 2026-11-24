import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

/**
 * AssistantWorkspace.tsx 對 session.pendingInterrupt 的 wiring。跟
 * `procurement-assistant-workspace-handoff.test.ts` 同一個慣例：這個檔案在本
 * repo沒有 render-based 測試（沒有 JSX harness），改用 source-text 斷言鎖住
 * 關鍵行為，取代逐一手動點擊驗證。
 */
const root = fileURLToPath(new URL("..", import.meta.url));
const source = readFileSync(
	path.join(root, "features/procurement-assistant/AssistantWorkspace.tsx"),
	"utf8",
);

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
